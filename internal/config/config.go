package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"

	"github.com/hflms/hanfledge/internal/infrastructure/logger"
	"github.com/joho/godotenv"
)

var slogConfig = logger.L("Config")

// Config holds all application configuration.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Neo4j    Neo4jConfig
	Redis    RedisConfig
	JWT      JWTConfig
	LLM      LLMConfig
	Storage  StorageConfig
	I18n     I18nConfig
	Search   SearchConfig
	ASR      ASRConfig
	WeKnora  WeKnoraConfig
}

// ASRConfig holds speech-to-text service configuration.
type ASRConfig struct {
	Provider   string // "whisper" | "dashscope" | "local"
	WhisperURL string // Whisper API endpoint
	APIKey     string
	ModelSize  string // "tiny" | "base" | "small" | "medium" | "large-v3"
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
	Provider           string // ollama | dashscope | gemini
	OllamaHost         string
	OllamaModel        string
	DashScopeKey       string
	DashScopeModel     string
	DashScopeCompatURL string
	EmbeddingProvider  string
	EmbeddingModel     string

	// Multi-tier routing (§8.3.3)
	RouterEnabled bool   // Enable ModelRouter multi-tier routing
	Tier1Model    string // Local small model (Ollama 7B) — low complexity
	Tier2Model    string // Mid-range model (Qwen-Plus) — medium complexity
	Tier3Model    string // Flagship model (Qwen-Max) — high complexity
}

// StorageConfig holds file storage settings.
type StorageConfig struct {
	Backend   string // "local" or "oss"
	LocalRoot string // Root directory for local storage (default: "uploads")
	// OSS settings (for future cloud deployment)
	OSSEndpoint  string
	OSSBucket    string
	OSSAccessKey string
	OSSSecretKey string
}

// I18nConfig holds internationalization settings.
type I18nConfig struct {
	DefaultLocale string // Default locale (default: "zh-CN")
	LocaleDir     string // Directory containing locale JSON files (default: "locales")
}

// SearchConfig holds web search fallback settings.
type SearchConfig struct {
	Provider string // "searxng" | "google" | "bing"
	BaseURL  string // SearXNG instance URL or API endpoint
	APIKey   string // API key (for Google/Bing)
}

// WeKnoraConfig holds WeKnora knowledge base service settings.
type WeKnoraConfig struct {
	Enabled       bool   // Whether WeKnora integration is enabled
	BaseURL       string // WeKnora API base URL (e.g., "http://localhost:9380/api/v1")
	EncryptionKey string // Shared secret for generating user passwords
}

