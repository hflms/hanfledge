package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/hflms/hanfledge/internal/domain/model"
)

// ============================
// Soul Handler Unit Tests
// ============================

// setupSoulTestDB creates a test DB with SoulVersion table and a temp soul file.
func setupSoulTestDB(t *testing.T) (*SoulHandler, string) {
	t.Helper()
	db := setupTestDB(t)
	if err := db.AutoMigrate(&model.SoulVersion{}); err != nil {
		t.Fatalf("AutoMigrate SoulVersion failed: %v", err)
	}

	tmpDir := t.TempDir()
	soulPath := filepath.Join(tmpDir, "soul.md")
	if err := os.WriteFile(soulPath, []byte("# Soul Rules\nBe helpful."), 0644); err != nil {
		t.Fatalf("write soul.md failed: %v", err)
	}

	h := NewSoulHandler(db, soulPath, nil)
	return h, soulPath
}

// -- Constructor Tests ----------------------------------------

func TestNewSoulHandler(t *testing.T) {
	h := NewSoulHandler(nil, "/tmp/soul.md", nil)
	if h == nil {
		t.Fatal("NewSoulHandler returned nil")
	}
}

// -- GetSoul Tests --------------------------------------------

func TestGetSoul_Success(t *testing.T) {
	h, _ := setupSoulTestDB(t)

	w, c := newTestContextWithQuery("GET", "/api/v1/system/soul", 1)
	h.GetSoul(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, "# Soul Rules")
	assertBodyContains(t, w, "Be helpful.")
}

func TestGetSoul_FileNotFound(t *testing.T) {
	db := setupTestDB(t)
	db.AutoMigrate(&model.SoulVersion{})
	h := NewSoulHandler(db, "/nonexistent/soul.md", nil)

	w, c := newTestContextWithQuery("GET", "/api/v1/system/soul", 1)
	h.GetSoul(c)

	assertStatus(t, w, http.StatusInternalServerError)
	assertBodyContains(t, w, "读取失败")
}

func TestGetSoul_WithActiveVersion(t *testing.T) {
	h, _ := setupSoulTestDB(t)

	// Seed an active version
	h.db.Create(&model.SoulVersion{
		Version:   "1.0.1",
		Content:   "v1 content",
		UpdatedBy: 1,
		IsActive:  true,
	})

	w, c := newTestContextWithQuery("GET", "/api/v1/system/soul", 1)
	h.GetSoul(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, `"version":"1.0.1"`)
}

// -- UpdateSoul Tests -----------------------------------------

