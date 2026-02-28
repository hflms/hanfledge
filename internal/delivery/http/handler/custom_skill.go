package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/plugin"
	"gorm.io/gorm"
)

// CustomSkillHandler handles teacher-created custom skill CRUD operations.
// Reference: design.md §6.4 — 教师自定义 Skill 工作流
type CustomSkillHandler struct {
	DB       *gorm.DB
	Registry *plugin.Registry
}

// NewCustomSkillHandler creates a new CustomSkillHandler.
func NewCustomSkillHandler(db *gorm.DB, registry *plugin.Registry) *CustomSkillHandler {
	return &CustomSkillHandler{DB: db, Registry: registry}
}

// ── Request / Response Types ────────────────────────────────

// CreateCustomSkillRequest is the payload for creating a new custom skill.
type CreateCustomSkillRequest struct {
	SkillID     string                       `json:"skill_id" binding:"required"`
	Name        string                       `json:"name" binding:"required"`
	Description string                       `json:"description"`
	Category    string                       `json:"category"`
	Subjects    []string                     `json:"subjects"`
	Tags        []string                     `json:"tags"`
	SkillMD     string                       `json:"skill_md" binding:"required"`
	ToolsConfig map[string]plugin.ToolConfig `json:"tools_config,omitempty"`
	Templates   []CustomSkillTemplate        `json:"templates,omitempty"`
}

// UpdateCustomSkillRequest is the payload for updating a custom skill.
type UpdateCustomSkillRequest struct {
	Name        *string                      `json:"name,omitempty"`
	Description *string                      `json:"description,omitempty"`
	Category    *string                      `json:"category,omitempty"`
	Subjects    []string                     `json:"subjects,omitempty"`
	Tags        []string                     `json:"tags,omitempty"`
	SkillMD     *string                      `json:"skill_md,omitempty"`
	ToolsConfig map[string]plugin.ToolConfig `json:"tools_config,omitempty"`
	Templates   []CustomSkillTemplate        `json:"templates,omitempty"`
	ChangeLog   string                       `json:"change_log,omitempty"`
}

// ShareCustomSkillRequest is the payload for sharing a custom skill.
type ShareCustomSkillRequest struct {
	Visibility model.CustomSkillVisibility `json:"visibility" binding:"required"`
}

// CustomSkillTemplate represents a template entry in the JSON array.
type CustomSkillTemplate struct {
	ID       string `json:"id"`
	FileName string `json:"file_name"`
	Content  string `json:"content"`
}

// ── Create ──────────────────────────────────────────────────

// CreateCustomSkill creates a new custom skill in draft status.
// The skill_id must follow three-segment namespace: {subject}_{scenario}_{method}
// POST /api/v1/custom-skills
func (h *CustomSkillHandler) CreateCustomSkill(c *gin.Context) {
	userID, _ := c.Get("user_id")
	teacherID := userID.(uint)

	var req CreateCustomSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求数据格式错误: " + err.Error()})
		return
	}

	// Validate skill ID namespace (three-segment)
	validator := plugin.DefaultSkillValidator()
	if err := validator.ValidateNamespaceOnly(req.SkillID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "技能 ID 格式错误: " + err.Error()})
		return
	}

	// Check for duplicate skill ID (both registry and DB)
	if _, ok := h.Registry.GetSkill(req.SkillID); ok {
		c.JSON(http.StatusConflict, gin.H{"error": "技能 ID 已存在: " + req.SkillID})
		return
	}
	var existingCount int64
	h.DB.Model(&model.CustomSkill{}).Where("skill_id = ?", req.SkillID).Count(&existingCount)
	if existingCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "技能 ID 已存在: " + req.SkillID})
		return
	}

	// Validate SKILL.md token count
	tokens := estimateTokens(req.SkillMD)
	if tokens > 2000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("SKILL.md 内容超出 Token 限制: %d（上限 2000）", tokens)})
		return
	}

	// Marshal JSON fields
	subjectsJSON, _ := json.Marshal(req.Subjects)
	tagsJSON, _ := json.Marshal(req.Tags)
	toolsJSON, _ := json.Marshal(req.ToolsConfig)
	templatesJSON, _ := json.Marshal(req.Templates)

	// Get teacher's school ID
	var schoolID uint
	var role model.UserSchoolRole
	if err := h.DB.Where("user_id = ?", teacherID).First(&role).Error; err == nil && role.SchoolID != nil {
		schoolID = *role.SchoolID
	}

	now := time.Now().Format(time.RFC3339)
	skill := model.CustomSkill{
		SkillID:     req.SkillID,
		TeacherID:   teacherID,
		SchoolID:    schoolID,
		Name:        req.Name,
		Description: req.Description,
		Category:    req.Category,
		Subjects:    string(subjectsJSON),
		Tags:        string(tagsJSON),
		SkillMD:     req.SkillMD,
		ToolsConfig: string(toolsJSON),
		Templates:   string(templatesJSON),
		Status:      model.CustomSkillStatusDraft,
		Visibility:  model.VisibilityPrivate,
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.DB.Create(&skill).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建技能失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, skill)
}

