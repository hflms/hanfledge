package http

import (
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/agent"
	"github.com/hflms/hanfledge/internal/config"
	"github.com/hflms/hanfledge/internal/delivery/http/handler"
	"github.com/hflms/hanfledge/internal/delivery/http/middleware"
	"github.com/hflms/hanfledge/internal/infrastructure/asr"
	"github.com/hflms/hanfledge/internal/infrastructure/cache"
	"github.com/hflms/hanfledge/internal/infrastructure/i18n"
	"github.com/hflms/hanfledge/internal/infrastructure/llm"
	"github.com/hflms/hanfledge/internal/infrastructure/safety"
	"github.com/hflms/hanfledge/internal/infrastructure/storage"
	"github.com/hflms/hanfledge/internal/infrastructure/weknora"
	"github.com/hflms/hanfledge/internal/plugin"
	neo4jRepo "github.com/hflms/hanfledge/internal/repository/neo4j"
	pgRepo "github.com/hflms/hanfledge/internal/repository/postgres"
	"github.com/hflms/hanfledge/internal/usecase"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"

	_ "github.com/hflms/hanfledge/docs" // swagger generated docs
)

// RouterDeps holds all dependencies needed to construct the router.
//
// ARCHITECTURE CONSTRAINT: RouterDeps is the top-level dependency bag used ONLY
// by NewRouter to construct handlers. Individual registerXxxRoutes functions should
// receive only the specific handler(s) and db they need — never the full RouterDeps.
//
// When adding a new dependency:
//  1. If it's consumed by ONE handler → add it to that handler's constructor only.
//  2. If it's consumed by multiple handlers → add it to RouterDeps AND pass it to
//     each handler constructor. Do NOT pass RouterDeps to registerXxxRoutes.
//  3. If it's needed by route registration (middleware) → pass it as a named parameter
//     to the specific registerXxxRoutes function (e.g., jwtSecret string).
type RouterDeps struct {
	DB             *gorm.DB
	Cfg            *config.Config
	KARAG          *usecase.KARAGEngine
	Registry       *plugin.Registry
	Orchestrator   *agent.AgentOrchestrator
	InjectionGuard *safety.InjectionGuard
	Neo4jClient    *neo4jRepo.Client
	RedisCache     *cache.RedisCache
	PIIRedactor    *safety.PIIRedactor
	FileStorage    storage.FileStorage
	Translator     *i18n.Translator
	EventBus       *plugin.EventBus
	ASRProvider    asr.ASRProvider // 语音识别 (nil-safe)
	LLMProvider    llm.LLMProvider // AI Recommendations
	WeKnoraClient  *weknora.Client // WeKnora knowledge base client (nil when disabled)
}

