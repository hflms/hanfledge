package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// ============================
// LocalStorage Unit Tests
// ============================

// -- Constructor Tests ----------------------------------------

func TestNewLocalStorage(t *testing.T) {
	tmpDir := t.TempDir()
	root := filepath.Join(tmpDir, "uploads")

	s, err := NewLocalStorage(root)
	if err != nil {
		t.Fatalf("NewLocalStorage() error: %v", err)
	}
	if s == nil {
		t.Fatal("NewLocalStorage() returned nil")
	}

	// Root directory should be created
	info, err := os.Stat(root)
	if err != nil {
		t.Fatalf("root directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("root should be a directory")
	}
}

func TestNewLocalStorage_DefaultRoot(t *testing.T) {
	// Use empty string to get default "uploads" root
	// Run in temp dir to avoid polluting project
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	s, err := NewLocalStorage("")
	if err != nil {
		t.Fatalf("NewLocalStorage('') error: %v", err)
	}
	if s.root != "uploads" {
		t.Errorf("root = %q, want %q", s.root, "uploads")
	}
}

// -- Upload Tests ---------------------------------------------

func TestLocalStorage_Upload(t *testing.T) {
	tmpDir := t.TempDir()
	s, _ := NewLocalStorage(tmpDir)
	ctx := context.Background()

	content := []byte("hello world")
	err := s.Upload(ctx, "test.txt", bytes.NewReader(content), "text/plain")
	if err != nil {
		t.Fatalf("Upload() error: %v", err)
	}

	// Verify file exists on disk
	data, err := os.ReadFile(filepath.Join(tmpDir, "test.txt"))
	if err != nil {
		t.Fatalf("file not found on disk: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("file content = %q, want %q", string(data), "hello world")
	}
}

func TestLocalStorage_Upload_NestedKey(t *testing.T) {
	tmpDir := t.TempDir()
	s, _ := NewLocalStorage(tmpDir)
	ctx := context.Background()

	content := []byte("nested content")
	err := s.Upload(ctx, "courses/1/doc.pdf", bytes.NewReader(content), "application/pdf")
	if err != nil {
		t.Fatalf("Upload() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "courses", "1", "doc.pdf"))
	if err != nil {
		t.Fatalf("nested file not found: %v", err)
	}
	if string(data) != "nested content" {
		t.Errorf("content = %q, want %q", string(data), "nested content")
	}
}

// -- Download Tests -------------------------------------------

func TestLocalStorage_Download(t *testing.T) {
	tmpDir := t.TempDir()
	s, _ := NewLocalStorage(tmpDir)
	ctx := context.Background()

	// Upload first
	s.Upload(ctx, "download.txt", bytes.NewReader([]byte("download me")), "text/plain")

	// Download
	rc, err := s.Download(ctx, "download.txt")
	if err != nil {
		t.Fatalf("Download() error: %v", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll error: %v", err)
	}
	if string(data) != "download me" {
		t.Errorf("content = %q, want %q", string(data), "download me")
	}
}

func TestLocalStorage_Download_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	s, _ := NewLocalStorage(tmpDir)
	ctx := context.Background()

	_, err := s.Download(ctx, "nonexistent.txt")
	if err == nil {
		t.Error("Download() should return error for nonexistent file")
	}
}

// -- Delete Tests ---------------------------------------------

func TestLocalStorage_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	s, _ := NewLocalStorage(tmpDir)
	ctx := context.Background()

	// Upload then delete
	s.Upload(ctx, "delete-me.txt", bytes.NewReader([]byte("bye")), "text/plain")

	err := s.Delete(ctx, "delete-me.txt")
	if err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	// Verify file is gone
	exists, _ := s.Exists(ctx, "delete-me.txt")
	if exists {
		t.Error("File should not exist after delete")
	}
}

func TestLocalStorage_Delete_Nonexistent(t *testing.T) {
	tmpDir := t.TempDir()
	s, _ := NewLocalStorage(tmpDir)
	ctx := context.Background()

	// Deleting nonexistent file should not error
	err := s.Delete(ctx, "nonexistent.txt")
	if err != nil {
		t.Errorf("Delete() nonexistent file should not error, got: %v", err)
	}
}

// -- Exists Tests ---------------------------------------------

func TestLocalStorage_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	s, _ := NewLocalStorage(tmpDir)
	ctx := context.Background()

	// Before upload
	exists, err := s.Exists(ctx, "check.txt")
	if err != nil {
		t.Fatalf("Exists() error: %v", err)
	}
	if exists {
		t.Error("Exists() should return false before upload")
	}

	// After upload
	s.Upload(ctx, "check.txt", bytes.NewReader([]byte("data")), "text/plain")

	exists, err = s.Exists(ctx, "check.txt")
	if err != nil {
		t.Fatalf("Exists() error: %v", err)
	}
	if !exists {
		t.Error("Exists() should return true after upload")
	}
}

// -- Info Tests -----------------------------------------------

func TestLocalStorage_Info(t *testing.T) {
	tmpDir := t.TempDir()
	s, _ := NewLocalStorage(tmpDir)
	ctx := context.Background()

	content := []byte("some file content")
	s.Upload(ctx, "info-test.txt", bytes.NewReader(content), "text/plain")

	info, err := s.Info(ctx, "info-test.txt")
	if err != nil {
		t.Fatalf("Info() error: %v", err)
	}
	if info.Key != "info-test.txt" {
		t.Errorf("Key = %q, want %q", info.Key, "info-test.txt")
	}
	if info.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", info.Size, len(content))
	}
	if info.LastModified.IsZero() {
		t.Error("LastModified should not be zero")
	}
}

func TestLocalStorage_Info_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	s, _ := NewLocalStorage(tmpDir)
	ctx := context.Background()

	_, err := s.Info(ctx, "nonexistent.txt")
	if err == nil {
		t.Error("Info() should return error for nonexistent file")
	}
}

