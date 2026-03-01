package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/infrastructure/cache"
	neo4jRepo "github.com/hflms/hanfledge/internal/repository/neo4j"
	"gorm.io/gorm"
)

// HealthHandler provides liveness and readiness endpoints.
type HealthHandler struct {
	DB          *gorm.DB
	Neo4jClient *neo4jRepo.Client // may be nil
	RedisCache  *cache.RedisCache // may be nil
}

// NewHealthHandler creates a HealthHandler with optional dependency pointers.
func NewHealthHandler(db *gorm.DB, neo4j *neo4jRepo.Client, redis *cache.RedisCache) *HealthHandler {
	return &HealthHandler{DB: db, Neo4jClient: neo4j, RedisCache: redis}
}

// Liveness is a lightweight check for Docker HEALTHCHECK / load-balancer probes.
//
//	@Summary      Liveness 探针
//	@Description  轻量级健康检查，用于 Docker HEALTHCHECK 和负载均衡器
//	@Tags         Health
//	@Produce      json
//	@Success      200  {object}  map[string]string
//	@Router       /health [get]
func (h *HealthHandler) Liveness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "hanfledge-api",
		"version": "0.1.0",
	})
}

// componentStatus represents the health of a single dependency.
type componentStatus struct {
	Status  string `json:"status"`            // "ok" | "error" | "skipped"
	Latency string `json:"latency,omitempty"` // e.g. "2.1ms"
	Error   string `json:"error,omitempty"`
}

// Readiness performs deep checks on all dependencies.
// Returns 200 if all required components are healthy, 503 otherwise.
//
//	@Summary      Readiness 探针
//	@Description  深度检查 PostgreSQL、Neo4j、Redis 等依赖组件的可用性
//	@Tags         Health
//	@Produce      json
//	@Success      200  {object}  map[string]interface{}
//	@Failure      503  {object}  map[string]interface{}
//	@Router       /health/ready [get]
func (h *HealthHandler) Readiness(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	healthy := true
	result := make(map[string]componentStatus)

	// -- PostgreSQL -------------------------------------------
	result["postgres"] = h.checkPostgres(ctx)
	if result["postgres"].Status != "ok" {
		healthy = false
	}

	// -- Neo4j ------------------------------------------------
	result["neo4j"] = h.checkNeo4j(ctx)
	if result["neo4j"].Status == "error" {
		healthy = false
	}

	// -- Redis ------------------------------------------------
	result["redis"] = h.checkRedis(ctx)
	if result["redis"].Status == "error" {
		healthy = false
	}

	status := http.StatusOK
	if !healthy {
		status = http.StatusServiceUnavailable
	}

	c.JSON(status, gin.H{
		"status":     map[bool]string{true: "ok", false: "degraded"}[healthy],
		"components": result,
	})
}

func (h *HealthHandler) checkPostgres(ctx context.Context) componentStatus {
	start := time.Now()
	sqlDB, err := h.DB.DB()
	if err != nil {
		return componentStatus{Status: "error", Error: "无法获取数据库连接"}
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return componentStatus{Status: "error", Error: err.Error(), Latency: time.Since(start).String()}
	}
	return componentStatus{Status: "ok", Latency: time.Since(start).String()}
}

func (h *HealthHandler) checkNeo4j(ctx context.Context) componentStatus {
	if h.Neo4jClient == nil {
		return componentStatus{Status: "skipped"}
	}
	start := time.Now()
	if err := h.Neo4jClient.Driver.VerifyConnectivity(ctx); err != nil {
		return componentStatus{Status: "error", Error: err.Error(), Latency: time.Since(start).String()}
	}
	return componentStatus{Status: "ok", Latency: time.Since(start).String()}
}

func (h *HealthHandler) checkRedis(ctx context.Context) componentStatus {
	if h.RedisCache == nil {
		return componentStatus{Status: "skipped"}
	}
	start := time.Now()
	if err := h.RedisCache.Ping(ctx); err != nil {
		return componentStatus{Status: "error", Error: err.Error(), Latency: time.Since(start).String()}
	}
	return componentStatus{Status: "ok", Latency: time.Since(start).String()}
}
