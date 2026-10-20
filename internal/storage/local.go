package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/z123-piad/houttuynia-blog/internal/app"
)

type Local struct {
	root       string
	secret     string
	publicBase string
}

func NewLocal(root, secret, publicBase string) *Local {
	return &Local{root: root, secret: secret, publicBase: strings.TrimRight(publicBase, "/")}
}

func (d *Local) Name() string { return "local" }

func (d *Local) UploadCredential(_ context.Context, in UploadInput) (UploadCredential, error) {
	exp := time.Now().Add(15 * time.Minute)
	token, err := signToken(d.secret, tokenPayload{
		Key: in.ObjectKey, Mime: in.Mime, Size: in.Size, Visibility: in.Visibility,
		Driver: d.Name(), Exp: exp.Unix(), Kind: "upload",
	})
	if err != nil {
		return UploadCredential{}, err
	}
	public, err := d.PublicURL(in.ObjectKey)
	if err != nil {
		return UploadCredential{}, err
	}
	return UploadCredential{
		UploadURL: "/api/v1/storage/local/upload",
		Method:    "PUT",
		Headers:   map[string]string{"X-Upload-Token": token},
		ExpiresAt: exp,
		PublicURL: public,
	}, nil
}

func (d *Local) PublicURL(objectKey string) (string, error) {
	path := "/media/" + escapeKey(objectKey)
	if d.publicBase == "" {
		return path, nil
	}
	return d.publicBase + path, nil
}

func (d *Local) SignedURL(_ context.Context, objectKey string, expires time.Duration) (string, error) {
	exp := time.Now().Add(expires).Unix()
	sig := signQuery(d.secret, objectKey, exp)
	path := fmt.Sprintf("/media-signed/%s?exp=%d&sig=%s", escapeKey(objectKey), exp, sig)
	if d.publicBase == "" {
		return path, nil
	}
	return d.publicBase + path, nil
}

func (d *Local) Delete(_ context.Context, objectKey string) error {
	path, err := d.abs(objectKey)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (d *Local) abs(objectKey string) (string, error) {
	if objectKey == "" || strings.Contains(objectKey, "..") || strings.HasPrefix(objectKey, "/") {
		return "", app.Invalid("对象键不正确")
	}
	clean := filepath.Clean(objectKey)
	full := filepath.Join(d.root, clean)
	rel, err := filepath.Rel(d.root, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", app.Invalid("对象键不正确")
	}
	return full, nil
}

func (d *Local) Write(objectKey string, body []byte) error {
	path, err := d.abs(objectKey)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

func (d *Local) Open(objectKey string) (*os.File, error) {
	path, err := d.abs(objectKey)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, app.NotFound("文件不存在")
	}
	return f, err
}
