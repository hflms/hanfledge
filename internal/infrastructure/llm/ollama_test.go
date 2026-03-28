package llm

import (
	"testing"
	"time"
)

func TestNewOllamaClient(t *testing.T) {
	baseURL := "http://localhost:11434"
	chatModel := "qwen2.5:7b"
	embeddingModel := "bge-m3"

	client := NewOllamaClient(baseURL, chatModel, embeddingModel)

	if client == nil {
		t.Fatalf("Expected OllamaClient, got nil")
	}

	if client.BaseURL != baseURL {
		t.Errorf("Expected BaseURL %s, got %s", baseURL, client.BaseURL)
	}

	if client.ChatModel != chatModel {
		t.Errorf("Expected ChatModel %s, got %s", chatModel, client.ChatModel)
	}

	if client.EmbeddingModel != embeddingModel {
		t.Errorf("Expected EmbeddingModel %s, got %s", embeddingModel, client.EmbeddingModel)
	}

	if client.HTTPClient == nil {
		t.Fatalf("Expected HTTPClient to be initialized, got nil")
	}

	expectedTimeout := 120 * time.Second
	if client.HTTPClient.Timeout != expectedTimeout {
		t.Errorf("Expected HTTPClient Timeout to be %v, got %v", expectedTimeout, client.HTTPClient.Timeout)
	}
}

func TestOllamaClient_Name(t *testing.T) {
	client := NewOllamaClient("http://localhost:11434", "qwen2.5:7b", "bge-m3")

	expectedName := "ollama"
	if name := client.Name(); name != expectedName {
		t.Errorf("Expected Name %s, got %s", expectedName, name)
	}
}
