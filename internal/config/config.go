package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DatabaseURL       string
	SessionSecret     string
	HTTPAddr          string
	CookieSecure      bool
	StorageDriver     string
	LocalStoragePath  string
	PublicBaseURL     string
	WebDir            string
	SeedAdminEmail    string
	SeedAdminPassword string
	QiniuAccessKey    string
	QiniuSecretKey    string
	QiniuBucket       string
	QiniuCDNDomain    string
	S3Endpoint        string
	S3Region          string
	S3Bucket          string
	S3AccessKey       string
	S3SecretKey       string
	VisitorHashKey    string
}

func Load() (Config, error) {
	loadDotEnv(".env")
	cfg := Config{
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		SessionSecret:     os.Getenv("SESSION_SECRET"),
		HTTPAddr:          getenv("HTTP_ADDR", ":8080"),
		CookieSecure:      os.Getenv("COOKIE_SECURE") == "true",
		StorageDriver:     getenv("STORAGE_DRIVER", "local"),
		LocalStoragePath:  getenv("LOCAL_STORAGE_PATH", "./data/uploads"),
		PublicBaseURL:     strings.TrimRight(os.Getenv("PUBLIC_BASE_URL"), "/"),
		WebDir:            os.Getenv("WEB_DIR"),
		SeedAdminEmail:    os.Getenv("SEED_ADMIN_EMAIL"),
		SeedAdminPassword: os.Getenv("SEED_ADMIN_PASSWORD"),
		QiniuAccessKey:    os.Getenv("QINIU_ACCESS_KEY"),
		QiniuSecretKey:    os.Getenv("QINIU_SECRET_KEY"),
		QiniuBucket:       os.Getenv("QINIU_BUCKET"),
		QiniuCDNDomain:    strings.TrimRight(os.Getenv("QINIU_CDN_DOMAIN"), "/"),
		S3Endpoint:        strings.TrimRight(os.Getenv("S3_ENDPOINT"), "/"),
		S3Region:          os.Getenv("S3_REGION"),
		S3Bucket:          os.Getenv("S3_BUCKET"),
		S3AccessKey:       os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey:       os.Getenv("S3_SECRET_KEY"),
		VisitorHashKey:    os.Getenv("VISITOR_HASH_KEY"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if len(cfg.SessionSecret) < 16 {
		return Config{}, fmt.Errorf("SESSION_SECRET must be at least 16 characters")
	}
	switch cfg.StorageDriver {
	case "local", "qiniu", "s3":
	default:
		return Config{}, fmt.Errorf("STORAGE_DRIVER must be local, qiniu, or s3")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				unquoted, err := strconv.Unquote(val)
				if err == nil {
					val = unquoted
				} else {
					val = val[1 : len(val)-1]
				}
			}
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, val)
	}
}
