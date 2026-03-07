package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Compile-time check: OllamaClient implements LLMProvider.
var _ LLMProvider = (*OllamaClient)(nil)

// OllamaClient implements LLMProvider via Ollama REST API.
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

// Name returns the provider identifier.
func (c *OllamaClient) Name() string { return "ollama" }

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
	Temperature      float64 `json:"temperature,omitempty"`
	TopP             float64 `json:"top_p,omitempty"`
	MaxTokens        int     `json:"num_predict,omitempty"`
	ProviderOverride string  `json:"-"`
	ModelOverride    string  `json:"-"`
}

// ChatResponse is the response from Ollama /api/chat.
type ChatResponse struct {
	Model   string      `json:"model"`
	Message ChatMessage `json:"message"`
}

// StreamChatResponse represents a single chunk in the Ollama streaming response (NDJSON).
type StreamChatResponse struct {
	Model   string      `json:"model"`
	Message ChatMessage `json:"message"`
	Done    bool        `json:"done"`
}

// StreamChat sends a streaming chat request and invokes onToken for each token delta.
// It also accumulates and returns the full response text.
// The onToken callback may be nil (in which case tokens are accumulated silently).
func (c *OllamaClient) StreamChat(ctx context.Context, messages []ChatMessage, opts *ChatOptions, onToken func(token string)) (string, error) {
	reqBody := ChatRequest{
		Model:    c.ChatModel,
		Messages: messages,
		Stream:   true,
		Options:  opts,
	}

	resp, err := c.doPostStream(ctx, "/api/chat", reqBody)
	if err != nil {
		return "", fmt.Errorf("ollama stream chat failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama API error %d: %s", resp.StatusCode, string(body))
	}

	var fullResponse string
	scanner := bufio.NewScanner(resp.Body)
	// Increase scanner buffer for potentially large JSON lines
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		// Check context cancellation between lines
		select {
		case <-ctx.Done():
			return fullResponse, ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var chunk StreamChatResponse
		if err := json.Unmarshal(line, &chunk); err != nil {
			// Skip malformed lines
			continue
		}

		token := chunk.Message.Content
		if token != "" {
			fullResponse += token
			if onToken != nil {
				onToken(token)
			}
		}

		if chunk.Done {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return fullResponse, fmt.Errorf("ollama stream read error: %w", err)
	}

	return fullResponse, nil
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

// doPostStream sends a POST request and returns the raw *http.Response for streaming reads.
// The caller is responsible for closing resp.Body.
func (c *OllamaClient) doPostStream(ctx context.Context, path string, payload interface{}) (*http.Response, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+path, bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	// Use a client without timeout for streaming — context handles cancellation
	streamClient := &http.Client{}
	resp, err := streamClient.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

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
