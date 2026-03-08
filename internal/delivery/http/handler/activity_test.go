package handler

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/domain/model"
)

// ============================
// Activity Handler Unit Tests
// ============================

// -- ActivityHandler Constructor Test -------------------------

func TestNewActivityHandler(t *testing.T) {
	h := NewActivityHandler(nil, nil, nil, nil)
	if h == nil {
		t.Fatal("NewActivityHandler returned nil")
	}
	if h.DB != nil {
		t.Error("expected nil DB")
	}
	if h.Orchestrator != nil {
		t.Error("expected nil Orchestrator")
	}
	if h.EventBus != nil {
		t.Error("expected nil EventBus")
	}
}

// -- CreateActivityRequest Fields Test ------------------------

func TestCreateActivityRequestDefaults(t *testing.T) {
	req := CreateActivityRequest{}
	if req.CourseID != 0 {
		t.Errorf("CourseID = %d, want 0", req.CourseID)
	}
	if req.Title != "" {
		t.Error("Title should be empty by default")
	}
	if req.KPIDS != nil {
		t.Error("KPIDS should be nil by default")
	}
	if req.SkillConfig != nil {
		t.Error("SkillConfig should be nil by default")
	}
	if req.Deadline != nil {
		t.Error("Deadline should be nil by default")
	}
	if req.AllowRetry != nil {
		t.Error("AllowRetry should be nil by default")
	}
	if req.MaxAttempts != nil {
		t.Error("MaxAttempts should be nil by default")
	}
	if req.ClassIDs != nil {
		t.Error("ClassIDs should be nil by default")
	}
}

func TestCreateActivityRequestWithValues(t *testing.T) {
	allowRetry := true
	maxAttempts := 3
	deadline := "2026-12-31T23:59:59Z"

	req := CreateActivityRequest{
		CourseID:    1,
		Title:       "力学基础练习",
		KPIDS:       []uint{1, 2, 3},
		SkillConfig: map[string]interface{}{"scaffold": "high"},
		Deadline:    &deadline,
		AllowRetry:  &allowRetry,
		MaxAttempts: &maxAttempts,
		ClassIDs:    []uint{10, 20},
	}

	if req.CourseID != 1 {
		t.Errorf("CourseID = %d, want 1", req.CourseID)
	}
	if req.Title != "力学基础练习" {
		t.Errorf("Title = %q, want %q", req.Title, "力学基础练习")
	}
	if len(req.KPIDS) != 3 {
		t.Errorf("KPIDS count = %d, want 3", len(req.KPIDS))
	}
	if *req.AllowRetry != true {
		t.Error("AllowRetry should be true")
	}
	if *req.MaxAttempts != 3 {
		t.Errorf("MaxAttempts = %d, want 3", *req.MaxAttempts)
	}
	if len(req.ClassIDs) != 2 {
		t.Errorf("ClassIDs count = %d, want 2", len(req.ClassIDs))
	}
}

// -- ActivityStatus Constants Tests ---------------------------

func TestActivityStatusConstants(t *testing.T) {
	tests := []struct {
		name   string
		status model.ActivityStatus
		want   string
	}{
		{"draft", model.ActivityStatusDraft, "draft"},
		{"published", model.ActivityStatusPublished, "published"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.status) != tc.want {
				t.Errorf("ActivityStatus = %q, want %q", tc.status, tc.want)
			}
		})
	}
}

// -- SessionStatus Constants Tests ----------------------------

func TestSessionStatusConstants(t *testing.T) {
	tests := []struct {
		name   string
		status model.SessionStatus
		want   string
	}{
		{"active", model.SessionStatusActive, "active"},
		{"completed", model.SessionStatusCompleted, "completed"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.status) != tc.want {
				t.Errorf("SessionStatus = %q, want %q", tc.status, tc.want)
			}
		})
	}
}

// -- Publish Validation: Only draft can be published ----------

func TestPublishValidation_OnlyDraftAllowed(t *testing.T) {
	tests := []struct {
		status      model.ActivityStatus
		publishable bool
	}{
		{model.ActivityStatusDraft, true},
		{model.ActivityStatusPublished, false},
	}

	for _, tc := range tests {
		t.Run(string(tc.status), func(t *testing.T) {
			canPublish := tc.status == model.ActivityStatusDraft
			if canPublish != tc.publishable {
				t.Errorf("status %q: canPublish = %v, want %v",
					tc.status, canPublish, tc.publishable)
			}
		})
	}
}

// -- PreviewActivity Handler Tests (Sandbox) ------------------

func TestPreviewActivity_CreatesSession(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13900001001", "pass", "王老师", model.UserStatusActive)
	course := seedCourse(t, db, teacher.ID, "力学基础")
	act := seedActivity(t, db, teacher.ID, course.ID, "牛顿定律")
	// Set KPIDS so the preview can parse a target KP
	db.Model(&act).Update("kp_ids", "[1,2]")

	h := NewActivityHandler(db, nil, nil, nil)

	w, c := newTestContextWithParams("POST", "/api/v1/activities/1/preview", "",
		teacher.ID, gin.Params{{Key: "id", Value: "1"}})
	h.PreviewActivity(c)

	assertStatus(t, w, 201)
	assertBodyContains(t, w, "session_id")
	assertBodyContains(t, w, "is_sandbox")

	// Verify session was created with IsSandbox=true
	var session model.StudentSession
	if err := db.Where("student_id = ? AND is_sandbox = ?", teacher.ID, true).First(&session).Error; err != nil {
		t.Fatal("sandbox session not found in database")
	}
	if session.ActivityID != act.ID {
		t.Errorf("session.ActivityID = %d, want %d", session.ActivityID, act.ID)
	}
	if session.StudentID != teacher.ID {
		t.Errorf("session.StudentID = %d, want teacher.ID=%d", session.StudentID, teacher.ID)
	}
}

