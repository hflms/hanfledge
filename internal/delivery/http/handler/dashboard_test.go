package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/domain/model"
)

// ============================
// Dashboard Handler Unit Tests
// ============================

// -- DashboardHandler Constructor Test ------------------------

func TestNewDashboardHandler(t *testing.T) {
	h := NewDashboardHandler(nil)
	if h == nil {
		t.Fatal("NewDashboardHandler returned nil")
	}
	if h.DB != nil {
		t.Error("expected nil DB when no DB provided")
	}
}

// -- KnowledgeRadarResponse Fields Test -----------------------

func TestKnowledgeRadarResponseDefaults(t *testing.T) {
	resp := KnowledgeRadarResponse{}
	if resp.CourseID != 0 {
		t.Errorf("CourseID = %d, want 0", resp.CourseID)
	}
	if resp.CourseTitle != "" {
		t.Error("CourseTitle should be empty by default")
	}
	if resp.Labels != nil {
		t.Error("Labels should be nil by default")
	}
	if resp.Values != nil {
		t.Error("Values should be nil by default")
	}
	if resp.StudentCount != 0 {
		t.Errorf("StudentCount = %d, want 0", resp.StudentCount)
	}
}

func TestKnowledgeRadarResponseWithData(t *testing.T) {
	resp := KnowledgeRadarResponse{
		CourseID:     1,
		CourseTitle:  "物理学基础",
		Labels:       []string{"力学", "电磁学", "光学"},
		Values:       []float64{0.8, 0.6, 0.9},
		StudentCount: 30,
	}
	if len(resp.Labels) != 3 {
		t.Errorf("Labels count = %d, want 3", len(resp.Labels))
	}
	if len(resp.Values) != 3 {
		t.Errorf("Values count = %d, want 3", len(resp.Values))
	}
	if resp.StudentCount != 30 {
		t.Errorf("StudentCount = %d, want 30", resp.StudentCount)
	}
}

// -- CompletionRate Computation Tests -------------------------

func TestCompletionRateCalculation(t *testing.T) {
	tests := []struct {
		name              string
		totalSessions     int
		completedSessions int
		expectedRate      float64
	}{
		{"no sessions", 0, 0, 0.0},
		{"all completed", 10, 10, 100.0},
		{"none completed", 10, 0, 0.0},
		{"half completed", 10, 5, 50.0},
		{"one third", 30, 10, 100.0 / 3.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rate := 0.0
			if tc.totalSessions > 0 {
				rate = float64(tc.completedSessions) / float64(tc.totalSessions) * 100
			}
			// Use approximate comparison for floating point
			diff := rate - tc.expectedRate
			if diff < 0 {
				diff = -diff
			}
			if diff > 0.001 {
				t.Errorf("completionRate = %f, want %f", rate, tc.expectedRate)
			}
		})
	}
}

// -- AvgDuration Computation Tests ----------------------------

func TestAvgDurationCalculation(t *testing.T) {
	tests := []struct {
		name              string
		completedDuration float64
		completedSessions int
		expectedAvg       float64
	}{
		{"no completed sessions", 0, 0, 0.0},
		{"single session", 30.0, 1, 30.0},
		{"multiple sessions", 90.0, 3, 30.0},
		{"fractional average", 100.0, 3, 100.0 / 3.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			avg := 0.0
			if tc.completedSessions > 0 {
				avg = tc.completedDuration / float64(tc.completedSessions)
			}
			diff := avg - tc.expectedAvg
			if diff < 0 {
				diff = -diff
			}
			if diff > 0.001 {
				t.Errorf("avgDuration = %f, want %f", avg, tc.expectedAvg)
			}
		})
	}
}

// -- AvgMastery Computation Tests -----------------------------

func TestAvgMasteryCalculation(t *testing.T) {
	tests := []struct {
		name          string
		masteryScores []float64
		expectedAvg   float64
	}{
		{"empty", nil, 0.0},
		{"single score", []float64{0.8}, 0.8},
		{"multiple scores", []float64{0.6, 0.8, 1.0}, 0.8},
		{"all zero", []float64{0.0, 0.0, 0.0}, 0.0},
		{"all perfect", []float64{1.0, 1.0, 1.0}, 1.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			avg := 0.0
			if len(tc.masteryScores) > 0 {
				total := 0.0
				for _, s := range tc.masteryScores {
					total += s
				}
				avg = total / float64(len(tc.masteryScores))
			}
			diff := avg - tc.expectedAvg
			if diff < 0 {
				diff = -diff
			}
			if diff > 0.001 {
				t.Errorf("avgMastery = %f, want %f", avg, tc.expectedAvg)
			}
		})
	}
}

