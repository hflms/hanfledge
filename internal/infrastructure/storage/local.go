package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// LocalStorage implements FileStorage using the local filesystem.
type LocalStorage struct {
	root string
}

// NewLocalStorage creates a local filesystem storage backend.
func NewLocalStorage(root string) (*LocalStorage, error) {
	if root == "" {
		root = "uploads"
	}
	// Ensure root directory exists
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, fmt.Errorf("create storage root %s: %w", root, err)
	}
	return &LocalStorage{root: root}, nil
}

func (s *LocalStorage) Upload(ctx context.Context, key string, reader io.Reader, contentType string) error {
	path := filepath.Join(s.root, key)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create directory for %s: %w", key, err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file %s: %w", key, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, reader); err != nil {
		os.Remove(path) // cleanup on failure
		return fmt.Errorf("write file %s: %w", key, err)
	}

	return nil
}

func (s *LocalStorage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	path := filepath.Join(s.root, key)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", key)
		}
		return nil, fmt.Errorf("open file %s: %w", key, err)
	}
	return f, nil
}

func (s *LocalStorage) Delete(ctx context.Context, key string) error {
	path := filepath.Join(s.root, key)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete file %s: %w", key, err)
	}
	return nil
}

func (s *LocalStorage) Exists(ctx context.Context, key string) (bool, error) {
	path := filepath.Join(s.root, key)
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s *LocalStorage) Info(ctx context.Context, key string) (*FileInfo, error) {
	path := filepath.Join(s.root, key)
	stat, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", key)
		}
		return nil, err
	}
	return &FileInfo{
		Key:          key,
		Size:         stat.Size(),
		LastModified: stat.ModTime(),
	}, nil
}

func (s *LocalStorage) URL(ctx context.Context, key string) (string, error) {
	// For local storage, return the relative filesystem path
	return filepath.Join(s.root, key), nil
}
