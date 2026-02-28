package http

import (
	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/agent"
	"github.com/hflms/hanfledge/internal/config"
	"github.com/hflms/hanfledge/internal/delivery/http/handler"
	"github.com/hflms/hanfledge/internal/delivery/http/middleware"
	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/plugin"
	"github.com/hflms/hanfledge/internal/usecase"
	"gorm.io/gorm"
)

// NewRouter creates and configures the Gin router with all routes.
func NewRouter(db *gorm.DB, cfg *config.Config, karag *usecase.KARAGEngine, registry *plugin.Registry, orchestrator *agent.AgentOrchestrator) *gin.Engine {
	r := gin.Default()

	// Global middleware
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())

	// Health check
	r.GET("/health", handler.HealthCheck)

	// Initialize handlers
	authHandler := handler.NewAuthHandler(db, cfg.JWT.Secret, cfg.JWT.ExpiryHours)
	userHandler := handler.NewUserHandler(db)
	courseHandler := handler.NewCourseHandler(db, karag)
	skillHandler := handler.NewSkillHandler(db, registry)
	activityHandler := handler.NewActivityHandler(db, orchestrator)
	sessionHandler := handler.NewSessionHandler(db, orchestrator)

	// API v1 group
	v1 := r.Group("/api/v1")
	{
		// ── Auth (Public) ────────────────────────────────
		auth := v1.Group("/auth")
		{
			auth.POST("/login", authHandler.Login)
		}

		// ── Auth (Protected) ─────────────────────────────
		authProtected := v1.Group("/auth")
		authProtected.Use(middleware.JWTAuth(cfg.JWT.Secret))
		{
			authProtected.GET("/me", authHandler.GetMe)
		}

		// ── Protected Routes ─────────────────────────────
		protected := v1.Group("")
		protected.Use(middleware.JWTAuth(cfg.JWT.Secret))
		{
			// School management (SYS_ADMIN only)
			schools := protected.Group("/schools")
			schools.Use(middleware.RBAC(db, model.RoleSysAdmin))
			{
				schools.GET("", userHandler.ListSchools)
				schools.POST("", userHandler.CreateSchool)
			}

			// Class management (SYS_ADMIN or SCHOOL_ADMIN)
			classes := protected.Group("/classes")
			classes.Use(middleware.RBAC(db, model.RoleSysAdmin, model.RoleSchoolAdmin))
			{
				classes.GET("", userHandler.ListClasses)
				classes.POST("", userHandler.CreateClass)
			}

			// User management (SYS_ADMIN or SCHOOL_ADMIN)
			users := protected.Group("/users")
			users.Use(middleware.RBAC(db, model.RoleSysAdmin, model.RoleSchoolAdmin))
			{
				users.GET("", userHandler.ListUsers)
				users.POST("", userHandler.CreateUser)
				users.POST("/batch", userHandler.BatchCreateUsers)
			}

			// Course management (TEACHER)
			courses := protected.Group("/courses")
			courses.Use(middleware.RBAC(db, model.RoleTeacher, model.RoleSchoolAdmin, model.RoleSysAdmin))
			{
				courses.GET("", courseHandler.ListCourses)
				courses.POST("", courseHandler.CreateCourse)
				courses.POST("/:id/materials", courseHandler.UploadMaterial)
				courses.GET("/:id/outline", courseHandler.GetOutline)
				courses.GET("/:id/documents", courseHandler.GetDocumentStatus)
				courses.POST("/:id/search", courseHandler.SearchCourse)
			}
		}

		// Skill Store (TEACHER) — Phase 3
		skills := protected.Group("/skills")
		skills.Use(middleware.RBAC(db, model.RoleTeacher, model.RoleSchoolAdmin, model.RoleSysAdmin))
		{
			skills.GET("", skillHandler.ListSkills)
			skills.GET("/:id", skillHandler.GetSkillDetail)
		}

		// Skill Mounting (TEACHER) — Phase 3
		chapters := protected.Group("/chapters")
		chapters.Use(middleware.RBAC(db, model.RoleTeacher, model.RoleSchoolAdmin, model.RoleSysAdmin))
		{
			chapters.POST("/:id/skills", skillHandler.MountSkill)
			chapters.PATCH("/:id/skills/:mount_id", skillHandler.UpdateSkillConfig)
			chapters.DELETE("/:id/skills/:mount_id", skillHandler.UnmountSkill)
		}

		// TODO Phase 5: Dashboard routes

		// ── Learning Activities (TEACHER) — Phase 4 ──────
		activities := protected.Group("/activities")
		activities.Use(middleware.RBAC(db, model.RoleTeacher, model.RoleSchoolAdmin, model.RoleSysAdmin))
		{
			activities.POST("", activityHandler.CreateActivity)
			activities.GET("", activityHandler.ListActivities)
			activities.POST("/:id/publish", activityHandler.PublishActivity)
		}

		// ── Student Routes — Phase 4 ────────────────────
		student := protected.Group("/student")
		student.Use(middleware.RBAC(db, model.RoleStudent, model.RoleSysAdmin))
		{
			student.GET("/activities", activityHandler.StudentListActivities)
		}

		// ── Activity Join & Sessions (any authenticated) ─
		protected.POST("/activities/:id/join", activityHandler.JoinActivity)
		protected.GET("/sessions/:id", activityHandler.GetSession)

		// ── WebSocket Session Stream — Phase 4 ──────────
		protected.GET("/sessions/:id/stream", sessionHandler.StreamSession)
	}

	return r
}

// corsMiddleware handles CORS for frontend development.
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
