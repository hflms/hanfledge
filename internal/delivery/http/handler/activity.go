package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hflms/hanfledge/internal/agent"
	"github.com/hflms/hanfledge/internal/delivery/http/middleware"
	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/cache"
	"github.com/hflms/hanfledge/internal/infrastructure/llm"
	"github.com/hflms/hanfledge/internal/infrastructure/logger"
	"github.com/hflms/hanfledge/internal/infrastructure/storage"
	"github.com/hflms/hanfledge/internal/plugin"
	"gorm.io/gorm"
)

var slogActivity = logger.L("Activity")

// ============================
// 学习活动 Handler — Phase 4
// ============================

// ActivityHandler handles learning activity CRUD and session management.
type ActivityHandler struct {
	DB           *gorm.DB
	Orchestrator *agent.AgentOrchestrator
	EventBus     *plugin.EventBus
	Registry     *plugin.Registry
	Storage      storage.FileStorage
	LLMProvider  llm.LLMProvider
	Cache        *cache.RedisCache // Redis cache for session history invalidation
}

// NewActivityHandler creates a new ActivityHandler.
func NewActivityHandler(db *gorm.DB, orchestrator *agent.AgentOrchestrator, eventBus *plugin.EventBus, registry *plugin.Registry, fs storage.FileStorage, llmProvider llm.LLMProvider, redisCache *cache.RedisCache) *ActivityHandler {
	return &ActivityHandler{DB: db, Orchestrator: orchestrator, EventBus: eventBus, Registry: registry, Storage: fs, LLMProvider: llmProvider, Cache: redisCache}
}

// publishEvent fires an EventBus event if the bus is available (nil-safe).
func (h *ActivityHandler) publishEvent(ctx context.Context, hook plugin.HookPoint, payload map[string]interface{}) {
	plugin.PublishEvent(h.EventBus, ctx, hook, payload)
}

// ── Teacher: Activity CRUD ──────────────────────────────────

// CreateActivityRequest 创建学习活动请求。
type CreateActivityRequest struct {
	CourseID       uint                   `json:"course_id" binding:"required"`
	Title          string                 `json:"title" binding:"required"`
	Type           model.ActivityType     `json:"type,omitempty"`
	DesignerID     string                 `json:"designer_id,omitempty"`
	DesignerConfig map[string]interface{} `json:"designer_config,omitempty"`
	StepsConfig    []interface{}          `json:"steps_config,omitempty"`
	KPIDS          []uint                 `json:"kp_ids" binding:"required"`
	SkillConfig    map[string]interface{} `json:"skill_config,omitempty"`
	Deadline       *string                `json:"deadline,omitempty"`
	AllowRetry     *bool                  `json:"allow_retry,omitempty"`
	MaxAttempts    *int                   `json:"max_attempts,omitempty"`
	ClassIDs       []uint                 `json:"class_ids,omitempty"`
}

// CreateActivity creates a new learning activity.
//
//	@Summary      创建学习活动
//	@Description  教师创建新的学习活动，可指定知识点、技能配置、截止日期和班级分配
//	@Tags         Activities
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        body  body      CreateActivityRequest  true  "活动创建参数"
//	@Success      201   {object}  model.LearningActivity
//	@Failure      400   {object}  ErrorResponse
//	@Failure      500   {object}  ErrorResponse
//	@Router       /activities [post]
func (h *ActivityHandler) CreateActivity(c *gin.Context) {
	var req CreateActivityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	teacherID := middleware.GetUserID(c)

	// Serialize JSON fields
	kpIDsJSON, err := json.Marshal(req.KPIDS)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "知识点 ID 序列化失败"})
		return
	}
	skillConfigJSON := "{}"
	if req.SkillConfig != nil {
		data, err := json.Marshal(req.SkillConfig)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "技能配置序列化失败"})
			return
		}
		skillConfigJSON = string(data)
	}

	designerConfigJSON := "{}"
	if req.DesignerConfig != nil {
		data, err := json.Marshal(req.DesignerConfig)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "设计者配置序列化失败"})
			return
		}
		designerConfigJSON = string(data)
	}

	stepsConfigJSON := "[]"
	if req.StepsConfig != nil {
		data, err := json.Marshal(req.StepsConfig)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "环节配置序列化失败"})
			return
		}
		stepsConfigJSON = string(data)
	}

	activityType := req.Type
	if activityType == "" {
		activityType = model.ActivityTypeAutonomous
	}

	activity := model.LearningActivity{
		CourseID:       req.CourseID,
		TeacherID:      teacherID,
		Title:          req.Title,
		Type:           activityType,
		DesignerID:     req.DesignerID,
		DesignerConfig: designerConfigJSON,
		StepsConfig:    stepsConfigJSON,
		KPIDS:          string(kpIDsJSON),
		SkillConfig:    skillConfigJSON,
		Deadline:       req.Deadline,
		Status:         model.ActivityStatusDraft,
		CreatedAt:      time.Now().Format(time.RFC3339),
	}

	if req.AllowRetry != nil {
		activity.AllowRetry = *req.AllowRetry
	}
	if req.MaxAttempts != nil {
		activity.MaxAttempts = *req.MaxAttempts
	}

	if err := h.DB.Create(&activity).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建学习活动失败"})
		return
	}

	// Assign to classes if provided
	for _, classID := range req.ClassIDs {
		assignment := model.ActivityClassAssignment{
			ActivityID: activity.ID,
			ClassID:    classID,
		}
		h.DB.Create(&assignment)
	}

	c.JSON(http.StatusCreated, activity)
}