// -- ActivitySessionStats Fields Test -------------------------

func TestActivitySessionStatsDefaults(t *testing.T) {
	stats := ActivitySessionStats{}
	if stats.ActivityID != 0 {
		t.Errorf("ActivityID = %d, want 0", stats.ActivityID)
	}
	if stats.TotalSessions != 0 {
		t.Errorf("TotalSessions = %d, want 0", stats.TotalSessions)
	}
	if stats.CompletionRate != 0 {
		t.Errorf("CompletionRate = %f, want 0", stats.CompletionRate)
	}
	if stats.Sessions != nil {
		t.Error("Sessions should be nil by default")
	}
}

// -- SessionSummary Fields Test --------------------------------

func TestSessionSummaryFields(t *testing.T) {
	endedAt := "2026-01-15T10:30:00Z"
	summary := SessionSummary{
		SessionID:     1,
		StudentID:     42,
		StudentName:   "张三",
		Status:        "active",
		ScaffoldLevel: "high",
		StartedAt:     "2026-01-15T10:00:00Z",
		EndedAt:       &endedAt,
		DurationMin:   30.0,
		MasteryScore:  0.75,
	}

	if summary.SessionID != 1 {
		t.Errorf("SessionID = %d, want 1", summary.SessionID)
	}
	if summary.StudentName != "张三" {
		t.Errorf("StudentName = %q, want %q", summary.StudentName, "张三")
	}
	if summary.EndedAt == nil {
		t.Fatal("EndedAt should not be nil")
	}
	if *summary.EndedAt != endedAt {
		t.Errorf("EndedAt = %q, want %q", *summary.EndedAt, endedAt)
	}
}

func TestSessionSummaryEndedAtOptional(t *testing.T) {
	summary := SessionSummary{
		SessionID: 1,
		Status:    "active",
	}
	if summary.EndedAt != nil {
		t.Error("EndedAt should be nil for active session")
	}
}

// -- StudentMasteryItem Fields Test ---------------------------

func TestStudentMasteryItemFields(t *testing.T) {
	lastAttempt := "2026-01-15T10:00:00Z"
	item := StudentMasteryItem{
		KPID:          1,
		KPTitle:       "牛顿第二定律",
		ChapterTitle:  "力学",
		MasteryScore:  0.85,
		AttemptCount:  10,
		CorrectCount:  8,
		LastAttemptAt: &lastAttempt,
		UpdatedAt:     "2026-01-15T10:30:00Z",
	}

	if item.KPID != 1 {
		t.Errorf("KPID = %d, want 1", item.KPID)
	}
	if item.MasteryScore != 0.85 {
		t.Errorf("MasteryScore = %f, want 0.85", item.MasteryScore)
	}
	if item.AttemptCount != 10 {
		t.Errorf("AttemptCount = %d, want 10", item.AttemptCount)
	}
	if item.LastAttemptAt == nil {
		t.Fatal("LastAttemptAt should not be nil")
	}
}

// -- ErrorNotebookResponse Fields Test ------------------------

func TestErrorNotebookResponseDefaults(t *testing.T) {
	resp := ErrorNotebookResponse{}
	if resp.TotalCount != 0 {
		t.Errorf("TotalCount = %d, want 0", resp.TotalCount)
	}
	if resp.UnresolvedCnt != 0 {
		t.Errorf("UnresolvedCnt = %d, want 0", resp.UnresolvedCnt)
	}
	if resp.ResolvedCnt != 0 {
		t.Errorf("ResolvedCnt = %d, want 0", resp.ResolvedCnt)
	}
	if resp.Items != nil {
		t.Error("Items should be nil by default")
	}
}

func TestErrorNotebookResponseCounts(t *testing.T) {
	resp := ErrorNotebookResponse{
		TotalCount:    10,
		UnresolvedCnt: 4,
		ResolvedCnt:   6,
	}

	if resp.TotalCount != resp.UnresolvedCnt+resp.ResolvedCnt {
		t.Errorf("TotalCount (%d) != UnresolvedCnt (%d) + ResolvedCnt (%d)",
			resp.TotalCount, resp.UnresolvedCnt, resp.ResolvedCnt)
	}
}

// -- GetKnowledgeRadar HTTP Tests ----------------------------

