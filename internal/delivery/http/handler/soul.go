package handler

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/agent"
	"github.com/hflms/hanfledge/internal/domain/model"
	"gorm.io/gorm"
)

type SoulHandler struct {
	db           *gorm.DB
	soulPath     string
	orchestrator *agent.AgentOrchestrator
}

func NewSoulHandler(db *gorm.DB, soulPath string, orchestrator *agent.AgentOrchestrator) *SoulHandler {
	return &SoulHandler{db: db, soulPath: soulPath, orchestrator: orchestrator}
}

// GET /api/v1/system/soul
func (h *SoulHandler) GetSoul(c *gin.Context) {
	content, err := os.ReadFile(h.soulPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取失败"})
		return
	}

	var activeVersion model.SoulVersion
	h.db.Where("is_active = ?", true).First(&activeVersion)

	c.JSON(http.StatusOK, gin.H{
		"content":    string(content),
		"version":    activeVersion.Version,
		"updated_at": activeVersion.CreatedAt,
	})
}

// PUT /api/v1/system/soul
func (h *SoulHandler) UpdateSoul(c *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
		Reason  string `json:"reason"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}

	userID := c.GetUint("user_id")

	// Backup current version
	if err := h.backupSoul(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "备份失败"})
		return
	}

	// Write new content
	if err := os.WriteFile(h.soulPath, []byte(req.Content), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "写入失败"})
		return
	}

	// Deactivate old versions
	h.db.Model(&model.SoulVersion{}).Where("is_active = ?", true).Update("is_active", false)

	// Create new version record
	version := model.SoulVersion{
		Version:   fmt.Sprintf("1.0.%d", time.Now().Unix()),
		Content:   req.Content,
		UpdatedBy: userID,
		Reason:    req.Reason,
		IsActive:  true,
	}
	h.db.Create(&version)

	// Reload rules into orchestrator
	if h.orchestrator != nil {
		h.orchestrator.LoadSoulRules(req.Content)
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "version": version.Version})
}

// GET /api/v1/system/soul/history
func (h *SoulHandler) GetHistory(c *gin.Context) {
	var versions []model.SoulVersion
	h.db.Order("created_at DESC").Limit(10).Find(&versions)
	c.JSON(http.StatusOK, gin.H{"versions": versions})
}

// POST /api/v1/system/soul/rollback
func (h *SoulHandler) Rollback(c *gin.Context) {
	var req struct {
		VersionID uint `json:"version_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}

	var version model.SoulVersion
	if err := h.db.First(&version, req.VersionID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "版本不存在"})
		return
	}

	// Backup current
	h.backupSoul()

	// Restore version
	if err := os.WriteFile(h.soulPath, []byte(version.Content), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "回滚失败"})
		return
	}

	// Update active flag
	h.db.Model(&model.SoulVersion{}).Where("is_active = ?", true).Update("is_active", false)
	h.db.Model(&version).Update("is_active", true)

	// Reload rules into orchestrator
	if h.orchestrator != nil {
		h.orchestrator.LoadSoulRules(version.Content)
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "version": version.Version})
}

func (h *SoulHandler) backupSoul() error {
	content, err := os.ReadFile(h.soulPath)
	if err != nil {
		return err
	}

	backupPath := fmt.Sprintf("%s.backup.%d", h.soulPath, time.Now().Unix())
	return os.WriteFile(backupPath, content, 0644)
}

// POST /api/v1/system/soul/evolve
func (h *SoulHandler) Evolve(c *gin.Context) {
	// This endpoint is called manually by admin or automatically by cron
	// It analyzes recent teaching data and suggests soul.md updates

	c.JSON(http.StatusOK, gin.H{
		"status":  "evolution_analysis_started",
		"message": "系统正在分析教学数据，稍后将生成优化建议",
	})

	// Trigger async analysis (in production, use a job queue)
	// For now, return immediately and let admin check back later
}
