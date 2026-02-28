package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/agent"
	"github.com/hflms/hanfledge/internal/delivery/http/middleware"
	"github.com/hflms/hanfledge/internal/domain/model"
	"gorm.io/gorm"
)

// ============================
// 学习活动 Handler — Phase 4
// ============================

// ActivityHandler handles learning activity CRUD and session management.
type ActivityHandler struct {
	DB           *gorm.DB
	Orchestrator *agent.AgentOrchestrator
}

// NewActivityHandler creates a new ActivityHandler.
func NewActivityHandler(db *gorm.DB, orchestrator *agent.AgentOrchestrator) *ActivityHandler {
	return &ActivityHandler{DB: db, Orchestrator: orchestrator}
}

// ── Teacher: Activity CRUD ──────────────────────────────────

// CreateActivityRequest 创建学习活动请求。
type CreateActivityRequest struct {
	CourseID    uint                   `json:"course_id" binding:"required"`
	Title       string                 `json:"title" binding:"required"`
	KPIDS       []uint                 `json:"kp_ids" binding:"required"`
	SkillConfig map[string]interface{} `json:"skill_config,omitempty"`
	Deadline    *string                `json:"deadline,omitempty"`
	AllowRetry  *bool                  `json:"allow_retry,omitempty"`
	MaxAttempts *int                   `json:"max_attempts,omitempty"`
	ClassIDs    []uint                 `json:"class_ids,omitempty"`
}

// CreateActivity creates a new learning activity.
// POST /api/v1/activities
func (h *ActivityHandler) CreateActivity(c *gin.Context) {
	var req CreateActivityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	teacherID := middleware.GetUserID(c)

	// Serialize JSON fields
	kpIDsJSON, _ := json.Marshal(req.KPIDS)
	skillConfigJSON := "{}"
	if req.SkillConfig != nil {
		data, _ := json.Marshal(req.SkillConfig)
		skillConfigJSON = string(data)
	}

	activity := model.LearningActivity{
		CourseID:    req.CourseID,
		TeacherID:   teacherID,
		Title:       req.Title,
		KPIDS:       string(kpIDsJSON),
		SkillConfig: skillConfigJSON,
		Deadline:    req.Deadline,
		Status:      model.ActivityStatusDraft,
		CreatedAt:   time.Now().Format(time.RFC3339),
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

// ListActivities returns learning activities for a teacher.
// GET /api/v1/activities?course_id=1&status=published
func (h *ActivityHandler) ListActivities(c *gin.Context) {
	teacherID := middleware.GetUserID(c)

	query := h.DB.Where("teacher_id = ?", teacherID)

	if courseID := c.Query("course_id"); courseID != "" {
		query = query.Where("course_id = ?", courseID)
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	var activities []model.LearningActivity
	if err := query.Preload("AssignedClasses").Order("created_at DESC").Find(&activities).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询学习活动失败"})
		return
	}

	c.JSON(http.StatusOK, activities)
}

// PublishActivity publishes a learning activity (changes status to published).
// POST /api/v1/activities/:id/publish
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

	c.JSON(http.StatusOK, gin.H{"message": "活动已发布"})
}

// ── Teacher: Sandbox Preview (design.md §5.1 Step 3) ────────

// PreviewActivity allows a teacher to preview a learning activity in sandbox mode.
// Creates a sandbox session with IsSandbox=true, allowing the teacher to experience
// the activity as a student. Sandbox sessions are excluded from analytics and mastery updates.
// POST /api/v1/activities/:id/preview
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
	json.Unmarshal([]byte(activity.KPIDS), &kpIDs)
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
// GET /api/v1/student/activities
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
// POST /api/v1/activities/:id/join
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
	json.Unmarshal([]byte(activity.KPIDS), &kpIDs)
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
// GET /api/v1/sessions/:id
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

	// Load recent interactions
	var interactions []model.Interaction
	h.DB.Where("session_id = ?", sessionID).
		Order("created_at ASC").
		Limit(50).
		Find(&interactions)

	c.JSON(http.StatusOK, gin.H{
		"session":      session,
		"interactions": interactions,
	})
}
