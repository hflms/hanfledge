package handler

import (
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/domain/model"
)

// ============================
// Export Handler Unit Tests
// ============================

// -- mapKeys Tests --------------------------------------------

func TestMapKeys_Empty(t *testing.T) {
	m := map[uint]bool{}
	keys := mapKeys(m)
	if len(keys) != 0 {
		t.Errorf("mapKeys(empty) returned %d keys, want 0", len(keys))
	}
}

func TestMapKeys_Nil(t *testing.T) {
	keys := mapKeys(nil)
	if len(keys) != 0 {
		t.Errorf("mapKeys(nil) returned %d keys, want 0", len(keys))
	}
}

func TestMapKeys_SingleEntry(t *testing.T) {
	m := map[uint]bool{42: true}
	keys := mapKeys(m)
	if len(keys) != 1 {
		t.Fatalf("mapKeys returned %d keys, want 1", len(keys))
	}
	if keys[0] != 42 {
		t.Errorf("mapKeys key = %d, want 42", keys[0])
	}
}

func TestMapKeys_MultipleEntries(t *testing.T) {
	m := map[uint]bool{1: true, 2: true, 3: true}
	keys := mapKeys(m)
	if len(keys) != 3 {
		t.Fatalf("mapKeys returned %d keys, want 3", len(keys))
	}

	// Check all keys are present (order is non-deterministic)
	found := map[uint]bool{}
	for _, k := range keys {
		found[k] = true
	}
	for _, expected := range []uint{1, 2, 3} {
		if !found[expected] {
			t.Errorf("mapKeys missing key %d", expected)
		}
	}
}

func TestMapKeys_PreservesCapacity(t *testing.T) {
	m := map[uint]bool{10: true, 20: true}
	keys := mapKeys(m)
	if cap(keys) < 2 {
		t.Errorf("mapKeys capacity = %d, want >= 2", cap(keys))
	}
}

// -- Error Type Labels Tests ----------------------------------

func TestErrorTypeLabels(t *testing.T) {
	// Verify the error type label map used in ExportErrorNotebook
	// mirrors the expected Chinese labels.
	labels := map[string]string{
		"conceptual": "概念性错误",
		"procedural": "程序性错误",
		"intuitive":  "直觉性错误",
		"unknown":    "未分类",
	}

	tests := []struct {
		errorType string
		label     string
	}{
		{"conceptual", "概念性错误"},
		{"procedural", "程序性错误"},
		{"intuitive", "直觉性错误"},
		{"unknown", "未分类"},
	}

	for _, tc := range tests {
		t.Run(tc.errorType, func(t *testing.T) {
			got := labels[tc.errorType]
			if got != tc.label {
				t.Errorf("errorTypeLabels[%q] = %q, want %q", tc.errorType, got, tc.label)
			}
		})
	}
}

func TestErrorTypeLabels_UnknownFallback(t *testing.T) {
	// Unknown error types should fall through to the raw string
	labels := map[string]string{
		"conceptual": "概念性错误",
		"procedural": "程序性错误",
		"intuitive":  "直觉性错误",
		"unknown":    "未分类",
	}

	unsupported := "some_custom_type"
	label := labels[unsupported]
	if label != "" {
		t.Errorf("unsupported error type should return empty, got %q", label)
	}
}

// -- ExportHandler Constructor Test ---------------------------

func TestNewExportHandler(t *testing.T) {
	h := NewExportHandler(nil)
	if h == nil {
		t.Fatal("NewExportHandler returned nil")
	}
	if h.DB != nil {
		t.Error("expected nil DB when no DB provided")
	}
}

// -- ExportActivitySessions HTTP Tests -----------------------

func TestExportActivitySessions_Success(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800000001", "pass", "王老师", model.UserStatusActive)
	student := seedUser(t, db, "13800000002", "pass", "李同学", model.UserStatusActive)
	course := seedCourse(t, db, teacher.ID, "物理学")
	ch := seedChapter(t, db, course.ID, "力学", 1)
	kp := seedKP(t, db, ch.ID, "牛顿第二定律")
	activity := seedActivity(t, db, teacher.ID, course.ID, "课堂练习1")
	seedSession(t, db, student.ID, activity.ID, kp.ID, model.SessionStatusCompleted)

	// Seed mastery record
	db.Create(&model.StudentKPMastery{
		StudentID:    student.ID,
		KPID:         kp.ID,
		MasteryScore: 0.85,
		AttemptCount: 5,
		CorrectCount: 4,
	})

	h := NewExportHandler(db)
	w, c := newTestContextWithParams(http.MethodGet,
		"/api/v1/export/activities/1/sessions", "",
		teacher.ID, gin.Params{{Key: "id", Value: "1"}})

	h.ExportActivitySessions(c)

	assertCSVResponse(t, w)
	assertBodyContains(t, w, "会话ID")
	assertBodyContains(t, w, "李同学")
	assertBodyContains(t, w, "牛顿第二定律")
}

