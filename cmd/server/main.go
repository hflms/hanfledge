package main

import (
	"context"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/agent"
	"github.com/hflms/hanfledge/internal/config"
	delivery "github.com/hflms/hanfledge/internal/delivery/http"
	"github.com/hflms/hanfledge/internal/infrastructure/cache"
	"github.com/hflms/hanfledge/internal/infrastructure/llm"
	"github.com/hflms/hanfledge/internal/infrastructure/safety"
	"github.com/hflms/hanfledge/internal/plugin"
	neo4jRepo "github.com/hflms/hanfledge/internal/repository/neo4j"
	"github.com/hflms/hanfledge/internal/repository/postgres"
	"github.com/hflms/hanfledge/internal/usecase"
)

func main() {
	// ── Load Configuration ──────────────────────────────
	cfg := config.Load()
	gin.SetMode(cfg.Server.GinMode)

	log.Println("🚀 Hanfledge API Server starting...")
	log.Printf("   Port: %s | Mode: %s", cfg.Server.Port, cfg.Server.GinMode)

	// ── Database Connection ─────────────────────────────
	db, err := postgres.NewConnection(&cfg.Database)
	if err != nil {
		log.Fatalf("❌ Database connection failed: %v", err)
	}

	// ── Auto Migration ──────────────────────────────────
	if err := postgres.AutoMigrate(db); err != nil {
		log.Fatalf("❌ Migration failed: %v", err)
	}

	// ── Neo4j Connection ────────────────────────────────
	neo4jClient, err := neo4jRepo.NewClient(&cfg.Neo4j)
	if err != nil {
		log.Printf("⚠️  Neo4j connection failed (non-fatal): %v", err)
		neo4jClient = nil
	} else {
		defer neo4jClient.Close(context.Background())
		if err := neo4jClient.InitSchema(context.Background()); err != nil {
			log.Printf("⚠️  Neo4j schema init failed: %v", err)
		}
	}

	// ── LLM Provider ───────────────────────────────────────
	var llmProvider llm.LLMProvider

	switch cfg.LLM.Provider {
	case "dashscope":
		if cfg.LLM.DashScopeKey == "" {
			log.Fatalf("❌ DASHSCOPE_API_KEY is required when LLM_PROVIDER=dashscope")
		}
		embModel := cfg.LLM.EmbeddingModel
		if embModel == "" {
			embModel = "text-embedding-v3"
		}
		llmProvider = llm.NewDashScopeClient(llm.DashScopeConfig{
			APIKey:         cfg.LLM.DashScopeKey,
			ChatModel:      cfg.LLM.DashScopeModel,
			EmbeddingModel: embModel,
		})
		log.Printf("🤖 [LLM] Using DashScope provider: chat=%s embed=%s",
			cfg.LLM.DashScopeModel, embModel)
	default: // "ollama"
		llmProvider = llm.NewOllamaClient(
			cfg.LLM.OllamaHost,
			cfg.LLM.OllamaModel,
			cfg.LLM.EmbeddingModel,
		)
		log.Printf("🤖 [LLM] Using Ollama provider: chat=%s embed=%s host=%s",
			cfg.LLM.OllamaModel, cfg.LLM.EmbeddingModel, cfg.LLM.OllamaHost)
	}

	// ── Use Cases ───────────────────────────────────────
	karagEngine := usecase.NewKARAGEngine(db, neo4jClient, llmProvider)

	// ── Plugin Registry ─────────────────────────────────
	registry := plugin.NewRegistry()
	if err := registry.LoadSkills("plugins/skills"); err != nil {
		log.Printf("⚠️  Plugin loading failed (non-fatal): %v", err)
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
			log.Printf("⚠️  Redis connection failed (non-fatal): %v", err)
		} else {
			redisCache = rc
			defer redisCache.Close()
		}
	}

	// ── Agent Orchestrator ──────────────────────────────
	orchestrator := agent.NewAgentOrchestrator(db, llmProvider, neo4jClient, karagEngine, registry, piiRedactor, redisCache, outputGuard)

	// ── RAGAS Evaluator (§4.2 Background Quality Evaluation) ──
	evaluator := agent.NewRAGASEvaluator(db, llmProvider, agent.DefaultEvalConfig())
	evalCtx, evalCancel := context.WithCancel(context.Background())
	defer evalCancel()
	go evaluator.Start(evalCtx)

	// ── Router Setup ────────────────────────────────────
	router := delivery.NewRouter(db, cfg, karagEngine, registry, orchestrator, injectionGuard, neo4jClient, redisCache, piiRedactor)

	// ── Start Server ────────────────────────────────────
	addr := fmt.Sprintf(":%s", cfg.Server.Port)
	log.Printf("✅ Server ready at http://localhost%s", addr)
	log.Printf("   Health check: http://localhost%s/health", addr)

	if err := router.Run(addr); err != nil {
		log.Fatalf("❌ Server failed to start: %v", err)
	}
}
