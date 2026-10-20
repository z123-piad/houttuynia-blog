package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/z123-piad/houttuynia-blog/internal/app"
)

type S3 struct {
	endpoint  string
	region    string
	bucket    string
	accessKey string
	secretKey string
}

func NewS3(endpoint, region, bucket, accessKey, secretKey string) *S3 {
	return &S3{
		endpoint:  strings.TrimRight(endpoint, "/"),
		region:    region,
		bucket:    bucket,
		accessKey: accessKey,
		secretKey: secretKey,
	}
}

func (d *S3) Name() string { return "s3" }

func (d *S3) configured() error {
	if d.endpoint == "" || d.region == "" || d.bucket == "" || d.accessKey == "" || d.secretKey == "" {
		return app.NotConfigured("S3 存储未配置")
	}
	return nil
}

func (d *S3) client() (*s3.Client, error) {
	if err := d.configured(); err != nil {
		return nil, err
	}
	cfg := aws.Config{
		Region:       d.region,
		Credentials:  credentials.NewStaticCredentialsProvider(d.accessKey, d.secretKey, ""),
		BaseEndpoint: aws.String(d.endpoint),
	}
	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	}), nil
}

func (d *S3) UploadCredential(ctx context.Context, in UploadInput) (UploadCredential, error) {
	client, err := d.client()
	if err != nil {
		return UploadCredential{}, err
	}
	presign := s3.NewPresignClient(client)
	out, err := presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(d.bucket),
		Key:         aws.String(in.ObjectKey),
		ContentType: aws.String(in.Mime),
	}, s3.WithPresignExpires(15*time.Minute))
	if err != nil {
		return UploadCredential{}, err
	}
	public, err := d.PublicURL(in.ObjectKey)
	if err != nil {
		return UploadCredential{}, err
	}
	return UploadCredential{
		UploadURL: out.URL,
		Method:    out.Method,
		Headers:   map[string]string{"Content-Type": in.Mime},
		ExpiresAt: time.Now().Add(15 * time.Minute),
		PublicURL: public,
	}, nil
}

func (d *S3) PublicURL(objectKey string) (string, error) {
	if err := d.configured(); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s/%s", d.endpoint, d.bucket, escapeKey(objectKey)), nil
}

func (d *S3) SignedURL(ctx context.Context, objectKey string, expires time.Duration) (string, error) {
	client, err := d.client()
	if err != nil {
		return "", err
	}
	if expires <= 0 || expires > time.Hour {
		expires = 10 * time.Minute
	}
	out, err := s3.NewPresignClient(client).PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(d.bucket),
		Key:    aws.String(objectKey),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return "", err
	}
	return out.URL, nil
}

func (d *S3) Delete(ctx context.Context, objectKey string) error {
	client, err := d.client()
	if err != nil {
		return err
	}
	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(d.bucket),
		Key:    aws.String(objectKey),
	})
	return err
}