// GetActivity returns a single activity with its steps.
//
//	@Summary      获取活动详情
//	@Description  返回指定活动的完整信息，包括环节列表
//	@Tags         Activities
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id  path      int  true  "活动 ID"
//	@Success      200 {object}  model.LearningActivity
//	@Failure      400 {object}  ErrorResponse
//	@Failure      403 {object}  ErrorResponse
//	@Failure      404 {object}  ErrorResponse
//	@Router       /activities/{id} [get]
func (h *ActivityHandler) GetActivity(c *gin.Context) {
	activityID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的活动 ID"})
		return
	}

	teacherID := middleware.GetUserID(c)

	var activity model.LearningActivity
	if err := h.DB.Preload("Steps", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	}).Preload("AssignedClasses").First(&activity, activityID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "学习活动不存在"})
		return
	}

	if activity.TeacherID != teacherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问此活动"})
		return
	}

	c.JSON(http.StatusOK, activity)
}

// UpdateActivityRequest 更新学习活动请求。
type UpdateActivityRequest struct {
	Title          *string                `json:"title,omitempty"`
	Description    *string                `json:"description,omitempty"`
	Type           *model.ActivityType    `json:"type,omitempty"`
	DesignerID     *string                `json:"designer_id,omitempty"`
	DesignerConfig map[string]interface{} `json:"designer_config,omitempty"`
	KPIDS          []uint                 `json:"kp_ids,omitempty"`
	SkillConfig    map[string]interface{} `json:"skill_config,omitempty"`
	Deadline       *string                `json:"deadline,omitempty"`
	AllowRetry     *bool                  `json:"allow_retry,omitempty"`
	MaxAttempts    *int                   `json:"max_attempts,omitempty"`
	ClassIDs       []uint                 `json:"class_ids,omitempty"`
}

// UpdateActivity updates an existing draft activity.
//
//	@Summary      更新学习活动
//	@Description  更新草稿状态的学习活动信息
//	@Tags         Activities
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id    path      int                   true  "活动 ID"
//	@Param        body  body      UpdateActivityRequest true  "更新参数"
//	@Success      200   {object}  model.LearningActivity
//	@Failure      400   {object}  ErrorResponse
//	@Failure      403   {object}  ErrorResponse
//	@Failure      404   {object}  ErrorResponse
//	@Router       /activities/{id} [put]
func (h *ActivityHandler) UpdateActivity(c *gin.Context) {
	activityID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的活动 ID"})
		return
	}

	teacherID := middleware.GetUserID(c)

	var activity model.LearningActivity
	if err := h.DB.First(&activity, activityID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "学习活动不存在"})
		return
	}

	if activity.TeacherID != teacherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权操作此活动"})
		return
	}

	if activity.Status != model.ActivityStatusDraft {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只有草稿状态的活动可以编辑"})
		return
	}

	var req UpdateActivityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	updates := map[string]interface{}{
		"updated_at": time.Now().Format(time.RFC3339),
	}

	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Type != nil {
		updates["type"] = *req.Type
	}
	if req.DesignerID != nil {
		updates["designer_id"] = *req.DesignerID
	}
	if req.DesignerConfig != nil {
		data, _ := json.Marshal(req.DesignerConfig)
		updates["designer_config"] = string(data)
	}
	if req.KPIDS != nil {
		data, _ := json.Marshal(req.KPIDS)
		updates["kp_ids"] = string(data)
	}
	if req.SkillConfig != nil {
		data, _ := json.Marshal(req.SkillConfig)
		updates["skill_config"] = string(data)
	}
	if req.Deadline != nil {
		updates["deadline"] = *req.Deadline
	}
	if req.AllowRetry != nil {
		updates["allow_retry"] = *req.AllowRetry
	}
	if req.MaxAttempts != nil {
		updates["max_attempts"] = *req.MaxAttempts
	}

	if err := h.DB.Model(&activity).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新学习活动失败"})
		return
	}

	// Update class assignments if provided
	if req.ClassIDs != nil {
		h.DB.Where("activity_id = ?", activityID).Delete(&model.ActivityClassAssignment{})
		for _, classID := range req.ClassIDs {
			h.DB.Create(&model.ActivityClassAssignment{
				ActivityID: uint(activityID),
				ClassID:    classID,
			})
		}
	}

	// Reload with associations
	h.DB.Preload("Steps", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	}).Preload("AssignedClasses").First(&activity, activityID)

	c.JSON(http.StatusOK, activity)
}

// ── Activity Steps CRUD ─────────────────────────────────────

// SaveStepsRequest 批量保存环节请求。
type SaveStepsRequest struct {
	Steps []StepData `json:"steps" binding:"required"`
}

// StepData 单个环节数据。
type StepData struct {
	ID            uint           `json:"id,omitempty"`
	Title         string         `json:"title" binding:"required"`
	Description   string         `json:"description,omitempty"`
	StepType      model.StepType `json:"step_type,omitempty"`
	SortOrder     int            `json:"sort_order"`
	ContentBlocks string         `json:"content_blocks,omitempty"` // JSON string
	Duration      int            `json:"duration,omitempty"`
}

