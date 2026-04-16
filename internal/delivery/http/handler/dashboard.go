package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/delivery/http/middleware"
	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/logger"
	"github.com/hflms/hanfledge/internal/repository"
	"gorm.io/gorm"
)

var slogDash = logger.L("Dashboard")

// ============================
// 学情仪表盘 Handler — Phase 5
// ============================

// DashboardHandler handles learning analytics dashboard APIs.
type DashboardHandler struct {
	DB         *gorm.DB
	Courses    repository.CourseRepository
	Users      repository.UserRepository
	KPs        repository.KnowledgePointRepository
	Mastery    repository.MasteryRepository
	Sessions   repository.SessionRepository
	Activities repository.ActivityRepository
}

// NewDashboardHandler creates a new DashboardHandler.
func NewDashboardHandler(
	db *gorm.DB,
	courses repository.CourseRepository,
	users repository.UserRepository,
	kps repository.KnowledgePointRepository,
	mastery repository.MasteryRepository,
	sessions repository.SessionRepository,
	activities repository.ActivityRepository,
) *DashboardHandler {
	return &DashboardHandler{
		DB:         db,
		Courses:    courses,
		Users:      users,
		KPs:        kps,
		Mastery:    mastery,
		Sessions:   sessions,
		Activities: activities,
	}
}

// -- Knowledge Radar ----------------------------------------

// KnowledgeRadarResponse 全班知识漏洞雷达图响应。
type KnowledgeRadarResponse struct {
	CourseID     uint      `json:"course_id"`
	CourseTitle  string    `json:"course_title"`
	Labels       []string  `json:"labels"`
	Values       []float64 `json:"values"`
	StudentCount int       `json:"student_count"`
}

// GetKnowledgeRadar returns class-wide mastery aggregation for radar chart.
//
//	@Summary      知识雷达图
//	@Description  返回班级维度的知识点掌握度聚合数据，用于雷达图可视化
//	@Tags         Dashboard
//	@Produce      json
//	@Security     BearerAuth
//	@Param        course_id  query     int  true   "课程 ID"
//	@Param        class_id   query     int  false  "班级 ID"
//	@Success      200        {object}  KnowledgeRadarResponse
//	@Failure      400        {object}  ErrorResponse
//	@Failure      500        {object}  ErrorResponse
//	@Router       /dashboard/knowledge-radar [get]
func (h *DashboardHandler) GetKnowledgeRadar(c *gin.Context) {
	courseIDStr := c.Query("course_id")
	if courseIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "course_id 参数必填"})
		return
	}
	courseID, err := strconv.ParseUint(courseIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 course_id"})
		return
	}

	ctx := c.Request.Context()

	// Verify the teacher owns this course
	teacherID := middleware.GetUserID(c)
	course, err := h.Courses.FindByID(ctx, uint(courseID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "课程不存在"})
		return
	}
	if course.TeacherID != teacherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权查看该课程数据"})
		return
	}

	// Get all knowledge points for this course
	kps, err := h.KPs.FindByCourseID(ctx, uint(courseID))
	if err != nil {
		slogDash.Warn("failed to query knowledge points", "course_id", courseID, "err", err)
	}

	if len(kps) == 0 {
		c.JSON(http.StatusOK, KnowledgeRadarResponse{
			CourseID:    uint(courseID),
			CourseTitle: course.Title,
			Labels:      []string{},
			Values:      []float64{},
		})
		return
	}

	// Collect KP IDs
	kpIDs := make([]uint, len(kps))
	labels := make([]string, len(kps))
	for i, kp := range kps {
		kpIDs[i] = kp.ID
		labels[i] = kp.Title
	}

	// Build student filter by class_id (optional)
	var studentIDs []uint
	classIDStr := c.Query("class_id")
	if classIDStr != "" {
		classID, err := strconv.ParseUint(classIDStr, 10, 64)
		if err == nil {
			studentIDs, _ = h.Users.FindStudentIDsByClassID(ctx, uint(classID))
		}
	}

	// Aggregate average mastery per knowledge point
	values := make([]float64, len(kps))
	for i, kpID := range kpIDs {
		avg, _, err := h.Mastery.AggregateAvgByKP(ctx, kpID, studentIDs)
		if err != nil {
			slogDash.Warn("failed to aggregate mastery", "kp_id", kpID, "err", err)
		}
		values[i] = avg
	}

	// Count distinct students
	studentCount, _ := h.Mastery.CountDistinctStudents(ctx, kpIDs, studentIDs)

	c.JSON(http.StatusOK, KnowledgeRadarResponse{
		CourseID:     uint(courseID),
		CourseTitle:  course.Title,
		Labels:       labels,
		Values:       values,
		StudentCount: int(studentCount),
	})
}

