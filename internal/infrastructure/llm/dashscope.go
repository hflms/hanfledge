package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ============================
// DashScope LLM Provider
// ============================
//
// 职责：通过阿里云 DashScope API 接入通义千问系列模型。
// 支持 Chat (non-streaming)、StreamChat (SSE streaming)、Embed。
//
// API 文档：https://help.aliyun.com/zh/dashscope/developer-reference/api-details
//
// Reference: design.md section 7.7

// Compile-time check: DashScopeClient implements LLMProvider.
var _ LLMProvider = (*DashScopeClient)(nil)

const (
	dashScopeBaseURL        = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	dashScopeEmbedEndpoint  = "https://dashscope.aliyuncs.com/compatible-mode/v1/embeddings"
	dashScopeCompatChatPath = "/chat/completions"
)

// DashScopeClient implements LLMProvider via Alibaba Cloud DashScope API.
type DashScopeClient struct {
	APIKey         string
	ChatModel      string
	EmbeddingModel string
	MaxTokens      int
	CompatBaseURL  string
	HTTPClient     *http.Client
}

// DashScopeConfig holds configuration for creating a DashScope client.
type DashScopeConfig struct {
	APIKey         string
	ChatModel      string // e.g., "qwen-max", "qwen-plus", "qwen-turbo"
	EmbeddingModel string // e.g., "text-embedding-v3"
	MaxTokens      int
	TimeoutSeconds int
	CompatBaseURL  string
}

// NewDashScopeClient creates a new DashScope API client.
func NewDashScopeClient(cfg DashScopeConfig) *DashScopeClient {
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 4096
	}
	timeout := 60 * time.Second
	if cfg.TimeoutSeconds > 0 {
		timeout = time.Duration(cfg.TimeoutSeconds) * time.Second
	}

	return &DashScopeClient{
		APIKey:         cfg.APIKey,
		ChatModel:      cfg.ChatModel,
		EmbeddingModel: cfg.EmbeddingModel,
		MaxTokens:      cfg.MaxTokens,
		CompatBaseURL:  cfg.CompatBaseURL,
		HTTPClient:     &http.Client{Timeout: timeout},
	}
}

// Name returns the provider identifier.
func (c *DashScopeClient) Name() string { return "dashscope" }

// -- Chat API (Non-Streaming) ------------------------------------

// DashScope request/response types (internal).

type dsInput struct {
	Messages []ChatMessage `json:"messages"`
}

type dsParameters struct {
	MaxTokens    int     `json:"max_tokens,omitempty"`
	Temperature  float64 `json:"temperature,omitempty"`
	TopP         float64 `json:"top_p,omitempty"`
	ResultFormat string  `json:"result_format"`
}

type dsChatRequest struct {
	Model      string       `json:"model"`
	Input      dsInput      `json:"input"`
	Parameters dsParameters `json:"parameters"`
}

