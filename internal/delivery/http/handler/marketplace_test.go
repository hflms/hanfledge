package handler

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/domain/model"
)

// ============================
// Marketplace Handler Unit Tests
// ============================

// -- Setup Helpers --------------------------------------------

func setupMarketplaceDB(t *testing.T) *MarketplaceHandler {
	t.Helper()
	db := setupTestDB(t)

	// Migrate marketplace tables
	if err := db.AutoMigrate(
		&model.MarketplacePlugin{},
		&model.MarketplaceReview{},
		&model.InstalledPlugin{},
	); err != nil {
		t.Fatalf("AutoMigrate marketplace tables failed: %v", err)
	}

	return NewMarketplaceHandler(db)
}

func seedMarketplacePlugin(t *testing.T, h *MarketplaceHandler, pluginID, name, status, category, pluginType string) model.MarketplacePlugin {
	t.Helper()
	p := model.MarketplacePlugin{
		PluginID:    pluginID,
		Name:        name,
		Description: "Test plugin: " + name,
		Version:     "1.0.0",
		Author:      "Test Author",
		Type:        pluginType,
		TrustLevel:  "community",
		Category:    category,
		Status:      status,
		Downloads:   0,
	}
	if err := h.DB.Create(&p).Error; err != nil {
		t.Fatalf("seedMarketplacePlugin failed: %v", err)
	}
	return p
}

// -- Constructor Tests ----------------------------------------

func TestNewMarketplaceHandler(t *testing.T) {
	h := NewMarketplaceHandler(nil)
	if h == nil {
		t.Fatal("NewMarketplaceHandler returned nil")
	}
}

// -- ListPlugins Tests ----------------------------------------

func TestMarketplace_ListPlugins_Empty(t *testing.T) {
	h := setupMarketplaceDB(t)

	w, c := newTestContextWithQuery(http.MethodGet, "/api/v1/marketplace/plugins", 0)
	h.ListPlugins(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, `"total":0`)
}

func TestMarketplace_ListPlugins_OnlyApproved(t *testing.T) {
	h := setupMarketplaceDB(t)

	seedMarketplacePlugin(t, h, "approved-1", "Approved Plugin", "approved", "diagnosis", "skill")
	seedMarketplacePlugin(t, h, "pending-1", "Pending Plugin", "pending", "diagnosis", "skill")
	seedMarketplacePlugin(t, h, "rejected-1", "Rejected Plugin", "rejected", "diagnosis", "skill")

	w, c := newTestContextWithQuery(http.MethodGet, "/api/v1/marketplace/plugins", 0)
	h.ListPlugins(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, `"total":1`)
	assertBodyContains(t, w, "approved-1")
	assertBodyNotContains(t, w, "pending-1")
	assertBodyNotContains(t, w, "rejected-1")
}

func TestMarketplace_ListPlugins_FilterByType(t *testing.T) {
	h := setupMarketplaceDB(t)

	seedMarketplacePlugin(t, h, "skill-1", "Skill Plugin", "approved", "diagnosis", "skill")
	seedMarketplacePlugin(t, h, "theme-1", "Theme Plugin", "approved", "dark", "theme")

	w, c := newTestContextWithQuery(http.MethodGet, "/api/v1/marketplace/plugins?type=skill", 0)
	h.ListPlugins(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, "skill-1")
	assertBodyNotContains(t, w, "theme-1")
}

func TestMarketplace_ListPlugins_FilterByCategory(t *testing.T) {
	h := setupMarketplaceDB(t)

	seedMarketplacePlugin(t, h, "diag-1", "Diagnosis", "approved", "diagnosis", "skill")
	seedMarketplacePlugin(t, h, "socratic-1", "Socratic", "approved", "socratic", "skill")

	w, c := newTestContextWithQuery(http.MethodGet, "/api/v1/marketplace/plugins?category=diagnosis", 0)
	h.ListPlugins(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, "diag-1")
	assertBodyNotContains(t, w, "socratic-1")
}

func TestMarketplace_ListPlugins_Search(t *testing.T) {
	h := setupMarketplaceDB(t)

	seedMarketplacePlugin(t, h, "math-skill", "Math Helper", "approved", "socratic", "skill")
	seedMarketplacePlugin(t, h, "phys-skill", "Physics Helper", "approved", "socratic", "skill")

	w, c := newTestContextWithQuery(http.MethodGet, "/api/v1/marketplace/plugins?q=Math", 0)
	h.ListPlugins(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, "math-skill")
	assertBodyNotContains(t, w, "phys-skill")
}

