package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// OSSStorage implements FileStorage using Alibaba Cloud OSS.
type OSSStorage struct {
	client     *oss.Client
	bucketName string
	bucket     *oss.Bucket
	endpoint   string
	region     string
}

// NewOSSStorage creates an OSS storage backend.
func NewOSSStorage(cfg StorageConfig) (*OSSStorage, error) {
	if cfg.OSSEndpoint == "" || cfg.OSSBucket == "" || cfg.OSSAccessKey == "" || cfg.OSSSecretKey == "" {
		return nil, fmt.Errorf("OSS storage requires endpoint, bucket, access key, and secret key configuration")
	}

	client, err := oss.New(cfg.OSSEndpoint, cfg.OSSAccessKey, cfg.OSSSecretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create OSS client: %w", err)
	}

	bucket, err := client.Bucket(cfg.OSSBucket)
	if err != nil {
		return nil, fmt.Errorf("failed to get OSS bucket: %w", err)
	}

	return &OSSStorage{
		client:     client,
		bucketName: cfg.OSSBucket,
		bucket:     bucket,
		endpoint:   cfg.OSSEndpoint,
		region:     cfg.OSSRegion,
	}, nil
}

func (s *OSSStorage) Upload(ctx context.Context, key string, reader io.Reader, contentType string) error {
	options := []oss.Option{}
	if contentType != "" {
		options = append(options, oss.ContentType(contentType))
	}

	err := s.bucket.PutObject(key, reader, options...)
	if err != nil {
		return fmt.Errorf("failed to upload object to OSS: %w", err)
	}
	return nil
}

func (s *OSSStorage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	body, err := s.bucket.GetObject(key)
	if err != nil {
		return nil, fmt.Errorf("failed to download object from OSS: %w", err)
	}
	return body, nil
}

func (s *OSSStorage) Delete(ctx context.Context, key string) error {
	err := s.bucket.DeleteObject(key)
	if err != nil {
		return fmt.Errorf("failed to delete object from OSS: %w", err)
	}
	return nil
}

func (s *OSSStorage) Exists(ctx context.Context, key string) (bool, error) {
	exist, err := s.bucket.IsObjectExist(key)
	if err != nil {
		return false, fmt.Errorf("failed to check if object exists in OSS: %w", err)
	}
	return exist, nil
}

func (s *OSSStorage) Info(ctx context.Context, key string) (*FileInfo, error) {
	props, err := s.bucket.GetObjectDetailedMeta(key)
	if err != nil {
		return nil, fmt.Errorf("failed to get object info from OSS: %w", err)
	}

	sizeStr := props.Get("Content-Length")
	size, _ := strconv.ParseInt(sizeStr, 10, 64)

	contentType := props.Get("Content-Type")

	lastModifiedStr := props.Get("Last-Modified")
	lastModified, _ := time.Parse(http.TimeFormat, lastModifiedStr)

	return &FileInfo{
		Key:          key,
		Size:         size,
		ContentType:  contentType,
		LastModified: lastModified,
	}, nil
}

func (s *OSSStorage) URL(ctx context.Context, key string) (string, error) {
	signedURL, err := s.bucket.SignURL(key, oss.HTTPGet, 3600) // 1 hour expiration
	if err != nil {
		return "", fmt.Errorf("failed to generate signed URL for OSS object: %w", err)
	}
	return signedURL, nil
}
