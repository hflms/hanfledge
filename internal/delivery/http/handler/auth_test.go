package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/hflms/hanfledge/internal/domain/model"
)

// ============================
// Auth Handler Unit Tests
// ============================

// -- AuthHandler Constructor Tests ----------------------------

func TestNewAuthHandler(t *testing.T) {
	h := NewAuthHandler(nil, "test-secret", 24, nil)
	if h == nil {
		t.Fatal("NewAuthHandler returned nil")
	}
	if h.DB != nil {
		t.Error("expected nil DB")
	}
	if h.JWTSecret != "test-secret" {
		t.Errorf("JWTSecret = %q, want %q", h.JWTSecret, "test-secret")
	}
	if h.JWTExpiry != 24 {
		t.Errorf("JWTExpiry = %d, want 24", h.JWTExpiry)
	}
}

func TestNewAuthHandler_EmptySecret(t *testing.T) {
	h := NewAuthHandler(nil, "", 0, nil)
	if h == nil {
		t.Fatal("NewAuthHandler returned nil")
	}
	if h.JWTSecret != "" {
		t.Errorf("JWTSecret = %q, want empty", h.JWTSecret)
	}
	if h.JWTExpiry != 0 {
		t.Errorf("JWTExpiry = %d, want 0", h.JWTExpiry)
	}
}

// -- LoginRequest Fields Test --------------------------------

func TestLoginRequestFields(t *testing.T) {
	req := LoginRequest{
		Phone:    "13800138000",
		Password: "123456",
	}
	if req.Phone != "13800138000" {
		t.Errorf("Phone = %q, want %q", req.Phone, "13800138000")
	}
	if req.Password != "123456" {
		t.Errorf("Password = %q, want %q", req.Password, "123456")
	}
}

func TestLoginRequestDefaults(t *testing.T) {
	req := LoginRequest{}
	if req.Phone != "" {
		t.Error("default Phone should be empty")
	}
	if req.Password != "" {
		t.Error("default Password should be empty")
	}
}

// -- LoginResponse Fields Test --------------------------------

func TestLoginResponseFields(t *testing.T) {
	resp := LoginResponse{
		Token: "jwt-token-here",
	}
	if resp.Token != "jwt-token-here" {
		t.Errorf("Token = %q, want %q", resp.Token, "jwt-token-here")
	}
}

// -- Login HTTP Tests ----------------------------------------

func TestLogin_Success(t *testing.T) {
	db := setupTestDB(t)
	seedUser(t, db, "13800138000", "password123", "张三", model.UserStatusActive)
	h := NewAuthHandler(db, "test-jwt-secret", 24, nil)

	w, c := newTestContext(http.MethodPost, "/api/v1/auth/login",
		`{"phone":"13800138000","password":"password123"}`, 0)

	h.Login(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, "token")
	assertBodyContains(t, w, "张三")
	// Password hash should never be in the response
	assertBodyNotContains(t, w, "password_hash")
}

func TestLogin_WrongPassword(t *testing.T) {
	db := setupTestDB(t)
	seedUser(t, db, "13800138000", "password123", "张三", model.UserStatusActive)
	h := NewAuthHandler(db, "test-jwt-secret", 24, nil)

	w, c := newTestContext(http.MethodPost, "/api/v1/auth/login",
		`{"phone":"13800138000","password":"wrongpassword"}`, 0)

	h.Login(c)

	assertStatus(t, w, http.StatusUnauthorized)
	assertBodyContains(t, w, "手机号或密码错误")
}

func TestLogin_UserNotFound(t *testing.T) {
	db := setupTestDB(t)
	h := NewAuthHandler(db, "test-jwt-secret", 24, nil)

	w, c := newTestContext(http.MethodPost, "/api/v1/auth/login",
		`{"phone":"19999999999","password":"password123"}`, 0)

	h.Login(c)

	assertStatus(t, w, http.StatusUnauthorized)
	assertBodyContains(t, w, "手机号或密码错误")
}

func TestLogin_BannedUser(t *testing.T) {
	db := setupTestDB(t)
	seedUser(t, db, "13800138000", "password123", "张三", model.UserStatusBanned)
	h := NewAuthHandler(db, "test-jwt-secret", 24, nil)

	w, c := newTestContext(http.MethodPost, "/api/v1/auth/login",
		`{"phone":"13800138000","password":"password123"}`, 0)

	h.Login(c)

	assertStatus(t, w, http.StatusForbidden)
	assertBodyContains(t, w, "账户已被禁用")
}

func TestLogin_MissingFields(t *testing.T) {
	db := setupTestDB(t)
	h := NewAuthHandler(db, "test-jwt-secret", 24, nil)

	tests := []struct {
		name string
		body string
	}{
		{"missing phone", `{"password":"123456"}`},
		{"missing password", `{"phone":"13800138000"}`},
		{"empty body", `{}`},
		{"invalid json", `not-json`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, c := newTestContext(http.MethodPost, "/api/v1/auth/login", tc.body, 0)
			h.Login(c)
			assertStatus(t, w, http.StatusBadRequest)
			assertBodyContains(t, w, "请提供手机号和密码")
		})
	}
}

func TestLogin_TokenIsValidJWT(t *testing.T) {
	db := setupTestDB(t)
	seedUser(t, db, "13800138000", "password123", "张三", model.UserStatusActive)
	h := NewAuthHandler(db, "test-jwt-secret", 24, nil)

	w, c := newTestContext(http.MethodPost, "/api/v1/auth/login",
		`{"phone":"13800138000","password":"password123"}`, 0)

	h.Login(c)

	assertStatus(t, w, http.StatusOK)

	var resp LoginResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Token == "" {
		t.Error("token should not be empty")
	}
	if resp.User.DisplayName != "张三" {
		t.Errorf("user display_name = %q, want %q", resp.User.DisplayName, "张三")
	}
}

// -- GetMe HTTP Tests ----------------------------------------

func TestGetMe_Success(t *testing.T) {
	db := setupTestDB(t)
	user := seedUser(t, db, "13800138000", "password123", "张三", model.UserStatusActive)
	h := NewAuthHandler(db, "test-jwt-secret", 24, nil)

	w, c := newTestContext(http.MethodGet, "/api/v1/auth/me", "", user.ID)

	h.GetMe(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, "张三")
	assertBodyContains(t, w, "13800138000")
}

func TestGetMe_UserNotFound(t *testing.T) {
	db := setupTestDB(t)
	h := NewAuthHandler(db, "test-jwt-secret", 24, nil)

	w, c := newTestContext(http.MethodGet, "/api/v1/auth/me", "", uint(99999))

	h.GetMe(c)

	assertStatus(t, w, http.StatusNotFound)
	assertBodyContains(t, w, "用户不存在")
}
