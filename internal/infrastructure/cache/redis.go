package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/hflms/hanfledge/internal/infrastructure/logger"
	"github.com/redis/go-redis/v9"
)

var slogRedis = logger.L("Redis")

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
	client  *redis.Client
	metrics *CacheMetrics
}

// CacheMetrics tracks cache performance.
type CacheMetrics struct {
	Hits   int64
	Misses int64
}

// HitRate returns the cache hit rate.
func (m *CacheMetrics) HitRate() float64 {
	total := m.Hits + m.Misses
	if total == 0 {
		return 0
	}
	return float64(m.Hits) / float64(total)
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

	slogRedis.Info("connected", "addr", opts.Addr, "pool", opts.PoolSize)
	return &RedisCache{client: client, metrics: &CacheMetrics{}}, nil
}

// Close shuts down the Redis connection pool.
func (rc *RedisCache) Close() error {
	return rc.client.Close()
}

// Ping checks Redis connectivity. Used by the health check endpoint.
func (rc *RedisCache) Ping(ctx context.Context) error {
	return rc.client.Ping(ctx).Err()
}

// ── Generic String Cache ────────────────────────────────────

// GetString retrieves a plain string value by key.
// Returns ("", nil) if the key does not exist.
func (rc *RedisCache) GetString(ctx context.Context, key string) (string, error) {
	val, err := rc.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

// SetString stores a plain string value with TTL.
func (rc *RedisCache) SetString(ctx context.Context, key, value string, ttl time.Duration) error {
	return rc.client.Set(ctx, key, value, ttl).Err()
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

// ── L2 Semantic Cache (§8.1.3) ──────────────────────────────
//
// 语义缓存：将 query embedding 与已缓存的 embedding 做余弦相似度匹配。
// 当相似度 > 0.95 时命中缓存，直接返回之前的 LLM 响应，避免完整 RAG+LLM 流程。
//
// 存储策略：
//   semantic:index:{courseID}  → Set of cache entry keys (用于按课程失效)
//   semantic:entry:{hash}     → JSON(SemanticCacheEntry) 含 embedding + 响应

const (
	semanticEntryPrefix = "semantic:entry:"
	semanticIndexPrefix = "semantic:index:"
	semanticCacheTTL    = 2 * time.Hour
	// semanticSimilarityThreshold is the cosine similarity threshold for cache hits.
	// Per design.md §8.1.3: > 0.95.
	semanticSimilarityThreshold = 0.95
	// semanticMaxEntries is the max number of entries scanned per course.
	// Brute-force search is feasible at this scale (private deployment, moderate traffic).
	semanticMaxEntries = 200
)

// SemanticCacheEntry stores a cached LLM response with its query embedding.
type SemanticCacheEntry struct {
	QueryText string    `json:"query_text"`
	Embedding []float64 `json:"embedding"`
	Response  string    `json:"response"`
	SkillID   string    `json:"skill_id,omitempty"`
	CourseID  uint      `json:"course_id"`
	CreatedAt int64     `json:"created_at"`
}

// SemanticCacheHit is returned when a cache hit is found.
type SemanticCacheHit struct {
	Entry      SemanticCacheEntry
	Similarity float64
}

func semanticEntryKey(hash string) string {
	return semanticEntryPrefix + hash
}

func semanticIndexKey(courseID uint) string {
	return fmt.Sprintf("%s%d", semanticIndexPrefix, courseID)
}

// embeddingHash generates a compact hash key from an embedding vector.
// Uses SHA-256 of the float64 values formatted at reduced precision.
func embeddingHash(embedding []float64) string {
	var sb strings.Builder
	for i, v := range embedding {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, "%.4f", v)
	}
	h := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(h[:16]) // 128-bit, 32-char hex
}

// SetSemanticCache stores a query-response pair in the L2 semantic cache.
func (rc *RedisCache) SetSemanticCache(ctx context.Context, entry SemanticCacheEntry) error {
	hash := embeddingHash(entry.Embedding)
	key := semanticEntryKey(hash)
	idxKey := semanticIndexKey(entry.CourseID)

	entry.CreatedAt = time.Now().Unix()

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal semantic cache entry: %w", err)
	}

	pipe := rc.client.Pipeline()
	pipe.Set(ctx, key, string(data), semanticCacheTTL)
	pipe.SAdd(ctx, idxKey, hash)
	pipe.Expire(ctx, idxKey, semanticCacheTTL)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("redis set semantic cache: %w", err)
	}

	slogRedis.Debug("l2 cache stored", "query", truncateStr(entry.QueryText, 40), "course_id", entry.CourseID, "key", hash[:12])
	return nil
}

