package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LocalStorage implements FileStorage using the local filesystem.
type LocalStorage struct {
	root string
}

func (s *LocalStorage) resolvePath(key string) (string, error) {
	cleanRoot := filepath.Clean(s.root)

	// Check for absolute paths passed by the user
	if filepath.IsAbs(filepath.Clean(key)) {
		return "", fmt.Errorf("absolute paths are not allowed: %s", key)
	}

	// Use filepath.Join to resolve paths. Join automatically calls Clean
	path := filepath.Join(cleanRoot, key)

	// In Go, filepath.Join(cleanRoot, key) can escape if key has ..
	// However, if we evaluate whether the resulting path has a prefix
	// of the clean root directory, we can block it.

	// Prevent root bypass: `cleanRoot+string(filepath.Separator)` prevents `/uploads2` bypassing `/uploads`
	// Handle edge case where cleanRoot is `/` and Separator makes `//`
	prefix := cleanRoot
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}

	if path != cleanRoot && !strings.HasPrefix(path, prefix) {
		return "", fmt.Errorf("invalid path: %s", key)
	}

	return path, nil
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
	path, err := s.resolvePath(key)
	if err != nil {
		return err
	}

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
	path, err := s.resolvePath(key)
	if err != nil {
		return nil, err
	}
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
	path, err := s.resolvePath(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete file %s: %w", key, err)
	}
	return nil
}

func (s *LocalStorage) Exists(ctx context.Context, key string) (bool, error) {
	path, err := s.resolvePath(key)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s *LocalStorage) Info(ctx context.Context, key string) (*FileInfo, error) {
	path, err := s.resolvePath(key)
	if err != nil {
		return nil, err
	}
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
	path, err := s.resolvePath(key)
	if err != nil {
		return "", err
	}
	return path, nil
}