type dsChatResponse struct {
	Output struct {
		Choices []struct {
			Message ChatMessage `json:"message"`
		} `json:"choices"`
		Text string `json:"text"` // fallback for non-message format
	} `json:"output"`
	Usage struct {
		TotalTokens  int `json:"total_tokens"`
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	RequestID string `json:"request_id"`
}

type dsOpenAIChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Stream      bool          `json:"stream"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	TopP        float64       `json:"top_p,omitempty"`
	// DashScope 推理模型 (qwen3.5-plus 等) 的思考开关。
	// 默认 true，设为 false 可关闭推理以加快响应。
	EnableThinking *bool `json:"enable_thinking,omitempty"`
}

type dsOpenAIChatResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
		Delta   struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// Chat sends a non-streaming chat request via DashScope API.
func (c *DashScopeClient) Chat(ctx context.Context, messages []ChatMessage, opts *ChatOptions) (string, error) {
	reqBody := c.buildOpenAIRequest(messages, opts, false)
	body, err := c.doPost(ctx, c.compatChatURL(), reqBody, false)
	if err != nil {
		return "", fmt.Errorf("dashscope chat failed: %w", err)
	}

	var resp dsOpenAIChatResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("dashscope parse response failed: %w", err)
	}
	if len(resp.Choices) > 0 {
		return resp.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("dashscope returned empty response")
}

// -- StreamChat API (SSE Streaming) ------------------------------

// dsSSEEvent supports both DashScope native and OpenAI-compatible SSE formats.
// DashScope native: {"output":{"choices":[{"message":{"content":"cumulative"},"finish_reason":"stop"}]}}
// OpenAI compat:    {"choices":[{"delta":{"content":"delta","reasoning_content":"thinking"},"finish_reason":"stop"}]}
type dsSSEEvent struct {
	// OpenAI-compatible format (used by /compatible-mode/v1/chat/completions)
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"delta"`
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`

	// DashScope native format (legacy)
	Output struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	} `json:"output"`
}

// StreamChat sends a streaming chat request via DashScope SSE API.
func (c *DashScopeClient) StreamChat(ctx context.Context, messages []ChatMessage, opts *ChatOptions, onToken func(token string)) (string, error) {
	reqBody := c.buildOpenAIRequest(messages, opts, true)
	resp, err := c.doPostStream(ctx, c.compatChatURL(), reqBody)
	if err != nil {
		return "", fmt.Errorf("dashscope stream chat failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("dashscope API error %d: %s", resp.StatusCode, string(body))
	}

	// DashScope uses SSE format: "data: {...}\n\n"
	var fullResponse strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	// Track previous content length for cumulative mode (DashScope native)
	prevContentLen := 0
	isOpenAIFormat := false // auto-detected on first event

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return fullResponse.String(), ctx.Err()
		default:
		}

		line := scanner.Text()

		// SSE format: lines prefixed with "data: "
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimPrefix(line, "data:")
		data = strings.TrimSpace(data)
		if data == "" || data == "[DONE]" {
			continue
		}

		var event dsSSEEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		// Try OpenAI-compatible format first (choices[].delta.content)
		if len(event.Choices) > 0 {
			isOpenAIFormat = true
			choice := event.Choices[0]

			// Delta content (incremental tokens)
			if choice.Delta.Content != "" {
				fullResponse.WriteString(choice.Delta.Content)
				if onToken != nil {
					onToken(choice.Delta.Content)
				}
			}
			// Message content (some responses use message instead of delta)
			if choice.Message.Content != "" && choice.Delta.Content == "" {
				// Cumulative mode fallback within OpenAI format
				content := choice.Message.Content
				if len(content) > prevContentLen {
					delta := content[prevContentLen:]
					prevContentLen = len(content)
					fullResponse.Reset()
					fullResponse.WriteString(content)
					if delta != "" && onToken != nil {
						onToken(delta)
					}
				}
			}

			if choice.FinishReason == "stop" {
				break
			}
			continue
		}

		// Fallback: DashScope native format (output.choices[].message.content)
		if !isOpenAIFormat && len(event.Output.Choices) > 0 {
			choice := event.Output.Choices[0]
			content := choice.Message.Content

			// DashScope native returns cumulative content
			if len(content) > prevContentLen {
				delta := content[prevContentLen:]
				prevContentLen = len(content)
				fullResponse.Reset()
				fullResponse.WriteString(content)

				if delta != "" && onToken != nil {
					onToken(delta)
				}
			}

			if choice.FinishReason == "stop" {
				break
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fullResponse.String(), fmt.Errorf("dashscope stream read error: %w", err)
	}

	return fullResponse.String(), nil
}

// -- Embedding API -----------------------------------------------

type dsEmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type dsEmbedResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
}

// Embed generates an embedding vector for a single text.
func (c *DashScopeClient) Embed(ctx context.Context, text string) ([]float64, error) {
	results, err := c.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("dashscope returned empty embeddings")
	}
	return results[0], nil
}

// EmbedBatch generates embedding vectors for multiple texts.
// DashScope/Bailian compatible mode batch limit is typically 10 for v4 or larger for v3, using 10 to be safe.
func (c *DashScopeClient) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	const batchSize = 10
	var allResults [][]float64

	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		reqBody := dsEmbedRequest{
			Model: c.EmbeddingModel,
			Input: batch,
		}

		body, err := c.doPost(ctx, c.compatEmbedURL(), reqBody, false)
		if err != nil {
			return nil, fmt.Errorf("dashscope embed batch failed: %w", err)
		}

		var resp dsEmbedResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("dashscope embed parse failed: %w", err)
		}

		if len(resp.Data) == 0 {
			return nil, fmt.Errorf("dashscope returned empty embeddings")
		}

		// Sort by index to maintain order
		batchResults := make([][]float64, len(batch))
		for _, emb := range resp.Data {
			if emb.Index < len(batchResults) {
				batchResults[emb.Index] = emb.Embedding
			}
		}
		allResults = append(allResults, batchResults...)
	}

	return allResults, nil
}

// -- HTTP Helpers ------------------------------------------------

func (c *DashScopeClient) buildParameters(opts *ChatOptions) dsParameters {
	params := dsParameters{
		MaxTokens:    c.MaxTokens,
		ResultFormat: "message",
	}
	if opts != nil {
		if opts.Temperature > 0 {
			params.Temperature = opts.Temperature
		}
		if opts.TopP > 0 {
			params.TopP = opts.TopP
		}
		if opts.MaxTokens > 0 {
			params.MaxTokens = opts.MaxTokens
		}
	}
	return params
}

func (c *DashScopeClient) buildOpenAIRequest(messages []ChatMessage, opts *ChatOptions, stream bool) dsOpenAIChatRequest {
	params := c.buildParameters(opts)
	req := dsOpenAIChatRequest{
		Model:       c.ChatModel,
		Messages:    messages,
		Stream:      stream,
		MaxTokens:   params.MaxTokens,
		Temperature: params.Temperature,
		TopP:        params.TopP,
	}
	// 对于非流式调用，禁用推理模式 (qwen3.5-plus 等模型默认开启 thinking，
	// 会导致响应时间极长。非流式场景如 QueryExpander、Reranker 不需要推理。)
	if !stream {
		noThink := false
		req.EnableThinking = &noThink
	}
	return req
}

func (c *DashScopeClient) compatBase() string {
	if c.CompatBaseURL != "" {
		return strings.TrimRight(c.CompatBaseURL, "/")
	}
	return dashScopeBaseURL
}

func (c *DashScopeClient) compatChatURL() string {
	return c.compatBase() + dashScopeCompatChatPath
}

func (c *DashScopeClient) compatEmbedURL() string {
	return c.compatBase() + "/embeddings"
}

func (c *DashScopeClient) doPost(ctx context.Context, endpoint string, payload interface{}, stream bool) ([]byte, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := endpoint
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		url = c.compatBase() + "/" + strings.TrimLeft(endpoint, "/")
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	_ = stream // reserved for future use

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
		return nil, fmt.Errorf("dashscope API error %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func (c *DashScopeClient) doPostStream(ctx context.Context, endpoint string, payload interface{}) (*http.Response, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := endpoint
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		url = c.compatBase() + "/" + strings.TrimLeft(endpoint, "/")
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	// No timeout for streaming — context handles cancellation
	streamClient := &http.Client{}
	return streamClient.Do(req)
}