func TestUpdateSoul_Success(t *testing.T) {
	h, soulPath := setupSoulTestDB(t)

	body := `{"content":"# Updated Soul\nNew rules.","reason":"testing"}`
	w, c := newTestContext("PUT", "/api/v1/system/soul", body, 1)
	h.UpdateSoul(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, `"status":"ok"`)
	assertBodyContains(t, w, `"version"`)

	// Verify file was updated
	data, err := os.ReadFile(soulPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(data) != "# Updated Soul\nNew rules." {
		t.Errorf("file content = %q, want %q", string(data), "# Updated Soul\nNew rules.")
	}

	// Verify version record in DB
	var version model.SoulVersion
	h.db.Where("is_active = ?", true).First(&version)
	if version.Content != "# Updated Soul\nNew rules." {
		t.Errorf("DB content = %q", version.Content)
	}
	if version.Reason != "testing" {
		t.Errorf("DB reason = %q, want %q", version.Reason, "testing")
	}
	if version.UpdatedBy != 1 {
		t.Errorf("DB updated_by = %d, want 1", version.UpdatedBy)
	}
}

func TestUpdateSoul_MissingContent(t *testing.T) {
	h, _ := setupSoulTestDB(t)

	body := `{"reason":"no content"}`
	w, c := newTestContext("PUT", "/api/v1/system/soul", body, 1)
	h.UpdateSoul(c)

	assertStatus(t, w, http.StatusBadRequest)
	assertBodyContains(t, w, "请求格式错误")
}

func TestUpdateSoul_InvalidJSON(t *testing.T) {
	h, _ := setupSoulTestDB(t)

	w, c := newTestContext("PUT", "/api/v1/system/soul", "not json", 1)
	h.UpdateSoul(c)

	assertStatus(t, w, http.StatusBadRequest)
	assertBodyContains(t, w, "请求格式错误")
}

func TestUpdateSoul_CreatesBackup(t *testing.T) {
	h, soulPath := setupSoulTestDB(t)

	body := `{"content":"new content"}`
	w, c := newTestContext("PUT", "/api/v1/system/soul", body, 1)
	h.UpdateSoul(c)

	assertStatus(t, w, http.StatusOK)

	// Check that a backup file was created in the same directory
	dir := filepath.Dir(soulPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	backupFound := false
	for _, entry := range entries {
		if entry.Name() != "soul.md" && len(entry.Name()) > 7 {
			backupFound = true
			break
		}
	}
	if !backupFound {
		t.Error("expected backup file to be created")
	}
}

func TestUpdateSoul_DeactivatesPreviousVersion(t *testing.T) {
	h, _ := setupSoulTestDB(t)

	// Seed an active version
	h.db.Create(&model.SoulVersion{
		Version:   "1.0.0",
		Content:   "old",
		UpdatedBy: 1,
		IsActive:  true,
	})

	body := `{"content":"new content"}`
	w, c := newTestContext("PUT", "/api/v1/system/soul", body, 1)
	h.UpdateSoul(c)

	assertStatus(t, w, http.StatusOK)

	// Old version should be deactivated
	var old model.SoulVersion
	h.db.Where("version = ?", "1.0.0").First(&old)
	if old.IsActive {
		t.Error("old version should be deactivated")
	}
}

// -- GetHistory Tests -----------------------------------------

func TestGetHistory_Empty(t *testing.T) {
	h, _ := setupSoulTestDB(t)

	w, c := newTestContextWithQuery("GET", "/api/v1/system/soul/history", 1)
	h.GetHistory(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, `"versions"`)
}

func TestGetHistory_ReturnsVersions(t *testing.T) {
	h, _ := setupSoulTestDB(t)

	h.db.Create(&model.SoulVersion{Version: "1.0.1", Content: "c1", UpdatedBy: 1})
	h.db.Create(&model.SoulVersion{Version: "1.0.2", Content: "c2", UpdatedBy: 1})

	w, c := newTestContextWithQuery("GET", "/api/v1/system/soul/history", 1)
	h.GetHistory(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, "1.0.1")
	assertBodyContains(t, w, "1.0.2")
}

func TestGetHistory_LimitTo10(t *testing.T) {
	h, _ := setupSoulTestDB(t)

	// Seed 15 versions
	for i := 0; i < 15; i++ {
		h.db.Create(&model.SoulVersion{
			Version:   "1.0." + itoa(i),
			Content:   "content " + itoa(i),
			UpdatedBy: 1,
		})
	}

	w, c := newTestContextWithQuery("GET", "/api/v1/system/soul/history", 1)
	h.GetHistory(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, `"versions"`)
}

// -- Rollback Tests -------------------------------------------

func TestRollback_Success(t *testing.T) {
	h, soulPath := setupSoulTestDB(t)

	// Create a version to rollback to
	v := model.SoulVersion{Version: "1.0.0", Content: "rollback content", UpdatedBy: 1}
	h.db.Create(&v)

	body := `{"version_id":` + itoa(int(v.ID)) + `}`
	w, c := newTestContext("POST", "/api/v1/system/soul/rollback", body, 1)
	h.Rollback(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, `"status":"ok"`)
	assertBodyContains(t, w, `"version":"1.0.0"`)

	// Verify file was restored
	data, err := os.ReadFile(soulPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(data) != "rollback content" {
		t.Errorf("file content = %q, want %q", string(data), "rollback content")
	}

	// Verify version is now active
	var updated model.SoulVersion
	h.db.First(&updated, v.ID)
	if !updated.IsActive {
		t.Error("rolled-back version should be active")
	}
}

func TestRollback_VersionNotFound(t *testing.T) {
	h, _ := setupSoulTestDB(t)

	body := `{"version_id":99999}`
	w, c := newTestContext("POST", "/api/v1/system/soul/rollback", body, 1)
	h.Rollback(c)

	assertStatus(t, w, http.StatusNotFound)
	assertBodyContains(t, w, "版本不存在")
}

func TestRollback_InvalidJSON(t *testing.T) {
	h, _ := setupSoulTestDB(t)

	w, c := newTestContext("POST", "/api/v1/system/soul/rollback", "not json", 1)
	h.Rollback(c)

	assertStatus(t, w, http.StatusBadRequest)
	assertBodyContains(t, w, "请求格式错误")
}

func TestRollback_MissingVersionID(t *testing.T) {
	h, _ := setupSoulTestDB(t)

	w, c := newTestContext("POST", "/api/v1/system/soul/rollback", `{}`, 1)
	h.Rollback(c)

	assertStatus(t, w, http.StatusBadRequest)
	assertBodyContains(t, w, "请求格式错误")
}

// -- Evolve Tests ---------------------------------------------

func TestEvolve_ReturnsImmediately(t *testing.T) {
	h, _ := setupSoulTestDB(t)

	w, c := newTestContextWithQuery("POST", "/api/v1/system/soul/evolve", 1)
	h.Evolve(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, "evolution_analysis_started")
}
