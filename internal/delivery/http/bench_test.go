package http_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
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

// -- Benchmark Helpers ------------------------------------------------

func setupBenchDB(b *testing.B) *gorm.DB {
	b.Helper()

	dsn := "host=localhost port=5433 user=hanfledge password=hanfledge_secret dbname=hanfledge_test sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		b.Skipf("Skipping benchmark: cannot connect to test database: %v", err)
	}

	db.Exec("CREATE EXTENSION IF NOT EXISTS vector")

	err = db.AutoMigrate(
		&model.User{}, &model.School{}, &model.Class{}, &model.Role{},
		&model.UserSchoolRole{}, &model.ClassStudent{}, &model.Course{},
		&model.Chapter{}, &model.KnowledgePoint{}, &model.KPSkillMount{},
		&model.LearningActivity{}, &model.ActivityClassAssignment{},
		&model.StudentSession{}, &model.Interaction{}, &model.StudentKPMastery{},
		&model.Document{}, &model.DocumentChunk{},
	)
	if err != nil {
		b.Fatalf("AutoMigrate failed: %v", err)
	}

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

func generateBenchToken(b *testing.B, userID uint, phone, displayName string) string {
	b.Helper()
	claims := middleware.JWTClaims{
		UserID:      userID,
		Phone:       phone,
		DisplayName: displayName,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testJWTSecret))
	if err != nil {
		b.Fatalf("Failed to sign token: %v", err)
	}
	return signed
}

func setupBenchEnv(b *testing.B) *testEnv {
	b.Helper()

	// Suppress Gin's per-request logging so benchmark output is clean.
	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = io.Discard

	db := setupBenchDB(b)

	hash, _ := bcrypt.GenerateFromPassword([]byte("test123"), bcrypt.MinCost)

	admin := model.User{Phone: "19900000001", PasswordHash: string(hash), DisplayName: "基准管理员", Status: model.UserStatusActive}
	db.Create(&admin)
	teacher := model.User{Phone: "19900000010", PasswordHash: string(hash), DisplayName: "基准张老师", Status: model.UserStatusActive}
	db.Create(&teacher)
	student := model.User{Phone: "19900000100", PasswordHash: string(hash), DisplayName: "基准王同学", Status: model.UserStatusActive}
	db.Create(&student)

	school := model.School{Name: "基准学校", Code: "BENCH01", Region: "测试区", Status: model.SchoolStatusActive}
	db.Create(&school)
	class1 := model.Class{SchoolID: school.ID, Name: "基准1班", GradeLevel: 10, AcademicYear: "2025-2026"}
	db.Create(&class1)

	db.Create(&model.UserSchoolRole{UserID: admin.ID, SchoolID: nil, RoleID: 1})
	db.Create(&model.UserSchoolRole{UserID: teacher.ID, SchoolID: &school.ID, RoleID: 3})
	db.Create(&model.UserSchoolRole{UserID: student.ID, SchoolID: &school.ID, RoleID: 4})
	db.Create(&model.ClassStudent{ClassID: class1.ID, StudentID: student.ID})

	registry := plugin.NewRegistry()
	guard := safety.NewInjectionGuard()
	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:      testJWTSecret,
			ExpiryHours: testJWTExpiry,
		},
	}
	router := delivery.NewRouter(delivery.RouterDeps{
		DB:             db,
		Cfg:            cfg,
		Registry:       registry,
		InjectionGuard: guard,
	})

	return &testEnv{
		db:           db,
		router:       router,
		adminToken:   generateBenchToken(b, admin.ID, admin.Phone, admin.DisplayName),
		teacherToken: generateBenchToken(b, teacher.ID, teacher.Phone, teacher.DisplayName),
		studentToken: generateBenchToken(b, student.ID, student.Phone, student.DisplayName),
		adminID:      admin.ID,
		teacherID:    teacher.ID,
		studentID:    student.ID,
		schoolID:     school.ID,
		class1ID:     class1.ID,
	}
}

// -- Benchmark: Login Throughput --------------------------------------

func BenchmarkLogin(b *testing.B) {
	env := setupBenchEnv(b)

	body := map[string]interface{}{
		"phone":    "19900000100",
		"password": "test123",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := doRequest(env.router, "POST", "/api/v1/auth/login", body, "")
			if w.Code != http.StatusOK {
				b.Errorf("login failed: %d", w.Code)
			}
		}
	})
}

