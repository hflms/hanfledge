package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/domain/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ============================
// RBAC Middleware Unit Tests
// ============================

// setupRBACTestDB creates an in-memory SQLite database with RBAC-related tables.
func setupRBACTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{},
		&model.School{},
		&model.Role{},
		&model.UserSchoolRole{},
	); err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}
	return db
}

// seedRBACUser creates a user with a specific role for RBAC testing.
func seedRBACUser(t *testing.T, db *gorm.DB, userID uint, roleName model.RoleName) {
	t.Helper()
	// Create user
	user := model.User{
		Phone:        "1380000" + fmt.Sprintf("%04d", userID),
		PasswordHash: "hash",
		DisplayName:  "User " + string(roleName),
		Status:       model.UserStatusActive,
	}
	user.ID = userID
	db.Create(&user)

	// Create role
	var role model.Role
	if err := db.Where("name = ?", roleName).First(&role).Error; err != nil {
		role = model.Role{Name: roleName}
		db.Create(&role)
	}

	// Create user-school-role association
	db.Create(&model.UserSchoolRole{
		UserID: userID,
		RoleID: role.ID,
	})
}

// -- RBAC Middleware Tests -------------------------------------

func TestRBAC_UserHasRequiredRole(t *testing.T) {
	db := setupRBACTestDB(t)
	seedRBACUser(t, db, 1, model.RoleTeacher)

	// Generate JWT token
	token := generateTestToken(t, 1, "13800000001", "Teacher", 24*time.Hour)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	r := gin.New()
	r.Use(JWTAuth(testSecret))
	r.Use(RBAC(db, model.RoleTeacher))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestRBAC_UserLacksRequiredRole(t *testing.T) {
	db := setupRBACTestDB(t)
	seedRBACUser(t, db, 1, model.RoleStudent)

	token := generateTestToken(t, 1, "13800000001", "Student", 24*time.Hour)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	r := gin.New()
	r.Use(JWTAuth(testSecret))
	r.Use(RBAC(db, model.RoleSysAdmin)) // requires SYS_ADMIN
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusForbidden, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "权限不足") {
		t.Errorf("body = %s, want permission error", w.Body.String())
	}
}

func TestRBAC_UserHasOneOfMultipleRoles(t *testing.T) {
	db := setupRBACTestDB(t)
	seedRBACUser(t, db, 1, model.RoleTeacher)

	token := generateTestToken(t, 1, "13800000001", "Teacher", 24*time.Hour)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	r := gin.New()
	r.Use(JWTAuth(testSecret))
	r.Use(RBAC(db, model.RoleSysAdmin, model.RoleTeacher)) // either role works
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestRBAC_NoRoles(t *testing.T) {
	db := setupRBACTestDB(t)
	// User exists but has no roles
	db.Create(&model.User{
		Phone:        "13800000099",
		PasswordHash: "hash",
		DisplayName:  "No Roles",
		Status:       model.UserStatusActive,
	})

	token := generateTestToken(t, 1, "13800000099", "NoRoles", 24*time.Hour)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	r := gin.New()
	r.Use(JWTAuth(testSecret))
	r.Use(RBAC(db, model.RoleTeacher))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestRBAC_SetsUserRolesInContext(t *testing.T) {
	db := setupRBACTestDB(t)
	seedRBACUser(t, db, 1, model.RoleTeacher)

	token := generateTestToken(t, 1, "13800000001", "Teacher", 24*time.Hour)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	r := gin.New()
	r.Use(JWTAuth(testSecret))
	r.Use(RBAC(db, model.RoleTeacher))
	r.GET("/test", func(c *gin.Context) {
		roles, exists := c.Get("user_roles")
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "no user_roles"})
			return
		}
		userRoles := roles.([]model.UserSchoolRole)
		c.JSON(http.StatusOK, gin.H{"role_count": len(userRoles)})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"role_count":1`) {
		t.Errorf("body = %s, want role_count 1", w.Body.String())
	}
}

// -- joinRoleNames Tests --------------------------------------

func TestJoinRoleNames_Single(t *testing.T) {
	result := joinRoleNames([]model.RoleName{model.RoleTeacher})
	if result != "TEACHER" {
		t.Errorf("joinRoleNames = %q, want %q", result, "TEACHER")
	}
}

func TestJoinRoleNames_Multiple(t *testing.T) {
	result := joinRoleNames([]model.RoleName{model.RoleSysAdmin, model.RoleTeacher})
	if result != "SYS_ADMIN, TEACHER" {
		t.Errorf("joinRoleNames = %q, want %q", result, "SYS_ADMIN, TEACHER")
	}
}

func TestJoinRoleNames_Empty(t *testing.T) {
	result := joinRoleNames([]model.RoleName{})
	if result != "" {
		t.Errorf("joinRoleNames = %q, want empty", result)
	}
}

// -- generateTestToken for RBAC (reuses from jwt_test.go) -----
// This function is already defined in jwt_test.go in the same package.
// The compiler will find it since both files are in package middleware.

// -- newJWTTestContext for RBAC (reuses from jwt_test.go) -----
// Same as above.
