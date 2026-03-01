package http

import (
	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/delivery/http/handler"
	"github.com/hflms/hanfledge/internal/delivery/http/middleware"
)

// registerAuthRoutes sets up authentication endpoints.
//
//	POST /api/v1/auth/login  — public
//	GET  /api/v1/auth/me     — protected (JWT)
func registerAuthRoutes(v1 *gin.RouterGroup, jwtSecret string, authHandler *handler.AuthHandler) {
	// Public
	auth := v1.Group("/auth")
	{
		auth.POST("/login", authHandler.Login)
	}

	// Protected
	authProtected := v1.Group("/auth")
	authProtected.Use(middleware.JWTAuth(jwtSecret))
	{
		authProtected.GET("/me", authHandler.GetMe)
	}
}
