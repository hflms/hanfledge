package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/config"
	delivery "github.com/hflms/hanfledge/internal/delivery/http"
	"github.com/hflms/hanfledge/internal/repository/postgres"
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

	// ── TODO: Neo4j Connection ──────────────────────────
	// neo4jDriver, err := neo4j.NewConnection(&cfg.Neo4j)

	// ── TODO: Redis Connection ──────────────────────────
	// redisClient, err := redis.NewConnection(&cfg.Redis)

	// ── Router Setup ────────────────────────────────────
	router := delivery.NewRouter(db, cfg)

	// ── Start Server ────────────────────────────────────
	addr := fmt.Sprintf(":%s", cfg.Server.Port)
	log.Printf("✅ Server ready at http://localhost%s", addr)
	log.Printf("   Health check: http://localhost%s/health", addr)

	if err := router.Run(addr); err != nil {
		log.Fatalf("❌ Server failed to start: %v", err)
	}
}