// -- Benchmark: GetMe (Authenticated) --------------------------------

func BenchmarkGetMe(b *testing.B) {
	env := setupBenchEnv(b)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := doRequest(env.router, "GET", "/api/v1/auth/me", nil, env.studentToken)
			if w.Code != http.StatusOK {
				b.Errorf("GetMe failed: %d", w.Code)
			}
		}
	})
}

// -- Benchmark: ListCourses (Teacher) ---------------------------------

func BenchmarkListCourses(b *testing.B) {
	env := setupBenchEnv(b)

	for i := 0; i < 10; i++ {
		env.db.Create(&model.Course{
			SchoolID: env.schoolID, TeacherID: env.teacherID,
			Title: fmt.Sprintf("基准课程%d", i), Subject: "math", GradeLevel: 10,
		})
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := doRequest(env.router, "GET", "/api/v1/courses", nil, env.teacherToken)
			if w.Code != http.StatusOK {
				b.Errorf("ListCourses failed: %d", w.Code)
			}
		}
	})
}

// -- Benchmark: Health Check ------------------------------------------

func BenchmarkHealthCheck(b *testing.B) {
	env := setupBenchEnv(b)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := doRequest(env.router, "GET", "/health", nil, "")
			if w.Code != http.StatusOK {
				b.Errorf("health check failed: %d", w.Code)
			}
		}
	})
}

// -- Concurrent Session Simulation ------------------------------------

// TestConcurrentAPICalls simulates concurrent users hitting various endpoints
// and reports throughput and error rate.
func TestConcurrentAPICalls(t *testing.T) {
	env := setupTestEnv(t)

	course := model.Course{SchoolID: env.schoolID, TeacherID: env.teacherID, Title: "并发测试课程", Subject: "math", GradeLevel: 10}
	env.db.Create(&course)

	activity := model.LearningActivity{
		CourseID: course.ID, TeacherID: env.teacherID,
		Title: "并发测试活动", KPIDS: "[]", SkillConfig: "{}",
		Status: model.ActivityStatusPublished, CreatedAt: time.Now().Format(time.RFC3339),
	}
	env.db.Create(&activity)

	concurrency := 50
	requestsPerWorker := 20

	var totalOK atomic.Int64
	var totalErr atomic.Int64

	endpoints := []struct {
		method string
		path   string
		token  string
	}{
		{"GET", "/health", ""},
		{"GET", "/api/v1/auth/me", env.teacherToken},
		{"GET", "/api/v1/courses", env.teacherToken},
		{"GET", "/api/v1/auth/me", env.studentToken},
		{"GET", fmt.Sprintf("/api/v1/activities/%d/sessions", activity.ID), env.teacherToken},
	}

	start := time.Now()

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < requestsPerWorker; j++ {
				ep := endpoints[(workerID+j)%len(endpoints)]
				w := httptest.NewRecorder()
				req := httptest.NewRequest(ep.method, ep.path, nil)
				if ep.token != "" {
					req.Header.Set("Authorization", "Bearer "+ep.token)
				}
				env.router.ServeHTTP(w, req)
				if w.Code >= 200 && w.Code < 400 {
					totalOK.Add(1)
				} else {
					totalErr.Add(1)
				}
			}
		}(i)
	}
	wg.Wait()

	elapsed := time.Since(start)
	total := int64(concurrency * requestsPerWorker)
	rps := float64(total) / elapsed.Seconds()

	t.Logf("Concurrent API benchmark:")
	t.Logf("  Workers:       %d", concurrency)
	t.Logf("  Requests/each: %d", requestsPerWorker)
	t.Logf("  Total:         %d", total)
	t.Logf("  Duration:      %s", elapsed.Round(time.Millisecond))
	t.Logf("  Throughput:    %.0f req/s", rps)
	t.Logf("  Success:       %d", totalOK.Load())
	t.Logf("  Errors:        %d", totalErr.Load())
	t.Logf("  Error rate:    %.2f%%", float64(totalErr.Load())/float64(total)*100)

	if totalErr.Load() > 0 {
		t.Logf("WARNING: %d requests returned error status codes", totalErr.Load())
	}
}
