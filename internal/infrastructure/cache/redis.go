package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// ============================
// Redis Cache Client
// ============================
//
// 职责：
// 1. 会话上下文缓存 — 减少每轮对话的 DB 查询
// 2. L2 语义缓存 — 相似问题的 LLM 响应缓存（预留）
//
// Key 设计:
//   session:{id}:history   → 最近 N 轮对话历史 (List)
//   session:{id}:state     → 会话元数据: scaffold, current_kp 等 (Hash)
//   semantic:{hash}        → LLM 响应缓存 (String, 预留)

// RedisCache wraps a go-redis client with domain-specific caching methods.
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache creates a new Redis cache client from a Redis URL.
// URL format: redis://[:password@]host:port/db
func NewRedisCache(redisURL string) (*RedisCache, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}

	// Connection pool settings
	opts.PoolSize = 20
	opts.MinIdleConns = 5
	opts.DialTimeout = 5 * time.Second
	opts.ReadTimeout = 3 * time.Second
	opts.WriteTimeout = 3 * time.Second

	client := redis.NewClient(opts)

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	log.Printf("🔗 [Redis] Connected to %s (pool=%d)", opts.Addr, opts.PoolSize)
	return &RedisCache{client: client}, nil
}

// Close shuts down the Redis connection pool.
func (rc *RedisCache) Close() error {
	return rc.client.Close()
}

// ── Session History Cache ───────────────────────────────────

// CachedMessage represents a single conversation message in the cache.
type CachedMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

const (
	sessionHistoryKeyPrefix = "session:"
	sessionHistoryKeySuffix = ":history"
	sessionHistoryTTL       = 30 * time.Minute
	sessionHistoryMaxLen    = 20 // Keep last 20 messages (10 rounds)
)

func sessionHistoryKey(sessionID uint) string {
	return fmt.Sprintf("%s%d%s", sessionHistoryKeyPrefix, sessionID, sessionHistoryKeySuffix)
}

// GetSessionHistory retrieves cached conversation history for a session.
// Returns nil (not an error) if the cache is empty or expired.
func (rc *RedisCache) GetSessionHistory(ctx context.Context, sessionID uint) ([]CachedMessage, error) {
	key := sessionHistoryKey(sessionID)
	vals, err := rc.client.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("redis lrange %s: %w", key, err)
	}

	if len(vals) == 0 {
		return nil, nil
	}

	messages := make([]CachedMessage, 0, len(vals))
	for _, v := range vals {
		var msg CachedMessage
		if err := json.Unmarshal([]byte(v), &msg); err != nil {
			continue // Skip malformed entries
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// AppendSessionHistory adds messages to the session history cache.
// Trims the list to keep only the most recent `sessionHistoryMaxLen` entries.
func (rc *RedisCache) AppendSessionHistory(ctx context.Context, sessionID uint, messages ...CachedMessage) error {
	if len(messages) == 0 {
		return nil
	}

	key := sessionHistoryKey(sessionID)
	pipe := rc.client.Pipeline()

	for _, msg := range messages {
		data, err := json.Marshal(msg)
		if err != nil {
			continue
		}
		pipe.RPush(ctx, key, string(data))
	}

	// Trim to keep only recent messages
	pipe.LTrim(ctx, key, -int64(sessionHistoryMaxLen), -1)
	// Refresh TTL
	pipe.Expire(ctx, key, sessionHistoryTTL)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("redis append history session=%d: %w", sessionID, err)
	}

	return nil
}

// InvalidateSessionHistory removes the session history cache.
func (rc *RedisCache) InvalidateSessionHistory(ctx context.Context, sessionID uint) error {
	return rc.client.Del(ctx, sessionHistoryKey(sessionID)).Err()
}

// ── Session State Cache ─────────────────────────────────────

const (
	sessionStateKeyPrefix = "session:"
	sessionStateKeySuffix = ":state"
	sessionStateTTL       = 30 * time.Minute
)

// SessionState holds cached session metadata to avoid DB queries.
type SessionState struct {
	Scaffold  string `json:"scaffold"`
	CurrentKP uint   `json:"current_kp"`
	StudentID uint   `json:"student_id"`
}

func sessionStateKey(sessionID uint) string {
	return fmt.Sprintf("%s%d%s", sessionStateKeyPrefix, sessionID, sessionStateKeySuffix)
}

// GetSessionState retrieves cached session state.
// Returns nil (not error) if not cached.
func (rc *RedisCache) GetSessionState(ctx context.Context, sessionID uint) (*SessionState, error) {
	key := sessionStateKey(sessionID)
	val, err := rc.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("redis get %s: %w", key, err)
	}

	var state SessionState
	if err := json.Unmarshal([]byte(val), &state); err != nil {
		return nil, fmt.Errorf("redis unmarshal session state: %w", err)
	}

	return &state, nil
}

// SetSessionState caches session state with TTL.
func (rc *RedisCache) SetSessionState(ctx context.Context, sessionID uint, state *SessionState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal session state: %w", err)
	}

	key := sessionStateKey(sessionID)
	return rc.client.Set(ctx, key, string(data), sessionStateTTL).Err()
}

// InvalidateSessionState removes the session state cache.
func (rc *RedisCache) InvalidateSessionState(ctx context.Context, sessionID uint) error {
	return rc.client.Del(ctx, sessionStateKey(sessionID)).Err()
}