// FindSemanticMatch searches the L2 cache for an entry whose embedding is
// similar to queryEmbedding (cosine similarity > 0.95).
// Performs brute-force search over all entries for the given course.
// Returns nil if no match is found.
func (rc *RedisCache) FindSemanticMatch(ctx context.Context, courseID uint, queryEmbedding []float64) (*SemanticCacheHit, error) {
	idxKey := semanticIndexKey(courseID)

	// Get all entry hashes for this course
	hashes, err := rc.client.SMembers(ctx, idxKey).Result()
	if err != nil {
		if err == redis.Nil {
			rc.metrics.Misses++
			return nil, nil
		}
		return nil, fmt.Errorf("redis smembers %s: %w", idxKey, err)
	}

	if len(hashes) == 0 {
		rc.metrics.Misses++
		return nil, nil
	}

	// Cap scan size to prevent excessive reads
	if len(hashes) > semanticMaxEntries {
		hashes = hashes[:semanticMaxEntries]
	}

	// Build keys for batch fetch
	keys := make([]string, len(hashes))
	for i, h := range hashes {
		keys[i] = semanticEntryKey(h)
	}

	// Batch fetch all entries
	vals, err := rc.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("redis mget semantic entries: %w", err)
	}

	// Find best match above threshold
	var bestHit *SemanticCacheHit
	var bestSim float64

	for i, val := range vals {
		if val == nil {
			// Entry expired but index not cleaned — remove stale hash
			rc.client.SRem(ctx, idxKey, hashes[i])
			continue
		}

		strVal, ok := val.(string)
		if !ok {
			continue
		}

		var entry SemanticCacheEntry
		if err := json.Unmarshal([]byte(strVal), &entry); err != nil {
			continue
		}

		sim := CosineSimilarity(queryEmbedding, entry.Embedding)
		if sim > semanticSimilarityThreshold && sim > bestSim {
			bestSim = sim
			bestHit = &SemanticCacheHit{
				Entry:      entry,
				Similarity: sim,
			}
		}
	}

	if bestHit != nil {
		rc.metrics.Hits++
		slogRedis.Debug("l2 cache hit", "similarity", bestHit.Similarity, "cached_query", truncateStr(bestHit.Entry.QueryText, 40))
	} else {
		rc.metrics.Misses++
	}

	return bestHit, nil
}

// InvalidateSemanticCacheByCourse removes all L2 semantic cache entries for a course.
// Called when course materials are updated (KA-RAG graph rebuild).
func (rc *RedisCache) InvalidateSemanticCacheByCourse(ctx context.Context, courseID uint) error {
	idxKey := semanticIndexKey(courseID)

	hashes, err := rc.client.SMembers(ctx, idxKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil
		}
		return fmt.Errorf("redis smembers %s: %w", idxKey, err)
	}

	if len(hashes) == 0 {
		return nil
	}

	// Delete all entry keys + the index key
	keys := make([]string, 0, len(hashes)+1)
	for _, h := range hashes {
		keys = append(keys, semanticEntryKey(h))
	}
	keys = append(keys, idxKey)

	if err := rc.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("redis del semantic cache course=%d: %w", courseID, err)
	}

	slogRedis.Info("l2 cache invalidated", "count", len(hashes), "course_id", courseID)
	return nil
}

// ── L3 Output Cache (§8.1.3) ────────────────────────────────
//
// 精确哈希缓存：对完整 prompt（系统提示 + 历史 + 用户输入）做 SHA-256 哈希。
// 完全相同的上下文 → 直接返回缓存的 LLM 输出。

const (
	outputCachePrefix = "output:"
	outputCacheTTL    = 1 * time.Hour
)

// OutputCacheEntry stores a cached LLM response with exact prompt hash.
type OutputCacheEntry struct {
	Response  string `json:"response"`
	SkillID   string `json:"skill_id,omitempty"`
	CourseID  uint   `json:"course_id"`
	CreatedAt int64  `json:"created_at"`
}