// SaveSteps 批量保存活动环节（全量替换策略）。
// 前端传入完整的环节列表（含排序），后端删旧建新。
//
//	@Summary      批量保存环节
//	@Description  全量保存活动的环节列表，支持排序
//	@Tags         Activities
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id    path  int              true  "活动 ID"
//	@Param        body  body  SaveStepsRequest true  "环节列表"
//	@Success      200   {array}  model.ActivityStep
//	@Router       /activities/{id}/steps [put]
func (h *ActivityHandler) SaveSteps(c *gin.Context) {
	activityID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的活动 ID"})
		return
	}

	teacherID := middleware.GetUserID(c)

	var activity model.LearningActivity
	if err := h.DB.First(&activity, activityID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "学习活动不存在"})
		return
	}

	if activity.TeacherID != teacherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权操作此活动"})
		return
	}

	if activity.Status != model.ActivityStatusDraft {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只有草稿状态的活动可以编辑环节"})
		return
	}

	var req SaveStepsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	now := time.Now().Format(time.RFC3339)

	// Use transaction for atomic replace
	tx := h.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Delete existing steps
	if err := tx.Where("activity_id = ?", activityID).Delete(&model.ActivityStep{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "清除旧环节失败"})
		return
	}

	// Insert new steps
	steps := make([]model.ActivityStep, 0, len(req.Steps))
	for i, s := range req.Steps {
		contentBlocks := s.ContentBlocks
		if contentBlocks == "" {
			contentBlocks = "[]"
		}
		stepType := s.StepType
		if stepType == "" {
			stepType = model.StepTypeLecture
		}
		step := model.ActivityStep{
			ActivityID:    uint(activityID),
			Title:         s.Title,
			Description:   s.Description,
			StepType:      stepType,
			SortOrder:     i, // Enforce sequential ordering from array position
			ContentBlocks: contentBlocks,
			Duration:      s.Duration,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := tx.Create(&step).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存环节失败"})
			return
		}
		steps = append(steps, step)
	}

	tx.Commit()

	c.JSON(http.StatusOK, steps)
}

// @Summary      获取教学设计者列表
// @Description  返回可用的教学设计者风格（苏格拉底式、项目式、精熟式、探究式）
// @Tags         Activities
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}  model.InstructionalDesigner
// @Router       /designers [get]
func (h *ActivityHandler) ListDesigners(c *gin.Context) {
	var designers []model.InstructionalDesigner
	if err := h.DB.Order("is_built_in DESC, name ASC").Find(&designers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取设计者列表失败"})
		return
	}
	c.JSON(http.StatusOK, designers)
}

// ListActivities returns learning activities for a teacher.
//
//	@Summary      教师活动列表
//	@Description  返回当前教师创建的学习活动列表（支持分页和筛选）
//	@Tags         Activities
//	@Produce      json
//	@Security     BearerAuth
//	@Param        course_id  query     int     false  "课程 ID"
//	@Param        status     query     string  false  "活动状态（draft/published）"
//	@Param        page       query     int     false  "页码"   default(1)
//	@Param        limit      query     int     false  "每页数量" default(20)
//	@Success      200        {object}  PaginatedResponse
//	@Failure      500        {object}  ErrorResponse
//	@Router       /activities [get]
func (h *ActivityHandler) ListActivities(c *gin.Context) {
	teacherID := middleware.GetUserID(c)
	p := ParsePagination(c)

	query := h.DB.Where("teacher_id = ?", teacherID)

	if courseID := c.Query("course_id"); courseID != "" {
		query = query.Where("course_id = ?", courseID)
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Model(&model.LearningActivity{}).Count(&total)

	var activities []model.LearningActivity
	if err := query.Preload("AssignedClasses").Order("created_at DESC").
		Offset(p.Offset).Limit(p.Limit).Find(&activities).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询学习活动失败"})
		return
	}

	c.JSON(http.StatusOK, NewPaginatedResponse(activities, total, p))
}

// PublishActivity publishes a learning activity (changes status to published).
//
//	@Summary      发布学习活动
//	@Description  将草稿状态的学习活动发布，使其对学生可见
//	@Tags         Activities
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id  path      int  true  "活动 ID"
//	@Success      200 {object}  map[string]string
//	@Failure      400 {object}  ErrorResponse
//	@Failure      403 {object}  ErrorResponse
//	@Failure      404 {object}  ErrorResponse
//	@Router       /activities/{id}/publish [post]
func (h *ActivityHandler) PublishActivity(c *gin.Context) {
	activityID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的活动 ID"})
		return
	}

	teacherID := middleware.GetUserID(c)

	var activity model.LearningActivity
	if err := h.DB.First(&activity, activityID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "学习活动不存在"})
		return
	}

	if activity.TeacherID != teacherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权操作此活动"})
		return
	}

	if activity.Status != model.ActivityStatusDraft {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只有草稿状态的活动可以发布"})
		return
	}

	now := time.Now().Format(time.RFC3339)
	h.DB.Model(&activity).Updates(map[string]interface{}{
		"status":       model.ActivityStatusPublished,
		"published_at": now,
	})

	// Hook: on activity publish
	h.publishEvent(c.Request.Context(), plugin.HookOnActivityPublish, map[string]interface{}{
		"activity_id": activity.ID,
		"teacher_id":  teacherID,
		"course_id":   activity.CourseID,
		"title":       activity.Title,
	})

	c.JSON(http.StatusOK, gin.H{"message": "活动已发布"})
}

