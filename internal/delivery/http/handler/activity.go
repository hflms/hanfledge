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
}

// NewActivityHandler creates a new ActivityHandler.
func NewActivityHandler(db *gorm.DB, orchestrator *agent.AgentOrchestrator, eventBus *plugin.EventBus, registry *plugin.Registry, fs storage.FileStorage) *ActivityHandler {
	return &ActivityHandler{DB: db, Orchestrator: orchestrator, EventBus: eventBus, Registry: registry, Storage: fs}
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
	ID            uint   `json:"id,omitempty"`
	Title         string `json:"title" binding:"required"`
	Description   string `json:"description,omitempty"`
	SortOrder     int    `json:"sort_order"`
	ContentBlocks string `json:"content_blocks,omitempty"` // JSON string
	Duration      int    `json:"duration,omitempty"`
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
		step := model.ActivityStep{
			ActivityID:    uint(activityID),
			Title:         s.Title,
			Description:   s.Description,
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

	// Create sandbox session — StudentID = teacherID (teacher acts as student)
	session := model.StudentSession{
		StudentID:  teacherID,
		ActivityID: uint(activityID),
		CurrentKP:  firstKP,
		Scaffold:   model.ScaffoldHigh,
		IsSandbox:  true,
		Status:     model.SessionStatusActive,
		StartedAt:  time.Now(),
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

	// Create new session
	session := model.StudentSession{
		StudentID:  studentID,
		ActivityID: uint(activityID),
		CurrentKP:  firstKP,
		Scaffold:   model.ScaffoldHigh, // Start with high scaffold
		Status:     model.SessionStatusActive,
		StartedAt:  time.Now(),
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

// UpdateSessionStep updates the active skill and current KP for a session
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

	query := h.DB.Model(&model.StudentSession{}).Where("id = ? AND student_id = ?", sessionID, studentID)

	if err := query.Updates(map[string]interface{}{
		"current_kp":   req.KPID,
		"active_skill": req.ActiveSkill,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新会话步骤失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "会话步骤更新成功"})
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
