package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/infrastructure/cache"
)

// MetricsHandler handles cache metrics endpoints.
type MetricsHandler struct {
	cache *cache.RedisCache
}

// NewMetricsHandler creates a new metrics handler.
func NewMetricsHandler(cache *cache.RedisCache) *MetricsHandler {
	return &MetricsHandler{cache: cache}
}

// GetCacheMetrics godoc
// @Summary Get cache metrics
// @Tags    metrics
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router  /metrics/cache [get]
func (h *MetricsHandler) GetCacheMetrics(c *gin.Context) {
	if h.cache == nil {
		c.JSON(http.StatusOK, gin.H{
			"enabled": false,
			"message": "Redis cache not available",
		})
		return
	}

	metrics := h.cache.GetMetrics()
	c.JSON(http.StatusOK, gin.H{
		"enabled":  true,
		"hits":     metrics.Hits,
		"misses":   metrics.Misses,
		"hit_rate": metrics.HitRate(),
	})
}

// InvalidateCache godoc
// @Summary Invalidate cache by pattern
// @Tags    metrics
// @Param   pattern query string true "Cache key pattern (e.g., session:*, semantic:*)"
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router  /metrics/cache/invalidate [post]
func (h *MetricsHandler) InvalidateCache(c *gin.Context) {
	if h.cache == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Redis cache not available"})
		return
	}

	pattern := c.Query("pattern")
	if pattern == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pattern is required"})
		return
	}

	deleted, err := h.cache.InvalidateByPattern(c.Request.Context(), pattern)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Cache invalidated",
		"pattern": pattern,
		"deleted": deleted,
	})
}
