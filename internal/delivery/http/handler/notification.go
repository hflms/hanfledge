package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/hflms/hanfledge/internal/domain/model"
)

// NotificationHandler 处理通知相关请求。
type NotificationHandler struct {
	db *gorm.DB
}

// NewNotificationHandler 创建通知处理器。
func NewNotificationHandler(db *gorm.DB) *NotificationHandler {
	return &NotificationHandler{db: db}
}

// GetUnread 获取当前用户未读通知。
// GET /api/v1/notifications/unread
func (h *NotificationHandler) GetUnread(c *gin.Context) {
	userID := c.GetUint("user_id")

	var notifs []model.Notification
	if err := h.db.Where("user_id = ? AND is_read = ?", userID, false).
		Order("created_at DESC").
		Limit(20).
		Find(&notifs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询通知失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"notifications": notifs})
}

// MarkRead 标记通知为已读。
// POST /api/v1/notifications/:id/read
func (h *NotificationHandler) MarkRead(c *gin.Context) {
	notifID := c.Param("id")

	if err := h.db.Model(&model.Notification{}).
		Where("id = ?", notifID).
		Update("is_read", true).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "标记失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已标记为已读"})
}