func TestPreviewActivity_RequiresOwnership(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13900001002", "pass", "王老师", model.UserStatusActive)
	otherTeacher := seedUser(t, db, "13900001003", "pass", "李老师", model.UserStatusActive)
	course := seedCourse(t, db, teacher.ID, "力学基础")
	act := seedActivity(t, db, teacher.ID, course.ID, "牛顿定律")

	h := NewActivityHandler(db, nil, nil, nil)

	// otherTeacher tries to preview teacher's activity
	w, c := newTestContextWithParams("POST", "/api/v1/activities/1/preview", "",
		otherTeacher.ID, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", act.ID)}})
	h.PreviewActivity(c)

	assertStatus(t, w, 403)
	assertBodyContains(t, w, "无权预览此活动")
}

func TestPreviewActivity_ReusesExistingSandbox(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13900001004", "pass", "王老师", model.UserStatusActive)
	course := seedCourse(t, db, teacher.ID, "力学基础")
	act := seedActivity(t, db, teacher.ID, course.ID, "牛顿定律")

	h := NewActivityHandler(db, nil, nil, nil)

	// First preview — creates a session
	w1, c1 := newTestContextWithParams("POST", "/api/v1/activities/1/preview", "",
		teacher.ID, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", act.ID)}})
	h.PreviewActivity(c1)
	assertStatus(t, w1, 201)

	var resp1 map[string]interface{}
	json.Unmarshal(w1.Body.Bytes(), &resp1)
	firstSessionID := resp1["session_id"]

	// Second preview — should reuse the same session
	w2, c2 := newTestContextWithParams("POST", "/api/v1/activities/1/preview", "",
		teacher.ID, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", act.ID)}})
	h.PreviewActivity(c2)
	assertStatus(t, w2, 200) // 200, not 201 — reused existing

	var resp2 map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &resp2)
	if resp2["session_id"] != firstSessionID {
		t.Errorf("expected reused session_id=%v, got=%v", firstSessionID, resp2["session_id"])
	}
	assertBodyContains(t, w2, "已有进行中的沙盒会话")
}

func TestPreviewActivity_WorksOnDraftActivity(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13900001005", "pass", "王老师", model.UserStatusActive)
	course := seedCourse(t, db, teacher.ID, "力学基础")
	act := seedActivity(t, db, teacher.ID, course.ID, "草稿活动")
	// Activity starts as draft (no status set in seedActivity, verify)
	db.Model(&act).Update("status", model.ActivityStatusDraft)

	h := NewActivityHandler(db, nil, nil, nil)

	w, c := newTestContextWithParams("POST", "/api/v1/activities/1/preview", "",
		teacher.ID, gin.Params{{Key: "id", Value: fmt.Sprintf("%d", act.ID)}})
	h.PreviewActivity(c)

	// Should succeed — preview works on drafts
	assertStatus(t, w, 201)
	assertBodyContains(t, w, "session_id")
}

func TestPreviewActivity_InvalidID(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13900001006", "pass", "王老师", model.UserStatusActive)

	h := NewActivityHandler(db, nil, nil, nil)

	w, c := newTestContextWithParams("POST", "/api/v1/activities/abc/preview", "",
		teacher.ID, gin.Params{{Key: "id", Value: "abc"}})
	h.PreviewActivity(c)

	assertStatus(t, w, 400)
	assertBodyContains(t, w, "无效的活动 ID")
}

func TestPreviewActivity_NotFound(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13900001007", "pass", "王老师", model.UserStatusActive)

	h := NewActivityHandler(db, nil, nil, nil)

	w, c := newTestContextWithParams("POST", "/api/v1/activities/999/preview", "",
		teacher.ID, gin.Params{{Key: "id", Value: "999"}})
	h.PreviewActivity(c)

	assertStatus(t, w, 404)
	assertBodyContains(t, w, "学习活动不存在")
}

func TestSandboxSessionsExcludedFromDashboard(t *testing.T) {
	db := setupTestDB(t)
	teacher := seedUser(t, db, "13900001008", "pass", "王老师", model.UserStatusActive)
	student := seedUser(t, db, "13900001009", "pass", "张同学", model.UserStatusActive)
	course := seedCourse(t, db, teacher.ID, "力学基础")
	act := seedActivity(t, db, teacher.ID, course.ID, "牛顿定律")

	// Create a normal session (student)
	seedSession(t, db, student.ID, act.ID, 0, model.SessionStatusActive)

	// Create a sandbox session (teacher preview)
	sandboxSession := model.StudentSession{
		StudentID:  teacher.ID,
		ActivityID: act.ID,
		Scaffold:   model.ScaffoldHigh,
		IsSandbox:  true,
		Status:     model.SessionStatusActive,
	}
	db.Create(&sandboxSession)

	// Verify that only 1 non-sandbox session exists when querying with sandbox exclusion
	var count int64
	db.Model(&model.StudentSession{}).
		Where("activity_id = ? AND is_sandbox = ?", act.ID, false).
		Count(&count)
	if count != 1 {
		t.Errorf("expected 1 non-sandbox session, got %d", count)
	}

	// Verify total count is 2 (with sandbox)
	db.Model(&model.StudentSession{}).
		Where("activity_id = ?", act.ID).
		Count(&count)
	if count != 2 {
		t.Errorf("expected 2 total sessions, got %d", count)
	}
}