func TestGetKnowledgeRadar_Success(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800000001", "pass", "王老师", model.UserStatusActive)
	student := seedUser(t, db, "13800000002", "pass", "李同学", model.UserStatusActive)
	course := seedCourse(t, db, teacher.ID, "物理学")
	ch := seedChapter(t, db, course.ID, "力学", 1)
	kp1 := seedKP(t, db, ch.ID, "牛顿第一定律")
	kp2 := seedKP(t, db, ch.ID, "牛顿第二定律")

	db.Create(&model.StudentKPMastery{StudentID: student.ID, KPID: kp1.ID, MasteryScore: 0.9})
	db.Create(&model.StudentKPMastery{StudentID: student.ID, KPID: kp2.ID, MasteryScore: 0.6})

	h := NewDashboardHandler(db)
	w, c := newTestContextWithQuery(http.MethodGet,
		fmt.Sprintf("/api/v1/dashboard/knowledge-radar?course_id=%d", course.ID),
		teacher.ID)

	h.GetKnowledgeRadar(c)

	assertStatus(t, w, http.StatusOK)

	var resp KnowledgeRadarResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Labels) != 2 {
		t.Errorf("Labels count = %d, want 2", len(resp.Labels))
	}
	if resp.StudentCount != 1 {
		t.Errorf("StudentCount = %d, want 1", resp.StudentCount)
	}
}

func TestGetKnowledgeRadar_MissingCourseID(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800000001", "pass", "王老师", model.UserStatusActive)
	h := NewDashboardHandler(db)

	w, c := newTestContextWithQuery(http.MethodGet,
		"/api/v1/dashboard/knowledge-radar", teacher.ID)

	h.GetKnowledgeRadar(c)

	assertStatus(t, w, http.StatusBadRequest)
	assertBodyContains(t, w, "course_id 参数必填")
}

func TestGetKnowledgeRadar_CourseNotFound(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800000001", "pass", "王老师", model.UserStatusActive)
	h := NewDashboardHandler(db)

	w, c := newTestContextWithQuery(http.MethodGet,
		"/api/v1/dashboard/knowledge-radar?course_id=999", teacher.ID)

	h.GetKnowledgeRadar(c)

	assertStatus(t, w, http.StatusNotFound)
}

func TestGetKnowledgeRadar_Forbidden(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800000001", "pass", "王老师", model.UserStatusActive)
	other := seedUser(t, db, "13800000002", "pass", "赵老师", model.UserStatusActive)
	course := seedCourse(t, db, teacher.ID, "物理学")
	h := NewDashboardHandler(db)

	w, c := newTestContextWithQuery(http.MethodGet,
		fmt.Sprintf("/api/v1/dashboard/knowledge-radar?course_id=%d", course.ID),
		other.ID)

	h.GetKnowledgeRadar(c)

	assertStatus(t, w, http.StatusForbidden)
}

func TestGetKnowledgeRadar_EmptyKPs(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800000001", "pass", "王老师", model.UserStatusActive)
	course := seedCourse(t, db, teacher.ID, "空课程")
	h := NewDashboardHandler(db)

	w, c := newTestContextWithQuery(http.MethodGet,
		fmt.Sprintf("/api/v1/dashboard/knowledge-radar?course_id=%d", course.ID),
		teacher.ID)

	h.GetKnowledgeRadar(c)

	assertStatus(t, w, http.StatusOK)

	var resp KnowledgeRadarResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Labels) != 0 {
		t.Errorf("Labels count = %d, want 0", len(resp.Labels))
	}
}

// -- GetStudentMastery HTTP Tests ----------------------------

func TestGetStudentMastery_Success(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800000001", "pass", "王老师", model.UserStatusActive)
	student := seedUser(t, db, "13800000002", "pass", "李同学", model.UserStatusActive)
	course := seedCourse(t, db, teacher.ID, "物理学")
	ch := seedChapter(t, db, course.ID, "力学", 1)
	kp := seedKP(t, db, ch.ID, "牛顿第二定律")

	now := time.Now()
	db.Create(&model.StudentKPMastery{
		StudentID:    student.ID,
		KPID:         kp.ID,
		MasteryScore: 0.75,
		AttemptCount: 5,
		CorrectCount: 3,
		UpdatedAt:    now,
	})

	h := NewDashboardHandler(db)
	w, c := newTestContextWithParams(http.MethodGet,
		fmt.Sprintf("/api/v1/students/%d/mastery", student.ID), "",
		teacher.ID,
		gin.Params{{Key: "id", Value: fmt.Sprintf("%d", student.ID)}})

	h.GetStudentMastery(c)

	assertStatus(t, w, http.StatusOK)

	var resp StudentMasteryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.StudentName != "李同学" {
		t.Errorf("StudentName = %q, want %q", resp.StudentName, "李同学")
	}
	if len(resp.Items) != 1 {
		t.Errorf("Items count = %d, want 1", len(resp.Items))
	}
}

