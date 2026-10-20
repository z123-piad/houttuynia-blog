package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/z123-piad/houttuynia-blog/internal/app"
)

type Qiniu struct {
	accessKey string
	secretKey string
	bucket    string
	cdnDomain string
}

func NewQiniu(accessKey, secretKey, bucket, cdnDomain string) *Qiniu {
	return &Qiniu{
		accessKey: accessKey,
		secretKey: secretKey,
		bucket:    bucket,
		cdnDomain: strings.TrimRight(cdnDomain, "/"),
	}
}

func (d *Qiniu) Name() string { return "qiniu" }

func (d *Qiniu) configured() error {
	if d.accessKey == "" || d.secretKey == "" || d.bucket == "" || d.cdnDomain == "" {
		return app.NotConfigured("七牛存储未配置")
	}
	return nil
}

func (d *Qiniu) UploadCredential(_ context.Context, in UploadInput) (UploadCredential, error) {
	if err := d.configured(); err != nil {
		return UploadCredential{}, err
	}
	exp := time.Now().Add(15 * time.Minute)
	policy, _ := json.Marshal(map[string]any{
		"scope":    d.bucket + ":" + in.ObjectKey,
		"deadline": exp.Unix(),
	})
	encoded := base64.URLEncoding.EncodeToString(policy)
	mac := hmac.New(sha1.New, []byte(d.secretKey))
	_, _ = mac.Write([]byte(encoded))
	sign := base64.URLEncoding.EncodeToString(mac.Sum(nil))
	token := d.accessKey + ":" + sign + ":" + encoded
	public, err := d.PublicURL(in.ObjectKey)
	if err != nil {
		return UploadCredential{}, err
	}
	return UploadCredential{
		UploadURL: "https://upload.qiniup.com",
		Method:    "POST",
		Form:      map[string]string{"token": token, "key": in.ObjectKey},
		ExpiresAt: exp,
		PublicURL: public,
	}, nil
}

func (d *Qiniu) PublicURL(objectKey string) (string, error) {
	if err := d.configured(); err != nil {
		return "", err
	}
	return d.cdnDomain + "/" + escapeKey(objectKey), nil
}

func (d *Qiniu) SignedURL(_ context.Context, objectKey string, expires time.Duration) (string, error) {
	public, err := d.PublicURL(objectKey)
	if err != nil {
		return "", err
	}
	exp := time.Now().Add(expires).Unix()
	base := fmt.Sprintf("%s?e=%d", public, exp)
	mac := hmac.New(sha1.New, []byte(d.secretKey))
	_, _ = mac.Write([]byte(base))
	sign := base64.URLEncoding.EncodeToString(mac.Sum(nil))
	return base + "&token=" + d.accessKey + ":" + sign, nil
}

func (d *Qiniu) Delete(ctx context.Context, objectKey string) error {
	if err := d.configured(); err != nil {
		return err
	}
	entry := base64.URLEncoding.EncodeToString([]byte(d.bucket + ":" + objectKey))
	path := "/delete/" + entry
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://rs.qiniu.com"+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Qiniu "+d.manageToken(path+"\n"))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)
	if res.StatusCode == 612 || res.StatusCode == http.StatusOK || res.StatusCode == http.StatusNoContent {
		return nil
	}
	return fmt.Errorf("qiniu delete status %d", res.StatusCode)
}

func (d *Qiniu) manageToken(signing string) string {
	mac := hmac.New(sha1.New, []byte(d.secretKey))
	_, _ = mac.Write([]byte(signing))
	return d.accessKey + ":" + base64.URLEncoding.EncodeToString(mac.Sum(nil))
}