// ── List ────────────────────────────────────────────────────

// ListCustomSkills lists custom skills for the current teacher.
// Supports query filters: status, visibility
// GET /api/v1/custom-skills?status=draft&visibility=private
func (h *CustomSkillHandler) ListCustomSkills(c *gin.Context) {
	userID, _ := c.Get("user_id")
	teacherID := userID.(uint)

	query := h.DB.Where("teacher_id = ?", teacherID)

	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if visibility := c.Query("visibility"); visibility != "" {
		query = query.Where("visibility = ?", visibility)
	}

	var skills []model.CustomSkill
	if err := query.Order("updated_at DESC").Find(&skills).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询技能列表失败"})
		return
	}

	c.JSON(http.StatusOK, skills)
}

// ── Get Detail ──────────────────────────────────────────────

// GetCustomSkill returns the detail of a custom skill by ID.
// GET /api/v1/custom-skills/:id
func (h *CustomSkillHandler) GetCustomSkill(c *gin.Context) {
	userID, _ := c.Get("user_id")
	teacherID := userID.(uint)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的技能 ID"})
		return
	}

	var skill model.CustomSkill
	if err := h.DB.First(&skill, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "技能不存在"})
		return
	}

	// Only owner or shared skills are accessible
	if skill.TeacherID != teacherID && skill.Visibility == model.VisibilityPrivate {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问此技能"})
		return
	}

	c.JSON(http.StatusOK, skill)
}

// ── Update ──────────────────────────────────────────────────

// UpdateCustomSkill updates a custom skill. If the skill is published,
// the current version is saved to version history before updating.
// PUT /api/v1/custom-skills/:id
func (h *CustomSkillHandler) UpdateCustomSkill(c *gin.Context) {
	userID, _ := c.Get("user_id")
	teacherID := userID.(uint)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的技能 ID"})
		return
	}

	var skill model.CustomSkill
	if err := h.DB.First(&skill, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "技能不存在"})
		return
	}

	// Only the owner can update
	if skill.TeacherID != teacherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权修改此技能"})
		return
	}

	var req UpdateCustomSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求数据格式错误: " + err.Error()})
		return
	}

	// Validate SKILL.md token count if being updated
	if req.SkillMD != nil {
		tokens := estimateTokens(*req.SkillMD)
		if tokens > 2000 {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("SKILL.md 内容超出 Token 限制: %d（上限 2000）", tokens)})
			return
		}
	}

	// If skill is published/shared, save current version to history before update
	if skill.Status == model.CustomSkillStatusPublished || skill.Status == model.CustomSkillStatusShared {
		version := model.CustomSkillVersion{
			CustomSkillID: skill.ID,
			Version:       skill.Version,
			SkillMD:       skill.SkillMD,
			ToolsConfig:   skill.ToolsConfig,
			Templates:     skill.Templates,
			ChangeLog:     req.ChangeLog,
			CreatedAt:     time.Now().Format(time.RFC3339),
		}
		if err := h.DB.Create(&version).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存版本历史失败: " + err.Error()})
			return
		}
		skill.Version++
	}

	// Apply updates
	if req.Name != nil {
		skill.Name = *req.Name
	}
	if req.Description != nil {
		skill.Description = *req.Description
	}
	if req.Category != nil {
		skill.Category = *req.Category
	}
	if req.Subjects != nil {
		data, _ := json.Marshal(req.Subjects)
		skill.Subjects = string(data)
	}
	if req.Tags != nil {
		data, _ := json.Marshal(req.Tags)
		skill.Tags = string(data)
	}
	if req.SkillMD != nil {
		skill.SkillMD = *req.SkillMD
	}
	if req.ToolsConfig != nil {
		data, _ := json.Marshal(req.ToolsConfig)
		skill.ToolsConfig = string(data)
	}
	if req.Templates != nil {
		data, _ := json.Marshal(req.Templates)
		skill.Templates = string(data)
	}

	skill.UpdatedAt = time.Now().Format(time.RFC3339)

	if err := h.DB.Save(&skill).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新技能失败: " + err.Error()})
		return
	}

	// If published/shared, update the registry with new content
	if skill.Status == model.CustomSkillStatusPublished || skill.Status == model.CustomSkillStatusShared {
		h.registerSkillInRegistry(&skill)
	}

	c.JSON(http.StatusOK, skill)
}

