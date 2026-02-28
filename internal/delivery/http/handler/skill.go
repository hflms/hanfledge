package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/plugin"
	"gorm.io/gorm"
)

// SkillHandler handles skill-related requests (Skill Store + mounting).
type SkillHandler struct {
	DB       *gorm.DB
	Registry *plugin.Registry
}

// NewSkillHandler creates a new SkillHandler.
func NewSkillHandler(db *gorm.DB, registry *plugin.Registry) *SkillHandler {
	return &SkillHandler{DB: db, Registry: registry}
}

// ── Skill Store ─────────────────────────────────────────────

// ListSkills returns all registered skills from the Plugin Registry.
// Supports filtering by subject and category.
// GET /api/v1/skills?subject=math&category=inquiry-based
func (h *SkillHandler) ListSkills(c *gin.Context) {
	subject := c.Query("subject")
	category := c.Query("category")

	skills := h.Registry.ListSkills(subject, category)

	// Return metadata only (not internal paths)
	result := make([]plugin.SkillMetadata, 0, len(skills))
	for _, s := range skills {
		result = append(result, s.Metadata)
	}

	c.JSON(http.StatusOK, result)
}

// GetSkillDetail returns the full detail of a skill including constraints.
// GET /api/v1/skills/:id
func (h *SkillHandler) GetSkillDetail(c *gin.Context) {
	skillID := c.Param("id")

	skill, ok := h.Registry.GetSkill(skillID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "技能不存在"})
		return
	}

	// Load constraints (SKILL.md)
	constraints, _ := h.Registry.LoadConstraints(skillID)

	c.JSON(http.StatusOK, gin.H{
		"metadata":    skill.Metadata,
		"constraints": constraints,
	})
}

// ── Skill Mounting ──────────────────────────────────────────

// MountSkillRequest represents the request body for mounting a skill to a knowledge point.
type MountSkillRequest struct {
	SkillID         string                 `json:"skill_id" binding:"required"`
	ScaffoldLevel   model.ScaffoldLevel    `json:"scaffold_level"`
	ConstraintsJSON map[string]interface{} `json:"constraints_json,omitempty"`
	Priority        int                    `json:"priority"`
	ProgressiveRule map[string]interface{} `json:"progressive_rule,omitempty"`
}

// MountSkill mounts a skill to a knowledge point under the given chapter.
// POST /api/v1/chapters/:id/skills
func (h *SkillHandler) MountSkill(c *gin.Context) {
	chapterID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的章节 ID"})
		return
	}

	var req MountSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求数据格式错误: " + err.Error()})
		return
	}

	// Verify the skill exists in registry
	if _, ok := h.Registry.GetSkill(req.SkillID); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "技能不存在: " + req.SkillID})
		return
	}

	// Verify the chapter exists and get its knowledge points
	var chapter model.Chapter
	if err := h.DB.Preload("KnowledgePoints").First(&chapter, chapterID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "章节不存在"})
		return
	}

	if len(chapter.KnowledgePoints) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该章节下没有知识点"})
		return
	}

	// Default scaffold level
	if req.ScaffoldLevel == "" {
		req.ScaffoldLevel = model.ScaffoldHigh
	}

	// Mount skill to all knowledge points in this chapter
	var mounts []model.KPSkillMount
	for _, kp := range chapter.KnowledgePoints {
		// Check if already mounted
		var existing model.KPSkillMount
		if err := h.DB.Where("kp_id = ? AND skill_id = ?", kp.ID, req.SkillID).First(&existing).Error; err == nil {
			continue // Skip if already mounted
		}

		constraintsJSON := "{}"
		if req.ConstraintsJSON != nil {
			if data, err := marshalJSON(req.ConstraintsJSON); err == nil {
				constraintsJSON = string(data)
			}
		}

		var progressiveRule *string
		if req.ProgressiveRule != nil {
			if data, err := marshalJSON(req.ProgressiveRule); err == nil {
				s := string(data)
				progressiveRule = &s
			}
		}

		mount := model.KPSkillMount{
			KPID:            kp.ID,
			SkillID:         req.SkillID,
			ScaffoldLevel:   req.ScaffoldLevel,
			ConstraintsJSON: constraintsJSON,
			Priority:        req.Priority,
			ProgressiveRule: progressiveRule,
		}

		if err := h.DB.Create(&mount).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "挂载技能失败: " + err.Error()})
			return
		}
		mounts = append(mounts, mount)
	}

	if len(mounts) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "该技能已挂载到所有知识点"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "技能挂载成功",
		"count":   len(mounts),
		"mounts":  mounts,
	})
}