// -- Student Mastery ----------------------------------------

// StudentMasteryItem 单个知识点的掌握度信息。
type StudentMasteryItem struct {
	KPID          uint    `json:"kp_id"`
	KPTitle       string  `json:"kp_title"`
	ChapterTitle  string  `json:"chapter_title"`
	MasteryScore  float64 `json:"mastery_score"`
	AttemptCount  int     `json:"attempt_count"`
	CorrectCount  int     `json:"correct_count"`
	LastAttemptAt *string `json:"last_attempt_at,omitempty"`
	UpdatedAt     string  `json:"updated_at"`
}

// StudentMasteryResponse 学生个人掌握度响应。
type StudentMasteryResponse struct {
	StudentID   uint                  `json:"student_id"`
	StudentName string                `json:"student_name"`
	Items       []StudentMasteryItem  `json:"items"`
	History     []MasteryHistoryPoint `json:"history"`
}

// MasteryHistoryPoint 掌握度历史变化点。
type MasteryHistoryPoint struct {
	Date         string  `json:"date"`
	AvgMastery   float64 `json:"avg_mastery"`
	AttemptCount int     `json:"attempt_count"`
}

// GetStudentMastery returns mastery data for a specific student.
//
//	@Summary      学生掌握度详情
//	@Description  返回指定学生的知识点掌握度和历史趋势数据
//	@Tags         Dashboard
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id         path      int  true   "学生 ID"
//	@Param        course_id  query     int  false  "课程 ID（不传则返回所有课程）"
//	@Success      200        {object}  StudentMasteryResponse
//	@Failure      400        {object}  ErrorResponse
//	@Failure      500        {object}  ErrorResponse
//	@Router       /students/{id}/mastery [get]
func (h *DashboardHandler) GetStudentMastery(c *gin.Context) {
	studentIDStr := c.Param("id")
	studentID, err := strconv.ParseUint(studentIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的学生 ID"})
		return
	}

	ctx := c.Request.Context()

	// Verify the student exists
	student, err := h.Users.FindByID(ctx, uint(studentID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "学生不存在"})
		return
	}

	// Optional course_id filter — get KP IDs belonging to this course
	courseIDStr := c.Query("course_id")
	var kpIDs []uint
	if courseIDStr != "" {
		courseID, err := strconv.ParseUint(courseIDStr, 10, 64)
		if err == nil {
			kpIDs, _ = h.KPs.FindIDsByCourseID(ctx, uint(courseID))
		}
	}

	// Query mastery records (kpIDs may be nil → no KP filter)
	masteries, err := h.Mastery.FindByStudent(ctx, uint(studentID), kpIDs)
	if err != nil {
		slogDash.Warn("failed to query mastery", "student_id", studentID, "err", err)
	}

	// Build response items with KP and chapter details (batch-loaded to avoid N+1)
	kpIDsForMastery := make([]uint, 0, len(masteries))
	for _, m := range masteries {
		kpIDsForMastery = append(kpIDsForMastery, m.KPID)
	}

	// Batch-load all needed KPs with their chapters
	kpMap := make(map[uint]model.KnowledgePoint)
	if len(kpIDsForMastery) > 0 {
		kpList, err := h.KPs.FindByIDsWithChapter(ctx, kpIDsForMastery)
		if err != nil {
			slogDash.Warn("failed to load knowledge points for mastery", "err", err)
		}
		for _, kp := range kpList {
			kpMap[kp.ID] = kp
		}
	}

	items := make([]StudentMasteryItem, 0, len(masteries))
	for _, m := range masteries {
		kp, ok := kpMap[m.KPID]
		if !ok {
			continue
		}

		item := StudentMasteryItem{
			KPID:         m.KPID,
			KPTitle:      kp.Title,
			ChapterTitle: kp.Chapter.Title,
			MasteryScore: m.MasteryScore,
			AttemptCount: m.AttemptCount,
			CorrectCount: m.CorrectCount,
			UpdatedAt:    m.UpdatedAt.Format(time.RFC3339),
		}
		if m.LastAttemptAt != nil {
			t := m.LastAttemptAt.Format(time.RFC3339)
			item.LastAttemptAt = &t
		}
		items = append(items, item)
	}

	// Build history trend (aggregate by date)
	daily, err := h.Mastery.AggregateDailyMastery(ctx, uint(studentID), kpIDs)
	if err != nil {
		slogDash.Warn("failed to query mastery history", "student_id", studentID, "err", err)
	}

	history := make([]MasteryHistoryPoint, len(daily))
	for i, d := range daily {
		history[i] = MasteryHistoryPoint{
			Date:         d.Date,
			AvgMastery:   d.AvgScore,
			AttemptCount: d.Count,
		}
	}

	c.JSON(http.StatusOK, StudentMasteryResponse{
		StudentID:   uint(studentID),
		StudentName: student.DisplayName,
		Items:       items,
		History:     history,
	})
}