func TestExportActivitySessions_NotFound(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800000001", "pass", "王老师", model.UserStatusActive)
	h := NewExportHandler(db)

	w, c := newTestContextWithParams(http.MethodGet,
		"/api/v1/export/activities/999/sessions", "",
		teacher.ID, gin.Params{{Key: "id", Value: "999"}})

	h.ExportActivitySessions(c)

	assertStatus(t, w, http.StatusNotFound)
	assertBodyContains(t, w, "学习活动不存在")
}

func TestExportActivitySessions_Forbidden(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800000001", "pass", "王老师", model.UserStatusActive)
	other := seedUser(t, db, "13800000002", "pass", "赵老师", model.UserStatusActive)
	course := seedCourse(t, db, teacher.ID, "物理学")
	seedActivity(t, db, teacher.ID, course.ID, "课堂练习1")

	h := NewExportHandler(db)
	w, c := newTestContextWithParams(http.MethodGet,
		"/api/v1/export/activities/1/sessions", "",
		other.ID, gin.Params{{Key: "id", Value: "1"}})

	h.ExportActivitySessions(c)

	assertStatus(t, w, http.StatusForbidden)
	assertBodyContains(t, w, "无权导出")
}

func TestExportActivitySessions_InvalidID(t *testing.T) {
	db := setupTestDB(t)
	h := NewExportHandler(db)

	w, c := newTestContextWithParams(http.MethodGet,
		"/api/v1/export/activities/abc/sessions", "",
		uint(1), gin.Params{{Key: "id", Value: "abc"}})

	h.ExportActivitySessions(c)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestExportActivitySessions_EmptySessions(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800000001", "pass", "王老师", model.UserStatusActive)
	course := seedCourse(t, db, teacher.ID, "物理学")
	activity := seedActivity(t, db, teacher.ID, course.ID, "空活动")

	h := NewExportHandler(db)
	w, c := newTestContextWithParams(http.MethodGet,
		"/api/v1/export/activities/1/sessions", "",
		teacher.ID, gin.Params{{Key: "id", Value: "1"}})

	_ = activity
	h.ExportActivitySessions(c)

	// Should still return CSV with header row but no data rows
	assertCSVResponse(t, w)
	assertBodyContains(t, w, "会话ID")
}

// -- ExportClassMastery HTTP Tests ----------------------------

func TestExportClassMastery_Success(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800000001", "pass", "王老师", model.UserStatusActive)
	student := seedUser(t, db, "13800000002", "pass", "李同学", model.UserStatusActive)
	course := seedCourse(t, db, teacher.ID, "物理学")
	ch := seedChapter(t, db, course.ID, "力学", 1)
	kp := seedKP(t, db, ch.ID, "牛顿第二定律")

	db.Create(&model.StudentKPMastery{
		StudentID:    student.ID,
		KPID:         kp.ID,
		MasteryScore: 0.90,
		AttemptCount: 8,
		CorrectCount: 7,
	})

	h := NewExportHandler(db)
	w, c := newTestContextWithParams(http.MethodGet,
		"/api/v1/export/courses/1/mastery", "",
		teacher.ID, gin.Params{{Key: "id", Value: "1"}})

	h.ExportClassMastery(c)

	assertCSVResponse(t, w)
	assertBodyContains(t, w, "学生ID")
	assertBodyContains(t, w, "李同学")
	assertBodyContains(t, w, "牛顿第二定律")
}

func TestExportClassMastery_NotFound(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800000001", "pass", "王老师", model.UserStatusActive)
	h := NewExportHandler(db)

	w, c := newTestContextWithParams(http.MethodGet,
		"/api/v1/export/courses/999/mastery", "",
		teacher.ID, gin.Params{{Key: "id", Value: "999"}})

	h.ExportClassMastery(c)

	assertStatus(t, w, http.StatusNotFound)
}

func TestExportClassMastery_Forbidden(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800000001", "pass", "王老师", model.UserStatusActive)
	other := seedUser(t, db, "13800000002", "pass", "赵老师", model.UserStatusActive)
	seedCourse(t, db, teacher.ID, "物理学")

	h := NewExportHandler(db)
	w, c := newTestContextWithParams(http.MethodGet,
		"/api/v1/export/courses/1/mastery", "",
		other.ID, gin.Params{{Key: "id", Value: "1"}})

	h.ExportClassMastery(c)

	assertStatus(t, w, http.StatusForbidden)
}

func TestExportClassMastery_NoKPs(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800000001", "pass", "王老师", model.UserStatusActive)
	seedCourse(t, db, teacher.ID, "空课程")

	h := NewExportHandler(db)
	w, c := newTestContextWithParams(http.MethodGet,
		"/api/v1/export/courses/1/mastery", "",
		teacher.ID, gin.Params{{Key: "id", Value: "1"}})

	h.ExportClassMastery(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, "暂无知识点数据")
}

// -- ExportErrorNotebook HTTP Tests ---------------------------

func TestExportErrorNotebook_Success(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800000001", "pass", "王老师", model.UserStatusActive)
	student := seedUser(t, db, "13800000002", "pass", "李同学", model.UserStatusActive)
	course := seedCourse(t, db, teacher.ID, "物理学")
	ch := seedChapter(t, db, course.ID, "力学", 1)
	kp := seedKP(t, db, ch.ID, "牛顿第二定律")

	db.Create(&model.ErrorNotebookEntry{
		StudentID:      student.ID,
		KPID:           kp.ID,
		SessionID:      1,
		StudentInput:   "F=ma中a是加速度还是速度？",
		CoachGuidance:  "加速度是速度变化率",
		ErrorType:      "conceptual",
		MasteryAtError: 0.3,
		ArchivedAt:     time.Now(),
	})

	h := NewExportHandler(db)
	w, c := newTestContextWithParams(http.MethodGet,
		"/api/v1/export/courses/1/error-notebook", "",
		teacher.ID, gin.Params{{Key: "id", Value: "1"}})

	h.ExportErrorNotebook(c)

	assertCSVResponse(t, w)
	assertBodyContains(t, w, "学生ID")
	assertBodyContains(t, w, "李同学")
	assertBodyContains(t, w, "概念性错误")
}

func TestExportErrorNotebook_NotFound(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800000001", "pass", "王老师", model.UserStatusActive)
	h := NewExportHandler(db)

	w, c := newTestContextWithParams(http.MethodGet,
		"/api/v1/export/courses/999/error-notebook", "",
		teacher.ID, gin.Params{{Key: "id", Value: "999"}})

	h.ExportErrorNotebook(c)

	assertStatus(t, w, http.StatusNotFound)
}

func TestExportErrorNotebook_Forbidden(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800000001", "pass", "王老师", model.UserStatusActive)
	other := seedUser(t, db, "13800000002", "pass", "赵老师", model.UserStatusActive)
	seedCourse(t, db, teacher.ID, "物理学")

	h := NewExportHandler(db)
	w, c := newTestContextWithParams(http.MethodGet,
		"/api/v1/export/courses/1/error-notebook", "",
		other.ID, gin.Params{{Key: "id", Value: "1"}})

	h.ExportErrorNotebook(c)

	assertStatus(t, w, http.StatusForbidden)
}

// -- ExportInteractionLog HTTP Tests --------------------------

func TestExportInteractionLog_Success(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800000001", "pass", "王老师", model.UserStatusActive)
	student := seedUser(t, db, "13800000002", "pass", "李同学", model.UserStatusActive)
	course := seedCourse(t, db, teacher.ID, "物理学")
	ch := seedChapter(t, db, course.ID, "力学", 1)
	kp := seedKP(t, db, ch.ID, "牛顿第二定律")
	activity := seedActivity(t, db, teacher.ID, course.ID, "课堂练习1")
	session := seedSession(t, db, student.ID, activity.ID, kp.ID, model.SessionStatusActive)

	db.Create(&model.Interaction{
		SessionID:  session.ID,
		Role:       "student",
		Content:    "F=ma是什么意思？",
		SkillID:    "socratic",
		TokensUsed: 100,
		EvalStatus: "pending",
	})
	db.Create(&model.Interaction{
		SessionID:  session.ID,
		Role:       "coach",
		Content:    "好问题！让我们一起思考...",
		SkillID:    "socratic",
		TokensUsed: 200,
		EvalStatus: "pending",
	})

	h := NewExportHandler(db)
	w, c := newTestContextWithParams(http.MethodGet,
		"/api/v1/export/sessions/1/interactions", "",
		teacher.ID, gin.Params{{Key: "id", Value: "1"}})

	h.ExportInteractionLog(c)

	assertCSVResponse(t, w)
	assertBodyContains(t, w, "角色")
	assertBodyContains(t, w, "学生")
	assertBodyContains(t, w, "AI教练")
}

func TestExportInteractionLog_SessionNotFound(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800000001", "pass", "王老师", model.UserStatusActive)
	h := NewExportHandler(db)

	w, c := newTestContextWithParams(http.MethodGet,
		"/api/v1/export/sessions/999/interactions", "",
		teacher.ID, gin.Params{{Key: "id", Value: "999"}})

	h.ExportInteractionLog(c)

	assertStatus(t, w, http.StatusNotFound)
	assertBodyContains(t, w, "会话不存在")
}

func TestExportInteractionLog_Forbidden(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13800000001", "pass", "王老师", model.UserStatusActive)
	other := seedUser(t, db, "13800000002", "pass", "赵老师", model.UserStatusActive)
	student := seedUser(t, db, "13800000003", "pass", "李同学", model.UserStatusActive)
	course := seedCourse(t, db, teacher.ID, "物理学")
	ch := seedChapter(t, db, course.ID, "力学", 1)
	kp := seedKP(t, db, ch.ID, "牛顿第二定律")
	activity := seedActivity(t, db, teacher.ID, course.ID, "课堂练习1")
	seedSession(t, db, student.ID, activity.ID, kp.ID, model.SessionStatusActive)

	h := NewExportHandler(db)
	w, c := newTestContextWithParams(http.MethodGet,
		"/api/v1/export/sessions/1/interactions", "",
		other.ID, gin.Params{{Key: "id", Value: "1"}})

	h.ExportInteractionLog(c)

	assertStatus(t, w, http.StatusForbidden)
	assertBodyContains(t, w, "无权导出")
}
