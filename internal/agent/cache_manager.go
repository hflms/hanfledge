package agent

import (
	"fmt"
	"time"

	"github.com/hflms/hanfledge/internal/infrastructure/cache"
	"github.com/hflms/hanfledge/internal/infrastructure/llm"
	"github.com/hflms/hanfledge/internal/infrastructure/logger"
)

var slogCache = logger.L("OrchestratorCache")

// CacheManager 管理 orchestrator 的语义缓存和输出缓存。
type CacheManager struct {
	cache *cache.RedisCache
	llm   llm.LLMProvider
}

// NewCacheManager 创建缓存管理器。
func NewCacheManager(cache *cache.RedisCache, llm llm.LLMProvider) *CacheManager {
	return &CacheManager{cache: cache, llm: llm}
}

// CheckSemanticCache 检查 L2 语义缓存。
func (c *CacheManager) CheckSemanticCache(tc *TurnContext, getCourseID func(uint) (uint, error)) (*cache.SemanticCacheHit, error) {
	if c.cache == nil || c.llm == nil {
		return nil, nil
	}

	courseID, err := getCourseID(tc.SessionID)
	if err != nil {
		return nil, fmt.Errorf("get course_id for cache: %w", err)
	}

	embedding, err := c.llm.Embed(tc.Ctx, tc.UserInput)
	if err != nil {
		return nil, fmt.Errorf("embed query for cache: %w", err)
	}

	tc.queryEmbedding = embedding
	tc.queryCourseID = courseID

	hit, err := c.cache.FindSemanticMatch(tc.Ctx, courseID, embedding)
	if err != nil {
		return nil, err
	}

	return hit, nil
}

// ReturnCachedResponse 发送缓存响应并持久化交互。
func (c *CacheManager) ReturnCachedResponse(
	tc *TurnContext,
	response, skillID string,
	start time.Time,
	saveInteraction func(*TurnContext, *DraftResponse) error,
	updateMastery func(*TurnContext),
) error {
	if tc.OnTokenDelta != nil {
		tc.OnTokenDelta(response)
	}

	cached := &DraftResponse{
		SessionID:  tc.SessionID,
		Content:    response,
		SkillID:    skillID,
		TokensUsed: 0,
	}
	if err := saveInteraction(tc, cached); err != nil {
		slogCache.Warn("save cached interaction failed", "err", err)
	}

	updateMastery(tc)

	elapsed := time.Since(start)
	slogCache.Info("turn complete (cached)", "session_id", tc.SessionID, "elapsed", elapsed)

	if tc.OnTurnComplete != nil {
		tc.OnTurnComplete(0)
	}

	return nil
}

// WriteToCache 写入 L2 和 L3 缓存。
func (c *CacheManager) WriteToCache(tc *TurnContext, material PersonalizedMaterial, response *DraftResponse) {
	if c.cache == nil {
		return
	}

	ctx := tc.Ctx

	if tc.queryEmbedding != nil {
		entry := cache.SemanticCacheEntry{
			QueryText: tc.UserInput,
			Embedding: tc.queryEmbedding,
			Response:  response.Content,
			SkillID:   response.SkillID,
			CourseID:  tc.queryCourseID,
		}
		if err := c.cache.SetSemanticCache(ctx, entry); err != nil {
			slogCache.Warn("L2 cache write failed", "err", err)
		}
	}

	promptHash := cache.PromptHash(material.SystemPrompt, tc.UserInput, nil, nil)
	outputEntry := cache.OutputCacheEntry{
		Response: response.Content,
		SkillID:  response.SkillID,
		CourseID: tc.queryCourseID,
	}
	if err := c.cache.SetOutputCache(ctx, promptHash, outputEntry); err != nil {
		slogCache.Warn("L3 cache write failed", "err", err)
	}
}
