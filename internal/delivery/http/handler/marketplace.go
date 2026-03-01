package handler

import (
	"net/http"
	"strings"

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
//
//	@Summary      插件列表
//	@Description  返回已审核通过的插件列表（支持分页、类型/分类/关键词筛选）
//	@Tags         Marketplace
//	@Produce      json
//	@Param        type      query     string  false  "插件类型（如 skill）"
//	@Param        category  query     string  false  "插件分类（如 diagnosis）"
//	@Param        q         query     string  false  "搜索关键词"
//	@Param        page      query     int     false  "页码"   default(1)
//	@Param        limit     query     int     false  "每页数量" default(20)
//	@Success      200       {object}  PaginatedResponse
//	@Failure      500       {object}  ErrorResponse
//	@Router       /marketplace/plugins [get]
func (h *MarketplaceHandler) ListPlugins(c *gin.Context) {
	query := h.DB.Where("status = ?", "approved")

	if t := c.Query("type"); t != "" {
		query = query.Where("type = ?", t)
	}
	if cat := c.Query("category"); cat != "" {
		query = query.Where("category = ?", cat)
	}
	if search := c.Query("q"); search != "" {
		escaped := escapeLike(search)
		query = query.Where("name LIKE ? OR description LIKE ?", "%"+escaped+"%", "%"+escaped+"%")
	}

	// Pagination
	p := ParsePagination(c)

	var total int64
	query.Model(&model.MarketplacePlugin{}).Count(&total)

	var plugins []model.MarketplacePlugin
	if err := query.Order("downloads DESC").Offset(p.Offset).Limit(p.Limit).Find(&plugins).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询插件列表失败"})
		return
	}

	c.JSON(http.StatusOK, NewPaginatedResponse(plugins, total, p))
}

// GetPlugin returns details of a specific marketplace plugin.
//
//	@Summary      插件详情
//	@Description  返回指定插件的详细信息及最近 10 条评价
//	@Tags         Marketplace
//	@Produce      json
//	@Param        plugin_id  path      string  true  "插件 ID"
//	@Success      200        {object}  map[string]interface{}
//	@Failure      404        {object}  ErrorResponse
//	@Router       /marketplace/plugins/{plugin_id} [get]
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
//
//	@Summary      提交插件
//	@Description  提交新插件到市场，初始状态为待审核
//	@Tags         Marketplace
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        body  body      model.MarketplacePlugin  true  "插件信息"
//	@Success      201   {object}  map[string]interface{}
//	@Failure      400   {object}  ErrorResponse
//	@Failure      500   {object}  ErrorResponse
//	@Router       /marketplace/plugins [post]
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
//
//	@Summary      安装插件
//	@Description  为指定学校安装已审核通过的插件
//	@Tags         Marketplace
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        body  body      object  true  "安装参数（school_id, plugin_id）"
//	@Success      200   {object}  map[string]interface{}
//	@Failure      400   {object}  ErrorResponse
//	@Failure      404   {object}  ErrorResponse
//	@Failure      409   {object}  ErrorResponse
//	@Failure      500   {object}  ErrorResponse
//	@Router       /marketplace/install [post]
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
//
//	@Summary      卸载插件
//	@Description  从学校移除已安装的插件
//	@Tags         Marketplace
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id  path      int  true  "已安装插件记录 ID"
//	@Success      200 {object}  map[string]string
//	@Failure      500 {object}  ErrorResponse
//	@Router       /marketplace/installed/{id} [delete]
func (h *MarketplaceHandler) UninstallPlugin(c *gin.Context) {
	id := c.Param("id")

	if err := h.DB.Delete(&model.InstalledPlugin{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "卸载插件失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "插件已卸载"})
}

// ListInstalled returns installed plugins for a school.
//
//	@Summary      已安装插件列表
//	@Description  返回指定学校已安装的所有插件
//	@Tags         Marketplace
//	@Produce      json
//	@Security     BearerAuth
//	@Param        school_id  query     int  true  "学校 ID"
//	@Success      200        {array}   model.InstalledPlugin
//	@Failure      400        {object}  ErrorResponse
//	@Failure      500        {object}  ErrorResponse
//	@Router       /marketplace/installed [get]
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

// -- Helpers --------------------------------------------------

// escapeLike escapes SQL LIKE wildcards (%, _) in user input to prevent
// unintended pattern matching.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}
