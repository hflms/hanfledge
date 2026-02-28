package config

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all application configuration.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Neo4j    Neo4jConfig
	Redis    RedisConfig
	JWT      JWTConfig
	LLM      LLMConfig
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port        string
	GinMode     string
	CORSOrigins string // Comma-separated allowed origins; "*" for dev, explicit for prod
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// DSN returns the PostgreSQL connection string.
func (d *DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode,
	)
}

// Neo4jConfig holds Neo4j connection settings.
type Neo4jConfig struct {
	URI      string
	User     string
	Password string
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	URL string
}

// JWTConfig holds JWT authentication settings.
type JWTConfig struct {
	Secret      string
	ExpiryHours int
}

// LLMConfig holds LLM provider settings.
type LLMConfig struct {
	Provider          string // ollama | dashscope | gemini
	OllamaHost        string
	OllamaModel       string
	DashScopeKey      string
	DashScopeModel    string
	EmbeddingProvider string
	EmbeddingModel    string

	// Multi-tier routing (§8.3.3)
	RouterEnabled bool   // Enable ModelRouter multi-tier routing
	Tier1Model    string // Local small model (Ollama 7B) — low complexity
	Tier2Model    string // Mid-range model (Qwen-Plus) — medium complexity
	Tier3Model    string // Flagship model (Qwen-Max) — high complexity
}

// Load reads configuration from .env file and environment variables.
// Environment variables take precedence over .env file values.
func Load() *Config {
	// Load .env file if it exists (ignore error if missing)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	return &Config{
		Server: ServerConfig{
			Port:        getEnv("SERVER_PORT", "8080"),
			GinMode:     getEnv("GIN_MODE", "debug"),
			CORSOrigins: getEnv("CORS_ORIGINS", "http://localhost:3000"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "hanfledge"),
			Password: getEnv("DB_PASSWORD", "hanfledge_secret"),
			DBName:   getEnv("DB_NAME", "hanfledge"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Neo4j: Neo4jConfig{
			URI:      getEnv("NEO4J_URI", "bolt://localhost:7687"),
			User:     getEnv("NEO4J_USER", "neo4j"),
			Password: getEnv("NEO4J_PASSWORD", "neo4j_secret"),
		},
		Redis: RedisConfig{
			URL: getEnv("REDIS_URL", "redis://localhost:6379/0"),
		},
		JWT: JWTConfig{
			Secret:      getEnv("JWT_SECRET", "dev-secret-change-me"),
			ExpiryHours: getEnvInt("JWT_EXPIRY_HOURS", 24),
		},
		LLM: LLMConfig{
			Provider:          getEnv("LLM_PROVIDER", "ollama"),
			OllamaHost:        getEnv("OLLAMA_HOST", "http://localhost:11434"),
			OllamaModel:       getEnv("OLLAMA_MODEL", "qwen2.5:7b"),
			DashScopeKey:      getEnv("DASHSCOPE_API_KEY", ""),
			DashScopeModel:    getEnv("DASHSCOPE_MODEL", "qwen-max"),
			EmbeddingProvider: getEnv("EMBEDDING_PROVIDER", "ollama"),
			EmbeddingModel:    getEnv("EMBEDDING_MODEL", "bge-m3"),

			// Multi-tier routing
			RouterEnabled: getEnv("LLM_ROUTER_ENABLED", "false") == "true",
			Tier1Model:    getEnv("LLM_TIER1_MODEL", ""), // e.g., "qwen2.5:7b"
			Tier2Model:    getEnv("LLM_TIER2_MODEL", ""), // e.g., "qwen-plus"
			Tier3Model:    getEnv("LLM_TIER3_MODEL", ""), // e.g., "qwen-max"
		},
	}
}

// getEnv reads an environment variable with a fallback default value.
func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}

// getEnvInt reads an integer environment variable with a fallback.
func getEnvInt(key string, fallback int) int {
	if val, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return fallback
}
