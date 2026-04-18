package llm

import (
	"context"
	"testing"

	"github.com/hflms/hanfledge/internal/domain/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open sqlite in-memory db: %v", err)
	}

	err = db.AutoMigrate(&model.SystemConfig{})
	if err != nil {
		t.Fatalf("failed to migrate system config: %v", err)
	}

	return db
}

func TestNewDynamicProvider(t *testing.T) {
	db := setupTestDB(t)
	fallback := &MockLLMProvider{ProviderName: "mock-fallback"}
	dp := NewDynamicProvider(db, fallback)

	if dp.DB != db {
		t.Errorf("Expected DB to be set")
	}
	if dp.FallbackProvider != fallback {
		t.Errorf("Expected FallbackProvider to be set")
	}
	if dp.chatClients == nil {
		t.Errorf("Expected chatClients to be initialized")
	}
	if dp.embedClients == nil {
		t.Errorf("Expected embedClients to be initialized")
	}
}

func TestDynamicProvider_loadConfigs(t *testing.T) {
	db := setupTestDB(t)
	db.Create(&model.SystemConfig{Key: "LLM_PROVIDER", Value: "ollama"})
	db.Create(&model.SystemConfig{Key: "OLLAMA_MODEL", Value: "test-model"})

	dp := NewDynamicProvider(db, nil)
	configs := dp.loadConfigs()

	if configs["LLM_PROVIDER"] != "ollama" {
		t.Errorf("Expected LLM_PROVIDER to be ollama, got %s", configs["LLM_PROVIDER"])
	}
	if configs["OLLAMA_MODEL"] != "test-model" {
		t.Errorf("Expected OLLAMA_MODEL to be test-model, got %s", configs["OLLAMA_MODEL"])
	}
}

func TestDynamicProvider_getChatProvider(t *testing.T) {
	db := setupTestDB(t)
	fallback := &MockLLMProvider{ProviderName: "mock-fallback"}
	dp := NewDynamicProvider(db, fallback)

	// Case 1: No configs, should use fallback
	provider := dp.getChatProvider()
	if provider.Name() != "mock-fallback" {
		t.Errorf("Expected fallback provider, got %s", provider.Name())
	}

	// Case 2: Use specific provider
	db.Create(&model.SystemConfig{Key: "LLM_PROVIDER", Value: "ollama"})
	db.Create(&model.SystemConfig{Key: "OLLAMA_BASE_URL", Value: "http://localhost:11434"})
	db.Create(&model.SystemConfig{Key: "OLLAMA_MODEL", Value: "test-model"})

	dp.ClearCache() // clear cache since we added configs
	provider = dp.getChatProvider()
	if provider.Name() != "ollama" {
		t.Errorf("Expected ollama provider, got %s", provider.Name())
	}

	// Case 3: Cache is used
	provider2 := dp.getChatProvider()
	if provider != provider2 {
		t.Errorf("Expected same provider instance due to caching")
	}
}

func TestDynamicProvider_getEmbeddingProvider(t *testing.T) {
	db := setupTestDB(t)
	fallback := &MockLLMProvider{ProviderName: "mock-fallback"}
	dp := NewDynamicProvider(db, fallback)

	// Case 1: No configs, should use fallback
	provider := dp.getEmbeddingProvider()
	if provider.Name() != "mock-fallback" {
		t.Errorf("Expected fallback provider, got %s", provider.Name())
	}

	// Case 2: EMBEDDING_PROVIDER config is used
	db.Create(&model.SystemConfig{Key: "EMBEDDING_PROVIDER", Value: "ollama"})
	db.Create(&model.SystemConfig{Key: "EMBEDDING_MODEL", Value: "test-embed"})

	dp.ClearCache()
	provider = dp.getEmbeddingProvider()
	if provider.Name() != "ollama" {
		t.Errorf("Expected ollama provider, got %s", provider.Name())
	}
}

func TestDynamicProvider_getProviderForRequest(t *testing.T) {
	db := setupTestDB(t)
	dp := NewDynamicProvider(db, nil)

	db.Create(&model.SystemConfig{Key: "LLM_PROVIDER", Value: "ollama"})

	// Case 1: No override, uses global default
	provider := dp.getProviderForRequest(nil)
	if provider.Name() != "ollama" {
		t.Errorf("Expected global provider ollama, got %s", provider.Name())
	}

	// Case 2: Override Provider
	opts := &ChatOptions{
		ProviderOverride: "dashscope",
		ModelOverride:    "qwen-test",
	}
	provider = dp.getProviderForRequest(opts)
	if provider.Name() != "dashscope" {
		t.Errorf("Expected dashscope override, got %s", provider.Name())
	}
}

func TestDynamicProvider_ChatAndEmbed(t *testing.T) {
	db := setupTestDB(t)

	fallback := &MockLLMProvider{
		ProviderName:  "mock-fallback",
		ChatResponse:  "mock chat response",
		EmbedResponse: []float64{0.1, 0.2, 0.3},
	}
	dp := NewDynamicProvider(db, fallback)

	// Test Name
	if dp.Name() != "dynamic" {
		t.Errorf("Expected name dynamic, got %s", dp.Name())
	}

	// Test Chat
	ctx := context.Background()
	resp, err := dp.Chat(ctx, []ChatMessage{{Role: "user", Content: "hello"}}, nil)
	if err != nil {
		t.Fatalf("Unexpected error in Chat: %v", err)
	}
	if resp != "mock chat response" {
		t.Errorf("Expected 'mock chat response', got '%s'", resp)
	}

	// Test StreamChat
	streamResp, err := dp.StreamChat(ctx, []ChatMessage{{Role: "user", Content: "hello"}}, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error in StreamChat: %v", err)
	}
	if streamResp != "mock chat response" {
		t.Errorf("Expected 'mock chat response', got '%s'", streamResp)
	}

	// Test Embed
	embedResp, err := dp.Embed(ctx, "test text")
	if err != nil {
		t.Fatalf("Unexpected error in Embed: %v", err)
	}
	if len(embedResp) != 3 || embedResp[0] != 0.1 {
		t.Errorf("Unexpected embed response: %v", embedResp)
	}

	// EmbedBatch is generally implemented in the provider.
	// Since MockLLMProvider doesn't have EmbedBatch implemented explicitly if it uses default or returns empty...
	// We need to check if MockLLMProvider implements EmbedBatch.
}

func TestDynamicProvider_EmbedBatch(t *testing.T) {
	db := setupTestDB(t)
	fallback := &MockLLMProvider{
		ProviderName:  "mock-fallback",
		EmbedResponse: []float64{0.1, 0.2},
	}
	dp := NewDynamicProvider(db, fallback)

	ctx := context.Background()
	batchResp, err := dp.EmbedBatch(ctx, []string{"t1", "t2"})
	if err != nil {
		t.Fatalf("Unexpected error in EmbedBatch: %v", err)
	}
	if len(batchResp) != 2 || batchResp[0][0] != 0.1 {
		t.Errorf("Unexpected batch resp: %v", batchResp)
	}
}
