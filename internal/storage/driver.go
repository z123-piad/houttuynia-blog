package storage

import (
	"context"
	"time"
)

type UploadInput struct {
	ObjectKey  string
	Mime       string
	Size       int64
	Visibility string
}

type UploadCredential struct {
	UploadURL string
	Method    string
	Headers   map[string]string
	Form      map[string]string
	ExpiresAt time.Time
	PublicURL string
}

// Driver is the four-method storage boundary.
type Driver interface {
	Name() string
	UploadCredential(ctx context.Context, in UploadInput) (UploadCredential, error)
	PublicURL(objectKey string) (string, error)
	SignedURL(ctx context.Context, objectKey string, expires time.Duration) (string, error)
	Delete(ctx context.Context, objectKey string) error
}
