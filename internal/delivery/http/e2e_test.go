package http_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/hflms/hanfledge/internal/config"
	delivery "github.com/hflms/hanfledge/internal/delivery/http"
	"github.com/hflms/hanfledge/internal/delivery/http/middleware"
	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/safety"
	"github.com/hflms/hanfledge/internal/plugin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// -- Test Configuration -----------------------------------------------

const (
	testJWTSecret = "test-e2e-secret"
	testJWTExpiry = 24
)

// testEnv holds the shared test environment.
type testEnv struct {
	db     *gorm.DB
	router http.Handler

	adminToken   string
	teacherToken string
	studentToken string

	adminID   uint
	teacherID uint
	studentID uint
	schoolID  uint
	class1ID  uint
	class2ID  uint
}

// -- Test Setup -------------------------------------------------------

// setupTestDB connects to the test PostgreSQL and migrates all tables.
// It uses a database named "hanfledge_test" on the dev Postgres (port 5433).
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := "host=localhost port=5433 user=hanfledge password=hanfledge_secret dbname=hanfledge_test sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Skipf("Skipping E2E tests: cannot connect to test database: %v", err)
	}

	// Enable pgvector extension
	db.Exec("CREATE EXTENSION IF NOT EXISTS vector")

	// Migrate all tables
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
		&model.KPSkillMount{},
		&model.LearningActivity{},
		&model.ActivityClassAssignment{},
		&model.StudentSession{},
		&model.Interaction{},
		&model.StudentKPMastery{},
		&model.Document{},
		&model.DocumentChunk{},
	)
	if err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}

	// Clean all tables (reverse dependency order)
	tables := []string{
		"document_chunks", "documents",
		"student_kp_masteries", "interactions", "student_sessions",
		"activity_class_assignments", "learning_activities",
		"kp_skill_mounts", "knowledge_points", "chapters", "courses",
		"class_students", "user_school_roles",
		"classes", "schools", "users", "roles",
	}
	for _, table := range tables {
		db.Exec("DELETE FROM " + table)
	}

	// Seed default roles
	roles := []model.Role{
		{ID: 1, Name: model.RoleSysAdmin},
		{ID: 2, Name: model.RoleSchoolAdmin},
		{ID: 3, Name: model.RoleTeacher},
		{ID: 4, Name: model.RoleStudent},
	}
	for _, r := range roles {
		db.FirstOrCreate(&r, model.Role{Name: r.Name})
	}

	return db
}

