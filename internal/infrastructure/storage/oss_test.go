package storage

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================
// OSSStorage Unit Tests
// ============================

// -- Constructor Tests ----------------------------------------

func TestNewOSSStorage(t *testing.T) {
	t.Run("valid configuration", func(t *testing.T) {
		cfg := StorageConfig{
			OSSEndpoint:  "oss-cn-hangzhou.aliyuncs.com",
			OSSBucket:    "my-bucket",
			OSSAccessKey: "access-key",
			OSSSecretKey: "secret-key",
			OSSRegion:    "cn-hangzhou",
		}

		storage, err := NewOSSStorage(cfg)
		require.NoError(t, err)
		require.NotNil(t, storage)

		assert.Equal(t, "oss-cn-hangzhou.aliyuncs.com", storage.endpoint)
		assert.Equal(t, "my-bucket", storage.bucket)
		assert.Equal(t, "access-key", storage.accessKey)
		assert.Equal(t, "secret-key", storage.secretKey)
		assert.Equal(t, "cn-hangzhou", storage.region)
	})

	t.Run("missing endpoint", func(t *testing.T) {
		cfg := StorageConfig{
			OSSBucket: "my-bucket",
		}

		storage, err := NewOSSStorage(cfg)
		require.Error(t, err)
		assert.Nil(t, storage)
		assert.Contains(t, err.Error(), "requires endpoint and bucket configuration")
	})

	t.Run("missing bucket", func(t *testing.T) {
		cfg := StorageConfig{
			OSSEndpoint: "oss-cn-hangzhou.aliyuncs.com",
		}

		storage, err := NewOSSStorage(cfg)
		require.Error(t, err)
		assert.Nil(t, storage)
		assert.Contains(t, err.Error(), "requires endpoint and bucket configuration")
	})
}

// -- Method Tests ---------------------------------------------

func TestOSSStorage_Methods(t *testing.T) {
	cfg := StorageConfig{
		OSSEndpoint: "oss-cn-hangzhou.aliyuncs.com",
		OSSBucket:   "my-bucket",
	}

	storage, err := NewOSSStorage(cfg)
	require.NoError(t, err)
	require.NotNil(t, storage)

	ctx := context.Background()

	t.Run("Upload", func(t *testing.T) {
		reader := bytes.NewReader([]byte("test content"))
		err := storage.Upload(ctx, "test-key", reader, "text/plain")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not yet implemented")
		assert.Contains(t, err.Error(), "Upload(test-key)")
	})

	t.Run("Download", func(t *testing.T) {
		rc, err := storage.Download(ctx, "test-key")
		require.Error(t, err)
		assert.Nil(t, rc)
		assert.Contains(t, err.Error(), "not yet implemented")
		assert.Contains(t, err.Error(), "Download(test-key)")
	})

	t.Run("Delete", func(t *testing.T) {
		err := storage.Delete(ctx, "test-key")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not yet implemented")
		assert.Contains(t, err.Error(), "Delete(test-key)")
	})

	t.Run("Exists", func(t *testing.T) {
		exists, err := storage.Exists(ctx, "test-key")
		require.Error(t, err)
		assert.False(t, exists)
		assert.Contains(t, err.Error(), "not yet implemented")
		assert.Contains(t, err.Error(), "Exists(test-key)")
	})

	t.Run("Info", func(t *testing.T) {
		info, err := storage.Info(ctx, "test-key")
		require.Error(t, err)
		assert.Nil(t, info)
		assert.Contains(t, err.Error(), "not yet implemented")
		assert.Contains(t, err.Error(), "Info(test-key)")
	})

	t.Run("URL", func(t *testing.T) {
		url, err := storage.URL(ctx, "test-key")
		require.Error(t, err)
		assert.Empty(t, url)
		assert.Contains(t, err.Error(), "not yet implemented")
		assert.Contains(t, err.Error(), "URL(test-key)")
	})
}