// ── Teacher: Sandbox Preview (design.md §5.1 Step 3) ────────

// PreviewActivity allows a teacher to preview a learning activity in sandbox mode.
// Creates a sandbox session with IsSandbox=true, allowing the teacher to experience
// the activity as a student. Sandbox sessions are excluded from analytics and mastery updates.
//
//	@Summary      沙盒预览活动
//	@Description  教师以学生视角预览学习活动，创建沙盒会话（不计入分析统计）
//	@Tags         Activities
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id  path      int  true  "活动 ID"
//	@Success      200 {object}  map[string]interface{}  "已有沙盒会话"
//	@Success      201 {object}  map[string]interface{}  "新创建沙盒会话"
//	@Failure      400 {object}  ErrorResponse
//	@Failure      403 {object}  ErrorResponse
//	@Failure      404 {object}  ErrorResponse
//	@Failure      500 {object}  ErrorResponse
//	@Router       /activities/{id}/preview [post]
func (h *ActivityHandler) PreviewActivity(c *gin.Context) {
	activityID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的活动 ID"})
		return
	}

	teacherID := middleware.GetUserID(c)

	// Verify activity exists
	var activity model.LearningActivity
	if err := h.DB.First(&activity, activityID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "学习活动不存在"})
		return
	}

	// Verify teacher owns this activity
	if activity.TeacherID != teacherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权预览此活动"})
		return
	}

	// Check for existing active sandbox session (reuse if exists)
	var existingSession model.StudentSession
	err = h.DB.Where("student_id = ? AND activity_id = ? AND is_sandbox = ? AND status = ?",
		teacherID, activityID, true, model.SessionStatusActive).
		First(&existingSession).Error
	if err == nil {
		c.JSON(http.StatusOK, gin.H{
			"message":    "已有进行中的沙盒会话",
			"session_id": existingSession.ID,
			"is_sandbox": true,
		})
		return
	}

	// Parse KP IDs to find first target KP
	var kpIDs []uint
	if err := json.Unmarshal([]byte(activity.KPIDS), &kpIDs); err != nil {
		slogActivity.Warn("failed to parse kp ids for preview", "activity_id", activityID, "err", err)
	}
	firstKP := uint(0)
	if len(kpIDs) > 0 {
		firstKP = kpIDs[0]
	}

	var firstStepID *uint
	if activity.Type == model.ActivityTypeGuided {
		var firstStep model.ActivityStep
		if err := h.DB.Where("activity_id = ?", activityID).Order("sort_order ASC").First(&firstStep).Error; err == nil {
			firstStepID = &firstStep.ID
		}
	}

	// Create sandbox session — StudentID = teacherID (teacher acts as student)
	session := model.StudentSession{
		StudentID:     teacherID,
		ActivityID:    uint(activityID),
		CurrentKP:     firstKP,
		CurrentStepID: firstStepID,
		Scaffold:      model.ScaffoldHigh,
		IsSandbox:     true,
		Status:        model.SessionStatusActive,
		StartedAt:     time.Now(),
	}

	if err := h.DB.Create(&session).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建沙盒会话失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "沙盒预览会话已创建",
		"session_id": session.ID,
		"is_sandbox": true,
	})
}

// ── Student: Activity List & Join ───────────────────────────

// StudentListActivities returns published activities available to a student.
//
//	@Summary      学生活动列表
//	@Description  返回当前学生可参加的已发布学习活动，附带会话参与状态
//	@Tags         Student
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200  {array}  map[string]interface{}
//	@Router       /student/activities [get]
func (h *ActivityHandler) StudentListActivities(c *gin.Context) {
	studentID := middleware.GetUserID(c)

	// Find the student's class IDs
	var classIDs []uint
	h.DB.Raw(`
		SELECT uscr.school_id FROM user_school_roles uscr
		WHERE uscr.user_id = ? AND uscr.role_name = 'STUDENT'
	`, studentID).Scan(&classIDs)

	// Find student's class memberships
	var studentClassIDs []uint
	h.DB.Raw(`
		SELECT class_id FROM class_students WHERE user_id = ?
	`, studentID).Scan(&studentClassIDs)

	// Query published activities assigned to the student's classes
	var activities []model.LearningActivity
	query := h.DB.Where("status = ?", model.ActivityStatusPublished)

	if len(studentClassIDs) > 0 {
		query = query.Where("id IN (SELECT activity_id FROM activity_class_assignments WHERE class_id IN ?)", studentClassIDs)
	}

	query.Order("created_at DESC").Find(&activities)

	// Annotate with session status for this student
	type activityWithStatus struct {
		model.LearningActivity
		HasSession    bool    `json:"has_session"`
		SessionID     *uint   `json:"session_id,omitempty"`
		SessionStatus *string `json:"session_status,omitempty"`
	}

	result := make([]activityWithStatus, len(activities))
	for i, a := range activities {
		result[i].LearningActivity = a

		var session model.StudentSession
		err := h.DB.Where("student_id = ? AND activity_id = ? AND is_sandbox = ?", studentID, a.ID, false).
			First(&session).Error
		if err == nil {
			result[i].HasSession = true
			result[i].SessionID = &session.ID
			status := string(session.Status)
			result[i].SessionStatus = &status
		}
	}

	c.JSON(http.StatusOK, result)
}

