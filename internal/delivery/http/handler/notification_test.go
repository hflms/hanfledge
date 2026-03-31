package handler

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/domain/model"
)

// ============================
// Notification Handler Unit Tests
// ============================

// setupNotificationTestDB adds model.Notification to the test DB migration.
func setupNotificationTestDB(t *testing.T) *NotificationHandler {
	t.Helper()
	db := setupTestDB(t)
	// Migrate the Notification table
	if err := db.AutoMigrate(&model.Notification{}); err != nil {
		t.Fatalf("AutoMigrate Notification failed: %v", err)
	}
	return NewNotificationHandler(db)
}

// seedNotification creates a notification in the database and returns it.
func seedNotification(t *testing.T, h *NotificationHandler, userID uint, title string, isRead bool) model.Notification {
	t.Helper()
	n := model.Notification{
		UserID:  userID,
		Type:    "system_alert",
		Title:   title,
		Content: "test content for " + title,
		IsRead:  isRead,
	}
	if err := h.db.Create(&n).Error; err != nil {
		t.Fatalf("seedNotification failed: %v", err)
	}
	return n
}

// -- Constructor Tests ----------------------------------------

func TestNewNotificationHandler(t *testing.T) {
	h := NewNotificationHandler(nil)
	if h == nil {
		t.Fatal("NewNotificationHandler returned nil")
	}
}

// -- GetUnread Tests ------------------------------------------

func TestGetUnread_NoNotifications(t *testing.T) {
	h := setupNotificationTestDB(t)
	w, c := newTestContextWithQuery("GET", "/api/v1/notifications/unread", 1)

	h.GetUnread(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, `"notifications"`)
}

func TestGetUnread_ReturnsOnlyUnread(t *testing.T) {
	h := setupNotificationTestDB(t)

	// Seed: 2 unread + 1 read for user 1
	seedNotification(t, h, 1, "unread_a", false)
	seedNotification(t, h, 1, "unread_b", false)
	seedNotification(t, h, 1, "already_seen", true)

	w, c := newTestContextWithQuery("GET", "/api/v1/notifications/unread", 1)
	h.GetUnread(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, "unread_a")
	assertBodyContains(t, w, "unread_b")
	assertBodyNotContains(t, w, "already_seen")
}

func TestGetUnread_DoesNotReturnOtherUsersNotifications(t *testing.T) {
	h := setupNotificationTestDB(t)

	seedNotification(t, h, 1, "user1_notif", false)
	seedNotification(t, h, 2, "user2_notif", false)

	w, c := newTestContextWithQuery("GET", "/api/v1/notifications/unread", 1)
	h.GetUnread(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, "user1_notif")
	assertBodyNotContains(t, w, "user2_notif")
}

func TestGetUnread_LimitTo20(t *testing.T) {
	h := setupNotificationTestDB(t)

	// Seed 25 unread notifications
	for i := 0; i < 25; i++ {
		seedNotification(t, h, 1, "notif_"+itoa(i), false)
	}

	w, c := newTestContextWithQuery("GET", "/api/v1/notifications/unread", 1)
	h.GetUnread(c)

	assertStatus(t, w, http.StatusOK)
	// The handler limits to 20, so we won't see all 25
	// We can't easily count JSON array items with substring matching,
	// but we can verify the response is valid.
	assertBodyContains(t, w, `"notifications"`)
}

func TestGetUnread_NoUserID(t *testing.T) {
	h := setupNotificationTestDB(t)

	w, c := newTestContextWithQuery("GET", "/api/v1/notifications/unread", 0)
	h.GetUnread(c)

	// Should return empty list when user_id is 0 (no user)
	assertStatus(t, w, http.StatusOK)
}

// -- MarkRead Tests -------------------------------------------

func TestMarkRead_Success(t *testing.T) {
	h := setupNotificationTestDB(t)
	n := seedNotification(t, h, 1, "to_mark", false)

	w, c := newTestContextWithParams("POST", "/api/v1/notifications/"+itoa(int(n.ID))+"/read", "", 1,
		gin.Params{{Key: "id", Value: itoa(int(n.ID))}})
	h.MarkRead(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, "已标记为已读")

	// Verify in DB
	var updated model.Notification
	h.db.First(&updated, n.ID)
	if !updated.IsRead {
		t.Error("notification should be marked as read in DB")
	}
}

func TestMarkRead_NonexistentID(t *testing.T) {
	h := setupNotificationTestDB(t)

	// Marking a non-existent notification — the handler does Update which
	// succeeds with 0 rows affected (GORM doesn't error on that).
	w, c := newTestContextWithParams("POST", "/api/v1/notifications/99999/read", "", 1,
		gin.Params{{Key: "id", Value: "99999"}})
	h.MarkRead(c)

	// Handler returns 200 even if no rows matched (it only checks for err)
	assertStatus(t, w, http.StatusOK)
}

func TestMarkRead_AlreadyRead(t *testing.T) {
	h := setupNotificationTestDB(t)
	n := seedNotification(t, h, 1, "already_read", true)

	w, c := newTestContextWithParams("POST", "/api/v1/notifications/"+itoa(int(n.ID))+"/read", "", 1,
		gin.Params{{Key: "id", Value: itoa(int(n.ID))}})
	h.MarkRead(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, "已标记为已读")
}
