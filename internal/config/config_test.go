package config

import (
	"os"
	"testing"
)

func TestDatabaseConfig_DSN(t *testing.T) {
	db := DatabaseConfig{
		Host:     "localhost",
		Port:     "5432",
		User:     "user",
		Password: "password",
		DBName:   "db",
		SSLMode:  "disable",
	}

	expected := "host=localhost port=5432 user=user password=password dbname=db sslmode=disable"
	if dsn := db.DSN(); dsn != expected {
		t.Errorf("expected %q, got %q", expected, dsn)
	}
}

func TestConfig_Validate(t *testing.T) {
	t.Run("debug mode", func(t *testing.T) {
		cfg := &Config{
			Server: ServerConfig{GinMode: "debug"},
			JWT:    JWTConfig{Secret: "dev-secret-change-me"},
		}
		if err := cfg.Validate(); err != nil {
			t.Errorf("expected no error in debug mode, got %v", err)
		}
	})

	t.Run("release mode insecure", func(t *testing.T) {
		cfg := &Config{
			Server: ServerConfig{GinMode: "release"},
			JWT:    JWTConfig{Secret: "dev-secret-change-me"},
		}
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for insecure default in release mode")
		}
	})

	t.Run("release mode empty neo4j password", func(t *testing.T) {
		cfg := &Config{
			Server:   ServerConfig{GinMode: "release"},
			JWT:      JWTConfig{Secret: "secure-secret-123"},
			Database: DatabaseConfig{Password: "secure-db-pass"},
			Neo4j:    Neo4jConfig{Password: ""},
		}
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for empty Neo4j password in release mode")
		}
	})

	t.Run("release mode secure", func(t *testing.T) {
		cfg := &Config{
			Server:   ServerConfig{GinMode: "release"},
			JWT:      JWTConfig{Secret: "secure-secret-123"},
			Database: DatabaseConfig{Password: "secure-db-pass"},
			Neo4j:    Neo4jConfig{Password: "secure-neo4j-pass"},
		}
		if err := cfg.Validate(); err != nil {
			t.Errorf("expected no error for secure config in release mode, got %v", err)
		}
	})
}

func TestLoad(t *testing.T) {
	// Unset environment variables to ensure default values are used
	os.Clearenv()

	// Test setting an environment variable
	os.Setenv("SERVER_PORT", "9999")
	os.Setenv("WEKNORA_ENABLED", "true")
	os.Setenv("JWT_EXPIRY_HOURS", "48")

	cfg := Load()

	if cfg.Server.Port != "9999" {
		t.Errorf("expected SERVER_PORT to be 9999, got %s", cfg.Server.Port)
	}
	if !cfg.WeKnora.Enabled {
		t.Errorf("expected WEKNORA_ENABLED to be true, got %v", cfg.WeKnora.Enabled)
	}
	if cfg.JWT.ExpiryHours != 48 {
		t.Errorf("expected JWT_EXPIRY_HOURS to be 48, got %d", cfg.JWT.ExpiryHours)
	}
	// Verify that generateEphemeralSecret works when no JWT_SECRET is set
	if len(cfg.JWT.Secret) != 64 {
		t.Errorf("expected JWT_SECRET to be generated fallback of length 64, got %d", len(cfg.JWT.Secret))
	}

	// Clean up
	os.Clearenv()
}

func TestGetEnv(t *testing.T) {
	os.Clearenv()

	if val := getEnv("TEST_KEY", "fallback"); val != "fallback" {
		t.Errorf("expected fallback, got %s", val)
	}

	os.Setenv("TEST_KEY", "value")
	if val := getEnv("TEST_KEY", "fallback"); val != "value" {
		t.Errorf("expected value, got %s", val)
	}
}

func TestGetEnvInt(t *testing.T) {
	os.Clearenv()

	if val := getEnvInt("TEST_INT_KEY", 42); val != 42 {
		t.Errorf("expected fallback 42, got %d", val)
	}

	os.Setenv("TEST_INT_KEY", "100")
	if val := getEnvInt("TEST_INT_KEY", 42); val != 100 {
		t.Errorf("expected 100, got %d", val)
	}

	os.Setenv("TEST_INT_KEY", "invalid")
	if val := getEnvInt("TEST_INT_KEY", 42); val != 42 {
		t.Errorf("expected fallback 42 for invalid int, got %d", val)
	}
}

func TestGenerateEphemeralSecret(t *testing.T) {
	secret1 := generateEphemeralSecret()
	secret2 := generateEphemeralSecret()

	if len(secret1) != 64 {
		t.Errorf("expected secret length 64, got %d", len(secret1))
	}
	if secret1 == secret2 {
		t.Errorf("expected generated secrets to be unique, but got %s twice", secret1)
	}
}
