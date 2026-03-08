package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/agent"
	"github.com/hflms/hanfledge/internal/config"
	delivery "github.com/hflms/hanfledge/internal/delivery/http"
	"github.com/hflms/hanfledge/internal/infrastructure/asr"
	"github.com/hflms/hanfledge/internal/infrastructure/cache"
	"github.com/hflms/hanfledge/internal/infrastructure/i18n"
	"github.com/hflms/hanfledge/internal/infrastructure/llm"
	"github.com/hflms/hanfledge/internal/infrastructure/logger"
	"github.com/hflms/hanfledge/internal/infrastructure/safety"
	"github.com/hflms/hanfledge/internal/infrastructure/search"
	"github.com/hflms/hanfledge/internal/infrastructure/storage"
	"github.com/hflms/hanfledge/internal/infrastructure/weknora"
	"github.com/hflms/hanfledge/internal/plugin"
	neo4jRepo "github.com/hflms/hanfledge/internal/repository/neo4j"
	"github.com/hflms/hanfledge/internal/repository/postgres"
	"github.com/hflms/hanfledge/internal/usecase"
)

// @title           Hanfledge API
// @version         0.1.0
// @description     AI-Native EdTech Platform — Knowledge-Augmented RAG + Multi-Agent Orchestration
// @host            localhost:8080
// @BasePath        /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT Bearer token (e.g. "Bearer eyJhbG...")