// setupTestEnv creates the full test environment with router, test users, and JWT tokens.
func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	db := setupTestDB(t)

	// Create test users
	hash, _ := bcrypt.GenerateFromPassword([]byte("test123"), bcrypt.MinCost) // MinCost for speed

	admin := model.User{Phone: "19900000001", PasswordHash: string(hash), DisplayName: "测试管理员", Status: model.UserStatusActive}
	db.Create(&admin)

	teacher := model.User{Phone: "19900000010", PasswordHash: string(hash), DisplayName: "测试张老师", Status: model.UserStatusActive}
	db.Create(&teacher)

	student := model.User{Phone: "19900000100", PasswordHash: string(hash), DisplayName: "测试王同学", Status: model.UserStatusActive}
	db.Create(&student)

	student2 := model.User{Phone: "19900000101", PasswordHash: string(hash), DisplayName: "测试李同学", Status: model.UserStatusActive}
	db.Create(&student2)

	// Create school and classes
	school := model.School{Name: "测试学校", Code: "TEST001", Region: "测试区", Status: model.SchoolStatusActive}
	db.Create(&school)

	class1 := model.Class{SchoolID: school.ID, Name: "测试1班", GradeLevel: 10, AcademicYear: "2025-2026"}
	db.Create(&class1)

	class2 := model.Class{SchoolID: school.ID, Name: "测试2班", GradeLevel: 10, AcademicYear: "2025-2026"}
	db.Create(&class2)

	// Assign roles
	db.Create(&model.UserSchoolRole{UserID: admin.ID, SchoolID: nil, RoleID: 1})           // SYS_ADMIN
	db.Create(&model.UserSchoolRole{UserID: teacher.ID, SchoolID: &school.ID, RoleID: 3})  // TEACHER
	db.Create(&model.UserSchoolRole{UserID: student.ID, SchoolID: &school.ID, RoleID: 4})  // STUDENT
	db.Create(&model.UserSchoolRole{UserID: student2.ID, SchoolID: &school.ID, RoleID: 4}) // STUDENT

	// Assign students to classes
	db.Create(&model.ClassStudent{ClassID: class1.ID, StudentID: student.ID})
	db.Create(&model.ClassStudent{ClassID: class2.ID, StudentID: student2.ID})

	// Create router with nil dependencies for KA-RAG, orchestrator (not needed for HTTP tests)
	registry := plugin.NewRegistry()
	guard := safety.NewInjectionGuard()
	cfg := newTestConfig()
	router := delivery.NewRouter(delivery.RouterDeps{
		DB:             db,
		Cfg:            cfg,
		Registry:       registry,
		InjectionGuard: guard,
	})

	// Generate JWT tokens
	adminToken := generateToken(t, admin.ID, admin.Phone, admin.DisplayName)
	teacherToken := generateToken(t, teacher.ID, teacher.Phone, teacher.DisplayName)
	studentToken := generateToken(t, student.ID, student.Phone, student.DisplayName)

	return &testEnv{
		db:           db,
		router:       router,
		adminToken:   adminToken,
		teacherToken: teacherToken,
		studentToken: studentToken,
		adminID:      admin.ID,
		teacherID:    teacher.ID,
		studentID:    student.ID,
		schoolID:     school.ID,
		class1ID:     class1.ID,
		class2ID:     class2.ID,
	}
}

// newTestConfig builds a Config struct for testing.
func newTestConfig() *config.Config {
	return &config.Config{
		JWT: config.JWTConfig{
			Secret:      testJWTSecret,
			ExpiryHours: testJWTExpiry,
		},
		Server: config.ServerConfig{
			CORSOrigins: "http://localhost:3000",
		},
	}
}

// generateToken creates a signed JWT for a test user.
func generateToken(t *testing.T, userID uint, phone, displayName string) string {
	t.Helper()

	claims := &middleware.JWTClaims{
		UserID:      userID,
		Phone:       phone,
		DisplayName: displayName,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "hanfledge",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatalf("Failed to generate test JWT: %v", err)
	}
	return tokenStr
}

// -- HTTP Helpers -----------------------------------------------------

func doRequest(handler http.Handler, method, path string, body interface{}, token string) *httptest.ResponseRecorder {
	var reqBody *bytes.Buffer
	if body != nil {
		data, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(data)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func parseJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse JSON response: %v\nBody: %s", err, w.Body.String())
	}
	return result
}

func parseJSONArray(t *testing.T, w *httptest.ResponseRecorder) []interface{} {
	t.Helper()
	var result interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse JSON response: %v\nBody: %s", err, w.Body.String())
	}

	if m, ok := result.(map[string]interface{}); ok {
		if items, ok := m["items"].([]interface{}); ok {
			return items
		}
		// If it's a map but has no "items" array, it's not a standard paginated response
		t.Fatalf("Expected JSON array or paginated object with 'items', got: %s", w.Body.String())
	} else if arr, ok := result.([]interface{}); ok {
		return arr
	}

	t.Fatalf("Expected JSON array or paginated object, got: %s", w.Body.String())
	return nil
}

func expectStatus(t *testing.T, w *httptest.ResponseRecorder, expected int) {
	t.Helper()
	if w.Code != expected {
		t.Errorf("Expected status %d, got %d. Body: %s", expected, w.Code, w.Body.String())
	}
}

// -- Test: Health Check -----------------------------------------------

func TestHealthCheck(t *testing.T) {
	env := setupTestEnv(t)

	w := doRequest(env.router, "GET", "/health", nil, "")
	expectStatus(t, w, http.StatusOK)

	result := parseJSON(t, w)
	if result["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", result["status"])
	}
}

// -- Test: Auth -------------------------------------------------------