// -- Activity Sessions Statistics ----------------------------

// ActivitySessionStats 活动会话统计信息。
type ActivitySessionStats struct {
	ActivityID        uint             `json:"activity_id"`
	ActivityTitle     string           `json:"activity_title"`
	TotalSessions     int              `json:"total_sessions"`
	ActiveSessions    int              `json:"active_sessions"`
	CompletedSessions int              `json:"completed_sessions"`
	CompletionRate    float64          `json:"completion_rate"`
	AvgDurationMin    float64          `json:"avg_duration_min"`
	AvgMastery        float64          `json:"avg_mastery"`
	Sessions          []SessionSummary `json:"sessions"`
}

// SessionSummary 单个会话摘要。
type SessionSummary struct {
	SessionID     uint    `json:"session_id"`
	StudentID     uint    `json:"student_id"`
	StudentName   string  `json:"student_name"`
	Status        string  `json:"status"`
	ScaffoldLevel string  `json:"scaffold_level"`
	StartedAt     string  `json:"started_at"`
	EndedAt       *string `json:"ended_at,omitempty"`
	DurationMin   float64 `json:"duration_min"`
	MasteryScore  float64 `json:"mastery_score"`
}

// GetActivitySessions returns session statistics for a learning activity.
//
//	@Summary      活动会话统计
//	@Description  返回指定学习活动的会话列表与统计数据（完成率、平均时长、掌握度等）
//	@Tags         Dashboard
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id  path      int  true  "活动 ID"
//	@Success      200 {object}  ActivitySessionStats
//	@Failure      400 {object}  ErrorResponse
//	@Failure      403 {object}  ErrorResponse
//	@Failure      404 {object}  ErrorResponse
//	@Router       /activities/{id}/sessions [get]
func (h *DashboardHandler) GetActivitySessions(c *gin.Context) {
	activityID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的活动 ID"})
		return
	}

	ctx := c.Request.Context()

	// Verify activity exists
	activity, err := h.Activities.FindByID(ctx, uint(activityID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "学习活动不存在"})
		return
	}

	// Verify teacher owns the activity
	teacherID := middleware.GetUserID(c)
	if activity.TeacherID != teacherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权查看该活动数据"})
		return
	}

	// Get all sessions for this activity (exclude sandbox sessions)
	sessions, err := h.Sessions.ListByActivityID(ctx, uint(activityID), true)
	if err != nil {
		slogDash.Warn("failed to query sessions", "activity_id", activityID, "err", err)
	}

	totalSessions := len(sessions)
	activeSessions := 0
	completedSessions := 0
	var totalDuration float64
	var completedDuration float64

	// Batch-load student names and mastery scores to avoid N+1 queries
	sessionStudentIDs := make(map[uint]bool)
	sessionKPIDs := make(map[uint]bool)
	for _, s := range sessions {
		sessionStudentIDs[s.StudentID] = true
		sessionKPIDs[s.CurrentKP] = true
	}

	studentNameMap := make(map[uint]string)
	if len(sessionStudentIDs) > 0 {
		idList := make([]uint, 0, len(sessionStudentIDs))
		for id := range sessionStudentIDs {
			idList = append(idList, id)
		}
		students, err := h.Users.FindByIDs(ctx, idList, "id, display_name")
		if err != nil {
			slogDash.Warn("failed to load student names", "err", err)
		}
		for _, s := range students {
			studentNameMap[s.ID] = s.DisplayName
		}
	}

	type masteryKey struct{ StudentID, KPID uint }
	masteryMap := make(map[masteryKey]float64)
	if len(sessionStudentIDs) > 0 && len(sessionKPIDs) > 0 {
		sidList := make([]uint, 0, len(sessionStudentIDs))
		for id := range sessionStudentIDs {
			sidList = append(sidList, id)
		}
		kpList := make([]uint, 0, len(sessionKPIDs))
		for id := range sessionKPIDs {
			kpList = append(kpList, id)
		}
		masteries, err := h.Mastery.FindByStudentsAndKPs(ctx, sidList, kpList)
		if err != nil {
			slogDash.Warn("failed to load mastery data", "err", err)
		}
		for _, m := range masteries {
			masteryMap[masteryKey{m.StudentID, m.KPID}] = m.MasteryScore
		}
	}

	summaries := make([]SessionSummary, 0, totalSessions)
	for _, s := range sessions {
		// Count status
		switch s.Status {
		case model.SessionStatusActive:
			activeSessions++
		case model.SessionStatusCompleted:
			completedSessions++
		}

		// Calculate duration
		var durationMin float64
		endTime := time.Now()
		if s.EndedAt != nil {
			endTime = *s.EndedAt
		}
		durationMin = endTime.Sub(s.StartedAt).Minutes()
		totalDuration += durationMin
		if s.Status == model.SessionStatusCompleted {
			completedDuration += durationMin
		}

		masteryScore := masteryMap[masteryKey{s.StudentID, s.CurrentKP}]

		summary := SessionSummary{
			SessionID:     s.ID,
			StudentID:     s.StudentID,
			StudentName:   studentNameMap[s.StudentID],
			Status:        string(s.Status),
			ScaffoldLevel: string(s.Scaffold),
			StartedAt:     s.StartedAt.Format(time.RFC3339),
			DurationMin:   durationMin,
			MasteryScore:  masteryScore,
		}
		if s.EndedAt != nil {
			t := s.EndedAt.Format(time.RFC3339)
			summary.EndedAt = &t
		}
		summaries = append(summaries, summary)
	}

	// Calculate averages
	completionRate := 0.0
	if totalSessions > 0 {
		completionRate = float64(completedSessions) / float64(totalSessions) * 100
	}

	avgDuration := 0.0
	if completedSessions > 0 {
		avgDuration = completedDuration / float64(completedSessions)
	}

	// Average mastery across all sessions' students
	avgMastery := 0.0
	if len(summaries) > 0 {
		totalMastery := 0.0
		for _, s := range summaries {
			totalMastery += s.MasteryScore
		}
		avgMastery = totalMastery / float64(len(summaries))
	}

	c.JSON(http.StatusOK, ActivitySessionStats{
		ActivityID:        uint(activityID),
		ActivityTitle:     activity.Title,
		TotalSessions:     totalSessions,
		ActiveSessions:    activeSessions,
		CompletedSessions: completedSessions,
		CompletionRate:    completionRate,
		AvgDurationMin:    avgDuration,
		AvgMastery:        avgMastery,
		Sessions:          summaries,
	})
}

