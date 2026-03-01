package handler

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/delivery/http/middleware"
	"github.com/hflms/hanfledge/internal/domain/model"
	"gorm.io/gorm"
)

// ============================
// 数据导出 Handler — Data Export
// ============================

// ExportHandler handles CSV data export APIs for teachers.
type ExportHandler struct {
	DB *gorm.DB
}

// NewExportHandler creates a new ExportHandler.
func NewExportHandler(db *gorm.DB) *ExportHandler {
	return &ExportHandler{DB: db}
}

// -- Helper ---------------------------------------------------

// setCSVHeaders sets HTTP headers for CSV file download.
func setCSVHeaders(c *gin.Context, filename string) {
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	// Write UTF-8 BOM so Excel opens Chinese correctly
	c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
}

// -- Export Activity Sessions ---------------------------------

// ExportActivitySessions exports all session data for a learning activity as CSV.
// GET /api/v1/export/activities/:id/sessions
func (h *ExportHandler) ExportActivitySessions(c *gin.Context) {
	activityID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的活动 ID"})
		return
	}

	// Verify activity exists and teacher owns it
	var activity model.LearningActivity
	if err := h.DB.First(&activity, activityID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "学习活动不存在"})
		return
	}
	teacherID := middleware.GetUserID(c)
	if activity.TeacherID != teacherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权导出该活动数据"})
		return
	}

	// Fetch sessions
	var sessions []model.StudentSession
	h.DB.Where("activity_id = ?", activityID).Order("started_at DESC").Find(&sessions)

	// Set CSV headers
	ts := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("sessions_%d_%s.csv", activityID, ts)
	setCSVHeaders(c, filename)

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	// Header row
	writer.Write([]string{
		"会话ID", "学生ID", "学生姓名", "知识点ID", "知识点名称",
		"当前技能", "支架等级", "状态", "掌握度",
		"开始时间", "结束时间", "时长(分钟)",
	})

	// Batch-load student names and KP titles to avoid N+1 queries
	studentIDSet := make(map[uint]bool)
	kpIDSet := make(map[uint]bool)
	for _, s := range sessions {
		studentIDSet[s.StudentID] = true
		kpIDSet[s.CurrentKP] = true
	}

	studentNames := make(map[uint]string)
	if len(studentIDSet) > 0 {
		ids := mapKeys(studentIDSet)
		var users []model.User
		h.DB.Select("id, display_name").Where("id IN ?", ids).Find(&users)
		for _, u := range users {
			studentNames[u.ID] = u.DisplayName
		}
	}

	kpTitles := make(map[uint]string)
	if len(kpIDSet) > 0 {
		ids := mapKeys(kpIDSet)
		var kps []model.KnowledgePoint
		h.DB.Select("id, title").Where("id IN ?", ids).Find(&kps)
		for _, kp := range kps {
			kpTitles[kp.ID] = kp.Title
		}
	}

	// Batch-load mastery scores
	type masteryKey struct{ StudentID, KPID uint }
	masteryScores := make(map[masteryKey]float64)
	if len(studentIDSet) > 0 && len(kpIDSet) > 0 {
		var masteries []model.StudentKPMastery
		h.DB.Where("student_id IN ? AND kp_id IN ?", mapKeys(studentIDSet), mapKeys(kpIDSet)).
			Find(&masteries)
		for _, m := range masteries {
			masteryScores[masteryKey{m.StudentID, m.KPID}] = m.MasteryScore
		}
	}

	for _, s := range sessions {
		// Duration
		endTime := time.Now()
		endStr := ""
		if s.EndedAt != nil {
			endTime = *s.EndedAt
			endStr = s.EndedAt.Format("2006-01-02 15:04:05")
		}
		durationMin := endTime.Sub(s.StartedAt).Minutes()

		writer.Write([]string{
			strconv.FormatUint(uint64(s.ID), 10),
			strconv.FormatUint(uint64(s.StudentID), 10),
			studentNames[s.StudentID],
			strconv.FormatUint(uint64(s.CurrentKP), 10),
			kpTitles[s.CurrentKP],
			s.ActiveSkill,
			string(s.Scaffold),
			string(s.Status),
			fmt.Sprintf("%.2f", masteryScores[masteryKey{s.StudentID, s.CurrentKP}]),
			s.StartedAt.Format("2006-01-02 15:04:05"),
			endStr,
			fmt.Sprintf("%.1f", durationMin),
		})
	}
}

// -- Export Class Mastery -------------------------------------

