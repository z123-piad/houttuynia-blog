package storage

import (
	"fmt"
	"os"

	"github.com/z123-piad/houttuynia-blog/internal/config"
)

func NewDriver(cfg config.Config) (Driver, error) {
	switch cfg.StorageDriver {
	case "local":
		if err := os.MkdirAll(cfg.LocalStoragePath, 0o755); err != nil {
			return nil, err
		}
		return NewLocal(cfg.LocalStoragePath, cfg.SessionSecret, cfg.PublicBaseURL), nil
	case "qiniu":
		return NewQiniu(cfg.QiniuAccessKey, cfg.QiniuSecretKey, cfg.QiniuBucket, cfg.QiniuCDNDomain), nil
	case "s3":
		return NewS3(cfg.S3Endpoint, cfg.S3Region, cfg.S3Bucket, cfg.S3AccessKey, cfg.S3SecretKey), nil
	default:
		return nil, fmt.Errorf("unknown storage driver %s", cfg.StorageDriver)
	}
}
