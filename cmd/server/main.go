package main

import (
	"context"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/agent"
	"github.com/hflms/hanfledge/internal/config"
	delivery "github.com/hflms/hanfledge/internal/delivery/http"
	"github.com/hflms/hanfledge/internal/infrastructure/llm"
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

	// ── LLM Client ──────────────────────────────────────
	llmClient := llm.NewOllamaClient(
		cfg.LLM.OllamaHost,
		cfg.LLM.OllamaModel,
		cfg.LLM.EmbeddingModel,
	)

	// ── Use Cases ───────────────────────────────────────
	karagEngine := usecase.NewKARAGEngine(db, neo4jClient, llmClient)

	// ── Plugin Registry ─────────────────────────────────
	registry := plugin.NewRegistry()
	if err := registry.LoadSkills("plugins/skills"); err != nil {
		log.Printf("⚠️  Plugin loading failed (non-fatal): %v", err)
	}

	// ── Agent Orchestrator ──────────────────────────────
	orchestrator := agent.NewAgentOrchestrator(db, llmClient, neo4jClient, karagEngine, registry)

	// ── Router Setup ────────────────────────────────────
	router := delivery.NewRouter(db, cfg, karagEngine, registry, orchestrator)

	// ── Start Server ────────────────────────────────────
	addr := fmt.Sprintf(":%s", cfg.Server.Port)
	log.Printf("✅ Server ready at http://localhost%s", addr)
	log.Printf("   Health check: http://localhost%s/health", addr)

	if err := router.Run(addr); err != nil {
		log.Fatalf("❌ Server failed to start: %v", err)
	}
}