func TestMarketplace_ListPlugins_Pagination(t *testing.T) {
	h := setupMarketplaceDB(t)

	// Create 3 approved plugins
	seedMarketplacePlugin(t, h, "p1", "Plugin 1", "approved", "cat", "skill")
	seedMarketplacePlugin(t, h, "p2", "Plugin 2", "approved", "cat", "skill")
	seedMarketplacePlugin(t, h, "p3", "Plugin 3", "approved", "cat", "skill")

	// Page 1, limit 2
	w, c := newTestContextWithQuery(http.MethodGet, "/api/v1/marketplace/plugins?page=1&limit=2", 0)
	h.ListPlugins(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, `"total":3`)
	assertBodyContains(t, w, `"limit":2`)
}

// -- GetPlugin Tests ------------------------------------------

func TestMarketplace_GetPlugin_Found(t *testing.T) {
	h := setupMarketplaceDB(t)

	seedMarketplacePlugin(t, h, "get-test", "Get Test Plugin", "approved", "diagnosis", "skill")

	w, c := newTestContextWithParams(http.MethodGet, "/api/v1/marketplace/plugins/get-test", "", 0,
		gin.Params{{Key: "plugin_id", Value: "get-test"}})
	h.GetPlugin(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, "get-test")
	assertBodyContains(t, w, "Get Test Plugin")
	assertBodyContains(t, w, "reviews")
}

func TestMarketplace_GetPlugin_NotFound(t *testing.T) {
	h := setupMarketplaceDB(t)

	w, c := newTestContextWithParams(http.MethodGet, "/api/v1/marketplace/plugins/nonexistent", "", 0,
		gin.Params{{Key: "plugin_id", Value: "nonexistent"}})
	h.GetPlugin(c)

	assertStatus(t, w, http.StatusNotFound)
	assertBodyContains(t, w, "插件不存在")
}

// -- SubmitPlugin Tests ---------------------------------------

func TestMarketplace_SubmitPlugin_Success(t *testing.T) {
	h := setupMarketplaceDB(t)

	body := `{
		"plugin_id": "new-plugin",
		"name": "New Plugin",
		"description": "A test plugin",
		"version": "1.0.0",
		"type": "skill",
		"category": "socratic"
	}`

	w, c := newTestContext(http.MethodPost, "/api/v1/marketplace/plugins", body, 1)
	h.SubmitPlugin(c)

	assertStatus(t, w, http.StatusCreated)
	assertBodyContains(t, w, "插件已提交")
	assertBodyContains(t, w, "new-plugin")
	// Status should be set to pending
	assertBodyContains(t, w, "pending")
	// Trust level should be community
	assertBodyContains(t, w, "community")
}

func TestMarketplace_SubmitPlugin_InvalidJSON(t *testing.T) {
	h := setupMarketplaceDB(t)

	w, c := newTestContext(http.MethodPost, "/api/v1/marketplace/plugins", "not json", 1)
	h.SubmitPlugin(c)

	assertStatus(t, w, http.StatusBadRequest)
	assertBodyContains(t, w, "请求数据格式错误")
}

// -- InstallPlugin Tests --------------------------------------

func TestMarketplace_InstallPlugin_Success(t *testing.T) {
	h := setupMarketplaceDB(t)

	seedMarketplacePlugin(t, h, "install-me", "Install Me", "approved", "diagnosis", "skill")

	body := `{"school_id": 1, "plugin_id": "install-me"}`
	w, c := newTestContext(http.MethodPost, "/api/v1/marketplace/install", body, 1)
	h.InstallPlugin(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, "插件安装成功")
	assertBodyContains(t, w, "install-me")
}

func TestMarketplace_InstallPlugin_NotApproved(t *testing.T) {
	h := setupMarketplaceDB(t)

	seedMarketplacePlugin(t, h, "pending-plugin", "Pending", "pending", "diagnosis", "skill")

	body := `{"school_id": 1, "plugin_id": "pending-plugin"}`
	w, c := newTestContext(http.MethodPost, "/api/v1/marketplace/install", body, 1)
	h.InstallPlugin(c)

	assertStatus(t, w, http.StatusNotFound)
	assertBodyContains(t, w, "插件不存在或未通过审核")
}