// UnmountSkill removes a skill mount from a knowledge point.
// DELETE /api/v1/chapters/:id/skills/:mount_id
func (h *SkillHandler) UnmountSkill(c *gin.Context) {
	chapterID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的章节 ID"})
		return
	}

	mountID, err := strconv.ParseUint(c.Param("mount_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的挂载 ID"})
		return
	}

	// Verify the chapter exists
	var chapter model.Chapter
	if err := h.DB.First(&chapter, chapterID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "章节不存在"})
		return
	}

	// Find the mount and verify it belongs to a KP in this chapter
	var mount model.KPSkillMount
	if err := h.DB.First(&mount, mountID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "挂载记录不存在"})
		return
	}

	// Verify KP belongs to the chapter
	var kp model.KnowledgePoint
	if err := h.DB.First(&kp, mount.KPID).Error; err != nil || kp.ChapterID != uint(chapterID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "该挂载不属于此章节"})
		return
	}

	if err := h.DB.Delete(&mount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "卸载技能失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "技能已卸载"})
}

// ── Skill Config ────────────────────────────────────────────

// UpdateSkillConfigRequest represents the request body for updating skill mount config.
type UpdateSkillConfigRequest struct {
	ScaffoldLevel   *model.ScaffoldLevel   `json:"scaffold_level,omitempty"`
	ProgressiveRule map[string]interface{} `json:"progressive_rule,omitempty"`
}

// UpdateSkillConfig updates the scaffold level and progressive rule of a skill mount.
// PATCH /api/v1/chapters/:id/skills/:mount_id
func (h *SkillHandler) UpdateSkillConfig(c *gin.Context) {
	chapterID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的章节 ID"})
		return
	}

	mountID, err := strconv.ParseUint(c.Param("mount_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的挂载 ID"})
		return
	}

	var req UpdateSkillConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求数据格式错误: " + err.Error()})
		return
	}

	// Verify the chapter exists
	var chapter model.Chapter
	if err := h.DB.First(&chapter, chapterID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "章节不存在"})
		return
	}

	// Find the mount
	var mount model.KPSkillMount
	if err := h.DB.First(&mount, mountID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "挂载记录不存在"})
		return
	}

	// Verify KP belongs to the chapter
	var kp model.KnowledgePoint
	if err := h.DB.First(&kp, mount.KPID).Error; err != nil || kp.ChapterID != uint(chapterID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "该挂载不属于此章节"})
		return
	}

	// Build update map
	updates := map[string]interface{}{}

	if req.ScaffoldLevel != nil {
		level := *req.ScaffoldLevel
		if level != model.ScaffoldHigh && level != model.ScaffoldMedium && level != model.ScaffoldLow {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的支架等级，可选: high, medium, low"})
			return
		}
		updates["scaffold_level"] = level
	}

	if req.ProgressiveRule != nil {
		data, err := marshalJSON(req.ProgressiveRule)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "渐进规则格式错误"})
			return
		}
		s := string(data)
		updates["progressive_rule"] = s
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "没有需要更新的字段"})
		return
	}

	if err := h.DB.Model(&mount).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新配置失败: " + err.Error()})
		return
	}

	// Reload mount to return fresh data
	h.DB.First(&mount, mountID)

	c.JSON(http.StatusOK, gin.H{
		"message": "配置已更新",
		"mount":   mount,
	})
}

// marshalJSON is a helper to marshal map to JSON bytes.
func marshalJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
