package storage

import (
	"context"
	"io"
	"time"
)

// FileInfo describes a stored file.
type FileInfo struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	ContentType  string    `json:"content_type"`
	LastModified time.Time `json:"last_modified"`
}

// FileStorage defines the contract for file storage backends.
// Implementations: LocalStorage (filesystem), OSSStorage (Alibaba Cloud OSS).
type FileStorage interface {
	// Upload stores a file and returns its storage key.
	Upload(ctx context.Context, key string, reader io.Reader, contentType string) error

	// Download retrieves a file by its storage key.
	Download(ctx context.Context, key string) (io.ReadCloser, error)

	// Delete removes a file by its storage key.
	Delete(ctx context.Context, key string) error

	// Exists checks whether a file exists.
	Exists(ctx context.Context, key string) (bool, error)

	// Info returns metadata about a stored file.
	Info(ctx context.Context, key string) (*FileInfo, error)

	// URL returns a (possibly signed) URL for the file.
	// For local storage, returns a relative path; for OSS, returns a presigned URL.
	URL(ctx context.Context, key string) (string, error)
}

// StorageConfig holds configuration for the storage backend.
type StorageConfig struct {
	Backend   string // "local" or "oss"
	LocalRoot string // Root directory for local storage (default: "uploads")

	// OSS settings
	OSSEndpoint  string
	OSSBucket    string
	OSSAccessKey string
	OSSSecretKey string
	OSSRegion    string
}

// New creates a FileStorage implementation based on configuration.
func New(cfg StorageConfig) (FileStorage, error) {
	switch cfg.Backend {
	case "oss":
		return NewOSSStorage(cfg)
	default:
		return NewLocalStorage(cfg.LocalRoot)
	}
}
