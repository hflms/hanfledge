package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// ============================
// JWT Middleware Unit Tests
// ============================

const testSecret = "test-secret-key-for-unit-tests"

// generateTestToken creates a valid JWT token for testing.
func generateTestToken(t *testing.T, userID uint, phone, name string, expiry time.Duration) string {
	t.Helper()
	claims := &JWTClaims{
		UserID:      userID,
		Phone:       phone,
		DisplayName: name,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "hanfledge",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return tokenStr
}

// newJWTTestContext creates a gin test context with optional Authorization header.
func newJWTTestContext(method, path, authHeader string) (*httptest.ResponseRecorder, *gin.Context, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	r := gin.New()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, nil)
	if authHeader != "" {
		c.Request.Header.Set("Authorization", authHeader)
	}
	return w, c, r
}

// -- JWTAuth Middleware Tests ---------------------------------

func TestJWTAuth_ValidToken(t *testing.T) {
	token := generateTestToken(t, 42, "13800000001", "Teacher", 24*time.Hour)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	r := gin.New()
	r.Use(JWTAuth(testSecret))
	r.GET("/test", func(c *gin.Context) {
		userID := c.GetUint("user_id")
		phone, _ := c.Get("phone")
		name, _ := c.Get("display_name")
		c.JSON(http.StatusOK, gin.H{
			"user_id":      userID,
			"phone":        phone,
			"display_name": name,
		})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, `"user_id":42`) {
		t.Errorf("body missing user_id: %s", body)
	}
	if !strings.Contains(body, `"phone":"13800000001"`) {
		t.Errorf("body missing phone: %s", body)
	}
	if !strings.Contains(body, `"display_name":"Teacher"`) {
		t.Errorf("body missing display_name: %s", body)
	}
}

func TestJWTAuth_MissingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	r := gin.New()
	r.Use(JWTAuth(testSecret))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	if !strings.Contains(w.Body.String(), "缺少认证令牌") {
		t.Errorf("body = %s, want error about missing token", w.Body.String())
	}
}

func TestJWTAuth_InvalidFormat_NoBearerPrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	r := gin.New()
	r.Use(JWTAuth(testSecret))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Token abc123")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	if !strings.Contains(w.Body.String(), "认证令牌格式错误") {
		t.Errorf("body = %s, want format error", w.Body.String())
	}
}

func TestJWTAuth_InvalidFormat_JustBearer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	r := gin.New()
	r.Use(JWTAuth(testSecret))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestJWTAuth_ExpiredToken(t *testing.T) {
	// Generate a token that expired 1 hour ago
	claims := &JWTClaims{
		UserID:      1,
		Phone:       "13800000001",
		DisplayName: "User",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			Issuer:    "hanfledge",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte(testSecret))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	r := gin.New()
	r.Use(JWTAuth(testSecret))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	if !strings.Contains(w.Body.String(), "认证令牌无效或已过期") {
		t.Errorf("body = %s, want expired error", w.Body.String())
	}
}

func TestJWTAuth_WrongSecret(t *testing.T) {
	// Sign with a different secret
	claims := &JWTClaims{
		UserID: 1,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte("wrong-secret"))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	r := gin.New()
	r.Use(JWTAuth(testSecret))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestJWTAuth_MalformedToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	r := gin.New()
	r.Use(JWTAuth(testSecret))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer not.a.valid.jwt.token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestJWTAuth_QueryParamFallback(t *testing.T) {
	token := generateTestToken(t, 7, "13900000001", "Student", 24*time.Hour)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	r := gin.New()
	r.Use(JWTAuth(testSecret))
	r.GET("/ws", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user_id": c.GetUint("user_id")})
	})

	req := httptest.NewRequest("GET", "/ws?token="+token, nil)
	// No Authorization header
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"user_id":7`) {
		t.Errorf("body = %s, want user_id 7", w.Body.String())
	}
}

func TestJWTAuth_BearerCaseInsensitive(t *testing.T) {
	token := generateTestToken(t, 1, "13800000001", "User", 24*time.Hour)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	r := gin.New()
	r.Use(JWTAuth(testSecret))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "bearer "+token) // lowercase "bearer"
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

// -- GetUserID Tests ------------------------------------------

func TestGetUserID_UintValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", uint(42))

	if got := GetUserID(c); got != 42 {
		t.Errorf("GetUserID = %d, want 42", got)
	}
}

func TestGetUserID_Float64Value(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", float64(42))

	if got := GetUserID(c); got != 42 {
		t.Errorf("GetUserID = %d, want 42", got)
	}
}

func TestGetUserID_Missing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	if got := GetUserID(c); got != 0 {
		t.Errorf("GetUserID = %d, want 0 for missing", got)
	}
}

func TestGetUserID_WrongType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", "not a number")

	if got := GetUserID(c); got != 0 {
		t.Errorf("GetUserID = %d, want 0 for wrong type", got)
	}
}

func TestGetUserID_IntValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", 42) // int, not uint

	// int is not handled, should return 0
	if got := GetUserID(c); got != 0 {
		t.Errorf("GetUserID = %d, want 0 for int type", got)
	}
}