func TestLogin_Success(t *testing.T) {
	env := setupTestEnv(t)

	body := map[string]string{"phone": "19900000010", "password": "test123"}
	w := doRequest(env.router, "POST", "/api/v1/auth/login", body, "")
	expectStatus(t, w, http.StatusOK)

	result := parseJSON(t, w)
	if result["token"] == nil || result["token"] == "" {
		t.Error("Expected non-empty token")
	}
	user := result["user"].(map[string]interface{})
	if user["phone"] != "19900000010" {
		t.Errorf("Expected phone 19900000010, got %v", user["phone"])
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	env := setupTestEnv(t)

	body := map[string]string{"phone": "19900000010", "password": "wrongpass"}
	w := doRequest(env.router, "POST", "/api/v1/auth/login", body, "")
	expectStatus(t, w, http.StatusUnauthorized)
}

func TestLogin_MissingFields(t *testing.T) {
	env := setupTestEnv(t)

	body := map[string]string{"phone": "19900000010"}
	w := doRequest(env.router, "POST", "/api/v1/auth/login", body, "")
	expectStatus(t, w, http.StatusBadRequest)
}

func TestGetMe(t *testing.T) {
	env := setupTestEnv(t)

	w := doRequest(env.router, "GET", "/api/v1/auth/me", nil, env.teacherToken)
	expectStatus(t, w, http.StatusOK)

	result := parseJSON(t, w)
	if result["display_name"] != "测试张老师" {
		t.Errorf("Expected display_name '测试张老师', got %v", result["display_name"])
	}
}

func TestGetMe_Unauthorized(t *testing.T) {
	env := setupTestEnv(t)

	w := doRequest(env.router, "GET", "/api/v1/auth/me", nil, "")
	expectStatus(t, w, http.StatusUnauthorized)
}

// -- Test: RBAC -------------------------------------------------------

func TestRBAC_AdminCanAccessSchools(t *testing.T) {
	env := setupTestEnv(t)

	w := doRequest(env.router, "GET", "/api/v1/schools", nil, env.adminToken)
	expectStatus(t, w, http.StatusOK)
}

func TestRBAC_TeacherCannotAccessSchools(t *testing.T) {
	env := setupTestEnv(t)

	w := doRequest(env.router, "GET", "/api/v1/schools", nil, env.teacherToken)
	expectStatus(t, w, http.StatusForbidden)
}

func TestRBAC_StudentCannotAccessSchools(t *testing.T) {
	env := setupTestEnv(t)

	w := doRequest(env.router, "GET", "/api/v1/schools", nil, env.studentToken)
	expectStatus(t, w, http.StatusForbidden)
}

func TestRBAC_StudentCanAccessStudentRoutes(t *testing.T) {
	env := setupTestEnv(t)

	w := doRequest(env.router, "GET", "/api/v1/student/activities", nil, env.studentToken)
	expectStatus(t, w, http.StatusOK)
}

func TestRBAC_TeacherCannotAccessStudentRoutes(t *testing.T) {
	env := setupTestEnv(t)

	w := doRequest(env.router, "GET", "/api/v1/student/activities", nil, env.teacherToken)
	expectStatus(t, w, http.StatusForbidden)
}

// -- Test: School/Class/User CRUD (Admin) ----------------------------

func TestCreateSchool(t *testing.T) {
	env := setupTestEnv(t)

	body := map[string]interface{}{
		"name":   "新建测试学校",
		"code":   "NEW001",
		"region": "新区",
	}
	w := doRequest(env.router, "POST", "/api/v1/schools", body, env.adminToken)
	expectStatus(t, w, http.StatusCreated)

	result := parseJSON(t, w)
	if result["name"] != "新建测试学校" {
		t.Errorf("Expected school name '新建测试学校', got %v", result["name"])
	}
}

func TestListSchools(t *testing.T) {
	env := setupTestEnv(t)

	w := doRequest(env.router, "GET", "/api/v1/schools", nil, env.adminToken)
	expectStatus(t, w, http.StatusOK)

	result := parseJSONArray(t, w)
	if len(result) < 1 {
		t.Error("Expected at least 1 school")
	}
}

func TestCreateClass(t *testing.T) {
	env := setupTestEnv(t)

	body := map[string]interface{}{
		"school_id":     env.schoolID,
		"name":          "新建测试班",
		"grade_level":   11,
		"academic_year": "2025-2026",
	}
	w := doRequest(env.router, "POST", "/api/v1/classes", body, env.adminToken)
	expectStatus(t, w, http.StatusCreated)
}

func TestCreateUser(t *testing.T) {
	env := setupTestEnv(t)

	body := map[string]interface{}{
		"phone":        "19911111111",
		"password":     "newuser123",
		"display_name": "新建用户",
		"role":         "STUDENT",
		"school_id":    env.schoolID,
	}
	w := doRequest(env.router, "POST", "/api/v1/users", body, env.adminToken)
	expectStatus(t, w, http.StatusCreated)

	result := parseJSON(t, w)
	if result["phone"] != "19911111111" {
		t.Errorf("Expected phone 19911111111, got %v", result["phone"])
	}
}

// -- Test: Full Teacher → Student E2E Flow ----------------------------

func TestE2ETeacherStudentFlow(t *testing.T) {
	env := setupTestEnv(t)

	// ── Step 1: Teacher creates a course ─────────────────────
	courseBody := map[string]interface{}{
		"school_id":   env.schoolID,
		"title":       "高中数学(测试)",
		"subject":     "math",
		"grade_level": 10,
		"description": "E2E测试课程",
	}
	w := doRequest(env.router, "POST", "/api/v1/courses", courseBody, env.teacherToken)
	expectStatus(t, w, http.StatusCreated)
	courseResult := parseJSON(t, w)
	courseID := uint(courseResult["id"].(float64))
	if courseID == 0 {
		t.Fatal("Expected non-zero course ID")
	}

	// ── Step 2: Teacher lists courses ────────────────────────
	w = doRequest(env.router, "GET", "/api/v1/courses", nil, env.teacherToken)
	expectStatus(t, w, http.StatusOK)
	courses := parseJSONArray(t, w)
	if len(courses) != 1 {
		t.Errorf("Expected 1 course, got %d", len(courses))
	}

	// ── Step 3: Manually create chapter + knowledge points ───
	// (Skip PDF upload — requires real file + Ollama)
	chapter := model.Chapter{
		CourseID:  courseID,
		Title:     "第一章 函数基础",
		SortOrder: 1,
	}
	env.db.Create(&chapter)

	kp1 := model.KnowledgePoint{ChapterID: chapter.ID, Title: "函数的定义", Difficulty: 0.3, IsKeyPoint: true}
	kp2 := model.KnowledgePoint{ChapterID: chapter.ID, Title: "函数的图像", Difficulty: 0.5, IsKeyPoint: false}
	env.db.Create(&kp1)
	env.db.Create(&kp2)

	// ── Step 4: Teacher views course outline ─────────────────
	w = doRequest(env.router, "GET", fmt.Sprintf("/api/v1/courses/%d/outline", courseID), nil, env.teacherToken)
	expectStatus(t, w, http.StatusOK)
	outlineResult := parseJSON(t, w)
	courseData := outlineResult["course"].(map[string]interface{})
	chapters := courseData["chapters"].([]interface{})
	if len(chapters) != 1 {
		t.Errorf("Expected 1 chapter in outline, got %d", len(chapters))
	}
	kps := chapters[0].(map[string]interface{})["knowledge_points"].([]interface{})
	if len(kps) != 2 {
		t.Errorf("Expected 2 knowledge points, got %d", len(kps))
	}

	// ── Step 5: Teacher creates a learning activity ──────────
	activityBody := map[string]interface{}{
		"course_id": courseID,
		"title":     "函数基础练习",
		"kp_ids":    []uint{kp1.ID, kp2.ID},
		"class_ids": []uint{env.class1ID},
	}
	w = doRequest(env.router, "POST", "/api/v1/activities", activityBody, env.teacherToken)
	expectStatus(t, w, http.StatusCreated)
	actResult := parseJSON(t, w)
	activityID := uint(actResult["id"].(float64))
	if activityID == 0 {
		t.Fatal("Expected non-zero activity ID")
	}
	if actResult["status"] != "draft" {
		t.Errorf("Expected status 'draft', got %v", actResult["status"])
	}

	// ── Step 6: Teacher lists activities ─────────────────────
	w = doRequest(env.router, "GET", "/api/v1/activities", nil, env.teacherToken)
	expectStatus(t, w, http.StatusOK)
	activities := parseJSONArray(t, w)
	if len(activities) != 1 {
		t.Errorf("Expected 1 activity, got %d", len(activities))
	}

	// ── Step 7: Teacher publishes the activity ───────────────
	w = doRequest(env.router, "POST", fmt.Sprintf("/api/v1/activities/%d/publish", activityID), nil, env.teacherToken)
	expectStatus(t, w, http.StatusOK)

	// Verify cannot publish again
	w = doRequest(env.router, "POST", fmt.Sprintf("/api/v1/activities/%d/publish", activityID), nil, env.teacherToken)
	expectStatus(t, w, http.StatusBadRequest)

	// ── Step 8: Student sees the published activity ──────────
	w = doRequest(env.router, "GET", "/api/v1/student/activities", nil, env.studentToken)
	expectStatus(t, w, http.StatusOK)
	studentActivities := parseJSONArray(t, w)
	if len(studentActivities) != 1 {
		t.Errorf("Expected 1 activity for student, got %d", len(studentActivities))
	}
	firstAct := studentActivities[0].(map[string]interface{})
	if firstAct["has_session"] != false {
		t.Error("Expected has_session=false before joining")
	}

	// ── Step 9: Student joins the activity ───────────────────
	w = doRequest(env.router, "POST", fmt.Sprintf("/api/v1/activities/%d/join", activityID), nil, env.studentToken)
	expectStatus(t, w, http.StatusCreated)
	joinResult := parseJSON(t, w)
	sessionID := uint(joinResult["session_id"].(float64))
	if sessionID == 0 {
		t.Fatal("Expected non-zero session ID")
	}

	// Joining again should return existing session
	w = doRequest(env.router, "POST", fmt.Sprintf("/api/v1/activities/%d/join", activityID), nil, env.studentToken)
	expectStatus(t, w, http.StatusOK)
	rejoinResult := parseJSON(t, w)
	if uint(rejoinResult["session_id"].(float64)) != sessionID {
		t.Error("Expected same session_id on rejoin")
	}

	// ── Step 10: Student views session details ───────────────
	w = doRequest(env.router, "GET", fmt.Sprintf("/api/v1/sessions/%d", sessionID), nil, env.studentToken)
	expectStatus(t, w, http.StatusOK)
	sessionResult := parseJSON(t, w)
	session := sessionResult["session"].(map[string]interface{})
	if session["scaffold_level"] != "high" {
		t.Errorf("Expected scaffold_level 'high', got %v", session["scaffold_level"])
	}
	if session["status"] != "active" {
		t.Errorf("Expected status 'active', got %v", session["status"])
	}

	// ── Step 11: Simulate mastery data for dashboard tests ───
	now := time.Now()
	mastery1 := model.StudentKPMastery{
		StudentID:     env.studentID,
		KPID:          kp1.ID,
		MasteryScore:  0.75,
		AttemptCount:  5,
		CorrectCount:  3,
		LastAttemptAt: &now,
		UpdatedAt:     now,
	}
	mastery2 := model.StudentKPMastery{
		StudentID:     env.studentID,
		KPID:          kp2.ID,
		MasteryScore:  0.45,
		AttemptCount:  3,
		CorrectCount:  1,
		LastAttemptAt: &now,
		UpdatedAt:     now,
	}
	env.db.Create(&mastery1)
	env.db.Create(&mastery2)

	// ── Step 12: Teacher views knowledge radar ───────────────
	w = doRequest(env.router, "GET",
		fmt.Sprintf("/api/v1/dashboard/knowledge-radar?course_id=%d", courseID), nil, env.teacherToken)
	expectStatus(t, w, http.StatusOK)
	radarResult := parseJSON(t, w)
	labels := radarResult["labels"].([]interface{})
	values := radarResult["values"].([]interface{})
	if len(labels) != 2 {
		t.Errorf("Expected 2 radar labels, got %d", len(labels))
	}
	if len(values) != 2 {
		t.Errorf("Expected 2 radar values, got %d", len(values))
	}
	if radarResult["student_count"].(float64) < 1 {
		t.Error("Expected at least 1 student in radar")
	}

	// ── Step 13: Teacher views student mastery ───────────────
	w = doRequest(env.router, "GET",
		fmt.Sprintf("/api/v1/students/%d/mastery?course_id=%d", env.studentID, courseID), nil, env.teacherToken)
	expectStatus(t, w, http.StatusOK)
	masteryResult := parseJSON(t, w)
	items := masteryResult["items"].([]interface{})
	if len(items) != 2 {
		t.Errorf("Expected 2 mastery items, got %d", len(items))
	}

	// ── Step 14: Teacher views activity session stats ────────
	w = doRequest(env.router, "GET",
		fmt.Sprintf("/api/v1/activities/%d/sessions", activityID), nil, env.teacherToken)
	expectStatus(t, w, http.StatusOK)
	sessStats := parseJSON(t, w)
	if sessStats["total_sessions"].(float64) != 1 {
		t.Errorf("Expected 1 total session, got %v", sessStats["total_sessions"])
	}
	if sessStats["active_sessions"].(float64) != 1 {
		t.Errorf("Expected 1 active session, got %v", sessStats["active_sessions"])
	}

	// ── Step 15: Student views own mastery ───────────────────
	w = doRequest(env.router, "GET",
		fmt.Sprintf("/api/v1/student/mastery?course_id=%d", courseID), nil, env.studentToken)
	expectStatus(t, w, http.StatusOK)
	selfMastery := parseJSON(t, w)
	selfItems := selfMastery["items"].([]interface{})
	if len(selfItems) != 2 {
		t.Errorf("Expected 2 self mastery items, got %d", len(selfItems))
	}
}

// -- Test: Session Access Control -------------------------------------

func TestSessionAccessControl(t *testing.T) {
	env := setupTestEnv(t)

	// Create a course + activity for the session to be valid
	course := model.Course{SchoolID: env.schoolID, TeacherID: env.teacherID, Title: "Course X", Subject: "math", GradeLevel: 10}
	env.db.Create(&course)

	activity := model.LearningActivity{
		CourseID:    course.ID,
		TeacherID:   env.teacherID,
		Title:       "Activity X",
		KPIDS:       "[]",
		SkillConfig: "{}",
		Status:      model.ActivityStatusPublished,
		CreatedAt:   time.Now().Format(time.RFC3339),
	}
	env.db.Create(&activity)

	// Create a session for student
	session := model.StudentSession{
		StudentID:  env.studentID,
		ActivityID: activity.ID,
		CurrentKP:  1,
		Scaffold:   model.ScaffoldHigh,
		Status:     model.SessionStatusActive,
		StartedAt:  time.Now(),
	}
	env.db.Create(&session)

	// Student can access their own session
	w := doRequest(env.router, "GET", fmt.Sprintf("/api/v1/sessions/%d", session.ID), nil, env.studentToken)
	expectStatus(t, w, http.StatusOK)

	// Teacher cannot access student's session (ownership check)
	w = doRequest(env.router, "GET", fmt.Sprintf("/api/v1/sessions/%d", session.ID), nil, env.teacherToken)
	expectStatus(t, w, http.StatusForbidden)
}

// -- Test: Activity Ownership -----------------------------------------

func TestActivityOwnershipOnPublish(t *testing.T) {
	env := setupTestEnv(t)

	// Create a course + activity for the teacher
	course := model.Course{SchoolID: env.schoolID, TeacherID: env.teacherID, Title: "课程A", Subject: "math", GradeLevel: 10}
	env.db.Create(&course)

	activity := model.LearningActivity{
		CourseID:    course.ID,
		TeacherID:   env.teacherID,
		Title:       "测试活动",
		KPIDS:       "[]",
		SkillConfig: "{}",
		Status:      model.ActivityStatusDraft,
		CreatedAt:   time.Now().Format(time.RFC3339),
	}
	if err := env.db.Create(&activity).Error; err != nil {
		t.Fatalf("Failed to create activity: %v", err)
	}

	// Admin (who is not the teacher) tries to publish — should get 403
	w := doRequest(env.router, "POST", fmt.Sprintf("/api/v1/activities/%d/publish", activity.ID), nil, env.adminToken)
	expectStatus(t, w, http.StatusForbidden)

	// The actual teacher can publish
	w = doRequest(env.router, "POST", fmt.Sprintf("/api/v1/activities/%d/publish", activity.ID), nil, env.teacherToken)
	expectStatus(t, w, http.StatusOK)
}

// -- Test: Dashboard Access Control -----------------------------------

func TestKnowledgeRadar_RequiresCourseID(t *testing.T) {
	env := setupTestEnv(t)

	// Missing course_id
	w := doRequest(env.router, "GET", "/api/v1/dashboard/knowledge-radar", nil, env.teacherToken)
	expectStatus(t, w, http.StatusBadRequest)
}

func TestKnowledgeRadar_ForbiddenForNonOwner(t *testing.T) {
	env := setupTestEnv(t)

	// Create course owned by teacher
	course := model.Course{SchoolID: env.schoolID, TeacherID: env.teacherID, Title: "课程B", Subject: "math", GradeLevel: 10}
	env.db.Create(&course)

	// Admin (not the course owner) tries to view radar — should get 403
	w := doRequest(env.router, "GET",
		fmt.Sprintf("/api/v1/dashboard/knowledge-radar?course_id=%d", course.ID), nil, env.adminToken)
	expectStatus(t, w, http.StatusForbidden)
}

// -- Test: JoinActivity Validation ------------------------------------

func TestJoinActivity_NotPublished(t *testing.T) {
	env := setupTestEnv(t)

	// Create a course first (FK requirement)
	course := model.Course{SchoolID: env.schoolID, TeacherID: env.teacherID, Title: "草稿课程", Subject: "math", GradeLevel: 10}
	if err := env.db.Create(&course).Error; err != nil {
		t.Fatalf("Failed to create course: %v", err)
	}

	activity := model.LearningActivity{
		CourseID:    course.ID,
		TeacherID:   env.teacherID,
		Title:       "草稿活动",
		KPIDS:       "[]",
		SkillConfig: "{}",
		Status:      model.ActivityStatusDraft,
		CreatedAt:   time.Now().Format(time.RFC3339),
	}
	if err := env.db.Create(&activity).Error; err != nil {
		t.Fatalf("Failed to create activity: %v", err)
	}

	// Student tries to join a draft activity
	w := doRequest(env.router, "POST", fmt.Sprintf("/api/v1/activities/%d/join", activity.ID), nil, env.studentToken)
	expectStatus(t, w, http.StatusBadRequest)
}

func TestJoinActivity_NotFound(t *testing.T) {
	env := setupTestEnv(t)

	w := doRequest(env.router, "POST", "/api/v1/activities/99999/join", nil, env.studentToken)
	expectStatus(t, w, http.StatusNotFound)
}

// -- Test: Course ListCourses is filtered by teacher ──────────────────

func TestListCourses_FilteredByTeacher(t *testing.T) {
	env := setupTestEnv(t)

	// Create course for teacher
	env.db.Create(&model.Course{SchoolID: env.schoolID, TeacherID: env.teacherID, Title: "我的课程", Subject: "math", GradeLevel: 10})

	// Admin should see 0 courses (ListCourses filters by teacher_id)
	w := doRequest(env.router, "GET", "/api/v1/courses", nil, env.adminToken)
	expectStatus(t, w, http.StatusOK)
	adminCourses := parseJSONArray(t, w)
	if len(adminCourses) != 0 {
		t.Errorf("Expected 0 courses for admin, got %d", len(adminCourses))
	}

	// Teacher should see 1 course
	w = doRequest(env.router, "GET", "/api/v1/courses", nil, env.teacherToken)
	expectStatus(t, w, http.StatusOK)
	teacherCourses := parseJSONArray(t, w)
	if len(teacherCourses) != 1 {
		t.Errorf("Expected 1 course for teacher, got %d", len(teacherCourses))
	}
}

// -- Test: Batch Create Users ─────────────────────────────────────────

func TestBatchCreateUsers(t *testing.T) {
	env := setupTestEnv(t)

	body := map[string]interface{}{
		"users": []map[string]interface{}{
			{"phone": "19922220001", "password": "test123", "display_name": "批量1", "role": "STUDENT", "school_id": env.schoolID},
			{"phone": "19922220002", "password": "test123", "display_name": "批量2", "role": "STUDENT", "school_id": env.schoolID},
		},
	}
	w := doRequest(env.router, "POST", "/api/v1/users/batch", body, env.adminToken)
	expectStatus(t, w, http.StatusOK)

	result := parseJSON(t, w)
	if result["created_count"].(float64) != 2 {
		t.Errorf("Expected 2 created, got %v", result["created_count"])
	}
	if result["error_count"].(float64) != 0 {
		t.Errorf("Expected 0 errors, got %v", result["error_count"])
	}
}

// -- Test: Document Status (empty) ------------------------------------

func TestGetDocumentStatus_Empty(t *testing.T) {
	env := setupTestEnv(t)

	// Create course
	course := model.Course{SchoolID: env.schoolID, TeacherID: env.teacherID, Title: "无文档课程", Subject: "math", GradeLevel: 10}
	env.db.Create(&course)

	w := doRequest(env.router, "GET", fmt.Sprintf("/api/v1/courses/%d/documents", course.ID), nil, env.teacherToken)
	expectStatus(t, w, http.StatusOK)
	docs := parseJSONArray(t, w)
	if len(docs) != 0 {
		t.Errorf("Expected 0 documents, got %d", len(docs))
	}
}

// -- Test: Activity Sessions Stats (empty) ----------------------------

func TestActivitySessions_OwnershipCheck(t *testing.T) {
	env := setupTestEnv(t)

	// Create a course first (FK requirement)
	course := model.Course{SchoolID: env.schoolID, TeacherID: env.teacherID, Title: "有权限课程", Subject: "math", GradeLevel: 10}
	if err := env.db.Create(&course).Error; err != nil {
		t.Fatalf("Failed to create course: %v", err)
	}

	// Create activity for teacher
	activity := model.LearningActivity{
		CourseID:    course.ID,
		TeacherID:   env.teacherID,
		Title:       "有权限活动",
		KPIDS:       "[]",
		SkillConfig: "{}",
		Status:      model.ActivityStatusPublished,
		CreatedAt:   time.Now().Format(time.RFC3339),
	}
	if err := env.db.Create(&activity).Error; err != nil {
		t.Fatalf("Failed to create activity: %v", err)
	}

	// Teacher (owner) can view
	w := doRequest(env.router, "GET",
		fmt.Sprintf("/api/v1/activities/%d/sessions", activity.ID), nil, env.teacherToken)
	expectStatus(t, w, http.StatusOK)

	// Admin (non-owner) gets forbidden
	w = doRequest(env.router, "GET",
		fmt.Sprintf("/api/v1/activities/%d/sessions", activity.ID), nil, env.adminToken)
	expectStatus(t, w, http.StatusForbidden)
}

// -- Test: CORS Preflight ---------------------------------------------

func TestCORSPreflight(t *testing.T) {
	env := setupTestEnv(t)

	req := httptest.NewRequest("OPTIONS", "/api/v1/auth/login", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	expectStatus(t, w, http.StatusNoContent)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("Expected CORS Allow-Origin 'http://localhost:3000', got %q", got)
	}
}

func TestCORSPreflight_DisallowedOrigin(t *testing.T) {
	env := setupTestEnv(t)

	req := httptest.NewRequest("OPTIONS", "/api/v1/auth/login", nil)
	req.Header.Set("Origin", "http://evil.example.com")
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	expectStatus(t, w, http.StatusNoContent)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Expected no CORS Allow-Origin for disallowed origin, got %q", got)
	}
}
