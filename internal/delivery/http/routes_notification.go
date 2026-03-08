package http

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/hflms/hanfledge/internal/delivery/http/handler"
)

// registerNotificationRoutes 注册通知路由。
func registerNotificationRoutes(protected *gin.RouterGroup, db *gorm.DB) {
	h := handler.NewNotificationHandler(db)

	notif := protected.Group("/notifications")
	{
		notif.GET("/unread", h.GetUnread)
		notif.POST("/:id/read", h.MarkRead)
	}
}
