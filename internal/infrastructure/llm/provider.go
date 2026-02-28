package llm

import "context"

// ============================
// LLM Provider Interface
// ============================
//
// 职责：定义大语言模型接入的标准契约。
// 所有 LLM Provider（Ollama、DashScope、Gemini 等）必须实现此接口。
// Agent 层通过此接口调用 LLM，不依赖具体实现。
//
// Reference: design.md section 7.3

// -- Core Interface -----------------------------------------------

// LLMProvider 定义大语言模型接入的标准契约。
// Chat 和 StreamChat 用于对话，Embed 和 EmbedBatch 用于向量化。
type LLMProvider interface {
	// Name returns the provider name (e.g., "ollama", "dashscope").
	Name() string

	// Chat sends a non-streaming chat request and returns the full response.
	Chat(ctx context.Context, messages []ChatMessage, opts *ChatOptions) (string, error)

	// StreamChat sends a streaming chat request and invokes onToken for each token delta.
	// Returns the accumulated full response text.
	// onToken may be nil (tokens are accumulated silently).
	StreamChat(ctx context.Context, messages []ChatMessage, opts *ChatOptions, onToken func(token string)) (string, error)

	// Embed generates an embedding vector for a single text.
	Embed(ctx context.Context, text string) ([]float64, error)

	// EmbedBatch generates embedding vectors for multiple texts.
	EmbedBatch(ctx context.Context, texts []string) ([][]float64, error)
}

// -- Shared Types -------------------------------------------------
// ChatMessage, ChatOptions, etc. are defined in ollama.go and shared
// across all providers. They are provider-agnostic request/response types.