// NewRouter creates and configures the Gin router with all routes.
// Route registration is split into domain-specific files (routes_*.go)
// for maintainability.
func NewRouter(deps RouterDeps) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(corsMiddleware(deps.Cfg.Server.CORSOrigins))

	// i18n locale detection
	if deps.Translator != nil {
		r.Use(i18n.Middleware(deps.Translator))
	}

	// ── Health Checks ───────────────────────────────────
	healthHandler := handler.NewHealthHandler(deps.DB, deps.Neo4jClient, deps.RedisCache)
	r.GET("/health", healthHandler.Liveness)
	r.GET("/health/ready", healthHandler.Readiness)

	// ── Swagger UI (dev/test only) ──────────────────────
	if deps.Cfg.Server.GinMode != "release" {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	// ── Initialize Repositories ─────────────────────────
	userRepo := pgRepo.NewUserRepo(deps.DB)
	courseRepo := pgRepo.NewCourseRepo(deps.DB)
	docRepo := pgRepo.NewDocumentRepo(deps.DB)
	kpRepo := pgRepo.NewKnowledgePointRepo(deps.DB)
	masteryRepo := pgRepo.NewMasteryRepo(deps.DB)
	sessionRepo := pgRepo.NewSessionRepo(deps.DB)
	activityRepo := pgRepo.NewActivityRepo(deps.DB)

	// ── Initialize Handlers ─────────────────────────────
	authHandler := handler.NewAuthHandler(userRepo, deps.Cfg.JWT.Secret, deps.Cfg.JWT.ExpiryHours, deps.EventBus)
	userHandler := handler.NewUserHandler(deps.DB)
	courseHandler := handler.NewCourseHandler(courseRepo, docRepo, deps.KARAG, deps.RedisCache, deps.FileStorage)
	skillHandler := handler.NewSkillHandler(deps.DB, deps.Registry, deps.LLMProvider)
	activityHandler := handler.NewActivityHandler(deps.DB, deps.Orchestrator, deps.EventBus, deps.Registry)
	sessionHandler := handler.NewSessionHandler(deps.DB, deps.Orchestrator, deps.InjectionGuard, deps.ASRProvider, deps.Cfg.Server.CORSOrigins, deps.Cfg.Server.GinMode)
	dashboardHandler := handler.NewDashboardHandler(courseRepo, userRepo, kpRepo, masteryRepo, sessionRepo, activityRepo)
	kgHandler := handler.NewKnowledgeGraphHandler(deps.DB, deps.Neo4jClient)
	metricsHandler := handler.NewMetricsHandler(deps.RedisCache)
	analyticsHandler := handler.NewAnalyticsHandler(deps.DB, deps.PIIRedactor)
	exportHandler := handler.NewExportHandler(deps.DB)
	achievementHandler := handler.NewAchievementHandler(deps.DB)
	customSkillHandler := handler.NewCustomSkillHandler(deps.DB, deps.Registry)
	marketplaceHandler := handler.NewMarketplaceHandler(deps.DB)
	// telemetryHandler := handler.NewTelemetryHandler(deps.DB) // Disabled
	soulHandler := handler.NewSoulHandler(deps.DB, "soul.md", deps.Orchestrator)

	// ── API v1 ──────────────────────────────────────────
	v1 := r.Group("/api/v1")

	// Auth routes (public + protected)
	registerAuthRoutes(v1, deps.Cfg.JWT.Secret, authHandler)

	// Protected routes — all require JWT
	protected := v1.Group("")
	protected.Use(middleware.JWTAuth(deps.Cfg.JWT.Secret))

	// Domain-specific route groups
	registerAdminRoutes(protected, deps.DB, userHandler)
	registerCourseRoutes(protected, deps.DB, courseHandler, skillHandler, customSkillHandler, kgHandler)
	registerActivityRoutes(protected, deps.DB, activityHandler, sessionHandler, dashboardHandler)
	registerStudentRoutes(protected, deps.DB, activityHandler, dashboardHandler, kgHandler, achievementHandler)
	registerAnalyticsRoutes(protected, deps.DB, dashboardHandler, analyticsHandler, exportHandler)
	registerSystemRoutes(protected, deps.DB, deps.LLMProvider)
	registerMarketplaceRoutes(protected, deps.DB, marketplaceHandler)
	registerSoulRoutes(protected, deps.DB, soulHandler)
	registerNotificationRoutes(protected, deps.DB)

	// Metrics endpoint (public)
	v1.GET("/metrics/cache", metricsHandler.GetCacheMetrics)
	v1.POST("/metrics/cache/invalidate", metricsHandler.InvalidateCache)

	// WeKnora integration (conditional — only when client is available)
	if deps.WeKnoraClient != nil {
		secret := deps.Cfg.WeKnora.EncryptionKey
		if secret == "" {
			secret = deps.Cfg.WeKnora.APIKey // Fallback for backward compatibility
		}
		slog.Info("WeKnora TokenManager init", "has_encryption_key", deps.Cfg.WeKnora.EncryptionKey != "", "has_api_key", deps.Cfg.WeKnora.APIKey != "", "secret_len", len(secret))
		tokenMgr := weknora.NewTokenManager(deps.WeKnoraClient, deps.DB, deps.RedisCache, secret)
		wkHandler := handler.NewWeKnoraHandler(deps.WeKnoraClient, tokenMgr, deps.DB)
		registerWeKnoraRoutes(protected, deps.DB, wkHandler)
		registerWeKnoraCourseRoutes(protected, deps.DB, wkHandler)
	}

	return r
}

// -- CORS Middleware --------------------------------------------------

// corsMiddleware handles CORS with configurable allowed origins.
// allowedOrigins is a comma-separated list (e.g. "http://localhost:3000,https://app.example.com")
// or "*" to allow all origins (dev only).
func corsMiddleware(allowedOrigins string) gin.HandlerFunc {
	// Pre-parse the allowed origins once at startup.
	allowed := make(map[string]struct{})
	wildcard := false
	for _, o := range strings.Split(allowedOrigins, ",") {
		o = strings.TrimSpace(o)
		if o == "*" {
			wildcard = true
		}
		if o != "" {
			allowed[o] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		if wildcard {
			c.Header("Access-Control-Allow-Origin", "*")
		} else if _, ok := allowed[origin]; ok {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
