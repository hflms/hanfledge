package http

import (
	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/delivery/http/handler"
	"github.com/hflms/hanfledge/internal/delivery/http/middleware"
	"github.com/hflms/hanfledge/internal/domain/model"
	"gorm.io/gorm"
)

// registerMarketplaceRoutes sets up plugin marketplace endpoints.
// Read endpoints are available to all authenticated users.
// Write operations require TEACHER role or above.
func registerMarketplaceRoutes(protected *gin.RouterGroup, db *gorm.DB, marketplaceHandler *handler.MarketplaceHandler) {
	marketplace := protected.Group("/marketplace")
	{
		// Read-only browsing — all authenticated roles
		marketplace.GET("/plugins", marketplaceHandler.ListPlugins)
		marketplace.GET("/plugins/:plugin_id", marketplaceHandler.GetPlugin)
		marketplace.GET("/installed", marketplaceHandler.ListInstalled)

		// Write operations — TEACHER and above
		marketplaceWrite := marketplace.Group("")
		marketplaceWrite.Use(middleware.RBAC(db, model.RoleTeacher, model.RoleSchoolAdmin, model.RoleSysAdmin))
		{
			marketplaceWrite.POST("/plugins", marketplaceHandler.SubmitPlugin)
			marketplaceWrite.POST("/install", marketplaceHandler.InstallPlugin)
			marketplaceWrite.DELETE("/installed/:id", marketplaceHandler.UninstallPlugin)
		}
	}
}
