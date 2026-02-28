package storage

import (
	"context"
	"fmt"
	"io"
)

// OSSStorage implements FileStorage using Alibaba Cloud OSS.
// This is a stub implementation — the actual OSS SDK integration
// will be added when the deployment moves to cloud infrastructure.
type OSSStorage struct {
	endpoint  string
	bucket    string
	accessKey string
	secretKey string
	region    string
}

// NewOSSStorage creates an OSS storage backend.
// Currently a stub — returns an error if OSS credentials are not provided.
func NewOSSStorage(cfg StorageConfig) (*OSSStorage, error) {
	if cfg.OSSEndpoint == "" || cfg.OSSBucket == "" {
		return nil, fmt.Errorf("OSS storage requires endpoint and bucket configuration")
	}
	return &OSSStorage{
		endpoint:  cfg.OSSEndpoint,
		bucket:    cfg.OSSBucket,
		accessKey: cfg.OSSAccessKey,
		secretKey: cfg.OSSSecretKey,
		region:    cfg.OSSRegion,
	}, nil
}

func (s *OSSStorage) Upload(ctx context.Context, key string, reader io.Reader, contentType string) error {
	// TODO: Implement with Alibaba Cloud OSS SDK
	return fmt.Errorf("OSS storage not yet implemented: Upload(%s)", key)
}

func (s *OSSStorage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("OSS storage not yet implemented: Download(%s)", key)
}

func (s *OSSStorage) Delete(ctx context.Context, key string) error {
	return fmt.Errorf("OSS storage not yet implemented: Delete(%s)", key)
}

func (s *OSSStorage) Exists(ctx context.Context, key string) (bool, error) {
	return false, fmt.Errorf("OSS storage not yet implemented: Exists(%s)", key)
}

func (s *OSSStorage) Info(ctx context.Context, key string) (*FileInfo, error) {
	return nil, fmt.Errorf("OSS storage not yet implemented: Info(%s)", key)
}

func (s *OSSStorage) URL(ctx context.Context, key string) (string, error) {
	return "", fmt.Errorf("OSS storage not yet implemented: URL(%s)", key)
}
