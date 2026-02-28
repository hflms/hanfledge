package http

import (
	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/agent"
	"github.com/hflms/hanfledge/internal/config"
	"github.com/hflms/hanfledge/internal/delivery/http/handler"
	"github.com/hflms/hanfledge/internal/delivery/http/middleware"
	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/cache"
	"github.com/hflms/hanfledge/internal/infrastructure/safety"
	"github.com/hflms/hanfledge/internal/plugin"
	neo4jRepo "github.com/hflms/hanfledge/internal/repository/neo4j"
	"github.com/hflms/hanfledge/internal/usecase"
	"gorm.io/gorm"
)

// NewRouter creates and configures the Gin router with all routes.
func NewRouter(db *gorm.DB, cfg *config.Config, karag *usecase.KARAGEngine, registry *plugin.Registry, orchestrator *agent.AgentOrchestrator, injectionGuard *safety.InjectionGuard, neo4jClient *neo4jRepo.Client, redisCache *cache.RedisCache, piiRedactor *safety.PIIRedactor) *gin.Engine {
	r := gin.Default()

	// Global middleware
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())

	// Health check
	r.GET("/health", handler.HealthCheck)

	// Initialize handlers
	authHandler := handler.NewAuthHandler(db, cfg.JWT.Secret, cfg.JWT.ExpiryHours)
	userHandler := handler.NewUserHandler(db)
	courseHandler := handler.NewCourseHandler(db, karag, redisCache)
	skillHandler := handler.NewSkillHandler(db, registry)
	activityHandler := handler.NewActivityHandler(db, orchestrator)
	sessionHandler := handler.NewSessionHandler(db, orchestrator, injectionGuard)
	dashboardHandler := handler.NewDashboardHandler(db)
	kgHandler := handler.NewKnowledgeGraphHandler(db, neo4jClient)
	analyticsHandler := handler.NewAnalyticsHandler(db, piiRedactor)
	exportHandler := handler.NewExportHandler(db)

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
				courses.DELETE("/:id/documents/:doc_id", courseHandler.DeleteDocument)
				courses.POST("/:id/documents/:doc_id/retry", courseHandler.RetryDocument)
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

		// ── Knowledge Graph Enrichment (TEACHER) — Phase B ─
		kps := protected.Group("/knowledge-points")
		kps.Use(middleware.RBAC(db, model.RoleTeacher, model.RoleSchoolAdmin, model.RoleSysAdmin))
		{
			// Misconception CRUD
			kps.POST("/:id/misconceptions", kgHandler.CreateMisconception)
			kps.GET("/:id/misconceptions", kgHandler.ListMisconceptions)
			kps.DELETE("/:id/misconceptions/:misconception_id", kgHandler.DeleteMisconception)

			// Cross-Disciplinary Links
			kps.POST("/:id/cross-links", kgHandler.CreateCrossLink)
			kps.GET("/:id/cross-links", kgHandler.ListCrossLinks)
			kps.DELETE("/:id/cross-links/:link_id", kgHandler.DeleteCrossLink)

			// Prerequisite Management
			kps.POST("/:id/prerequisites", kgHandler.CreatePrerequisite)
			kps.GET("/:id/prerequisites", kgHandler.GetPrerequisites)
		}

		// ── Dashboard Analytics (TEACHER) — Phase 5 + Phase G ─
		dashboard := protected.Group("/dashboard")
		dashboard.Use(middleware.RBAC(db, model.RoleTeacher, model.RoleSchoolAdmin, model.RoleSysAdmin))
		{
			dashboard.GET("/knowledge-radar", dashboardHandler.GetKnowledgeRadar)
			dashboard.GET("/skill-effectiveness", analyticsHandler.GetSkillEffectiveness) // Phase G
		}

		// ── Student Mastery (TEACHER) — Phase 5 ─────────
		students := protected.Group("/students")
		students.Use(middleware.RBAC(db, model.RoleTeacher, model.RoleSchoolAdmin, model.RoleSysAdmin))
		{
			students.GET("/:id/mastery", dashboardHandler.GetStudentMastery)
		}

		// ── Learning Activities (TEACHER) — Phase 4 ──────
		activities := protected.Group("/activities")
		activities.Use(middleware.RBAC(db, model.RoleTeacher, model.RoleSchoolAdmin, model.RoleSysAdmin))
		{
			activities.POST("", activityHandler.CreateActivity)
			activities.GET("", activityHandler.ListActivities)
			activities.POST("/:id/publish", activityHandler.PublishActivity)
			activities.GET("/:id/sessions", dashboardHandler.GetActivitySessions) // Phase 5
		}

		// ── Student Routes — Phase 4 ────────────────────
		student := protected.Group("/student")
		student.Use(middleware.RBAC(db, model.RoleStudent, model.RoleSysAdmin))
		{
			student.GET("/activities", activityHandler.StudentListActivities)
			student.GET("/mastery", dashboardHandler.GetSelfMastery)          // Phase 5
			student.GET("/knowledge-map", kgHandler.GetStudentKnowledgeMap)   // Knowledge Map
			student.GET("/error-notebook", dashboardHandler.GetErrorNotebook) // Error Notebook
		}

		// ── Activity Join & Sessions (any authenticated) ─
		protected.POST("/activities/:id/join", activityHandler.JoinActivity)
		protected.GET("/sessions/:id", activityHandler.GetSession)

		// ── Session Analytics (TEACHER) — Phase G ──────
		sessionAnalytics := protected.Group("/sessions")
		sessionAnalytics.Use(middleware.RBAC(db, model.RoleTeacher, model.RoleSchoolAdmin, model.RoleSysAdmin))
		{
			sessionAnalytics.GET("/:id/inquiry-tree", analyticsHandler.GetInquiryTree)
			sessionAnalytics.GET("/:id/interactions", analyticsHandler.GetInteractionLog)
		}

		// ── Data Export (TEACHER) — CSV Downloads ────────
		export := protected.Group("/export")
		export.Use(middleware.RBAC(db, model.RoleTeacher, model.RoleSchoolAdmin, model.RoleSysAdmin))
		{
			export.GET("/activities/:id/sessions", exportHandler.ExportActivitySessions)
			export.GET("/courses/:id/mastery", exportHandler.ExportClassMastery)
			export.GET("/courses/:id/error-notebook", exportHandler.ExportErrorNotebook)
			export.GET("/sessions/:id/interactions", exportHandler.ExportInteractionLog)
		}

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
