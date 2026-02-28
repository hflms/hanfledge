package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/domain/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ============================
// Shared Test Helpers
// ============================

// setupTestDB creates an in-memory SQLite database and auto-migrates the
// tables needed for handler HTTP tests. It skips DocumentChunk (pgvector)
// and other tables that are not needed.
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open sqlite in-memory db: %v", err)
	}

	// Migrate only the tables used by handlers under test.
	// Order matters: referenced tables first.
	err = db.AutoMigrate(
		&model.User{},
		&model.School{},
		&model.Class{},
		&model.Role{},
		&model.UserSchoolRole{},
		&model.ClassStudent{},
		&model.Course{},
		&model.Chapter{},
		&model.KnowledgePoint{},
		&model.LearningActivity{},
		&model.StudentSession{},
		&model.Interaction{},
		&model.StudentKPMastery{},
		&model.ErrorNotebookEntry{},
		&model.AchievementDefinition{},
		&model.StudentAchievement{},
		&model.KPSkillMount{},
		&model.CustomSkill{},
		&model.CustomSkillVersion{},
	)
	if err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}

	return db
}

// hashPassword returns a bcrypt hash for the given password.
func hashPassword(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt hash failed: %v", err)
	}
	return string(hash)
}

// seedUser creates a user in the database and returns it.
func seedUser(t *testing.T, db *gorm.DB, phone, password, name string, status model.UserStatus) model.User {
	t.Helper()
	user := model.User{
		Phone:        phone,
		PasswordHash: hashPassword(t, password),
		DisplayName:  name,
		Status:       status,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seedUser failed: %v", err)
	}
	return user
}

// seedCourse creates a course in the database and returns it.
func seedCourse(t *testing.T, db *gorm.DB, teacherID uint, title string) model.Course {
	t.Helper()
	course := model.Course{
		SchoolID:   1,
		TeacherID:  teacherID,
		Title:      title,
		Subject:    "物理",
		GradeLevel: 9,
	}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("seedCourse failed: %v", err)
	}
	return course
}

// seedChapter creates a chapter in the database and returns it.
func seedChapter(t *testing.T, db *gorm.DB, courseID uint, title string, order int) model.Chapter {
	t.Helper()
	ch := model.Chapter{
		CourseID:  courseID,
		Title:     title,
		SortOrder: order,
	}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("seedChapter failed: %v", err)
	}
	return ch
}

// seedKP creates a knowledge point in the database and returns it.
func seedKP(t *testing.T, db *gorm.DB, chapterID uint, title string) model.KnowledgePoint {
	t.Helper()
	kp := model.KnowledgePoint{
		ChapterID: chapterID,
		Title:     title,
	}
	if err := db.Create(&kp).Error; err != nil {
		t.Fatalf("seedKP failed: %v", err)
	}
	return kp
}

// seedActivity creates a learning activity in the database and returns it.
func seedActivity(t *testing.T, db *gorm.DB, teacherID, courseID uint, title string) model.LearningActivity {
	t.Helper()
	now := time.Now().Format(time.RFC3339)
	act := model.LearningActivity{
		CourseID:  courseID,
		TeacherID: teacherID,
		Title:     title,
		CreatedAt: now,
	}
	if err := db.Create(&act).Error; err != nil {
		t.Fatalf("seedActivity failed: %v", err)
	}
	return act
}

// seedSession creates a student session in the database and returns it.
func seedSession(t *testing.T, db *gorm.DB, studentID, activityID, kpID uint, status model.SessionStatus) model.StudentSession {
	t.Helper()
	now := time.Now()
	s := model.StudentSession{
		StudentID:  studentID,
		ActivityID: activityID,
		CurrentKP:  kpID,
		Scaffold:   model.ScaffoldHigh,
		Status:     status,
		StartedAt:  now.Add(-30 * time.Minute),
	}
	if status == model.SessionStatusCompleted {
		ended := now
		s.EndedAt = &ended
	}
	if err := db.Create(&s).Error; err != nil {
		t.Fatalf("seedSession failed: %v", err)
	}
	return s
}

// newTestContext creates a gin test context with a JSON request body and
// the user_id set in the context (simulating JWT middleware).
func newTestContext(method, path, body string, userID uint) (*httptest.ResponseRecorder, *gin.Context) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	if userID > 0 {
		c.Set("user_id", userID)
	}
	return w, c
}

// newTestContextWithParams creates a gin test context with URL params.
func newTestContextWithParams(method, path, body string, userID uint, params gin.Params) (*httptest.ResponseRecorder, *gin.Context) {
	w, c := newTestContext(method, path, body, userID)
	c.Params = params
	return w, c
}

// newTestContextWithQuery creates a gin test context with query string.
func newTestContextWithQuery(method, pathWithQuery string, userID uint) (*httptest.ResponseRecorder, *gin.Context) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, pathWithQuery, nil)
	if userID > 0 {
		c.Set("user_id", userID)
	}
	return w, c
}

// assertStatus checks that the response has the expected HTTP status code.
func assertStatus(t *testing.T, w *httptest.ResponseRecorder, expected int) {
	t.Helper()
	if w.Code != expected {
		t.Errorf("status = %d, want %d; body: %s", w.Code, expected, w.Body.String())
	}
}

// assertBodyContains checks that the response body contains the expected substring.
func assertBodyContains(t *testing.T, w *httptest.ResponseRecorder, substr string) {
	t.Helper()
	body := w.Body.String()
	if !strings.Contains(body, substr) {
		t.Errorf("body does not contain %q; got: %s", substr, body)
	}
}

// assertBodyNotContains checks that the response body does NOT contain a substring.
func assertBodyNotContains(t *testing.T, w *httptest.ResponseRecorder, substr string) {
	t.Helper()
	body := w.Body.String()
	if strings.Contains(body, substr) {
		t.Errorf("body should not contain %q; got: %s", substr, body)
	}
}

// assertHeader checks that a response header matches the expected value.
func assertHeader(t *testing.T, w *httptest.ResponseRecorder, key, expected string) {
	t.Helper()
	got := w.Header().Get(key)
	if got != expected {
		t.Errorf("header %q = %q, want %q", key, got, expected)
	}
}

// assertContentType checks the Content-Type header contains the expected value.
func assertContentType(t *testing.T, w *httptest.ResponseRecorder, expected string) {
	t.Helper()
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, expected) {
		t.Errorf("Content-Type = %q, want it to contain %q", ct, expected)
	}
}

// assertCSVResponse checks that the response has CSV content type and BOM.
func assertCSVResponse(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	assertStatus(t, w, http.StatusOK)
	assertContentType(t, w, "text/csv")

	// Check for UTF-8 BOM
	body := w.Body.Bytes()
	if len(body) < 3 || body[0] != 0xEF || body[1] != 0xBB || body[2] != 0xBF {
		t.Error("CSV response missing UTF-8 BOM")
	}
}
