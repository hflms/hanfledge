package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/hflms/hanfledge/internal/delivery/http/middleware"
	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/plugin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AuthHandler handles authentication-related requests.
type AuthHandler struct {
	DB        *gorm.DB
	JWTSecret string
	JWTExpiry int // hours
	EventBus  *plugin.EventBus
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(db *gorm.DB, jwtSecret string, jwtExpiry int, eventBus *plugin.EventBus) *AuthHandler {
	return &AuthHandler{
		DB:        db,
		JWTSecret: jwtSecret,
		JWTExpiry: jwtExpiry,
		EventBus:  eventBus,
	}
}

// publishEvent fires an EventBus event if the bus is available (nil-safe).
func (h *AuthHandler) publishEvent(ctx context.Context, hook plugin.HookPoint, payload map[string]interface{}) {
	if h.EventBus == nil {
		return
	}
	h.EventBus.Publish(ctx, plugin.HookEvent{Hook: hook, Payload: payload})
}

// LoginRequest represents the login request body.
type LoginRequest struct {
	Phone    string `json:"phone" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse represents the login response.
type LoginResponse struct {
	Token string     `json:"token"`
	User  model.User `json:"user"`
}

// Login handles user authentication and returns a JWT token.
// POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供手机号和密码"})
		return
	}

	// Find user by phone (preload roles so the response includes role info)
	var user model.User
	if err := h.DB.Preload("SchoolRoles.Role").Preload("SchoolRoles.School").
		Where("phone = ?", req.Phone).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "手机号或密码错误"})
		return
	}

	// Check password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "手机号或密码错误"})
		return
	}

	// Check user status
	if user.Status != model.UserStatusActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "账户已被禁用"})
		return
	}

	// Generate JWT token
	now := time.Now()
	claims := &middleware.JWTClaims{
		UserID:      user.ID,
		Phone:       user.Phone,
		DisplayName: user.DisplayName,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(h.JWTExpiry) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "hanfledge",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(h.JWTSecret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成令牌失败"})
		return
	}

	// Hook: on user login
	h.publishEvent(c.Request.Context(), plugin.HookOnUserLogin, map[string]interface{}{
		"user_id":      user.ID,
		"phone":        user.Phone,
		"display_name": user.DisplayName,
	})

	c.JSON(http.StatusOK, LoginResponse{
		Token: tokenStr,
		User:  user,
	})
}

// GetMe returns the current authenticated user with their roles.
// GET /api/v1/auth/me
func (h *AuthHandler) GetMe(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var user model.User
	if err := h.DB.Preload("SchoolRoles.Role").Preload("SchoolRoles.School").
		First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	c.JSON(http.StatusOK, user)
}
