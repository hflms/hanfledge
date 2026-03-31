package handler

import (
	"net/http"
	"testing"

	"github.com/hflms/hanfledge/internal/domain/model"
)

// ============================
// System Config Handler Unit Tests
// ============================

// setupSystemConfigTestDB creates a test DB with SystemConfig table migrated.
func setupSystemConfigTestDB(t *testing.T) *SystemConfigHandler {
	t.Helper()
	db := setupTestDB(t)
	if err := db.AutoMigrate(&model.SystemConfig{}); err != nil {
		t.Fatalf("AutoMigrate SystemConfig failed: %v", err)
	}
	return NewSystemConfigHandler(db, nil)
}

// -- Constructor Tests ----------------------------------------

func TestNewSystemConfigHandler(t *testing.T) {
	h := NewSystemConfigHandler(nil, nil)
	if h == nil {
		t.Fatal("NewSystemConfigHandler returned nil")
	}
}

// -- GetConfigs Tests -----------------------------------------

func TestGetConfigs_Empty(t *testing.T) {
	h := setupSystemConfigTestDB(t)

	w, c := newTestContextWithQuery("GET", "/api/v1/system/configs", 0)
	h.GetConfigs(c)

	assertStatus(t, w, http.StatusOK)
	// Empty map should be returned as {}
	assertBodyContains(t, w, "{}")
}

func TestGetConfigs_WithData(t *testing.T) {
	h := setupSystemConfigTestDB(t)

	// Seed some configs
	h.DB.Create(&model.SystemConfig{Key: "OLLAMA_BASE_URL", Value: "http://localhost:11434"})
	h.DB.Create(&model.SystemConfig{Key: "OLLAMA_MODEL", Value: "qwen2.5:7b"})

	w, c := newTestContextWithQuery("GET", "/api/v1/system/configs", 0)
	h.GetConfigs(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, "OLLAMA_BASE_URL")
	assertBodyContains(t, w, "http://localhost:11434")
	assertBodyContains(t, w, "OLLAMA_MODEL")
	assertBodyContains(t, w, "qwen2.5:7b")
}

func TestGetConfigs_ReturnsAsMap(t *testing.T) {
	h := setupSystemConfigTestDB(t)

	h.DB.Create(&model.SystemConfig{Key: "test_key", Value: "test_value"})

	w, c := newTestContextWithQuery("GET", "/api/v1/system/configs", 0)
	h.GetConfigs(c)

	assertStatus(t, w, http.StatusOK)
	// Result should be a flat map, not an array
	assertBodyContains(t, w, `"test_key":"test_value"`)
}

// -- UpdateConfigs Tests --------------------------------------

func TestUpdateConfigs_Success(t *testing.T) {
	h := setupSystemConfigTestDB(t)

	body := `{"OLLAMA_BASE_URL":"http://newhost:11434","OLLAMA_MODEL":"qwen2.5:14b"}`
	w, c := newTestContext("PUT", "/api/v1/system/configs", body, 1)
	h.UpdateConfigs(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, "配置更新成功")

	// Verify in DB
	var cfg model.SystemConfig
	h.DB.Where("key = ?", "OLLAMA_BASE_URL").First(&cfg)
	if cfg.Value != "http://newhost:11434" {
		t.Errorf("DB value = %q, want %q", cfg.Value, "http://newhost:11434")
	}
}

func TestUpdateConfigs_InvalidJSON(t *testing.T) {
	h := setupSystemConfigTestDB(t)

	w, c := newTestContext("PUT", "/api/v1/system/configs", "not json", 1)
	h.UpdateConfigs(c)

	assertStatus(t, w, http.StatusBadRequest)
	assertBodyContains(t, w, "无效的请求数据")
}

func TestUpdateConfigs_EmptyBody(t *testing.T) {
	h := setupSystemConfigTestDB(t)

	w, c := newTestContext("PUT", "/api/v1/system/configs", "{}", 1)
	h.UpdateConfigs(c)

	// Empty map should still succeed (no-op transaction)
	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, "配置更新成功")
}

func TestUpdateConfigs_OverwriteExisting(t *testing.T) {
	h := setupSystemConfigTestDB(t)

	// Seed existing config
	h.DB.Create(&model.SystemConfig{Key: "test_key", Value: "old_value"})

	body := `{"test_key":"new_value"}`
	w, c := newTestContext("PUT", "/api/v1/system/configs", body, 1)
	h.UpdateConfigs(c)

	assertStatus(t, w, http.StatusOK)

	// Verify overwrite
	var cfg model.SystemConfig
	h.DB.Where("key = ?", "test_key").First(&cfg)
	if cfg.Value != "new_value" {
		t.Errorf("DB value = %q, want %q", cfg.Value, "new_value")
	}
}

// -- loadConfigMap Tests --------------------------------------

func TestLoadConfigMap_Empty(t *testing.T) {
	h := setupSystemConfigTestDB(t)

	configMap := h.loadConfigMap()
	if configMap == nil {
		t.Fatal("loadConfigMap returned nil for empty table")
	}
	if len(configMap) != 0 {
		t.Errorf("loadConfigMap len = %d, want 0", len(configMap))
	}
}

func TestLoadConfigMap_WithData(t *testing.T) {
	h := setupSystemConfigTestDB(t)

	h.DB.Create(&model.SystemConfig{Key: "key1", Value: "val1"})
	h.DB.Create(&model.SystemConfig{Key: "key2", Value: "val2"})

	configMap := h.loadConfigMap()
	if len(configMap) != 2 {
		t.Fatalf("loadConfigMap len = %d, want 2", len(configMap))
	}
	if configMap["key1"] != "val1" {
		t.Errorf("configMap[key1] = %q, want %q", configMap["key1"], "val1")
	}
	if configMap["key2"] != "val2" {
		t.Errorf("configMap[key2] = %q, want %q", configMap["key2"], "val2")
	}
}