func TestGetStudentMastery_StudentNotFound(t *testing.T) {
	db := setupTestDB(t)
	h := NewDashboardHandler(db)

	w, c := newTestContextWithParams(http.MethodGet,
		"/api/v1/students/999/mastery", "",
		uint(1), gin.Params{{Key: "id", Value: "999"}})

	h.GetStudentMastery(c)

	assertStatus(t, w, http.StatusNotFound)
	assertBodyContains(t, w, "学生不存在")
}

func TestGetStudentMastery_InvalidID(t *testing.T) {
	db := setupTestDB(t)
	h := NewDashboardHandler(db)

	w, c := newTestContextWithParams(http.MethodGet,
		"/api/v1/students/abc/mastery", "",
		uint(1), gin.Params{{Key: "id", Value: "abc"}})

	h.GetStudentMastery(c)

	assertStatus(t, w, http.StatusBadRequest)
	assertBodyContains(t, w, "无效的学生 ID")
}

// -- GetActivitySessions HTTP Tests --------------------------

func TestGetActivitySessions_Success(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800000001", "pass", "王老师", model.UserStatusActive)
	student := seedUser(t, db, "13800000002", "pass", "李同学", model.UserStatusActive)
	course := seedCourse(t, db, teacher.ID, "物理学")
	ch := seedChapter(t, db, course.ID, "力学", 1)
	kp := seedKP(t, db, ch.ID, "牛顿第二定律")
	activity := seedActivity(t, db, teacher.ID, course.ID, "课堂练习1")
	seedSession(t, db, student.ID, activity.ID, kp.ID, model.SessionStatusCompleted)
	seedSession(t, db, student.ID, activity.ID, kp.ID, model.SessionStatusActive)

	h := NewDashboardHandler(db)
	w, c := newTestContextWithParams(http.MethodGet,
		"/api/v1/activities/1/sessions", "",
		teacher.ID, gin.Params{{Key: "id", Value: "1"}})

	h.GetActivitySessions(c)

	assertStatus(t, w, http.StatusOK)

	var resp ActivitySessionStats
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.TotalSessions != 2 {
		t.Errorf("TotalSessions = %d, want 2", resp.TotalSessions)
	}
	if resp.CompletedSessions != 1 {
		t.Errorf("CompletedSessions = %d, want 1", resp.CompletedSessions)
	}
	if resp.ActiveSessions != 1 {
		t.Errorf("ActiveSessions = %d, want 1", resp.ActiveSessions)
	}
	if resp.CompletionRate != 50.0 {
		t.Errorf("CompletionRate = %f, want 50.0", resp.CompletionRate)
	}
}

func TestGetActivitySessions_NotFound(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800000001", "pass", "王老师", model.UserStatusActive)
	h := NewDashboardHandler(db)

	w, c := newTestContextWithParams(http.MethodGet,
		"/api/v1/activities/999/sessions", "",
		teacher.ID, gin.Params{{Key: "id", Value: "999"}})

	h.GetActivitySessions(c)

	assertStatus(t, w, http.StatusNotFound)
}

func TestGetActivitySessions_Forbidden(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800000001", "pass", "王老师", model.UserStatusActive)
	other := seedUser(t, db, "13800000002", "pass", "赵老师", model.UserStatusActive)
	course := seedCourse(t, db, teacher.ID, "物理学")
	seedActivity(t, db, teacher.ID, course.ID, "课堂练习1")

	h := NewDashboardHandler(db)
	w, c := newTestContextWithParams(http.MethodGet,
		"/api/v1/activities/1/sessions", "",
		other.ID, gin.Params{{Key: "id", Value: "1"}})

	h.GetActivitySessions(c)

	assertStatus(t, w, http.StatusForbidden)
}

// -- GetSelfMastery HTTP Tests --------------------------------

func TestGetSelfMastery_Success(t *testing.T) {
	db := setupTestDB(t)
	student := seedUser(t, db, "13800000002", "pass", "李同学", model.UserStatusActive)
	teacher := seedUser(t, db, "13800000001", "pass", "王老师", model.UserStatusActive)
	course := seedCourse(t, db, teacher.ID, "物理学")
	ch := seedChapter(t, db, course.ID, "力学", 1)
	kp := seedKP(t, db, ch.ID, "牛顿第二定律")

	now := time.Now()
	db.Create(&model.StudentKPMastery{
		StudentID:    student.ID,
		KPID:         kp.ID,
		MasteryScore: 0.80,
		AttemptCount: 6,
		CorrectCount: 5,
		UpdatedAt:    now,
	})

	h := NewDashboardHandler(db)
	w, c := newTestContextWithQuery(http.MethodGet,
		"/api/v1/student/mastery", student.ID)

	h.GetSelfMastery(c)

	assertStatus(t, w, http.StatusOK)

	var resp StudentMasteryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.StudentName != "李同学" {
		t.Errorf("StudentName = %q, want %q", resp.StudentName, "李同学")
	}
}

