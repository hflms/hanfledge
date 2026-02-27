package http

import (
	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/config"
	"github.com/hflms/hanfledge/internal/delivery/http/handler"
	"github.com/hflms/hanfledge/internal/delivery/http/middleware"
	"github.com/hflms/hanfledge/internal/domain/model"
	"gorm.io/gorm"
)

// NewRouter creates and configures the Gin router with all routes.
func NewRouter(db *gorm.DB, cfg *config.Config) *gin.Engine {
	r := gin.Default()

	// Global middleware
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())

	// Health check
	r.GET("/health", handler.HealthCheck)

	// Initialize handlers
	authHandler := handler.NewAuthHandler(db, cfg.JWT.Secret, cfg.JWT.ExpiryHours)
	userHandler := handler.NewUserHandler(db)

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
		}

		// TODO Phase 2: Course routes
		// TODO Phase 3: Skill routes
		// TODO Phase 4: Session WebSocket routes
		// TODO Phase 5: Dashboard routes
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
