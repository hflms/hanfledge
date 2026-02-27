package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OllamaClient implements LLM and Embedding calls via Ollama REST API.
type OllamaClient struct {
	BaseURL        string
	ChatModel      string
	EmbeddingModel string
	HTTPClient     *http.Client
}

// NewOllamaClient creates a new Ollama API client.
func NewOllamaClient(baseURL, chatModel, embeddingModel string) *OllamaClient {
	return &OllamaClient{
		BaseURL:        baseURL,
		ChatModel:      chatModel,
		EmbeddingModel: embeddingModel,
		HTTPClient:     &http.Client{Timeout: 120 * time.Second},
	}
}

// ── Chat API ────────────────────────────────────────────────

// ChatMessage represents a single message in a conversation.
type ChatMessage struct {
	Role    string `json:"role"` // "system" | "user" | "assistant"
	Content string `json:"content"`
}

// ChatRequest is the request body for Ollama /api/chat.
type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
	Options  *ChatOptions  `json:"options,omitempty"`
}

// ChatOptions holds generation parameters.
type ChatOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
	MaxTokens   int     `json:"num_predict,omitempty"`
}

// ChatResponse is the response from Ollama /api/chat.
type ChatResponse struct {
	Model   string      `json:"model"`
	Message ChatMessage `json:"message"`
}

// Chat sends a non-streaming chat request and returns the full response.
func (c *OllamaClient) Chat(ctx context.Context, messages []ChatMessage, opts *ChatOptions) (string, error) {
	reqBody := ChatRequest{
		Model:    c.ChatModel,
		Messages: messages,
		Stream:   false,
		Options:  opts,
	}

	body, err := c.doPost(ctx, "/api/chat", reqBody)
	if err != nil {
		return "", fmt.Errorf("ollama chat failed: %w", err)
	}

	var resp ChatResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("ollama chat response parse failed: %w", err)
	}

	return resp.Message.Content, nil
}

// ── Embedding API ───────────────────────────────────────────

// EmbedRequest is the request body for Ollama /api/embed.
type EmbedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

// EmbedResponse is the response from Ollama /api/embed.
type EmbedResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
}

// Embed generates an embedding vector for the given text.
func (c *OllamaClient) Embed(ctx context.Context, text string) ([]float64, error) {
	reqBody := EmbedRequest{
		Model: c.EmbeddingModel,
		Input: text,
	}

	body, err := c.doPost(ctx, "/api/embed", reqBody)
	if err != nil {
		return nil, fmt.Errorf("ollama embed failed: %w", err)
	}

	var resp EmbedResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("ollama embed response parse failed: %w", err)
	}

	if len(resp.Embeddings) == 0 {
		return nil, fmt.Errorf("ollama returned empty embeddings")
	}

	return resp.Embeddings[0], nil
}

// EmbedBatch generates embedding vectors for multiple texts.
func (c *OllamaClient) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	results := make([][]float64, len(texts))
	for i, text := range texts {
		vec, err := c.Embed(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("embed batch item %d failed: %w", i, err)
		}
		results[i] = vec
	}
	return results, nil
}

// ── HTTP Helper ─────────────────────────────────────────────

func (c *OllamaClient) doPost(ctx context.Context, path string, payload interface{}) ([]byte, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+path, bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama API error %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}
