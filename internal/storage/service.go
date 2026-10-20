package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"path"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/z123-piad/houttuynia-blog/internal/app"
	"github.com/z123-piad/houttuynia-blog/internal/identity"
	"github.com/z123-piad/houttuynia-blog/internal/sqlc"
)

const maxImageBytes int64 = 8 << 20

type Credential struct {
	ID           int64             `json:"id,omitempty"`
	UploadURL    string            `json:"upload_url"`
	Method       string            `json:"method"`
	Headers      map[string]string `json:"headers,omitempty"`
	Form         map[string]string `json:"form,omitempty"`
	ObjectKey    string            `json:"object_key"`
	PublicURL    string            `json:"public_url"`
	ExpiresAt    time.Time         `json:"expires_at"`
	ConfirmToken string            `json:"confirm_token"`
}

type Service struct {
	q      *sqlc.Queries
	driver Driver
	secret string
	local  *Local
}

func NewService(pool *pgxpool.Pool, driver Driver, secret string) *Service {
	local, _ := driver.(*Local)
	return &Service{q: sqlc.New(pool), driver: driver, secret: secret, local: local}
}

func (s *Service) IssueCredential(ctx context.Context, actor identity.Principal, mime string, size int64, visibility string) (Credential, error) {
	if !actor.Has("asset.upload") {
		return Credential{}, app.Forbidden("无权上传文件")
	}
	mime = strings.ToLower(strings.TrimSpace(mime))
	if visibility == "" {
		visibility = "public"
	}
	if visibility != "public" && visibility != "private" {
		return Credential{}, app.Invalid("可见性不正确")
	}
	ext, ok := imageExt(mime)
	if !ok {
		return Credential{}, app.Invalid("只接受 jpeg、png、gif、webp 图片")
	}
	if size <= 0 || size > maxImageBytes {
		return Credential{}, app.Invalid("图片不能超过 8MB")
	}
	key := objectKey(ext)
	issued, err := s.driver.UploadCredential(ctx, UploadInput{
		ObjectKey: key, Mime: mime, Size: size, Visibility: visibility,
	})
	if err != nil {
		return Credential{}, err
	}
	exp := time.Now().Add(15 * time.Minute)
	confirm, err := signToken(s.secret, tokenPayload{
		Key: key, Mime: mime, Size: size, Visibility: visibility,
		OwnerID: actor.UserID, Driver: s.driver.Name(), Exp: exp.Unix(), Kind: "confirm",
	})
	if err != nil {
		return Credential{}, err
	}
	headers := issued.Headers
	if s.local != nil {
		upload, err := signToken(s.secret, tokenPayload{
			Key: key, Mime: mime, Size: size, Visibility: visibility,
			OwnerID: actor.UserID, Driver: "local", Exp: exp.Unix(), Kind: "upload",
		})
		if err != nil {
			return Credential{}, err
		}
		if headers == nil {
			headers = map[string]string{}
		}
		headers["X-Upload-Token"] = upload
	}
	return Credential{
		UploadURL:    issued.UploadURL,
		Method:       issued.Method,
		Headers:      headers,
		Form:         issued.Form,
		ObjectKey:    key,
		PublicURL:    issued.PublicURL,
		ExpiresAt:    exp,
		ConfirmToken: confirm,
	}, nil
}

func (s *Service) AcceptLocalUpload(token string, body []byte) error {
	if s.local == nil {
		return app.NotFound("本地上传未启用")
	}
	payload, err := openToken(s.secret, token, "upload")
	if err != nil {
		return app.Unauthorized("上传凭证无效")
	}
	if int64(len(body)) != payload.Size || int64(len(body)) > maxImageBytes {
		return app.Invalid("文件大小与凭证不一致")
	}
	if !magicOK(payload.Mime, body) {
		return app.Invalid("文件内容与类型不一致")
	}
	return s.local.Write(payload.Key, body)
}

type ConfirmInput struct {
	ObjectKey    string
	Checksum     string
	ConfirmToken string
}