func main() {
	// ── Load Configuration ──────────────────────────────
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		logger.Fatal("configuration error", "err", err)
	}
	gin.SetMode(cfg.Server.GinMode)

	// Initialize structured logger based on GIN_MODE
	logLevel := "info"
	if cfg.Server.GinMode == "debug" {
		logLevel = "debug"
	}
	logger.Init(logLevel)
	log := logger.L("Server")

	log.Info("Hanfledge API Server starting", "port", cfg.Server.Port, "mode", cfg.Server.GinMode)

	// ── Database Connection ─────────────────────────────
	db, err := postgres.NewConnection(&cfg.Database)
	if err != nil {
		logger.Fatal("database connection failed", "err", err)
	}

	// ── Auto Migration ──────────────────────────────────
	if err := postgres.AutoMigrate(db); err != nil {
		logger.Fatal("migration failed", "err", err)
	}

	// ── Neo4j Connection ────────────────────────────────
	neo4jClient, err := neo4jRepo.NewClient(&cfg.Neo4j)
	if err != nil {
		log.Warn("Neo4j connection failed (non-fatal)", "err", err)
		neo4jClient = nil
	} else {
		defer neo4jClient.Close(context.Background())
		if err := neo4jClient.InitSchema(context.Background()); err != nil {
			log.Warn("Neo4j schema init failed", "err", err)
		}
	}

	// ── LLM Provider ───────────────────────────────────────
	var llmProvider llm.LLMProvider

	switch cfg.LLM.Provider {
	case "dashscope":
		if cfg.LLM.DashScopeKey == "" {
			log.Warn("DASHSCOPE_API_KEY is not set, LLM features will be unavailable until configured via settings")
		} else {
			embModel := cfg.LLM.EmbeddingModel
			if embModel == "" {
				embModel = "text-embedding-v3"
			}
			llmProvider = llm.NewDashScopeClient(llm.DashScopeConfig{
				APIKey:         cfg.LLM.DashScopeKey,
				ChatModel:      cfg.LLM.DashScopeModel,
				EmbeddingModel: embModel,
				CompatBaseURL:  cfg.LLM.DashScopeCompatURL,
			})
			log.Info("using DashScope provider", "chat_model", cfg.LLM.DashScopeModel, "embed_model", embModel)
		}
	default: // "ollama"
		llmProvider = llm.NewOllamaClient(
			cfg.LLM.OllamaHost,
			cfg.LLM.OllamaModel,
			cfg.LLM.EmbeddingModel,
		)
		log.Info("using Ollama provider", "chat_model", cfg.LLM.OllamaModel, "embed_model", cfg.LLM.EmbeddingModel, "host", cfg.LLM.OllamaHost)
	}

	// ── ModelRouter — Multi-Tier Routing (§8.3.3) ──────────
	if cfg.LLM.RouterEnabled {
		var tier1, tier2, tier3 llm.LLMProvider

		// Tier1: 本地小模型（Ollama）
		if cfg.LLM.Tier1Model != "" {
			tier1 = llm.NewOllamaClient(
				cfg.LLM.OllamaHost,
				cfg.LLM.Tier1Model,
				cfg.LLM.EmbeddingModel,
			)
		}

		// Tier2: 中等模型（DashScope Qwen-Plus）
		if cfg.LLM.Tier2Model != "" && cfg.LLM.DashScopeKey != "" {
			tier2 = llm.NewDashScopeClient(llm.DashScopeConfig{
				APIKey:         cfg.LLM.DashScopeKey,
				ChatModel:      cfg.LLM.Tier2Model,
				EmbeddingModel: cfg.LLM.EmbeddingModel,
			})
		}

		// Tier3: 旗舰模型（DashScope Qwen-Max）
		if cfg.LLM.Tier3Model != "" && cfg.LLM.DashScopeKey != "" {
			tier3 = llm.NewDashScopeClient(llm.DashScopeConfig{
				APIKey:         cfg.LLM.DashScopeKey,
				ChatModel:      cfg.LLM.Tier3Model,
				EmbeddingModel: cfg.LLM.EmbeddingModel,
			})
		}

		llmProvider = llm.NewModelRouter(tier1, tier2, tier3, llmProvider)

		log.Info("ModelRouter enabled", "tier1", cfg.LLM.Tier1Model, "tier2", cfg.LLM.Tier2Model, "tier3", cfg.LLM.Tier3Model)
	}

	llmProvider = llm.NewDynamicProvider(db, llmProvider)

	// ── Use Cases ───────────────────────────────────────
	eventBus := plugin.NewEventBus()
	karagEngine := usecase.NewKARAGEngine(db, neo4jClient, llmProvider, eventBus)

	// ── Plugin Registry ─────────────────────────────────
	registry := plugin.NewRegistry()
	if err := registry.LoadSkills("plugins/skills"); err != nil {
		log.Warn("plugin loading failed (non-fatal)", "err", err)
	}

	// ── Safety Components ──────────────────────────────
	injectionGuard := safety.NewInjectionGuard()
	piiRedactor := safety.NewPIIRedactor(db)
	outputGuard := safety.NewOutputGuardWithLLM(llmProvider)

	// ── Redis Cache ────────────────────────────────────
	var redisCache *cache.RedisCache
	if cfg.Redis.URL != "" {
		rc, err := cache.NewRedisCache(cfg.Redis.URL)
		if err != nil {
			log.Warn("Redis connection failed (non-fatal)", "err", err)
		} else {
			redisCache = rc
			defer redisCache.Close()
		}
	}

	// ── File Storage (§11) ─────────────────────────────
	fileStorage, err := storage.New(storage.StorageConfig{
		Backend:      cfg.Storage.Backend,
		LocalRoot:    cfg.Storage.LocalRoot,
		OSSEndpoint:  cfg.Storage.OSSEndpoint,
		OSSBucket:    cfg.Storage.OSSBucket,
		OSSAccessKey: cfg.Storage.OSSAccessKey,
		OSSSecretKey: cfg.Storage.OSSSecretKey,
	})
	if err != nil {
		logger.Fatal("storage initialization failed", "err", err)
	}
	log.Info("storage backend initialized", "backend", cfg.Storage.Backend)

	// ── Internationalization (i18n) ────────────────────
	translator := i18n.NewTranslator(i18n.Locale(cfg.I18n.DefaultLocale))
	if err := translator.LoadDirectory(cfg.I18n.LocaleDir); err != nil {
		log.Warn("i18n loading failed (non-fatal)", "err", err)
	}

	// ── Web Search Dynamic Connector (§8.1.2) ─────────
	var searchConnector *search.DynamicConnector
	if cfg.Search.BaseURL != "" {
		searchCfg := search.SearchConfig{
			Provider:   cfg.Search.Provider,
			BaseURL:    cfg.Search.BaseURL,
			APIKey:     cfg.Search.APIKey,
			MaxResults: 10,
			Timeout:    10 * time.Second,
			SafeSearch: true,
			EduFilter:  true,
		}
		searchConnector = search.NewDynamicConnector(searchCfg)
		log.Info("search dynamic connector initialized", "provider", cfg.Search.Provider, "url", cfg.Search.BaseURL)
	}

	// ── ASR Provider (语音识别) ────────────────────────
	var asrProvider asr.ASRProvider
	if cfg.ASR.WhisperURL != "" {
		asrCfg := asr.ASRConfig{
			Provider:   cfg.ASR.Provider,
			WhisperURL: cfg.ASR.WhisperURL,
			APIKey:     cfg.ASR.APIKey,
			ModelSize:  cfg.ASR.ModelSize,
		}
		asrProvider = asr.NewWhisperProvider(asrCfg)
		log.Info("ASR provider initialized", "provider", cfg.ASR.Provider, "url", cfg.ASR.WhisperURL, "model", cfg.ASR.ModelSize)
	}

	// ── Agent Orchestrator ──────────────────────────────
	orchestrator := agent.NewAgentOrchestrator(db, llmProvider, neo4jClient, karagEngine, registry, eventBus, piiRedactor, redisCache, outputGuard, searchConnector)

	// ── RAGAS Evaluator (§4.2 Background Quality Evaluation) ──
	evaluator := agent.NewRAGASEvaluator(db, llmProvider, agent.DefaultEvalConfig())
	orchestrator.SetEvalNotifier(evaluator.Notify)
	evalCtx, evalCancel := context.WithCancel(context.Background())
	defer evalCancel()
	go evaluator.Start(evalCtx)

	// ── WeKnora Knowledge Base Client ─────────────────
	var wkClient *weknora.Client
	if cfg.WeKnora.Enabled && cfg.WeKnora.BaseURL != "" {
		// Use empty API key - we'll use per-user tokens via TokenManager
		wkClient = weknora.NewClient(cfg.WeKnora.BaseURL, "")
		// Skip ping check since we don't have a valid token yet
		log.Info("WeKnora integration enabled", "url", cfg.WeKnora.BaseURL)
	}

	// ── Router Setup ────────────────────────────────────
	router := delivery.NewRouter(delivery.RouterDeps{
		DB:             db,
		Cfg:            cfg,
		KARAG:          karagEngine,
		Registry:       registry,
		Orchestrator:   orchestrator,
		InjectionGuard: injectionGuard,
		Neo4jClient:    neo4jClient,
		RedisCache:     redisCache,
		PIIRedactor:    piiRedactor,
		FileStorage:    fileStorage,
		Translator:     translator,
		EventBus:       eventBus,
		ASRProvider:    asrProvider,
		LLMProvider:    llmProvider,
		WeKnoraClient:  wkClient,
	})

	// ── Start Server (Graceful Shutdown) ───────────────
	addr := fmt.Sprintf(":%s", cfg.Server.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		log.Info("server ready", "addr", "http://localhost"+addr, "health", "http://localhost"+addr+"/health")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server failed to start", "err", err)
		}
	}()

	// Wait for interrupt signal (SIGINT, SIGTERM)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info("received shutdown signal, shutting down gracefully", "signal", sig.String())

	// Give in-flight requests 5 seconds to complete
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("server forced to shutdown", "err", err)
	}

	log.Info("server exited cleanly")
}