// JoinActivity allows a student to join a learning activity (creates a session).
//
//	@Summary      加入学习活动
//	@Description  学生加入指定学习活动，若已有进行中的会话则返回该会话
//	@Tags         Activities
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id  path      int  true  "活动 ID"
//	@Success      200 {object}  map[string]interface{}  "已有进行中会话"
//	@Success      201 {object}  map[string]interface{}  "新创建会话"
//	@Failure      400 {object}  ErrorResponse
//	@Failure      404 {object}  ErrorResponse
//	@Failure      500 {object}  ErrorResponse
//	@Router       /activities/{id}/join [post]
func (h *ActivityHandler) JoinActivity(c *gin.Context) {
	activityID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的活动 ID"})
		return
	}

	studentID := middleware.GetUserID(c)

	// Verify activity exists and is published
	var activity model.LearningActivity
	if err := h.DB.First(&activity, activityID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "学习活动不存在"})
		return
	}

	if activity.Status != model.ActivityStatusPublished {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该活动尚未发布"})
		return
	}

	// Check for existing active session
	var existingSession model.StudentSession
	err = h.DB.Where("student_id = ? AND activity_id = ? AND status = ?",
		studentID, activityID, model.SessionStatusActive).
		First(&existingSession).Error
	if err == nil {
		// Session already exists, return it
		c.JSON(http.StatusOK, gin.H{
			"message":    "已有进行中的会话",
			"session_id": existingSession.ID,
		})
		return
	}

	// Parse KP IDs to find first target KP
	var kpIDs []uint
	if err := json.Unmarshal([]byte(activity.KPIDS), &kpIDs); err != nil {
		slogActivity.Warn("failed to parse kp ids for join", "activity_id", activityID, "err", err)
	}
	firstKP := uint(0)
	if len(kpIDs) > 0 {
		firstKP = kpIDs[0]
	}

	var firstStepID *uint
	if activity.Type == model.ActivityTypeGuided {
		var firstStep model.ActivityStep
		if err := h.DB.Where("activity_id = ?", activityID).Order("sort_order ASC").First(&firstStep).Error; err == nil {
			firstStepID = &firstStep.ID
		}
	}

	// Create new session
	session := model.StudentSession{
		StudentID:     studentID,
		ActivityID:    uint(activityID),
		CurrentKP:     firstKP,
		CurrentStepID: firstStepID,
		Scaffold:      model.ScaffoldHigh, // Start with high scaffold
		Status:        model.SessionStatusActive,
		StartedAt:     time.Now(),
	}

	if err := h.DB.Create(&session).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建学习会话失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "已加入学习活动",
		"session_id": session.ID,
	})
}

// NextGuidedStep advances the session to the next ActivityStep for guided activities.
//
//	@Summary      进入下一个环节 (Guided)
//	@Description  对于 Guided 模式的学习活动，将会话状态推进到下一个 ActivityStep
//	@Tags         Sessions
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id  path      int  true  "会话 ID"
//	@Success      200 {object}  map[string]interface{}
//	@Failure      400 {object}  ErrorResponse
//	@Failure      403 {object}  ErrorResponse
//	@Failure      404 {object}  ErrorResponse
//	@Router       /sessions/{id}/next-step [post]
func (h *ActivityHandler) NextGuidedStep(c *gin.Context) {
	sessionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的会话 ID"})
		return
	}

	studentID := middleware.GetUserID(c)

	var session model.StudentSession
	if err := h.DB.First(&session, "id = ? AND student_id = ?", sessionID, studentID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "会话不存在"})
		return
	}

	var activity model.LearningActivity
	if err := h.DB.Preload("Steps", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	}).First(&activity, session.ActivityID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法加载关联的活动信息"})
		return
	}

	if activity.Type != model.ActivityTypeGuided {
		c.JSON(http.StatusBadRequest, gin.H{"error": "此端点仅适用于引导式学习活动"})
		return
	}

	if len(activity.Steps) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该活动未配置任何学习环节"})
		return
	}

	// 查找当前步骤的索引
	currentIndex := -1
	for i, step := range activity.Steps {
		if session.CurrentStepID != nil && step.ID == *session.CurrentStepID {
			currentIndex = i
			break
		}
	}

	// 推进到下一步
	nextIndex := currentIndex + 1
	if nextIndex >= len(activity.Steps) {
		// 没有下一步，标记会话完成
		session.Status = model.SessionStatusCompleted
		now := time.Now()
		session.EndedAt = &now
		h.DB.Save(&session)

		c.JSON(http.StatusOK, gin.H{
			"message": "活动已完成",
			"status":  "completed",
		})
		return
	}

	nextStep := activity.Steps[nextIndex]

	// 更新 session
	session.CurrentStepID = &nextStep.ID
	session.SkillState = nil // 切换步骤时清除旧状态
	if err := h.DB.Save(&session).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新会话环节失败"})
		return
	}

	// Clear redis history cache
	if h.Cache != nil {
		h.Cache.InvalidateSessionHistory(c.Request.Context(), uint(sessionID))
	}

	c.JSON(http.StatusOK, gin.H{
		"message":         "已进入下一个环节",
		"current_step_id": nextStep.ID,
		"step_title":      nextStep.Title,
	})
}