// ExportClassMastery exports mastery data for all students in a course as CSV.
// GET /api/v1/export/courses/:id/mastery
func (h *ExportHandler) ExportClassMastery(c *gin.Context) {
	courseID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的课程 ID"})
		return
	}

	// Verify course exists and teacher owns it
	var course model.Course
	if err := h.DB.First(&course, courseID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "课程不存在"})
		return
	}
	teacherID := middleware.GetUserID(c)
	if course.TeacherID != teacherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权导出该课程数据"})
		return
	}

	// Get all KPs for this course
	var kps []model.KnowledgePoint
	h.DB.Joins("JOIN chapters ON chapters.id = knowledge_points.chapter_id").
		Where("chapters.course_id = ?", courseID).
		Order("chapters.sort_order, knowledge_points.id").
		Find(&kps)

	if len(kps) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "该课程暂无知识点数据"})
		return
	}

	// Collect all student IDs who have mastery records for these KPs
	kpIDs := make([]uint, len(kps))
	for i, kp := range kps {
		kpIDs[i] = kp.ID
	}

	var masteryRecords []model.StudentKPMastery
	h.DB.Where("kp_id IN ?", kpIDs).Find(&masteryRecords)

	// Build student set
	studentIDs := make(map[uint]bool)
	for _, m := range masteryRecords {
		studentIDs[m.StudentID] = true
	}

	// Build mastery lookup: studentID -> kpID -> mastery
	masteryMap := make(map[uint]map[uint]model.StudentKPMastery)
	for _, m := range masteryRecords {
		if masteryMap[m.StudentID] == nil {
			masteryMap[m.StudentID] = make(map[uint]model.StudentKPMastery)
		}
		masteryMap[m.StudentID][m.KPID] = m
	}

	// Get student names
	studentIDList := make([]uint, 0, len(studentIDs))
	for id := range studentIDs {
		studentIDList = append(studentIDList, id)
	}
	var students []model.User
	h.DB.Select("id, display_name").Where("id IN ?", studentIDList).Find(&students)
	studentNameMap := make(map[uint]string)
	for _, s := range students {
		studentNameMap[s.ID] = s.DisplayName
	}

	// Set CSV headers
	ts := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("mastery_%d_%s.csv", courseID, ts)
	setCSVHeaders(c, filename)

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	// Build header: 学生ID, 学生姓名, KP1名称, KP2名称, ..., 平均掌握度
	header := []string{"学生ID", "学生姓名"}
	for _, kp := range kps {
		header = append(header, kp.Title)
	}
	header = append(header, "平均掌握度", "尝试总次数", "正确总次数")
	writer.Write(header)

	// One row per student
	for _, sid := range studentIDList {
		row := []string{
			strconv.FormatUint(uint64(sid), 10),
			studentNameMap[sid],
		}

		totalMastery := 0.0
		totalAttempts := 0
		totalCorrect := 0
		kpCount := 0

		for _, kp := range kps {
			if m, ok := masteryMap[sid][kp.ID]; ok {
				row = append(row, fmt.Sprintf("%.2f", m.MasteryScore))
				totalMastery += m.MasteryScore
				totalAttempts += m.AttemptCount
				totalCorrect += m.CorrectCount
				kpCount++
			} else {
				row = append(row, "-")
			}
		}

		avgMastery := 0.0
		if kpCount > 0 {
			avgMastery = totalMastery / float64(kpCount)
		}
		row = append(row,
			fmt.Sprintf("%.2f", avgMastery),
			strconv.Itoa(totalAttempts),
			strconv.Itoa(totalCorrect),
		)
		writer.Write(row)
	}
}

// -- Export Error Notebook ------------------------------------