// ── Delete ──────────────────────────────────────────────────

// DeleteCustomSkill deletes a custom skill. Only draft skills can be deleted;
// published skills must be archived first.
// DELETE /api/v1/custom-skills/:id
func (h *CustomSkillHandler) DeleteCustomSkill(c *gin.Context) {
	userID, _ := c.Get("user_id")
	teacherID := userID.(uint)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的技能 ID"})
		return
	}

	var skill model.CustomSkill
	if err := h.DB.First(&skill, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "技能不存在"})
		return
	}

	// Only the owner can delete
	if skill.TeacherID != teacherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权删除此技能"})
		return
	}

	// Only draft or archived skills can be hard-deleted
	if skill.Status != model.CustomSkillStatusDraft && skill.Status != model.CustomSkillStatusArchived {
		c.JSON(http.StatusBadRequest, gin.H{"error": "已发布的技能不能直接删除，请先归档"})
		return
	}

	// Delete version history
	h.DB.Where("custom_skill_id = ?", skill.ID).Delete(&model.CustomSkillVersion{})

	// Unregister from registry (if it was registered)
	h.Registry.UnregisterSkill(skill.SkillID)

	if err := h.DB.Delete(&skill).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除技能失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "技能已删除"})
}

// ── Publish ─────────────────────────────────────────────────

// PublishCustomSkill publishes a draft custom skill, registering it in the
// Plugin Registry so it can be mounted to knowledge points.
// POST /api/v1/custom-skills/:id/publish
func (h *CustomSkillHandler) PublishCustomSkill(c *gin.Context) {
	userID, _ := c.Get("user_id")
	teacherID := userID.(uint)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的技能 ID"})
		return
	}

	var skill model.CustomSkill
	if err := h.DB.First(&skill, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "技能不存在"})
		return
	}

	if skill.TeacherID != teacherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权发布此技能"})
		return
	}

	if skill.Status != model.CustomSkillStatusDraft {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只有草稿状态的技能可以发布"})
		return
	}

	// Validate SKILL.md content exists
	if skill.SkillMD == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "技能约束内容（SKILL.md）不能为空"})
		return
	}

	// Register in Plugin Registry
	h.registerSkillInRegistry(&skill)

	// Update DB status
	skill.Status = model.CustomSkillStatusPublished
	skill.UpdatedAt = time.Now().Format(time.RFC3339)
	if err := h.DB.Save(&skill).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "发布技能失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "技能已发布",
		"skill":   skill,
	})
}

// ── Share ───────────────────────────────────────────────────

