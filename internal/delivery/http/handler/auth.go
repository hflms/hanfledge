package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/hflms/hanfledge/internal/delivery/http/middleware"
	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/cache"
	"github.com/hflms/hanfledge/internal/plugin"
	"github.com/hflms/hanfledge/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

// AuthHandler handles authentication-related requests.
type AuthHandler struct {
	Users      repository.UserRepository
	JWTSecret  string
	JWTExpiry  int // hours
	EventBus   *plugin.EventBus
	RedisCache *cache.RedisCache
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(users repository.UserRepository, jwtSecret string, jwtExpiry int, eventBus *plugin.EventBus, redisCache *cache.RedisCache) *AuthHandler {
	return &AuthHandler{
		Users:      users,
		JWTSecret:  jwtSecret,
		JWTExpiry:  jwtExpiry,
		EventBus:   eventBus,
		RedisCache: redisCache,
	}
}

// publishEvent fires an EventBus event if the bus is available (nil-safe).
func (h *AuthHandler) publishEvent(ctx context.Context, hook plugin.HookPoint, payload map[string]interface{}) {
	plugin.PublishEvent(h.EventBus, ctx, hook, payload)
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
//
//	@Summary      用户登录
//	@Description  通过手机号和密码进行身份验证，返回 JWT token
//	@Tags         Auth
//	@Accept       json
//	@Produce      json
//	@Param        body  body      LoginRequest   true  "登录请求"
//	@Success      200   {object}  LoginResponse
//	@Failure      400   {object}  ErrorResponse
//	@Failure      401   {object}  ErrorResponse
//	@Failure      403   {object}  ErrorResponse
//	@Router       /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供手机号和密码"})
		return
	}

	// Find user by phone (preload roles so the response includes role info)
	user, err := h.Users.FindByPhone(c.Request.Context(), req.Phone)
	if err != nil {
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

	if h.RedisCache != nil {
		if err := h.RedisCache.SetUserSession(c.Request.Context(), tokenStr, user.ID); err != nil {
			// Log error but don't fail the login
			// Use the existing context or simply ignore for now
		}
	}

	c.JSON(http.StatusOK, LoginResponse{
		Token: tokenStr,
		User:  *user,
	})
}

// GetMe returns the current authenticated user with their roles.
//
//	@Summary      获取当前用户信息
//	@Description  返回已认证用户的个人信息和角色列表
//	@Tags         Auth
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200  {object}  model.User
//	@Failure      404  {object}  ErrorResponse
//	@Router       /auth/me [get]
func (h *AuthHandler) GetMe(c *gin.Context) {
	userID := middleware.GetUserID(c)

	if h.RedisCache != nil {
		if cachedUserJSON, err := h.RedisCache.GetUserCache(c.Request.Context(), userID); err == nil && cachedUserJSON != "" {
			// Found in cache, stream back the JSON
			c.Data(http.StatusOK, "application/json", []byte(cachedUserJSON))
			return
		}
	}

	user, err := h.Users.FindByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	if h.RedisCache != nil {
		importJSON, err := json.Marshal(user)
		if err == nil {
			_ = h.RedisCache.SetUserCache(c.Request.Context(), userID, string(importJSON))
		}
	}

	c.JSON(http.StatusOK, user)
}