func TestMarketplace_InstallPlugin_AlreadyInstalled(t *testing.T) {
	h := setupMarketplaceDB(t)

	seedMarketplacePlugin(t, h, "dup-install", "Dup Plugin", "approved", "diagnosis", "skill")

	// First install
	body := `{"school_id": 1, "plugin_id": "dup-install"}`
	w1, c1 := newTestContext(http.MethodPost, "/api/v1/marketplace/install", body, 1)
	h.InstallPlugin(c1)
	assertStatus(t, w1, http.StatusOK)

	// Second install should conflict
	w2, c2 := newTestContext(http.MethodPost, "/api/v1/marketplace/install", body, 1)
	h.InstallPlugin(c2)
	assertStatus(t, w2, http.StatusConflict)
	assertBodyContains(t, w2, "该插件已安装")
}

func TestMarketplace_InstallPlugin_MissingFields(t *testing.T) {
	h := setupMarketplaceDB(t)

	body := `{"school_id": 1}`
	w, c := newTestContext(http.MethodPost, "/api/v1/marketplace/install", body, 1)
	h.InstallPlugin(c)

	assertStatus(t, w, http.StatusBadRequest)
	assertBodyContains(t, w, "请求数据格式错误")
}

func TestMarketplace_InstallPlugin_IncrementsDownloads(t *testing.T) {
	h := setupMarketplaceDB(t)

	p := seedMarketplacePlugin(t, h, "dl-test", "DL Test", "approved", "diagnosis", "skill")

	body := `{"school_id": 1, "plugin_id": "dl-test"}`
	w, c := newTestContext(http.MethodPost, "/api/v1/marketplace/install", body, 1)
	h.InstallPlugin(c)
	assertStatus(t, w, http.StatusOK)

	// Check download count incremented
	var updated model.MarketplacePlugin
	h.DB.First(&updated, p.ID)
	if updated.Downloads != 1 {
		t.Errorf("Downloads = %d, want 1", updated.Downloads)
	}
}

// -- UninstallPlugin Tests ------------------------------------

func TestMarketplace_UninstallPlugin_Success(t *testing.T) {
	h := setupMarketplaceDB(t)

	// Create an installed plugin record
	installed := model.InstalledPlugin{
		SchoolID: 1,
		PluginID: "uninstall-me",
		Version:  "1.0.0",
		Enabled:  true,
	}
	h.DB.Create(&installed)

	w, c := newTestContextWithParams(http.MethodDelete, "/api/v1/marketplace/installed/1", "", 1,
		gin.Params{{Key: "id", Value: "1"}})
	h.UninstallPlugin(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, "插件已卸载")
}

// -- ListInstalled Tests --------------------------------------

func TestMarketplace_ListInstalled_Success(t *testing.T) {
	h := setupMarketplaceDB(t)

	// Install two plugins for school 1
	h.DB.Create(&model.InstalledPlugin{SchoolID: 1, PluginID: "plugin-a", Version: "1.0", Enabled: true})
	h.DB.Create(&model.InstalledPlugin{SchoolID: 1, PluginID: "plugin-b", Version: "1.0", Enabled: true})
	// Different school
	h.DB.Create(&model.InstalledPlugin{SchoolID: 2, PluginID: "plugin-c", Version: "1.0", Enabled: true})

	w, c := newTestContextWithQuery(http.MethodGet, "/api/v1/marketplace/installed?school_id=1", 1)
	h.ListInstalled(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, "plugin-a")
	assertBodyContains(t, w, "plugin-b")
	assertBodyNotContains(t, w, "plugin-c")
}

func TestMarketplace_ListInstalled_MissingSchoolID(t *testing.T) {
	h := setupMarketplaceDB(t)

	w, c := newTestContextWithQuery(http.MethodGet, "/api/v1/marketplace/installed", 1)
	h.ListInstalled(c)

	assertStatus(t, w, http.StatusBadRequest)
	assertBodyContains(t, w, "请提供 school_id")
}

func TestMarketplace_ListInstalled_Empty(t *testing.T) {
	h := setupMarketplaceDB(t)

	w, c := newTestContextWithQuery(http.MethodGet, "/api/v1/marketplace/installed?school_id=999", 1)
	h.ListInstalled(c)

	assertStatus(t, w, http.StatusOK)
	// Empty array
	assertBodyContains(t, w, "[]")
}