// GetSession returns session details for a student.
//
//	@Summary      会话详情
//	@Description  返回指定会话的详情及最近 50 条对话记录
//	@Tags         Sessions
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id  path      int  true  "会话 ID"
//	@Success      200 {object}  map[string]interface{}
//	@Failure      400 {object}  ErrorResponse
//	@Failure      403 {object}  ErrorResponse
//	@Failure      404 {object}  ErrorResponse
//	@Router       /sessions/{id} [get]
func (h *ActivityHandler) GetSession(c *gin.Context) {
	sessionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的会话 ID"})
		return
	}

	studentID := middleware.GetUserID(c)

	var session model.StudentSession
	if err := h.DB.First(&session, sessionID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "会话不存在"})
		return
	}

	if session.StudentID != studentID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问此会话"})
		return
	}

	// Load activity details
	var activity model.LearningActivity
	if err := h.DB.First(&activity, session.ActivityID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法加载关联的活动信息"})
		return
	}

	// Load recent interactions
	var interactions []model.Interaction
	h.DB.Where("session_id = ?", sessionID).
		Order("created_at ASC").
		Limit(50).
		Find(&interactions)

	c.JSON(http.StatusOK, gin.H{
		"session":      session,
		"activity":     activity,
		"interactions": interactions,
	})
}

