package http

import (
	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/delivery/http/handler"
	"gorm.io/gorm"
)

// NewRouter creates and configures the Gin router with all routes.
func NewRouter(db *gorm.DB) *gin.Engine {
	r := gin.Default()

	// Global middleware
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())

	// Health check
	r.GET("/health", handler.HealthCheck)

	// API v1 group
	v1 := r.Group("/api/v1")
	{
		// Auth routes (public)
		auth := v1.Group("/auth")
		{
			_ = auth
			// TODO Phase 1: auth.POST("/login", handler.Login)
			// TODO Phase 1: auth.GET("/me", authMiddleware(), handler.GetMe)
		}

		// TODO Phase 1: Protected routes with JWT + RBAC middleware
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