func outputCacheKey(hash string) string {
	return outputCachePrefix + hash
}

// PromptHash computes SHA-256 of the full prompt context for L3 cache keying.
func PromptHash(systemPrompt, userInput string, historyRoles, historyContents []string) string {
	h := sha256.New()
	h.Write([]byte("sys:"))
	h.Write([]byte(systemPrompt))
	h.Write([]byte("\n"))
	for i := range historyRoles {
		h.Write([]byte(historyRoles[i]))
		h.Write([]byte(":"))
		if i < len(historyContents) {
			h.Write([]byte(historyContents[i]))
		}
		h.Write([]byte("\n"))
	}
	h.Write([]byte("usr:"))
	h.Write([]byte(userInput))
	return hex.EncodeToString(h.Sum(nil))
}

// SetOutputCache stores an LLM response in the L3 exact-match cache.
func (rc *RedisCache) SetOutputCache(ctx context.Context, promptHash string, entry OutputCacheEntry) error {
	key := outputCacheKey(promptHash)
	entry.CreatedAt = time.Now().Unix()

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal output cache entry: %w", err)
	}

	if err := rc.client.Set(ctx, key, string(data), outputCacheTTL).Err(); err != nil {
		return fmt.Errorf("redis set output cache: %w", err)
	}

	slogRedis.Debug("l3 cache stored", "hash", promptHash[:12])
	return nil
}

// GetOutputCache retrieves a cached LLM response by exact prompt hash.
// Returns nil if not cached.
func (rc *RedisCache) GetOutputCache(ctx context.Context, promptHash string) (*OutputCacheEntry, error) {
	key := outputCacheKey(promptHash)
	val, err := rc.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("redis get %s: %w", key, err)
	}

	var entry OutputCacheEntry
	if err := json.Unmarshal([]byte(val), &entry); err != nil {
		return nil, fmt.Errorf("unmarshal output cache: %w", err)
	}

	slogRedis.Debug("l3 cache hit", "hash", promptHash[:12])
	return &entry, nil
}

// InvalidateOutputCacheByCourse is a no-op for L3 since entries are prompt-hashed.
// Course material changes naturally invalidate L3 because the system prompt changes
// (different retrieved chunks → different prompt hash → no match).
// This method is provided for API consistency.
func (rc *RedisCache) InvalidateOutputCacheByCourse(ctx context.Context, courseID uint) error {
	// L3 is self-invalidating: changed materials → changed system prompt → different hash.
	slogRedis.Debug("l3 cache invalidation no-op (self-invalidating via prompt hash)", "course_id", courseID)
	return nil
}

// ── Cosine Similarity ───────────────────────────────────────

// CosineSimilarity computes the cosine similarity between two vectors.
// Returns a value in [-1, 1]. Higher values indicate more similar vectors.
// Returns 0 if either vector is zero-length or if dimensions don't match.
func CosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}

	return dot / denom
}

// ── Metrics ─────────────────────────────────────────────────

// GetMetrics returns current cache metrics.
func (rc *RedisCache) GetMetrics() CacheMetrics {
	return *rc.metrics
}

// ResetMetrics resets cache metrics counters.
func (rc *RedisCache) ResetMetrics() {
	rc.metrics.Hits = 0
	rc.metrics.Misses = 0
}

// ── Cache Management ────────────────────────────────────────

// InvalidateByPattern removes all keys matching a pattern.
// Pattern examples: "session:*", "semantic:course:123:*"
func (rc *RedisCache) InvalidateByPattern(ctx context.Context, pattern string) (int64, error) {
	var cursor uint64
	var deleted int64

	for {
		keys, nextCursor, err := rc.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return deleted, fmt.Errorf("redis scan: %w", err)
		}

		if len(keys) > 0 {
			n, err := rc.client.Del(ctx, keys...).Result()
			if err != nil {
				return deleted, fmt.Errorf("redis del: %w", err)
			}
			deleted += n
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	slogRedis.Info("cache invalidated", "pattern", pattern, "deleted", deleted)
	return deleted, nil
}

// ── Helpers ─────────────────────────────────────────────────

// truncateStr truncates a string for log output.
func truncateStr(s string, maxLen int) string {
	if maxLen <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
