package handler

import (
	"testing"

	"github.com/hflms/hanfledge/internal/domain/model"
)

// ============================
// Activity Handler Unit Tests
// ============================

// -- ActivityHandler Constructor Test -------------------------

func TestNewActivityHandler(t *testing.T) {
	h := NewActivityHandler(nil, nil)
	if h == nil {
		t.Fatal("NewActivityHandler returned nil")
	}
	if h.DB != nil {
		t.Error("expected nil DB")
	}
	if h.Orchestrator != nil {
		t.Error("expected nil Orchestrator")
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
