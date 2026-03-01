package http

import (
	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/delivery/http/handler"
	"github.com/hflms/hanfledge/internal/delivery/http/middleware"
	"github.com/hflms/hanfledge/internal/domain/model"
	"gorm.io/gorm"
)

// registerAdminRoutes sets up school, class, and user management endpoints.
// These are restricted to SYS_ADMIN and/or SCHOOL_ADMIN roles.
func registerAdminRoutes(protected *gin.RouterGroup, db *gorm.DB, userHandler *handler.UserHandler) {
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