// -- GetErrorNotebook HTTP Tests ------------------------------

func TestGetErrorNotebook_Success(t *testing.T) {
	db := setupTestDB(t)
	student := seedUser(t, db, "13800000002", "pass", "李同学", model.UserStatusActive)
	teacher := seedUser(t, db, "13800000001", "pass", "王老师", model.UserStatusActive)
	course := seedCourse(t, db, teacher.ID, "物理学")
	ch := seedChapter(t, db, course.ID, "力学", 1)
	kp := seedKP(t, db, ch.ID, "牛顿第二定律")

	db.Create(&model.ErrorNotebookEntry{
		StudentID:      student.ID,
		KPID:           kp.ID,
		SessionID:      1,
		StudentInput:   "力等于质量乘速度",
		CoachGuidance:  "不是速度，是加速度哦",
		ErrorType:      "conceptual",
		MasteryAtError: 0.2,
		ArchivedAt:     time.Now(),
	})
	db.Create(&model.ErrorNotebookEntry{
		StudentID:      student.ID,
		KPID:           kp.ID,
		SessionID:      1,
		StudentInput:   "加速度方向和速度方向一致",
		CoachGuidance:  "不一定，减速时加速度和速度方向相反",
		ErrorType:      "conceptual",
		MasteryAtError: 0.3,
		Resolved:       true,
		ArchivedAt:     time.Now(),
	})

	h := NewDashboardHandler(db)
	w, c := newTestContextWithQuery(http.MethodGet,
		"/api/v1/student/error-notebook", student.ID)

	h.GetErrorNotebook(c)

	assertStatus(t, w, http.StatusOK)

	var resp ErrorNotebookResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.TotalCount != 2 {
		t.Errorf("TotalCount = %d, want 2", resp.TotalCount)
	}
	if resp.UnresolvedCnt != 1 {
		t.Errorf("UnresolvedCnt = %d, want 1", resp.UnresolvedCnt)
	}
	if resp.ResolvedCnt != 1 {
		t.Errorf("ResolvedCnt = %d, want 1", resp.ResolvedCnt)
	}
	if len(resp.Items) != 2 {
		t.Errorf("Items count = %d, want 2", len(resp.Items))
	}
}

func TestGetErrorNotebook_FilterResolved(t *testing.T) {
	db := setupTestDB(t)
	student := seedUser(t, db, "13800000002", "pass", "李同学", model.UserStatusActive)

	db.Create(&model.ErrorNotebookEntry{
		StudentID:      student.ID,
		KPID:           1,
		SessionID:      1,
		StudentInput:   "错误1",
		CoachGuidance:  "引导1",
		ErrorType:      "conceptual",
		MasteryAtError: 0.2,
		ArchivedAt:     time.Now(),
	})
	db.Create(&model.ErrorNotebookEntry{
		StudentID:      student.ID,
		KPID:           1,
		SessionID:      1,
		StudentInput:   "错误2",
		CoachGuidance:  "引导2",
		ErrorType:      "conceptual",
		MasteryAtError: 0.3,
		Resolved:       true,
		ArchivedAt:     time.Now(),
	})

	h := NewDashboardHandler(db)

	// Filter unresolved only
	w, c := newTestContextWithQuery(http.MethodGet,
		"/api/v1/student/error-notebook?resolved=false", student.ID)
	h.GetErrorNotebook(c)
	assertStatus(t, w, http.StatusOK)

	var resp ErrorNotebookResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Errorf("filtered Items count = %d, want 1", len(resp.Items))
	}
}

func TestGetErrorNotebook_EmptyNotebook(t *testing.T) {
	db := setupTestDB(t)
	student := seedUser(t, db, "13800000002", "pass", "李同学", model.UserStatusActive)
	h := NewDashboardHandler(db)

	w, c := newTestContextWithQuery(http.MethodGet,
		"/api/v1/student/error-notebook", student.ID)

	h.GetErrorNotebook(c)

	assertStatus(t, w, http.StatusOK)

	var resp ErrorNotebookResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.TotalCount != 0 {
		t.Errorf("TotalCount = %d, want 0", resp.TotalCount)
	}
}
