package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/delivery/http/middleware"
	"github.com/hflms/hanfledge/internal/domain/model"
	"gorm.io/gorm"
)

// ============================
// 学情仪表盘 Handler — Phase 5
// ============================

// DashboardHandler handles learning analytics dashboard APIs.
type DashboardHandler struct {
	DB *gorm.DB
}

// NewDashboardHandler creates a new DashboardHandler.
func NewDashboardHandler(db *gorm.DB) *DashboardHandler {
	return &DashboardHandler{DB: db}
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
// GET /api/v1/dashboard/knowledge-radar?course_id=1&class_id=1
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

	// Verify the teacher owns this course
	teacherID := middleware.GetUserID(c)
	var course model.Course
	if err := h.DB.First(&course, courseID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "课程不存在"})
		return
	}
	if course.TeacherID != teacherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权查看该课程数据"})
		return
	}

	// Get all knowledge points for this course
	var kps []model.KnowledgePoint
	h.DB.Joins("JOIN chapters ON chapters.id = knowledge_points.chapter_id").
		Where("chapters.course_id = ?", courseID).
		Order("chapters.sort_order ASC, knowledge_points.id ASC").
		Find(&kps)

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
			h.DB.Model(&model.ClassStudent{}).
				Where("class_id = ?", classID).
				Pluck("student_id", &studentIDs)
		}
	}

	// Aggregate average mastery per knowledge point
	values := make([]float64, len(kps))
	for i, kpID := range kpIDs {
		var avgResult struct {
			Avg   float64
			Count int64
		}

		query := h.DB.Model(&model.StudentKPMastery{}).
			Select("COALESCE(AVG(mastery_score), 0.0) as avg, COUNT(*) as count").
			Where("kp_id = ?", kpID)

		if len(studentIDs) > 0 {
			query = query.Where("student_id IN ?", studentIDs)
		}

		query.Scan(&avgResult)
		values[i] = avgResult.Avg
	}

	// Count distinct students
	var studentCount int64
	countQuery := h.DB.Model(&model.StudentKPMastery{}).
		Where("kp_id IN ?", kpIDs)
	if len(studentIDs) > 0 {
		countQuery = countQuery.Where("student_id IN ?", studentIDs)
	}
	countQuery.Distinct("student_id").Count(&studentCount)

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
// GET /api/v1/students/:id/mastery?course_id=1
func (h *DashboardHandler) GetStudentMastery(c *gin.Context) {
	studentIDStr := c.Param("id")
	studentID, err := strconv.ParseUint(studentIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的学生 ID"})
		return
	}

	// Verify the student exists
	var student model.User
	if err := h.DB.First(&student, studentID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "学生不存在"})
		return
	}

	// Query mastery records
	query := h.DB.Model(&model.StudentKPMastery{}).
		Where("student_id = ?", studentID)

	// Optional course_id filter
	courseIDStr := c.Query("course_id")
	var kpIDs []uint
	if courseIDStr != "" {
		courseID, err := strconv.ParseUint(courseIDStr, 10, 64)
		if err == nil {
			h.DB.Raw(`
				SELECT kp.id FROM knowledge_points kp
				JOIN chapters c ON c.id = kp.chapter_id
				WHERE c.course_id = ?
			`, courseID).Scan(&kpIDs)
			if len(kpIDs) > 0 {
				query = query.Where("kp_id IN ?", kpIDs)
			}
		}
	}

	var masteries []model.StudentKPMastery
	query.Order("updated_at DESC").Find(&masteries)

	// Build response items with KP and chapter details
	items := make([]StudentMasteryItem, 0, len(masteries))
	for _, m := range masteries {
		var kp model.KnowledgePoint
		if err := h.DB.Preload("Chapter").First(&kp, m.KPID).Error; err != nil {
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
	type dailyAgg struct {
		Date     string  `json:"date"`
		AvgScore float64 `json:"avg_score"`
		Count    int     `json:"count"`
	}
	var daily []dailyAgg

	historyQuery := h.DB.Model(&model.StudentKPMastery{}).
		Select("DATE(updated_at) as date, AVG(mastery_score) as avg_score, SUM(attempt_count) as count").
		Where("student_id = ?", studentID).
		Group("DATE(updated_at)").
		Order("date ASC")

	if len(kpIDs) > 0 {
		historyQuery = historyQuery.Where("kp_id IN ?", kpIDs)
	}
	historyQuery.Scan(&daily)

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
// GET /api/v1/activities/:id/sessions
func (h *DashboardHandler) GetActivitySessions(c *gin.Context) {
	activityID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的活动 ID"})
		return
	}

	// Verify activity exists
	var activity model.LearningActivity
	if err := h.DB.First(&activity, activityID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "学习活动不存在"})
		return
	}

	// Verify teacher owns the activity
	teacherID := middleware.GetUserID(c)
	if activity.TeacherID != teacherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权查看该活动数据"})
		return
	}

	// Get all sessions for this activity
	var sessions []model.StudentSession
	h.DB.Where("activity_id = ?", activityID).
		Order("started_at DESC").
		Find(&sessions)

	totalSessions := len(sessions)
	activeSessions := 0
	completedSessions := 0
	var totalDuration float64
	var completedDuration float64

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

		// Get student name
		var student model.User
		h.DB.Select("id, display_name").First(&student, s.StudentID)

		// Get mastery score for the current KP
		var mastery model.StudentKPMastery
		masteryScore := 0.0
		if err := h.DB.Where("student_id = ? AND kp_id = ?", s.StudentID, s.CurrentKP).
			First(&mastery).Error; err == nil {
			masteryScore = mastery.MasteryScore
		}

		summary := SessionSummary{
			SessionID:     s.ID,
			StudentID:     s.StudentID,
			StudentName:   student.DisplayName,
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
// GET /api/v1/student/mastery?course_id=1
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
// GET /api/v1/student/error-notebook?resolved=false&kp_id=1
func (h *DashboardHandler) GetErrorNotebook(c *gin.Context) {
	studentID := middleware.GetUserID(c)

	query := h.DB.Where("student_id = ?", studentID)

	// Optional filter: resolved status
	if resolvedStr := c.Query("resolved"); resolvedStr != "" {
		if resolvedStr == "true" {
			query = query.Where("resolved = ?", true)
		} else if resolvedStr == "false" {
			query = query.Where("resolved = ?", false)
		}
	}

	// Optional filter: specific KP
	if kpIDStr := c.Query("kp_id"); kpIDStr != "" {
		kpID, err := strconv.ParseUint(kpIDStr, 10, 64)
		if err == nil {
			query = query.Where("kp_id = ?", kpID)
		}
	}

	var entries []model.ErrorNotebookEntry
	if err := query.Order("archived_at DESC").Find(&entries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询错题本失败"})
		return
	}

	// Count totals for the student
	var totalCount, unresolvedCnt, resolvedCnt int64
	h.DB.Model(&model.ErrorNotebookEntry{}).Where("student_id = ?", studentID).Count(&totalCount)
	h.DB.Model(&model.ErrorNotebookEntry{}).Where("student_id = ? AND resolved = ?", studentID, false).Count(&unresolvedCnt)
	resolvedCnt = totalCount - unresolvedCnt

	// Enrich with KP and chapter titles
	kpIDs := make([]uint, 0, len(entries))
	for _, e := range entries {
		kpIDs = append(kpIDs, e.KPID)
	}

	kpTitleMap := make(map[uint]string)
	chapterTitleMap := make(map[uint]string)
	if len(kpIDs) > 0 {
		var kps []struct {
			ID           uint   `gorm:"column:id"`
			Title        string `gorm:"column:title"`
			ChapterTitle string `gorm:"column:chapter_title"`
		}
		h.DB.Raw(`
			SELECT kp.id, kp.title, c.title AS chapter_title
			FROM knowledge_points kp
			JOIN chapters c ON c.id = kp.chapter_id
			WHERE kp.id IN ?
		`, kpIDs).Scan(&kps)

		for _, kp := range kps {
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