// -- URL Tests ------------------------------------------------

func TestLocalStorage_URL(t *testing.T) {
	tmpDir := t.TempDir()
	s, _ := NewLocalStorage(tmpDir)
	ctx := context.Background()

	url, err := s.URL(ctx, "test/doc.pdf")
	if err != nil {
		t.Fatalf("URL() error: %v", err)
	}

	expected := filepath.Join(tmpDir, "test/doc.pdf")
	if url != expected {
		t.Errorf("URL() = %q, want %q", url, expected)
	}
}

// -- Interface Compliance Test --------------------------------

func TestLocalStorage_ImplementsFileStorage(t *testing.T) {
	tmpDir := t.TempDir()
	s, _ := NewLocalStorage(tmpDir)

	// Compile-time check that LocalStorage implements FileStorage
	var _ FileStorage = s
}

// -- StorageConfig New() Tests --------------------------------

func TestNew_LocalBackend(t *testing.T) {
	tmpDir := t.TempDir()
	root := filepath.Join(tmpDir, "test-storage")

	fs, err := New(StorageConfig{
		Backend:   "local",
		LocalRoot: root,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if fs == nil {
		t.Fatal("New() returned nil")
	}
}

func TestNew_DefaultBackend(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	fs, err := New(StorageConfig{})
	if err != nil {
		t.Fatalf("New() with empty config error: %v", err)
	}
	if fs == nil {
		t.Fatal("New() returned nil")
	}
}

// -- Round-trip Test ------------------------------------------

func TestLocalStorage_UploadDownloadRoundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	s, _ := NewLocalStorage(tmpDir)
	ctx := context.Background()

	original := "This is the original content for round-trip test."
	key := "roundtrip/test.txt"

	// Upload
	if err := s.Upload(ctx, key, bytes.NewReader([]byte(original)), "text/plain"); err != nil {
		t.Fatalf("Upload error: %v", err)
	}

	// Exists
	exists, _ := s.Exists(ctx, key)
	if !exists {
		t.Fatal("File should exist after upload")
	}

	// Info
	info, _ := s.Info(ctx, key)
	if info.Size != int64(len(original)) {
		t.Errorf("Info size = %d, want %d", info.Size, len(original))
	}

	// Download
	rc, err := s.Download(ctx, key)
	if err != nil {
		t.Fatalf("Download error: %v", err)
	}
	data, _ := io.ReadAll(rc)
	rc.Close()

	if string(data) != original {
		t.Errorf("Round-trip content = %q, want %q", string(data), original)
	}

	// Delete
	s.Delete(ctx, key)
	exists, _ = s.Exists(ctx, key)
	if exists {
		t.Error("File should not exist after delete")
	}
}

func TestLocalStorage_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	s, _ := NewLocalStorage(tmpDir)
	ctx := context.Background()

	badKeys := []string{
		"../test.txt",
		"../../etc/passwd",
		"/etc/passwd",
		"subdir/../../test.txt", // cleans to ../test.txt
	}

	for _, key := range badKeys {
		t.Run(key, func(t *testing.T) {
			if err := s.Upload(ctx, key, bytes.NewReader([]byte("data")), "text/plain"); err == nil {
				t.Errorf("Upload() should reject traversal: %s", key)
			}
			if _, err := s.Download(ctx, key); err == nil {
				t.Errorf("Download() should reject traversal: %s", key)
			}
			if err := s.Delete(ctx, key); err == nil {
				t.Errorf("Delete() should reject traversal: %s", key)
			}
			if _, err := s.Exists(ctx, key); err == nil {
				t.Errorf("Exists() should reject traversal: %s", key)
			}
			if _, err := s.Info(ctx, key); err == nil {
				t.Errorf("Info() should reject traversal: %s", key)
			}
			if _, err := s.URL(ctx, key); err == nil {
				t.Errorf("URL() should reject traversal: %s", key)
			}
		})
	}
}