// -- Student Self Mastery -----------------------------------

// GetSelfMastery returns mastery data for the authenticated student.
//
//	@Summary      学生自身掌握度
//	@Description  返回当前登录学生自己的知识点掌握度和历史趋势（复用 GetStudentMastery 逻辑）
//	@Tags         Student
//	@Produce      json
//	@Security     BearerAuth
//	@Param        course_id  query     int  false  "课程 ID（不传则返回所有课程）"
//	@Success      200        {object}  StudentMasteryResponse
//	@Failure      400        {object}  ErrorResponse
//	@Failure      500        {object}  ErrorResponse
//	@Router       /student/mastery [get]
func (h *DashboardHandler) GetSelfMastery(c *gin.Context) {
	studentID := middleware.GetUserID(c)

	// Reuse the same logic as GetStudentMastery but for the current user
	c.Params = append(c.Params, gin.Param{Key: "id", Value: strconv.FormatUint(uint64(studentID), 10)})
	h.GetStudentMastery(c)
}

// -- Error Notebook (错题本) --------------------------------

// ErrorNotebookItem 错题本列表条目。
type ErrorNotebookItem struct {
	ID             uint       `json:"id"`
	KPID           uint       `json:"kp_id"`
	KPTitle        string     `json:"kp_title"`
	ChapterTitle   string     `json:"chapter_title"`
	SessionID      uint       `json:"session_id"`
	StudentInput   string     `json:"student_input"`
	CoachGuidance  string     `json:"coach_guidance"`
	ErrorType      string     `json:"error_type"`
	MasteryAtError float64    `json:"mastery_at_error"`
	Resolved       bool       `json:"resolved"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	ArchivedAt     time.Time  `json:"archived_at"`
}

// ErrorNotebookResponse 错题本列表响应。
type ErrorNotebookResponse struct {
	Items         []ErrorNotebookItem `json:"items"`
	TotalCount    int64               `json:"total_count"`
	UnresolvedCnt int64               `json:"unresolved_count"`
	ResolvedCnt   int64               `json:"resolved_count"`
}

// GetErrorNotebook returns the error notebook entries for the authenticated student.
//
//	@Summary      错题本
//	@Description  返回当前学生的错题本条目，支持按已解决状态和知识点筛选
//	@Tags         Student
//	@Produce      json
//	@Security     BearerAuth
//	@Param        resolved  query     bool  false  "是否已解决（true/false）"
//	@Param        kp_id     query     int   false  "知识点 ID"
//	@Success      200       {object}  ErrorNotebookResponse
//	@Failure      500       {object}  ErrorResponse
//	@Router       /student/error-notebook [get]
func (h *DashboardHandler) GetErrorNotebook(c *gin.Context) {
	studentID := middleware.GetUserID(c)
	ctx := c.Request.Context()

	// Parse optional filters
	var resolved *bool
	if resolvedStr := c.Query("resolved"); resolvedStr != "" {
		if resolvedStr == "true" {
			v := true
			resolved = &v
		} else if resolvedStr == "false" {
			v := false
			resolved = &v
		}
	}

	var kpID uint
	if kpIDStr := c.Query("kp_id"); kpIDStr != "" {
		id, err := strconv.ParseUint(kpIDStr, 10, 64)
		if err == nil {
			kpID = uint(id)
		}
	}

	entries, err := h.Mastery.ListErrorNotebook(ctx, studentID, resolved, kpID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询错题本失败"})
		return
	}

	// Count totals for the student
	totalCount, unresolvedCnt, err := h.Mastery.CountErrorNotebook(ctx, studentID)
	if err != nil {
		slogDash.Warn("failed to count error notebook", "student_id", studentID, "err", err)
	}
	resolvedCnt := totalCount - unresolvedCnt

	// Enrich with KP and chapter titles
	kpIDs := make([]uint, 0, len(entries))
	for _, e := range entries {
		kpIDs = append(kpIDs, e.KPID)
	}

	kpTitleMap := make(map[uint]string)
	chapterTitleMap := make(map[uint]string)
	if len(kpIDs) > 0 {
		kpWithTitles, err := h.KPs.FindWithChapterTitles(ctx, kpIDs)
		if err != nil {
			slogDash.Warn("failed to load kp titles", "err", err)
		}
		for _, kp := range kpWithTitles {
			kpTitleMap[kp.ID] = kp.Title
			chapterTitleMap[kp.ID] = kp.ChapterTitle
		}
	}

	items := make([]ErrorNotebookItem, 0, len(entries))
	for _, e := range entries {
		items = append(items, ErrorNotebookItem{
			ID:             e.ID,
			KPID:           e.KPID,
			KPTitle:        kpTitleMap[e.KPID],
			ChapterTitle:   chapterTitleMap[e.KPID],
			SessionID:      e.SessionID,
			StudentInput:   e.StudentInput,
			CoachGuidance:  e.CoachGuidance,
			ErrorType:      e.ErrorType,
			MasteryAtError: e.MasteryAtError,
			Resolved:       e.Resolved,
			ResolvedAt:     e.ResolvedAt,
			ArchivedAt:     e.ArchivedAt,
		})
	}

	c.JSON(http.StatusOK, ErrorNotebookResponse{
		Items:         items,
		TotalCount:    totalCount,
		UnresolvedCnt: unresolvedCnt,
		ResolvedCnt:   resolvedCnt,
	})
}

// -- Live Monitor Overview ----------------------------------------

// LiveActivitySummary 单个活动的实时监控摘要。
type LiveActivitySummary struct {
	ActivityID        uint    `json:"activity_id"`
	ActivityTitle     string  `json:"activity_title"`
	ActivityStatus    string  `json:"activity_status"`
	TotalStudents     int     `json:"total_students"`
	ActiveStudents    int     `json:"active_students"`
	CompletedStudents int     `json:"completed_students"`
	AvgMastery        float64 `json:"avg_mastery"`
	AvgDurationMin    float64 `json:"avg_duration_min"`
}

// LiveMonitorResponse 实时监控概览响应。
type LiveMonitorResponse struct {
	CourseID   uint                  `json:"course_id"`
	Timestamp  string                `json:"timestamp"`
	Activities []LiveActivitySummary `json:"activities"`
}

// GetLiveMonitor returns a real-time overview of all published activities for a course,
// showing live session counts, active/completed students, and average mastery.
// Designed for 10-second polling by the teacher dashboard.
//
//	@Summary      实时监控概览
//	@Description  返回课程下所有已发布活动的实时学情概览（活跃/完成学生数、平均掌握度等）
//	@Tags         Dashboard
//	@Produce      json
//	@Security     BearerAuth
//	@Param        course_id  query     int  true  "课程 ID"
//	@Success      200        {object}  LiveMonitorResponse
//	@Failure      400        {object}  ErrorResponse
//	@Failure      403        {object}  ErrorResponse
//	@Router       /dashboard/live-monitor [get]
func (h *DashboardHandler) GetLiveMonitor(c *gin.Context) {
	courseIDStr := c.Query("course_id")
	if courseIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "course_id 参数必填"})
		return
	}
	courseID, err := strconv.ParseUint(courseIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 course_id"})
		return
	}

	ctx := c.Request.Context()
	teacherID := middleware.GetUserID(c)

	// Verify teacher owns this course
	course, err := h.Courses.FindByID(ctx, uint(courseID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "课程不存在"})
		return
	}
	if course.TeacherID != teacherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权查看该课程数据"})
		return
	}

	// Get all published activities for this course
	var activities []model.LearningActivity
	if err := h.DB.WithContext(ctx).
		Where("course_id = ? AND status = ?", courseID, model.ActivityStatusPublished).
		Order("created_at DESC").
		Find(&activities).Error; err != nil {
		slogDash.Warn("failed to query published activities", "course_id", courseID, "err", err)
	}

	summaries := make([]LiveActivitySummary, 0, len(activities))

	for _, act := range activities {
		// Get all non-sandbox sessions for this activity
		sessions, err := h.Sessions.ListByActivityID(ctx, act.ID, true)
		if err != nil {
			slogDash.Warn("failed to query sessions", "activity_id", act.ID, "err", err)
			continue
		}

		activeCount := 0
		completedCount := 0
		var totalDuration float64
		now := time.Now()

		// Collect student IDs and KP IDs for batch mastery lookup
		studentKPMap := make(map[uint]uint) // studentID -> latest currentKP
		for _, s := range sessions {
			switch s.Status {
			case model.SessionStatusActive:
				activeCount++
			case model.SessionStatusCompleted:
				completedCount++
			}
			studentKPMap[s.StudentID] = s.CurrentKP

			endTime := now
			if s.EndedAt != nil {
				endTime = *s.EndedAt
			}
			totalDuration += endTime.Sub(s.StartedAt).Minutes()
		}

		avgDuration := 0.0
		if len(sessions) > 0 {
			avgDuration = totalDuration / float64(len(sessions))
		}

		// Batch mastery lookup
		avgMastery := 0.0
		if len(studentKPMap) > 0 {
			studentIDs := make([]uint, 0, len(studentKPMap))
			kpIDSet := make(map[uint]bool)
			for sid, kpid := range studentKPMap {
				studentIDs = append(studentIDs, sid)
				kpIDSet[kpid] = true
			}
			kpIDs := make([]uint, 0, len(kpIDSet))
			for kp := range kpIDSet {
				kpIDs = append(kpIDs, kp)
			}
			masteries, err := h.Mastery.FindByStudentsAndKPs(ctx, studentIDs, kpIDs)
			if err == nil && len(masteries) > 0 {
				totalM := 0.0
				for _, m := range masteries {
					totalM += m.MasteryScore
				}
				avgMastery = totalM / float64(len(masteries))
			}
		}

		summaries = append(summaries, LiveActivitySummary{
			ActivityID:        act.ID,
			ActivityTitle:     act.Title,
			ActivityStatus:    string(act.Status),
			TotalStudents:     len(sessions),
			ActiveStudents:    activeCount,
			CompletedStudents: completedCount,
			AvgMastery:        avgMastery,
			AvgDurationMin:    avgDuration,
		})
	}

	c.JSON(http.StatusOK, LiveMonitorResponse{
		CourseID:   uint(courseID),
		Timestamp:  time.Now().Format(time.RFC3339),
		Activities: summaries,
	})
}

// -- Activity Live Detail ----------------------------------------

// LiveStudentInfo 单个学生的实时学习状态。
type LiveStudentInfo struct {
	StudentID        uint    `json:"student_id"`
	StudentName      string  `json:"student_name"`
	SessionID        uint    `json:"session_id"`
	Status           string  `json:"status"`
	DurationMin      float64 `json:"duration_min"`
	MasteryScore     float64 `json:"mastery_score"`
	InteractionCount int64   `json:"interaction_count"`
	LastActiveAt     string  `json:"last_active_at"`
	ScaffoldLevel    string  `json:"scaffold_level"`
}

// LiveStepInfo 单个教学环节的学生分布。
type LiveStepInfo struct {
	KPID     uint              `json:"kp_id"`
	KPTitle  string            `json:"kp_title"`
	StepIdx  int               `json:"step_index"`
	Students []LiveStudentInfo `json:"students"`
}

// StudentAlert 学生预警信息。
type StudentAlert struct {
	StudentID   uint   `json:"student_id"`
	StudentName string `json:"student_name"`
	SessionID   uint   `json:"session_id"`
	AlertType   string `json:"alert_type"` // "stuck" | "struggling" | "idle"
	Message     string `json:"message"`
}

// KPSequenceItem 知识点序列条目。
type KPSequenceItem struct {
	KPID    uint   `json:"kp_id"`
	KPTitle string `json:"kp_title"`
}

// ActivityLiveDetailResponse 活动实时详情响应。
type ActivityLiveDetailResponse struct {
	ActivityID uint             `json:"activity_id"`
	Title      string           `json:"title"`
	KPSequence []KPSequenceItem `json:"kp_sequence"`
	Steps      []LiveStepInfo   `json:"steps"`
	Alerts     []StudentAlert   `json:"alerts"`
	Timestamp  string           `json:"timestamp"`
}

// GetActivityLiveDetail returns detailed per-step student breakdown for a specific activity.
// Shows which students are on which step, how long they've been there, and auto-detects
// students who may need intervention (stuck, struggling, idle).
//
//	@Summary      活动实时详情
//	@Description  返回指定活动的按环节学生分布、停留时长、掌握度和预警信息
//	@Tags         Dashboard
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id  path      int  true  "活动 ID"
//	@Success      200 {object}  ActivityLiveDetailResponse
//	@Failure      400 {object}  ErrorResponse
//	@Failure      403 {object}  ErrorResponse
//	@Failure      404 {object}  ErrorResponse
//	@Router       /dashboard/activities/{id}/live [get]
func (h *DashboardHandler) GetActivityLiveDetail(c *gin.Context) {
	activityID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的活动 ID"})
		return
	}

	ctx := c.Request.Context()

	// Verify activity exists and teacher owns it
	activity, err := h.Activities.FindByID(ctx, uint(activityID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "学习活动不存在"})
		return
	}
	teacherID := middleware.GetUserID(c)
	if activity.TeacherID != teacherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权查看该活动数据"})
		return
	}

	// 1. Load KP sequence
	kpIDs, kpSequence, kpIndexMap := h.loadKPSequence(ctx, activity)

	// 2. Get sessions
	sessions, err := h.Sessions.ListByActivityID(ctx, uint(activityID), true)
	if err != nil {
		slogDash.Warn("failed to query sessions", "activity_id", activityID, "err", err)
	}

	// 3. Batch-load student names and mastery data
	studentIDs, studentNameMap := h.loadStudentNames(ctx, sessions)
	masteryMap := h.loadStudentMasteryMap(ctx, studentIDs, kpIDs)

	now := time.Now()

	// 4. Process sessions to generate live info and alerts
	stepStudentsMap, alerts := h.processLiveSessions(ctx, sessions, kpIndexMap, studentNameMap, masteryMap, now)

	// Build ordered steps response
	steps := make([]LiveStepInfo, len(kpSequence))
	for i, kpItem := range kpSequence {
		students := stepStudentsMap[i]
		if students == nil {
			students = []LiveStudentInfo{}
		}
		steps[i] = LiveStepInfo{
			KPID:     kpItem.KPID,
			KPTitle:  kpItem.KPTitle,
			StepIdx:  i,
			Students: students,
		}
	}

	if alerts == nil {
		alerts = []StudentAlert{}
	}

	c.JSON(http.StatusOK, ActivityLiveDetailResponse{
		ActivityID: uint(activityID),
		Title:      activity.Title,
		KPSequence: kpSequence,
		Steps:      steps,
		Alerts:     alerts,
		Timestamp:  now.Format(time.RFC3339),
	})
}

type masteryKey struct{ StudentID, KPID uint }

func (h *DashboardHandler) loadKPSequence(ctx context.Context, activity *model.LearningActivity) ([]uint, []KPSequenceItem, map[uint]int) {
	var kpIDs []uint
	if activity.KPIDS != "" {
		var rawIDs []int
		if err := json.Unmarshal([]byte(activity.KPIDS), &rawIDs); err != nil {
			slogDash.Warn("failed to parse kp_ids", "activity_id", activity.ID, "err", err)
		} else {
			for _, id := range rawIDs {
				kpIDs = append(kpIDs, uint(id))
			}
		}
	}

	kpTitleMap := make(map[uint]string)
	if len(kpIDs) > 0 {
		kps, err := h.KPs.FindByIDs(ctx, kpIDs)
		if err != nil {
			slogDash.Warn("failed to load KPs", "err", err)
		}
		for _, kp := range kps {
			kpTitleMap[kp.ID] = kp.Title
		}
	}

	kpSequence := make([]KPSequenceItem, 0, len(kpIDs))
	kpIndexMap := make(map[uint]int)
	for i, kpID := range kpIDs {
		kpSequence = append(kpSequence, KPSequenceItem{
			KPID:    kpID,
			KPTitle: kpTitleMap[kpID],
		})
		kpIndexMap[kpID] = i
	}
	return kpIDs, kpSequence, kpIndexMap
}

func (h *DashboardHandler) loadStudentNames(ctx context.Context, sessions []model.StudentSession) ([]uint, map[uint]string) {
	studentIDSet := make(map[uint]bool)
	for _, s := range sessions {
		studentIDSet[s.StudentID] = true
	}
	studentNameMap := make(map[uint]string)
	var idList []uint
	if len(studentIDSet) > 0 {
		for id := range studentIDSet {
			idList = append(idList, id)
		}
		students, err := h.Users.FindByIDs(ctx, idList, "id, display_name")
		if err != nil {
			slogDash.Warn("failed to load student names", "err", err)
		}
		for _, s := range students {
			studentNameMap[s.ID] = s.DisplayName
		}
	}
	return idList, studentNameMap
}

func (h *DashboardHandler) loadStudentMasteryMap(ctx context.Context, studentIDs []uint, kpIDs []uint) map[masteryKey]float64 {
	masteryMap := make(map[masteryKey]float64)
	if len(studentIDs) > 0 && len(kpIDs) > 0 {
		masteries, err := h.Mastery.FindByStudentsAndKPs(ctx, studentIDs, kpIDs)
		if err != nil {
			slogDash.Warn("failed to load mastery data", "err", err)
		}
		for _, m := range masteries {
			masteryMap[masteryKey{m.StudentID, m.KPID}] = m.MasteryScore
		}
	}
	return masteryMap
}

func (h *DashboardHandler) processLiveSessions(ctx context.Context, sessions []model.StudentSession, kpIndexMap map[uint]int, studentNameMap map[uint]string, masteryMap map[masteryKey]float64, now time.Time) (map[int][]LiveStudentInfo, []StudentAlert) {
	stepStudentsMap := make(map[int][]LiveStudentInfo)
	var alerts []StudentAlert

	for _, s := range sessions {
		stepIdx, found := kpIndexMap[s.CurrentKP]
		if !found {
			stepIdx = 0
		}

		endTime := now
		if s.EndedAt != nil {
			endTime = *s.EndedAt
		}
		durationMin := endTime.Sub(s.StartedAt).Minutes()

		interactionCount, err := h.Sessions.CountStudentInteractions(ctx, s.ID)
		if err != nil {
			slogDash.Warn("failed to count interactions", "session_id", s.ID, "err", err)
		}

		lastActiveAt := s.StartedAt.Format(time.RFC3339)
		var lastInteraction model.Interaction
		if err := h.DB.WithContext(ctx).
			Where("session_id = ?", s.ID).
			Order("created_at DESC").
			First(&lastInteraction).Error; err == nil {
			lastActiveAt = lastInteraction.CreatedAt.Format(time.RFC3339)
		}

		masteryScore := masteryMap[masteryKey{s.StudentID, s.CurrentKP}]
		studentName := studentNameMap[s.StudentID]

		info := LiveStudentInfo{
			StudentID:        s.StudentID,
			StudentName:      studentName,
			SessionID:        s.ID,
			Status:           string(s.Status),
			DurationMin:      durationMin,
			MasteryScore:     masteryScore,
			InteractionCount: interactionCount,
			LastActiveAt:     lastActiveAt,
			ScaffoldLevel:    string(s.Scaffold),
		}
		stepStudentsMap[stepIdx] = append(stepStudentsMap[stepIdx], info)

		if s.Status != model.SessionStatusActive {
			continue
		}

		lastActive, parseErr := time.Parse(time.RFC3339, lastActiveAt)

		if parseErr == nil && now.Sub(lastActive).Minutes() > 10 {
			idleMins := int(now.Sub(lastActive).Minutes())
			alerts = append(alerts, StudentAlert{
				StudentID:   s.StudentID,
				StudentName: studentName,
				SessionID:   s.ID,
				AlertType:   "idle",
				Message:     "已超过 " + strconv.Itoa(idleMins) + " 分钟无互动",
			})
		}

		if durationMin > 15 && interactionCount < 3 {
			alerts = append(alerts, StudentAlert{
				StudentID:   s.StudentID,
				StudentName: studentName,
				SessionID:   s.ID,
				AlertType:   "stuck",
				Message:     "在当前环节停留超过 15 分钟且互动较少",
			})
		}

		if masteryScore < 0.3 && interactionCount > 5 {
			alerts = append(alerts, StudentAlert{
				StudentID:   s.StudentID,
				StudentName: studentName,
				SessionID:   s.ID,
				AlertType:   "struggling",
				Message:     "掌握度偏低（" + strconv.Itoa(int(masteryScore*100)) + "%），可能需要教师干预",
			})
		}
	}
	return stepStudentsMap, alerts
}
