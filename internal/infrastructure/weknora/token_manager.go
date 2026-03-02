package weknora

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/cache"
	"gorm.io/gorm"
)

// TokenManager manages per-user WeKnora tokens with Redis caching and DB persistence.
// Flow: Redis cache → DB lookup → auto-register + login (first time) → auto-refresh (expired)
type TokenManager struct {
	client *Client
	db     *gorm.DB
	cache  *cache.RedisCache // Optional, nil-safe
	secret string            // Used to generate deterministic passwords
}

// NewTokenManager creates a new TokenManager.
//
// Parameters:
//   - client: WeKnora API client (used for register/login/refresh calls)
//   - db: database for persisting token mappings
//   - redisCache: optional Redis cache (nil-safe)
//   - secret: shared secret for deterministic password generation (typically WEKNORA_API_KEY)
func NewTokenManager(client *Client, db *gorm.DB, redisCache *cache.RedisCache, secret string) *TokenManager {
	return &TokenManager{
		client: client,
		db:     db,
		cache:  redisCache,
		secret: secret,
	}
}

// GetToken returns a valid WeKnora access token for the given Hanfledge user.
// It follows the cascade: Redis → DB → register+login, with automatic refresh.
func (m *TokenManager) GetToken(ctx context.Context, userID uint) (string, error) {
	cacheKey := fmt.Sprintf("weknora:token:%d", userID)

	// 1. Try Redis cache
	if m.cache != nil {
		if token, err := m.cache.GetString(ctx, cacheKey); err == nil && token != "" {
			return token, nil
		}
	}

	// 2. Try DB lookup
	var wkToken model.WeKnoraToken
	result := m.db.Where("user_id = ?", userID).First(&wkToken)

	if result.Error == gorm.ErrRecordNotFound {
		// 3. First time — auto-register and login
		return m.registerAndLogin(ctx, userID, cacheKey)
	}
	if result.Error != nil {
		return "", fmt.Errorf("query token mapping: %w", result.Error)
	}

	// 4. Check if token is still valid (with 5-minute buffer)
	if time.Now().Add(5 * time.Minute).Before(wkToken.ExpiresAt) {
		// Token is valid — cache it and return
		m.cacheToken(ctx, cacheKey, wkToken.Token, time.Until(wkToken.ExpiresAt)-5*time.Minute)
		return wkToken.Token, nil
	}

	// 5. Token expired — try refresh
	return m.refreshAndSave(ctx, userID, &wkToken, cacheKey)
}

// GetClientForUser returns a WeKnora Client configured with the user's personal token.
func (m *TokenManager) GetClientForUser(ctx context.Context, userID uint) (*Client, error) {
	token, err := m.GetToken(ctx, userID)
	if err != nil {
		return nil, err
	}
	return m.client.WithToken(token), nil
}

// -- Internal methods ------------------------------------------------------

// generateEmail creates a deterministic email for a Hanfledge user on WeKnora.
func (m *TokenManager) generateEmail(userID uint) string {
	return fmt.Sprintf("user_%d@hanfledge.local", userID)
}

// generatePassword creates a deterministic password from the shared secret and user ID.
func (m *TokenManager) generatePassword(userID uint) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", m.secret, userID)))
	return fmt.Sprintf("%x", h)[:16]
}

// registerAndLogin handles first-time user setup: register on WeKnora, then login.
func (m *TokenManager) registerAndLogin(ctx context.Context, userID uint, cacheKey string) (string, error) {
	email := m.generateEmail(userID)
	password := m.generatePassword(userID)
	username := fmt.Sprintf("hanfledge_%d", userID)

	// Register (ignore "already exists" errors — user may have been registered in a previous run)
	_, err := m.client.Register(ctx, &RegisterRequest{
		Email:    email,
		Password: password,
		Username: username,
	})
	if err != nil {
		slog.Warn("WeKnora register attempt", "user_id", userID, "err", err)
		// Continue to login even if register fails (user may already exist)
	}

	// Login
	loginResp, err := m.client.Login(ctx, &LoginRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		return "", fmt.Errorf("WeKnora login failed for user %d: %w", userID, err)
	}

	// Determine WeKnora user ID
	wkUserID := ""
	if loginResp.User != nil {
		wkUserID = loginResp.User.ID
	}

	// Persist to DB
	wkToken := model.WeKnoraToken{
		UserID:       userID,
		WKUserID:     wkUserID,
		WKEmail:      email,
		Token:        loginResp.Token,
		RefreshToken: loginResp.RefreshToken,
		ExpiresAt:    time.Now().Add(24 * time.Hour), // Default 24h, will be refreshed
	}
	if err := m.db.Create(&wkToken).Error; err != nil {
		return "", fmt.Errorf("persist token mapping: %w", err)
	}

	// Cache in Redis
	m.cacheToken(ctx, cacheKey, loginResp.Token, 23*time.Hour)

	slog.Info("WeKnora user auto-registered and logged in", "user_id", userID, "wk_email", email)
	return loginResp.Token, nil
}

// refreshAndSave refreshes an expired token and updates DB + cache.
func (m *TokenManager) refreshAndSave(ctx context.Context, userID uint, wkToken *model.WeKnoraToken, cacheKey string) (string, error) {
	refreshResp, err := m.client.RefreshToken(ctx, wkToken.RefreshToken)
	if err != nil {
		// Refresh failed — fallback to re-login
		slog.Warn("WeKnora token refresh failed, re-logging in", "user_id", userID, "err", err)
		password := m.generatePassword(userID)
		loginResp, loginErr := m.client.Login(ctx, &LoginRequest{
			Email:    wkToken.WKEmail,
			Password: password,
		})
		if loginErr != nil {
			return "", fmt.Errorf("WeKnora re-login failed for user %d: %w", userID, loginErr)
		}
		refreshResp = loginResp
	}

	// Update DB
	wkToken.Token = refreshResp.Token
	if refreshResp.RefreshToken != "" {
		wkToken.RefreshToken = refreshResp.RefreshToken
	}
	wkToken.ExpiresAt = time.Now().Add(24 * time.Hour)

	if err := m.db.Save(wkToken).Error; err != nil {
		slog.Error("failed to update token in DB", "user_id", userID, "err", err)
	}

	// Update cache
	m.cacheToken(ctx, cacheKey, refreshResp.Token, 23*time.Hour)

	return refreshResp.Token, nil
}

// cacheToken stores a token in Redis (no-op if Redis is nil).
func (m *TokenManager) cacheToken(ctx context.Context, key, token string, ttl time.Duration) {
	if m.cache == nil {
		return
	}
	if ttl <= 0 {
		ttl = 1 * time.Hour
	}
	if err := m.cache.SetString(ctx, key, token, ttl); err != nil {
		slog.Warn("failed to cache WeKnora token in Redis", "key", key, "err", err)
	}
}