func (s *Service) Confirm(ctx context.Context, actor identity.Principal, in ConfirmInput) (Credential, error) {
	if !actor.Has("asset.upload") {
		return Credential{}, app.Forbidden("无权上传文件")
	}
	payload, err := openToken(s.secret, in.ConfirmToken, "confirm")
	if err != nil {
		return Credential{}, app.Invalid("确认凭证无效")
	}
	if payload.OwnerID != actor.UserID || payload.Driver != s.driver.Name() || payload.Key != in.ObjectKey {
		return Credential{}, app.Forbidden("确认凭证与当前用户不一致")
	}
	sum := strings.ToLower(strings.TrimSpace(in.Checksum))
	if s.local != nil {
		f, err := s.local.Open(payload.Key)
		if err != nil {
			return Credential{}, app.Invalid("还没有收到文件")
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			return Credential{}, err
		}
		if info.Size() != payload.Size {
			return Credential{}, app.Invalid("文件大小与凭证不一致")
		}
		hash := sha256.New()
		if _, err := io.Copy(hash, f); err != nil {
			return Credential{}, err
		}
		got := hex.EncodeToString(hash.Sum(nil))
		if sum == "" || sum != got {
			return Credential{}, app.Invalid("校验和不正确")
		}
	} else if sum == "" {
		return Credential{}, app.Invalid("需要校验和")
	}
	var bucket *string
	if named, ok := s.driver.(interface{ Bucket() string }); ok {
		if b := named.Bucket(); b != "" {
			bucket = &b
		}
	}
	row, err := s.q.CreateAsset(ctx, sqlc.CreateAssetParams{
		Storage:    s.driver.Name(),
		Bucket:     bucket,
		ObjectKey:  payload.Key,
		Mime:       payload.Mime,
		Size:       payload.Size,
		OwnerID:    actor.UserID,
		Visibility: payload.Visibility,
		Checksum:   sum,
	})
	if err != nil {
		return Credential{}, err
	}
	public := ""
	if row.Visibility == "public" {
		public, _ = s.driver.PublicURL(row.ObjectKey)
	}
	return Credential{
		ID:        row.ID,
		ObjectKey: row.ObjectKey,
		PublicURL: public,
		Method:    s.driver.Name(),
	}, nil
}

// SignedURL checks asset.read, then own/any against owner_id. No HTTP route calls this.
func (s *Service) SignedURL(ctx context.Context, actor identity.Principal, assetID int64) (string, error) {
	row, err := s.q.GetAssetByID(ctx, assetID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", app.NotFound("文件不存在")
	}
	if err != nil {
		return "", err
	}
	if !actor.Can("asset.read", row.OwnerID) {
		return "", app.Forbidden("无权读取这个文件")
	}
	if row.Storage != s.driver.Name() {
		return "", app.NotFound("文件不在当前存储")
	}
	return s.driver.SignedURL(ctx, row.ObjectKey, 10*time.Minute)
}

func (s *Service) Delete(ctx context.Context, actor identity.Principal, assetID int64) error {
	row, err := s.q.GetAssetByID(ctx, assetID)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.NotFound("文件不存在")
	}
	if err != nil {
		return err
	}
	if !actor.Can("asset.delete", row.OwnerID) {
		return app.Forbidden("无权删除这个文件")
	}
	if row.Storage == s.driver.Name() {
		if err := s.driver.Delete(ctx, row.ObjectKey); err != nil {
			return err
		}
	}
	n, err := s.q.SoftDeleteAsset(ctx, assetID)
	if err != nil {
		return err
	}
	if n == 0 {
		return app.NotFound("文件不存在")
	}
	return nil
}

func (s *Service) OpenPublic(ctx context.Context, objectKey string) (string, io.ReadCloser, error) {
	if s.local == nil {
		return "", nil, app.NotFound("文件不存在")
	}
	row, err := s.q.GetLiveAssetByKey(ctx, sqlc.GetLiveAssetByKeyParams{Storage: "local", ObjectKey: objectKey})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && row.Visibility != "public") {
		return "", nil, app.NotFound("文件不存在")
	}
	if err != nil {
		return "", nil, err
	}
	f, err := s.local.Open(objectKey)
	if err != nil {
		return "", nil, err
	}
	return row.Mime, f, nil
}

func (s *Service) OpenSigned(ctx context.Context, objectKey, sig string, exp int64) (string, io.ReadCloser, error) {
	if s.local == nil || !verifyQuery(s.secret, objectKey, sig, exp) {
		return "", nil, app.NotFound("文件不存在")
	}
	row, err := s.q.GetLiveAssetByKey(ctx, sqlc.GetLiveAssetByKeyParams{Storage: "local", ObjectKey: objectKey})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, app.NotFound("文件不存在")
	}
	if err != nil {
		return "", nil, err
	}
	f, err := s.local.Open(objectKey)
	if err != nil {
		return "", nil, err
	}
	return row.Mime, f, nil
}

func (q *Qiniu) Bucket() string { return q.bucket }
func (d *S3) Bucket() string    { return d.bucket }

func objectKey(ext string) string {
	now := time.Now().UTC()
	return path.Join("images", now.Format("2006"), now.Format("01"), randomHex(16)+ext)
}

func randomHex(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func imageExt(mime string) (string, bool) {
	switch mime {
	case "image/jpeg":
		return ".jpg", true
	case "image/png":
		return ".png", true
	case "image/gif":
		return ".gif", true
	case "image/webp":
		return ".webp", true
	default:
		return "", false
	}
}

func magicOK(mime string, body []byte) bool {
	switch mime {
	case "image/jpeg":
		return len(body) >= 3 && body[0] == 0xFF && body[1] == 0xD8 && body[2] == 0xFF
	case "image/png":
		return len(body) >= 8 && bytes.Equal(body[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A})
	case "image/gif":
		return len(body) >= 6 && (bytes.Equal(body[:6], []byte("GIF87a")) || bytes.Equal(body[:6], []byte("GIF89a")))
	case "image/webp":
		return len(body) >= 12 && bytes.Equal(body[:4], []byte("RIFF")) && bytes.Equal(body[8:12], []byte("WEBP"))
	default:
		return false
	}
}