// ShareCustomSkill changes the visibility of a published skill to school or
// platform level, allowing other teachers to discover and use it.
// POST /api/v1/custom-skills/:id/share
func (h *CustomSkillHandler) ShareCustomSkill(c *gin.Context) {
	userID, _ := c.Get("user_id")
	teacherID := userID.(uint)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的技能 ID"})
		return
	}

	var req ShareCustomSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求数据格式错误: " + err.Error()})
		return
	}

	// Validate visibility value
	if req.Visibility != model.VisibilitySchool && req.Visibility != model.VisibilityPlatform {
		c.JSON(http.StatusBadRequest, gin.H{"error": "可见范围只能设为 school 或 platform"})
		return
	}

	var skill model.CustomSkill
	if err := h.DB.First(&skill, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "技能不存在"})
		return
	}

	if skill.TeacherID != teacherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权分享此技能"})
		return
	}

	// Must be published before sharing
	if skill.Status != model.CustomSkillStatusPublished && skill.Status != model.CustomSkillStatusShared {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只有已发布的技能才能分享"})
		return
	}

	skill.Status = model.CustomSkillStatusShared
	skill.Visibility = req.Visibility
	skill.UpdatedAt = time.Now().Format(time.RFC3339)

	if err := h.DB.Save(&skill).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "分享技能失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "技能已分享",
		"skill":   skill,
	})
}

// ── Archive ─────────────────────────────────────────────────

// ArchiveCustomSkill archives a published skill, removing it from the
// Registry but preserving it in the database for version history.
// POST /api/v1/custom-skills/:id/archive
func (h *CustomSkillHandler) ArchiveCustomSkill(c *gin.Context) {
	userID, _ := c.Get("user_id")
	teacherID := userID.(uint)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的技能 ID"})
		return
	}

	var skill model.CustomSkill
	if err := h.DB.First(&skill, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "技能不存在"})
		return
	}

	if skill.TeacherID != teacherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权归档此技能"})
		return
	}

	if skill.Status == model.CustomSkillStatusArchived {
		c.JSON(http.StatusBadRequest, gin.H{"error": "技能已经是归档状态"})
		return
	}

	// Unregister from Plugin Registry
	h.Registry.UnregisterSkill(skill.SkillID)

	skill.Status = model.CustomSkillStatusArchived
	skill.UpdatedAt = time.Now().Format(time.RFC3339)

	if err := h.DB.Save(&skill).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "归档技能失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "技能已归档",
		"skill":   skill,
	})
}

// ── Version History ─────────────────────────────────────────

// ListVersions returns the version history for a custom skill.
// GET /api/v1/custom-skills/:id/versions
func (h *CustomSkillHandler) ListVersions(c *gin.Context) {
	userID, _ := c.Get("user_id")
	teacherID := userID.(uint)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的技能 ID"})
		return
	}

	var skill model.CustomSkill
	if err := h.DB.First(&skill, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "技能不存在"})
		return
	}

	if skill.TeacherID != teacherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权查看此技能的版本历史"})
		return
	}

	var versions []model.CustomSkillVersion
	if err := h.DB.Where("custom_skill_id = ?", id).Order("version DESC").Find(&versions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询版本历史失败"})
		return
	}

	c.JSON(http.StatusOK, versions)
}

// ── Internal Helpers ────────────────────────────────────────

// registerSkillInRegistry creates a SkillMetadata from a CustomSkill and
// registers it in the Plugin Registry so it can be used by the agent pipeline.
func (h *CustomSkillHandler) registerSkillInRegistry(skill *model.CustomSkill) {
	var subjects []string
	_ = json.Unmarshal([]byte(skill.Subjects), &subjects)

	var tags []string
	_ = json.Unmarshal([]byte(skill.Tags), &tags)

	meta := plugin.SkillMetadata{
		ID:          skill.SkillID,
		Name:        skill.Name,
		Description: skill.Description,
		Version:     fmt.Sprintf("v%d", skill.Version),
		Author:      fmt.Sprintf("teacher:%d", skill.TeacherID),
		Category:    skill.Category,
		Subjects:    subjects,
		Tags:        tags,
	}

	// Parse tools config into metadata
	var tools map[string]plugin.ToolConfig
	if err := json.Unmarshal([]byte(skill.ToolsConfig), &tools); err == nil {
		meta.Tools = tools
	}

	h.Registry.RegisterCustomSkill(meta, skill.SkillMD)
}

// estimateTokens provides a rough token count for SKILL.md content.
// Chinese ≈ 1.5 chars/token, English ≈ 4 chars/token.
func estimateTokens(text string) int {
	runes := []rune(text)
	chineseCount := 0
	englishCount := 0
	for _, r := range runes {
		if r > 127 {
			chineseCount++
		} else {
			englishCount++
		}
	}
	return int(float64(chineseCount)/1.5) + int(float64(englishCount)/4.0) + 1
}
