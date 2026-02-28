package handler

import (
	"testing"

	"github.com/hflms/hanfledge/internal/domain/model"
)

// ============================
// User Handler Unit Tests
// ============================

// -- UserHandler Constructor Test -----------------------------

func TestNewUserHandler(t *testing.T) {
	h := NewUserHandler(nil)
	if h == nil {
		t.Fatal("NewUserHandler returned nil")
	}
	if h.DB != nil {
		t.Error("expected nil DB when no DB provided")
	}
}

// -- CreateUserRequest Fields Test ----------------------------

func TestCreateUserRequestDefaults(t *testing.T) {
	req := CreateUserRequest{}
	if req.Phone != "" {
		t.Error("Phone should be empty by default")
	}
	if req.Password != "" {
		t.Error("Password should be empty by default")
	}
	if req.DisplayName != "" {
		t.Error("DisplayName should be empty by default")
	}
	if req.Email != nil {
		t.Error("Email should be nil by default")
	}
	if req.SchoolID != nil {
		t.Error("SchoolID should be nil by default")
	}
	if req.RoleName != "" {
		t.Error("RoleName should be empty by default")
	}
}

func TestCreateUserRequestWithValues(t *testing.T) {
	email := "teacher@school.edu"
	schoolID := uint(1)
	req := CreateUserRequest{
		Phone:       "13800138000",
		Password:    "password123",
		DisplayName: "张老师",
		Email:       &email,
		SchoolID:    &schoolID,
		RoleName:    model.RoleTeacher,
	}

	if req.Phone != "13800138000" {
		t.Errorf("Phone = %q, want %q", req.Phone, "13800138000")
	}
	if req.DisplayName != "张老师" {
		t.Errorf("DisplayName = %q, want %q", req.DisplayName, "张老师")
	}
	if *req.Email != email {
		t.Errorf("Email = %q, want %q", *req.Email, email)
	}
	if *req.SchoolID != 1 {
		t.Errorf("SchoolID = %d, want 1", *req.SchoolID)
	}
	if req.RoleName != model.RoleTeacher {
		t.Errorf("RoleName = %q, want %q", req.RoleName, model.RoleTeacher)
	}
}

// -- RoleName Constants Tests ---------------------------------

func TestRoleNameConstants(t *testing.T) {
	tests := []struct {
		name string
		role model.RoleName
		want string
	}{
		{"sys admin", model.RoleSysAdmin, "SYS_ADMIN"},
		{"school admin", model.RoleSchoolAdmin, "SCHOOL_ADMIN"},
		{"teacher", model.RoleTeacher, "TEACHER"},
		{"student", model.RoleStudent, "STUDENT"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.role) != tc.want {
				t.Errorf("RoleName = %q, want %q", tc.role, tc.want)
			}
		})
	}
}

// -- UserStatus Constants Tests --------------------------------

func TestUserStatusConstants(t *testing.T) {
	tests := []struct {
		name   string
		status model.UserStatus
		want   string
	}{
		{"active", model.UserStatusActive, "active"},
		{"inactive", model.UserStatusInactive, "inactive"},
		{"banned", model.UserStatusBanned, "banned"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.status) != tc.want {
				t.Errorf("UserStatus = %q, want %q", tc.status, tc.want)
			}
		})
	}
}

// -- BatchCreateRequest Fields Test ---------------------------

func TestBatchCreateRequestDefaults(t *testing.T) {
	req := BatchCreateRequest{}
	if req.Users != nil {
		t.Error("Users should be nil by default")
	}
}

func TestBatchCreateRequestWithUsers(t *testing.T) {
	req := BatchCreateRequest{
		Users: []CreateUserRequest{
			{Phone: "13800000001", Password: "pass1", DisplayName: "学生一", RoleName: model.RoleStudent},
			{Phone: "13800000002", Password: "pass2", DisplayName: "学生二", RoleName: model.RoleStudent},
		},
	}
	if len(req.Users) != 2 {
		t.Errorf("Users count = %d, want 2", len(req.Users))
	}
	if req.Users[0].Phone != "13800000001" {
		t.Errorf("Users[0].Phone = %q, want %q", req.Users[0].Phone, "13800000001")
	}
}