// Load reads configuration from .env file and environment variables.
// Environment variables take precedence over .env file values.
func Load() *Config {
	// Load .env file if it exists (ignore error if missing)
	if err := godotenv.Load(); err != nil {
		slogConfig.Info("no .env file found, using environment variables")
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
			Secret:      getEnv("JWT_SECRET", generateEphemeralSecret()),
			ExpiryHours: getEnvInt("JWT_EXPIRY_HOURS", 24),
		},
		LLM: LLMConfig{
			Provider:           getEnv("LLM_PROVIDER", "ollama"),
			OllamaHost:         getEnv("OLLAMA_HOST", "http://localhost:11434"),
			OllamaModel:        getEnv("OLLAMA_MODEL", "qwen2.5:7b"),
			DashScopeKey:       getEnv("DASHSCOPE_API_KEY", ""),
			DashScopeModel:     getEnv("DASHSCOPE_MODEL", "qwen-max"),
			DashScopeCompatURL: getEnv("DASHSCOPE_COMPAT_BASE_URL", ""),
			EmbeddingProvider:  getEnv("EMBEDDING_PROVIDER", "ollama"),
			EmbeddingModel:     getEnv("EMBEDDING_MODEL", "bge-m3"),

			// Multi-tier routing
			RouterEnabled: getEnv("LLM_ROUTER_ENABLED", "false") == "true",
			Tier1Model:    getEnv("LLM_TIER1_MODEL", ""), // e.g., "qwen2.5:7b"
			Tier2Model:    getEnv("LLM_TIER2_MODEL", ""), // e.g., "qwen-plus"
			Tier3Model:    getEnv("LLM_TIER3_MODEL", ""), // e.g., "qwen-max"
		},
		Storage: StorageConfig{
			Backend:      getEnv("STORAGE_BACKEND", "local"),
			LocalRoot:    getEnv("STORAGE_LOCAL_ROOT", "uploads"),
			OSSEndpoint:  getEnv("STORAGE_OSS_ENDPOINT", ""),
			OSSBucket:    getEnv("STORAGE_OSS_BUCKET", ""),
			OSSAccessKey: getEnv("STORAGE_OSS_ACCESS_KEY", ""),
			OSSSecretKey: getEnv("STORAGE_OSS_SECRET_KEY", ""),
		},
		I18n: I18nConfig{
			DefaultLocale: getEnv("I18N_DEFAULT_LOCALE", "zh-CN"),
			LocaleDir:     getEnv("I18N_LOCALE_DIR", "locales"),
		},
		Search: SearchConfig{
			Provider: getEnv("SEARCH_PROVIDER", "searxng"),
			BaseURL:  getEnv("SEARCH_BASE_URL", "http://localhost:8888"),
			APIKey:   getEnv("SEARCH_API_KEY", ""),
		},
		ASR: ASRConfig{
			Provider:   getEnv("ASR_PROVIDER", "whisper"),
			WhisperURL: getEnv("ASR_WHISPER_URL", "http://localhost:9000"),
			APIKey:     getEnv("ASR_API_KEY", ""),
			ModelSize:  getEnv("ASR_MODEL_SIZE", "large-v3"),
		},
		WeKnora: WeKnoraConfig{
			Enabled:       getEnv("WEKNORA_ENABLED", "false") == "true",
			BaseURL:       getEnv("WEKNORA_BASE_URL", "http://localhost:9380/api/v1"),
			EncryptionKey: getEnv("WEKNORA_ENCRYPTION_KEY", ""),
		},
	}
}

// -- Production Safety ------------------------------------------------

// insecureDefaults lists default values that must NOT be used in production.
var insecureDefaults = map[string]struct{}{
	"dev-secret-change-me": {},
	"hanfledge_secret":     {},
	"neo4j_secret":         {},
}

// Validate checks that security-sensitive configuration values are safe for
// the current environment. In release mode (GIN_MODE=release), using any of
// the hardcoded default passwords/secrets is a fatal configuration error.
func (cfg *Config) Validate() error {
	if cfg.Server.GinMode != "release" {
		return nil // no restrictions in debug/test mode
	}

	checks := []struct {
		name  string
		value string
	}{
		{"JWT_SECRET", cfg.JWT.Secret},
		{"DB_PASSWORD", cfg.Database.Password},
		{"NEO4J_PASSWORD", cfg.Neo4j.Password},
	}

	for _, c := range checks {
		if _, insecure := insecureDefaults[c.value]; insecure {
			return fmt.Errorf(
				"SECURITY: %s is using an insecure default value (%q) — set a strong value via environment variable before running in production",
				c.name, c.value,
			)
		}
	}

	return nil
}

// getEnv reads an environment variable with a fallback default value.
func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}

// generateEphemeralSecret generates a secure, random 32-byte hex string.
// This is used as a fallback for JWT_SECRET when no environment variable is provided,
// preventing the use of predictable, hardcoded secrets. Note that because this
// generates a new secret each time the application starts, any existing JWT tokens
// will become invalid upon restart.
func generateEphemeralSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		slogConfig.Error("failed to generate random JWT secret, falling back to insecure default", "error", err)
		return "dev-secret-change-me"
	}
	secret := hex.EncodeToString(b)
	slogConfig.Warn("using generated ephemeral JWT secret. User sessions will not survive restart. Set JWT_SECRET in production.")
	return secret
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