// ExportErrorNotebook exports error notebook entries for a course as CSV.
// GET /api/v1/export/courses/:id/error-notebook
func (h *ExportHandler) ExportErrorNotebook(c *gin.Context) {
	courseID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的课程 ID"})
		return
	}

	// Verify course exists and teacher owns it
	var course model.Course
	if err := h.DB.First(&course, courseID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "课程不存在"})
		return
	}
	teacherID := middleware.GetUserID(c)
	if course.TeacherID != teacherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权导出该课程数据"})
		return
	}

	// Get all KP IDs for this course
	var kpIDs []uint
	h.DB.Model(&model.KnowledgePoint{}).
		Joins("JOIN chapters ON chapters.id = knowledge_points.chapter_id").
		Where("chapters.course_id = ?", courseID).
		Pluck("knowledge_points.id", &kpIDs)

	if len(kpIDs) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "该课程暂无知识点数据"})
		return
	}

	// Fetch error notebook entries
	var entries []model.ErrorNotebookEntry
	h.DB.Where("kp_id IN ?", kpIDs).Order("archived_at DESC").Find(&entries)

	// Set CSV headers
	ts := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("error_notebook_%d_%s.csv", courseID, ts)
	setCSVHeaders(c, filename)

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	writer.Write([]string{
		"ID", "学生ID", "学生姓名", "知识点ID", "知识点名称",
		"学生错误回答", "AI引导回复", "错误类型", "出错时掌握度",
		"已解决", "解决时间", "归档时间",
	})

	// Batch-load student and KP names
	studentIDSet := make(map[uint]bool)
	kpIDSet := make(map[uint]bool)
	for _, e := range entries {
		studentIDSet[e.StudentID] = true
		kpIDSet[e.KPID] = true
	}

	studentNames := make(map[uint]string)
	if len(studentIDSet) > 0 {
		ids := mapKeys(studentIDSet)
		var users []model.User
		h.DB.Select("id, display_name").Where("id IN ?", ids).Find(&users)
		for _, u := range users {
			studentNames[u.ID] = u.DisplayName
		}
	}

	kpNames := make(map[uint]string)
	if len(kpIDSet) > 0 {
		ids := mapKeys(kpIDSet)
		var kps []model.KnowledgePoint
		h.DB.Select("id, title").Where("id IN ?", ids).Find(&kps)
		for _, kp := range kps {
			kpNames[kp.ID] = kp.Title
		}
	}

	errorTypeLabels := map[string]string{
		"conceptual": "概念性错误",
		"procedural": "程序性错误",
		"intuitive":  "直觉性错误",
		"unknown":    "未分类",
	}

	for _, e := range entries {
		resolvedStr := "否"
		resolvedAtStr := ""
		if e.Resolved {
			resolvedStr = "是"
			if e.ResolvedAt != nil {
				resolvedAtStr = e.ResolvedAt.Format("2006-01-02 15:04:05")
			}
		}

		errLabel := errorTypeLabels[e.ErrorType]
		if errLabel == "" {
			errLabel = e.ErrorType
		}

		writer.Write([]string{
			strconv.FormatUint(uint64(e.ID), 10),
			strconv.FormatUint(uint64(e.StudentID), 10),
			studentNames[e.StudentID],
			strconv.FormatUint(uint64(e.KPID), 10),
			kpNames[e.KPID],
			e.StudentInput,
			e.CoachGuidance,
			errLabel,
			fmt.Sprintf("%.2f", e.MasteryAtError),
			resolvedStr,
			resolvedAtStr,
			e.ArchivedAt.Format("2006-01-02 15:04:05"),
		})
	}
}

// -- Export Interaction Log -----------------------------------

// ExportInteractionLog exports AI interaction logs for a session as CSV.
// GET /api/v1/export/sessions/:id/interactions
func (h *ExportHandler) ExportInteractionLog(c *gin.Context) {
	sessionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的会话 ID"})
		return
	}

	// Verify session exists
	var session model.StudentSession
	if err := h.DB.First(&session, sessionID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "会话不存在"})
		return
	}

	// Verify teacher owns the activity for this session
	var activity model.LearningActivity
	if err := h.DB.First(&activity, session.ActivityID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "关联活动不存在"})
		return
	}
	teacherID := middleware.GetUserID(c)
	if activity.TeacherID != teacherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权导出该会话数据"})
		return
	}

	// Fetch interactions
	var interactions []model.Interaction
	h.DB.Where("session_id = ?", sessionID).Order("created_at ASC").Find(&interactions)

	// Set CSV headers
	ts := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("interactions_%d_%s.csv", sessionID, ts)
	setCSVHeaders(c, filename)

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	writer.Write([]string{
		"ID", "角色", "内容", "技能ID", "Token用量",
		"忠实度", "可操作性", "答案克制度",
		"上下文精度", "上下文召回", "评估状态", "时间",
	})

	roleLabels := map[string]string{
		"student": "学生",
		"coach":   "AI教练",
		"system":  "系统",
	}

	for _, i := range interactions {
		roleLabel := roleLabels[i.Role]
		if roleLabel == "" {
			roleLabel = i.Role
		}

		faithfulness := "-"
		if i.FaithfulnessScore != nil {
			faithfulness = fmt.Sprintf("%.3f", *i.FaithfulnessScore)
		}
		actionability := "-"
		if i.ActionabilityScore != nil {
			actionability = fmt.Sprintf("%.3f", *i.ActionabilityScore)
		}
		restraint := "-"
		if i.AnswerRestraintScore != nil {
			restraint = fmt.Sprintf("%.3f", *i.AnswerRestraintScore)
		}
		precision := "-"
		if i.ContextPrecision != nil {
			precision = fmt.Sprintf("%.3f", *i.ContextPrecision)
		}
		recall := "-"
		if i.ContextRecall != nil {
			recall = fmt.Sprintf("%.3f", *i.ContextRecall)
		}

		writer.Write([]string{
			strconv.FormatUint(uint64(i.ID), 10),
			roleLabel,
			i.Content,
			i.SkillID,
			strconv.Itoa(i.TokensUsed),
			faithfulness,
			actionability,
			restraint,
			precision,
			recall,
			i.EvalStatus,
			i.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
}

// -- Utility --------------------------------------------------

// mapKeys extracts keys from a map[uint]bool into a slice.
func mapKeys(m map[uint]bool) []uint {
	keys := make([]uint, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