// UpdateSessionStep updates the active skill and current KP for a session.
// PUT /sessions/:id/step
// On step transition, this handler:
//  1. Summarizes the prior step's interactions via LLM
//  2. Saves a StepSummary record for the completed step
//  3. Resets SkillState (clears stale skill session data)
//  4. Invalidates Redis session history cache
//  5. Updates the session's current_kp and active_skill
func (h *ActivityHandler) UpdateSessionStep(c *gin.Context) {
	sessionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的会话 ID"})
		return
	}

	var req struct {
		KPID        uint   `json:"kp_id"`
		ActiveSkill string `json:"active_skill"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求格式"})
		return
	}

	studentID := middleware.GetUserID(c)

	// 1. 加载当前会话，获取旧步骤信息
	var session model.StudentSession
	if err := h.DB.First(&session, "id = ? AND student_id = ?", sessionID, studentID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "会话不存在"})
		return
	}

	oldKPID := session.CurrentKP
	oldSkill := session.ActiveSkill
	summaryText := ""

	// 2. 如果有旧步骤（kp_id != 0），生成步骤摘要
	if oldKPID != 0 {
		summaryText = h.summarizePriorStep(c.Request.Context(), uint(sessionID), oldKPID, oldSkill)
	}

	// 3. 保存 StepSummary 记录
	if summaryText != "" {
		// 获取旧步骤的掌握度
		var mastery model.StudentKPMastery
		masteryEnd := 0.0
		if err := h.DB.Where("student_id = ? AND kp_id = ?", studentID, oldKPID).First(&mastery).Error; err == nil {
			masteryEnd = mastery.MasteryScore
		}

		// 统计旧步骤的交互轮次
		var turnCount int64
		h.DB.Model(&model.Interaction{}).Where("session_id = ? AND kp_id = ? AND role = ?", sessionID, oldKPID, "student").Count(&turnCount)

		// 计算步骤索引
		stepIndex := h.findStepIndex(uint(sessionID), oldKPID)

		stepSummary := model.StepSummary{
			SessionID:    uint(sessionID),
			KPID:         oldKPID,
			StepIndex:    stepIndex,
			Summary:      summaryText,
			MasteryStart: 0, // TODO: track start mastery in future
			MasteryEnd:   masteryEnd,
			TurnCount:    int(turnCount),
			SkillID:      oldSkill,
			CreatedAt:    time.Now().Format(time.RFC3339),
		}
		if err := h.DB.Create(&stepSummary).Error; err != nil {
			slogActivity.Warn("save step summary failed", "session_id", sessionID, "err", err)
		}
	}

	// 4. 重置 SkillState（清除旧步骤的技能会话状态）
	h.DB.Model(&model.StudentSession{}).Where("id = ?", sessionID).Update("skill_state", nil)

	// 5. 失效 Redis 缓存（旧步骤的历史不应在新步骤中被缓存命中）
	if h.Cache != nil {
		if err := h.Cache.InvalidateSessionHistory(c.Request.Context(), uint(sessionID)); err != nil {
			slogActivity.Warn("invalidate session history cache failed", "session_id", sessionID, "err", err)
		}
		if err := h.Cache.InvalidateSessionState(c.Request.Context(), uint(sessionID)); err != nil {
			slogActivity.Warn("invalidate session state cache failed", "session_id", sessionID, "err", err)
		}
	}

	// 6. 更新会话的 current_kp 和 active_skill
	if err := h.DB.Model(&model.StudentSession{}).Where("id = ? AND student_id = ?", sessionID, studentID).
		Updates(map[string]interface{}{
			"current_kp":   req.KPID,
			"active_skill": req.ActiveSkill,
		}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新会话步骤失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "会话步骤更新成功",
		"step_summary": summaryText,
		"old_kp_id":    oldKPID,
		"new_kp_id":    req.KPID,
	})
}

// summarizePriorStep 使用 LLM 为前一步骤的交互生成学习摘要。
func (h *ActivityHandler) summarizePriorStep(ctx context.Context, sessionID, kpID uint, skillID string) string {
	// 查询该步骤的所有交互记录
	var interactions []model.Interaction
	h.DB.Where("session_id = ? AND kp_id = ?", sessionID, kpID).
		Order("created_at ASC").
		Limit(30). // 限制上下文长度
		Find(&interactions)

	if len(interactions) == 0 {
		// 回退: 如果没有带 kp_id 的记录（旧数据），用全部交互
		h.DB.Where("session_id = ?", sessionID).
			Order("created_at ASC").
			Limit(20).
			Find(&interactions)
	}

	if len(interactions) < 2 {
		return ""
	}

	// 构建对话文本
	var sb strings.Builder
	for _, ix := range interactions {
		sb.WriteString(fmt.Sprintf("[%s]: %s\n", ix.Role, ix.Content))
	}

	prompt := fmt.Sprintf(`请用2-3句话概括以下学习对话中学生的学习成果和主要收获。
重点关注：学生理解了什么、还存在什么困难、掌握了哪些关键概念。
技能类型: %s

对话内容:
%s

学习摘要:`, skillID, sb.String())

	if h.LLMProvider == nil {
		return ""
	}

	summary, err := h.LLMProvider.Chat(ctx, []llm.ChatMessage{
		{Role: "user", Content: prompt},
	}, &llm.ChatOptions{MaxTokens: 200, Temperature: 0.3})
	if err != nil {
		slogActivity.Warn("step summary LLM call failed", "session_id", sessionID, "err", err)
		return ""
	}

	return strings.TrimSpace(summary)
}

// findStepIndex 查找知识点在活动 KP 序列中的位置。
func (h *ActivityHandler) findStepIndex(sessionID, kpID uint) int {
	var activityID uint
	h.DB.Model(&model.StudentSession{}).Where("id = ?", sessionID).Pluck("activity_id", &activityID)
	if activityID == 0 {
		return 0
	}

	var kpIDsJSON string
	h.DB.Model(&model.LearningActivity{}).Where("id = ?", activityID).Pluck("kpids", &kpIDsJSON)
	if kpIDsJSON == "" {
		return 0
	}

	var kpIDs []uint
	if err := json.Unmarshal([]byte(kpIDsJSON), &kpIDs); err != nil {
		return 0
	}
	for i, id := range kpIDs {
		if id == kpID {
			return i
		}
	}
	return 0
}

// ── Activity File Upload ────────────────────────────────────

// allowedMimeTypes maps allowed MIME types for activity asset uploads.
var allowedMimeTypes = map[string]bool{
	"image/jpeg":      true,
	"image/png":       true,
	"image/gif":       true,
	"image/webp":      true,
	"video/mp4":       true,
	"video/webm":      true,
	"application/pdf": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   true, // .docx
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": true, // .pptx
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         true, // .xlsx
}

// maxAssetSize is the maximum file size for activity assets (100 MB).
const maxAssetSize = 100 * 1024 * 1024

// UploadAsset handles file uploads for activity step content.
//
//	@Summary      上传活动资源文件
//	@Description  为活动环节上传图片、视频、文档等资源文件
//	@Tags         Activities
//	@Accept       multipart/form-data
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id    path  int   true  "活动 ID"
//	@Param        file  formData  file  true  "资源文件"
//	@Success      200   {object}  map[string]interface{}
//	@Failure      400   {object}  ErrorResponse
//	@Failure      413   {object}  ErrorResponse
//	@Router       /activities/{id}/upload [post]
func (h *ActivityHandler) UploadAsset(c *gin.Context) {
	activityID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的活动 ID"})
		return
	}

	teacherID := middleware.GetUserID(c)

	// Verify ownership
	var activity model.LearningActivity
	if err := h.DB.First(&activity, activityID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "学习活动不存在"})
		return
	}
	if activity.TeacherID != teacherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权操作此活动"})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请上传文件"})
		return
	}
	defer file.Close()

	if header.Size > maxAssetSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "文件大小不能超过 100 MB"})
		return
	}

	// Detect content type from extension
	ext := strings.ToLower(filepath.Ext(header.Filename))
	contentType := header.Header.Get("Content-Type")
	if !allowedMimeTypes[contentType] {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("不支持的文件类型: %s", contentType)})
		return
	}

	// Storage key: activities/{activityID}/{uuid}{ext}
	storageKey := fmt.Sprintf("activities/%d/%s%s", activityID, uuid.New().String(), ext)

	if err := h.Storage.Upload(c.Request.Context(), storageKey, file, contentType); err != nil {
		slogActivity.Error("文件上传失败", "key", storageKey, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "文件上传失败"})
		return
	}

	fileURL, _ := h.Storage.URL(c.Request.Context(), storageKey)

	c.JSON(http.StatusOK, gin.H{
		"file_name": header.Filename,
		"file_url":  fileURL,
		"file_size": header.Size,
		"mime_type": contentType,
		"key":       storageKey,
	})
}

// ── AI Step Suggestion ──────────────────────────────────────

// SuggestStepRequest 请求 AI 生成环节内容建议。
type SuggestStepRequest struct {
	StepType        model.StepType `json:"step_type" binding:"required"`
	StepTitle       string         `json:"step_title"`
	StepDescription string         `json:"step_description"`
	ActivityTitle   string         `json:"activity_title"`
	KnowledgePoints []string       `json:"knowledge_points"` // 前端可直接传知识点名称列表
}

// SuggestStepResponse AI 建议的环节内容。
type SuggestStepResponse struct {
	Title         string `json:"title"`
	Description   string `json:"description"`
	ContentBlocks []struct {
		Type    string `json:"type"`
		Content string `json:"content"`
	} `json:"content_blocks"`
	Duration int `json:"duration"`
}

// SuggestStepContent uses AI to generate content suggestions for a step.
//
//	@Summary      AI 建议环节内容
//	@Description  根据环节类型和活动上下文，使用 AI 生成环节内容建议
//	@Tags         Activities
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id    path  int                true  "活动 ID"
//	@Param        body  body  SuggestStepRequest true  "环节上下文"
//	@Success      200   {object}  SuggestStepResponse
//	@Failure      400   {object}  ErrorResponse
//	@Failure      500   {object}  ErrorResponse
//	@Router       /activities/{id}/steps/suggest [post]
func (h *ActivityHandler) SuggestStepContent(c *gin.Context) {
	if h.LLMProvider == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI 服务不可用"})
		return
	}

	activityID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的活动 ID"})
		return
	}

	var req SuggestStepRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 1. Fetch activity and course context
	var activity model.LearningActivity
	if err := h.DB.First(&activity, activityID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "活动不存在"})
		return
	}

	// 2. Fetch knowledge points via course → chapters → KPs
	var kpNames []string
	if len(req.KnowledgePoints) > 0 {
		kpNames = req.KnowledgePoints
	} else if activity.CourseID != 0 {
		var course model.Course
		if err := h.DB.Preload("Chapters.KnowledgePoints").First(&course, activity.CourseID).Error; err == nil {
			for _, ch := range course.Chapters {
				for _, kp := range ch.KnowledgePoints {
					kpNames = append(kpNames, kp.Title)
				}
			}
		}
	}

	// 3. Build step-type-specific prompt
	stepTypeDesc := stepTypePromptMap[req.StepType]
	if stepTypeDesc == "" {
		stepTypeDesc = "通用教学环节"
	}

	activityTitle := req.ActivityTitle
	if activityTitle == "" {
		activityTitle = activity.Title
	}

	kpContext := "无特定知识点"
	if len(kpNames) > 0 {
		kpContext = strings.Join(kpNames, "、")
	}

	prompt := fmt.Sprintf(`你是一位经验丰富的教学设计专家。请为以下教学环节生成详细的内容建议。

【活动名称】%s
【环节类型】%s（%s）
【环节标题】%s
【环节描述】%s
【相关知识点】%s

请生成以下内容，并以 JSON 格式返回（不要使用 markdown 代码块包裹）：
{
  "title": "建议的环节标题（如果原标题为空或可以改进）",
  "description": "环节描述（2-3句话概述目标和方法）",
  "content_blocks": [
    {"type": "markdown", "content": "详细的教学内容，使用 Markdown 格式，包含具体的教学步骤、示例或问题"}
  ],
  "duration": 建议时长（分钟，整数）
}

要求：
1. 内容要具体、可操作，不要泛泛而谈
2. content_blocks 中的 markdown 内容应包含具体的教学材料（如讲解要点、讨论问题、练习题目等）
3. 根据环节类型调整内容风格：
   - lecture: 提供讲解提纲和关键知识点
   - discussion: 提供讨论引导问题和预期讨论方向
   - quiz: 提供具体的测验题目（选择题或简答题）
   - practice: 提供练习任务和评分标准
   - reading: 提供阅读指引和思考问题
   - group_work: 提供小组活动方案和分工建议
   - reflection: 提供反思提纲和自评要点
   - ai_tutoring: 提供 AI 辅导的对话框架和知识检查点
4. 时长建议要合理（通常 5-30 分钟）`,
		activityTitle,
		string(req.StepType), stepTypeDesc,
		req.StepTitle,
		req.StepDescription,
		kpContext)

	// 4. Call LLM
	resp, err := h.LLMProvider.Chat(c.Request.Context(), []llm.ChatMessage{
		{Role: "system", Content: "你是一位专业的教学设计 AI 助手。你只输出合法的 JSON，不使用 ```json 或 ``` 包裹。"},
		{Role: "user", Content: prompt},
	}, &llm.ChatOptions{Temperature: 0.7})

	if err != nil {
		slogActivity.Error("🤖 AI 建议生成失败", "activity_id", activityID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI 建议生成失败: " + err.Error()})
		return
	}

	// 5. Parse response — try to extract JSON object
	rawJSON := resp
	start := strings.Index(rawJSON, "{")
	end := strings.LastIndex(rawJSON, "}")
	if start != -1 && end != -1 && end > start {
		rawJSON = rawJSON[start : end+1]
	}

	var suggestion SuggestStepResponse
	if err := json.Unmarshal([]byte(rawJSON), &suggestion); err != nil {
		slogActivity.Error("🤖 AI 响应解析失败", "raw", rawJSON[:min(len(rawJSON), 200)], "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI 响应解析失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"suggestion": suggestion})
}

// stepTypePromptMap 环节类型的中文描述，用于 AI 提示词。
var stepTypePromptMap = map[model.StepType]string{
	model.StepTypeLecture:    "讲授环节 — 教师讲解核心知识",
	model.StepTypeDiscussion: "讨论环节 — 引导学生交流观点",
	model.StepTypeQuiz:       "测验环节 — 检测学生掌握程度",
	model.StepTypePractice:   "练习环节 — 学生动手实践",
	model.StepTypeReading:    "阅读环节 — 自主阅读与理解",
	model.StepTypeGroupWork:  "小组协作 — 团队合作完成任务",
	model.StepTypeReflection: "反思总结 — 回顾学习过程与收获",
	model.StepTypeAITutoring: "AI辅导 — 智能个性化辅导",
}
