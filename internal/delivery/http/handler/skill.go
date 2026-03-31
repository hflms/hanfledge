package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/llm"
	"github.com/hflms/hanfledge/internal/plugin"
	"gorm.io/gorm"
)

// SkillHandler handles skill-related requests (Skill Store + mounting).
type SkillHandler struct {
	DB          *gorm.DB
	Registry    *plugin.Registry
	LLMProvider llm.LLMProvider
}

// NewSkillHandler creates a new SkillHandler.
func NewSkillHandler(db *gorm.DB, registry *plugin.Registry, llmProvider llm.LLMProvider) *SkillHandler {
	return &SkillHandler{DB: db, Registry: registry, LLMProvider: llmProvider}
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

// ── AI Auto-Mount ───────────────────────────────────────────

// RecommendMount represents an AI recommended skill mount.
type RecommendMount struct {
	KPID          uint                `json:"kp_id"`
	KPTitle       string              `json:"kp_title"`
	SkillID       string              `json:"skill_id"`
	SkillName     string              `json:"skill_name"`
	ScaffoldLevel model.ScaffoldLevel `json:"scaffold_level"`
	Reason        string              `json:"reason"`
}

type simpleKP struct {
	ID    uint   `json:"id"`
	Title string `json:"title"`
}

type simpleSkill struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Desc string `json:"description"`
}

func buildSkillRecommendationPrompt(courseTitle string, kps []simpleKP, available []simpleSkill) string {
	kpJSON, _ := json.Marshal(kps)
	skillJSON, _ := json.Marshal(available)

	return fmt.Sprintf(`You are an AI assistant helping a teacher design a course.
Here are the Knowledge Points (KPs) for the course "%s":
%s

Here are the available pedagogical skills:
%s

Please recommend up to 10 skill mounts for these knowledge points. Focus on key points.
Output ONLY a valid JSON array of objects with the following keys:
- "kp_id": integer
- "skill_id": string
- "scaffold_level": string (one of "high", "medium", "low")
- "reason": string (why this skill fits this KP)

JSON output:`, courseTitle, string(kpJSON), string(skillJSON))
}

func parseSkillRecommendations(rawJSON string) ([]RecommendMount, error) {
	start := strings.Index(rawJSON, "[")
	end := strings.LastIndex(rawJSON, "]")
	if start != -1 && end != -1 && end > start {
		rawJSON = rawJSON[start : end+1]
	}

	var mounts []RecommendMount
	if err := json.Unmarshal([]byte(rawJSON), &mounts); err != nil {
		return nil, err
	}
	return mounts, nil
}

// RecommendSkills uses AI to recommend skill mappings for a course.
// POST /api/v1/courses/:id/skills/recommend
func (h *SkillHandler) RecommendSkills(c *gin.Context) {
	courseID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的课程 ID"})
		return
	}

	// 1. Fetch Course and Knowledge Points
	var course model.Course
	if err := h.DB.Preload("Chapters.KnowledgePoints").First(&course, courseID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "课程不存在"})
		return
	}

	// 2. Fetch available skills
	skills := h.Registry.ListSkills(course.Subject, "")
	if len(skills) == 0 {
		skills = h.Registry.ListSkills("", "") // fallback to all if none match subject
	}

	// 3. Build prompt for AI
	var kps []simpleKP
	for _, ch := range course.Chapters {
		for _, kp := range ch.KnowledgePoints {
			kps = append(kps, simpleKP{ID: kp.ID, Title: kp.Title})
		}
	}

	if len(kps) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "课程下没有知识点，请先上传教材生成大纲"})
		return
	}

	var available []simpleSkill
	skillMap := make(map[string]string)
	for _, s := range skills {
		available = append(available, simpleSkill{ID: s.Metadata.ID, Name: s.Metadata.Name, Desc: s.Metadata.Description})
		skillMap[s.Metadata.ID] = s.Metadata.Name
	}

	prompt := buildSkillRecommendationPrompt(course.Title, kps, available)

	// 4. Call LLM
	resp, err := h.LLMProvider.Chat(c.Request.Context(), []llm.ChatMessage{
		{Role: "system", Content: "You are a helpful educational AI. You respond ONLY with valid JSON, without markdown formatting like ```json or ```."},
		{Role: "user", Content: prompt},
	}, &llm.ChatOptions{Temperature: 0.2})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI 分析失败: " + err.Error()})
		return
	}

	// 5. Parse response
	mounts, err := parseSkillRecommendations(resp)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析 AI 响应失败: " + err.Error()})
		return
	}

	// 6. Enrich with names
	kpNameMap := make(map[uint]string)
	for _, kp := range kps {
		kpNameMap[kp.ID] = kp.Title
	}

	for i := range mounts {
		mounts[i].KPTitle = kpNameMap[mounts[i].KPID]
		mounts[i].SkillName = skillMap[mounts[i].SkillID]
	}

	c.JSON(http.StatusOK, gin.H{"recommendations": mounts})
}

// BatchMountRequest represents a list of skill mounts.
type BatchMountRequest struct {
	Mounts []struct {
		KPID          uint                `json:"kp_id" binding:"required"`
		SkillID       string              `json:"skill_id" binding:"required"`
		ScaffoldLevel model.ScaffoldLevel `json:"scaffold_level"`
	} `json:"mounts" binding:"required"`
}

// BatchMountSkills applies a batch of skill mounts to a course's knowledge points.
// POST /api/v1/courses/:id/skills/batch-mount
func (h *SkillHandler) BatchMountSkills(c *gin.Context) {
	courseID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的课程 ID"})
		return
	}

	var req BatchMountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求数据格式错误: " + err.Error()})
		return
	}

	var savedCount int

	err = h.DB.Transaction(func(tx *gorm.DB) error {
		for _, m := range req.Mounts {
			// Ensure KP belongs to the course
			var count int64
			if err := tx.Model(&model.KnowledgePoint{}).
				Joins("JOIN chapters ON chapters.id = knowledge_points.chapter_id").
				Where("knowledge_points.id = ? AND chapters.course_id = ?", m.KPID, courseID).
				Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				continue // Skip if KP is not in this course
			}

			level := m.ScaffoldLevel
			if level == "" {
				level = model.ScaffoldHigh
			}

			// Upsert logic (if exists, skip or update. Let's just create if not exists)
			var existing model.KPSkillMount
			if err := tx.Where("kp_id = ? AND skill_id = ?", m.KPID, m.SkillID).First(&existing).Error; err == nil {
				// Already mounted, skip
				continue
			}

			newMount := model.KPSkillMount{
				KPID:            m.KPID,
				SkillID:         m.SkillID,
				ScaffoldLevel:   level,
				ConstraintsJSON: "{}",
			}
			if err := tx.Create(&newMount).Error; err != nil {
				return err
			}
			savedCount++
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "批量挂载失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "批量挂载成功", "count": savedCount})
}
