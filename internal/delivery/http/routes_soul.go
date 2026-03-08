package http

import (
	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/delivery/http/handler"
	"github.com/hflms/hanfledge/internal/delivery/http/middleware"
	"github.com/hflms/hanfledge/internal/domain/model"
	"gorm.io/gorm"
)

func registerSoulRoutes(protected *gin.RouterGroup, db *gorm.DB, soulHandler *handler.SoulHandler) {
	adminOnly := middleware.RBAC(db, model.RoleSysAdmin)

	soul := protected.Group("/system/soul")
	soul.Use(adminOnly)
	{
		soul.GET("", soulHandler.GetSoul)
		soul.PUT("", soulHandler.UpdateSoul)
		soul.GET("/history", soulHandler.GetHistory)
		soul.POST("/rollback", soulHandler.Rollback)
		soul.POST("/evolve", soulHandler.Evolve)
	}
}
