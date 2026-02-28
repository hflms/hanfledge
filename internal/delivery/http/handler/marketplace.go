package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/domain/model"
	"gorm.io/gorm"
)

// MarketplaceHandler handles plugin marketplace operations.
type MarketplaceHandler struct {
	DB *gorm.DB
}

// NewMarketplaceHandler creates a new MarketplaceHandler.
func NewMarketplaceHandler(db *gorm.DB) *MarketplaceHandler {
	return &MarketplaceHandler{DB: db}
}

// ListPlugins returns all approved marketplace plugins.
// GET /api/v1/marketplace/plugins?type=skill&category=diagnosis&page=1&limit=20
func (h *MarketplaceHandler) ListPlugins(c *gin.Context) {
	query := h.DB.Where("status = ?", "approved")

	if t := c.Query("type"); t != "" {
		query = query.Where("type = ?", t)
	}
	if cat := c.Query("category"); cat != "" {
		query = query.Where("category = ?", cat)
	}
	if search := c.Query("q"); search != "" {
		query = query.Where("name LIKE ? OR description LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	// Pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	var total int64
	query.Model(&model.MarketplacePlugin{}).Count(&total)

	var plugins []model.MarketplacePlugin
	if err := query.Order("downloads DESC").Offset(offset).Limit(limit).Find(&plugins).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询插件列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"plugins": plugins,
		"total":   total,
		"page":    page,
		"limit":   limit,
	})
}

// GetPlugin returns details of a specific marketplace plugin.
// GET /api/v1/marketplace/plugins/:plugin_id
func (h *MarketplaceHandler) GetPlugin(c *gin.Context) {
	pluginID := c.Param("plugin_id")

	var plugin model.MarketplacePlugin
	if err := h.DB.Where("plugin_id = ?", pluginID).First(&plugin).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "插件不存在"})
		return
	}

	// Get reviews
	var reviews []model.MarketplaceReview
	h.DB.Where("plugin_id = ?", pluginID).Order("created_at DESC").Limit(10).Find(&reviews)

	c.JSON(http.StatusOK, gin.H{
		"plugin":  plugin,
		"reviews": reviews,
	})
}

// SubmitPlugin submits a new plugin to the marketplace.
// POST /api/v1/marketplace/plugins
func (h *MarketplaceHandler) SubmitPlugin(c *gin.Context) {
	var plugin model.MarketplacePlugin
	if err := c.ShouldBindJSON(&plugin); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求数据格式错误: " + err.Error()})
		return
	}

	plugin.Status = "pending"
	plugin.TrustLevel = "community" // All marketplace submissions are community trust

	if err := h.DB.Create(&plugin).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "提交插件失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "插件已提交，等待审核",
		"plugin":  plugin,
	})
}

// InstallPlugin installs a marketplace plugin for a school.
// POST /api/v1/marketplace/install
func (h *MarketplaceHandler) InstallPlugin(c *gin.Context) {
	var req struct {
		SchoolID uint   `json:"school_id" binding:"required"`
		PluginID string `json:"plugin_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求数据格式错误"})
		return
	}

	// Verify plugin exists and is approved
	var plugin model.MarketplacePlugin
	if err := h.DB.Where("plugin_id = ? AND status = ?", req.PluginID, "approved").First(&plugin).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "插件不存在或未通过审核"})
		return
	}

	// Check if already installed
	var existing model.InstalledPlugin
	if err := h.DB.Where("school_id = ? AND plugin_id = ?", req.SchoolID, req.PluginID).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "该插件已安装"})
		return
	}

	installed := model.InstalledPlugin{
		SchoolID: req.SchoolID,
		PluginID: req.PluginID,
		Version:  plugin.Version,
		Enabled:  true,
	}

	if err := h.DB.Create(&installed).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "安装插件失败"})
		return
	}

	// Increment download count
	h.DB.Model(&plugin).Update("downloads", gorm.Expr("downloads + 1"))

	c.JSON(http.StatusOK, gin.H{
		"message":   "插件安装成功",
		"installed": installed,
	})
}

// UninstallPlugin removes an installed plugin from a school.
// DELETE /api/v1/marketplace/installed/:id
func (h *MarketplaceHandler) UninstallPlugin(c *gin.Context) {
	id := c.Param("id")

	if err := h.DB.Delete(&model.InstalledPlugin{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "卸载插件失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "插件已卸载"})
}

// ListInstalled returns installed plugins for a school.
// GET /api/v1/marketplace/installed?school_id=X
func (h *MarketplaceHandler) ListInstalled(c *gin.Context) {
	schoolID := c.Query("school_id")
	if schoolID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 school_id"})
		return
	}

	var installed []model.InstalledPlugin
	if err := h.DB.Where("school_id = ?", schoolID).Find(&installed).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询已安装插件失败"})
		return
	}

	c.JSON(http.StatusOK, installed)
}
