package storage

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/z123-piad/houttuynia-blog/internal/app"
)

func TestQiniuAndS3FailClosed(t *testing.T) {
	ctx := context.Background()
	in := UploadInput{ObjectKey: "images/a.jpg", Mime: "image/jpeg", Size: 10, Visibility: "public"}
	if _, err := NewQiniu("", "", "", "").UploadCredential(ctx, in); !errors.As(err, new(*app.Error)) {
		t.Fatalf("qiniu unconfigured: %v", err)
	}
	if _, err := NewS3("", "", "", "", "").UploadCredential(ctx, in); !errors.As(err, new(*app.Error)) {
		t.Fatalf("s3 unconfigured: %v", err)
	}
	qiniu := NewQiniu("AKID", "secret", "bucket", "https://cdn.example.com")
	cred, err := qiniu.UploadCredential(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if cred.Method != "POST" || !strings.Contains(cred.UploadURL, "upload.qiniup.com") {
		t.Fatalf("credential %+v", cred)
	}
	if !strings.Contains(cred.Form["token"], "AKID") {
		t.Fatalf("token %q", cred.Form["token"])
	}
}
