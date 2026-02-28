package llm

import (
	"context"
	"log"
)

// ============================
// Model Router — 分级模型路由
// ============================
//
// 职责：根据任务复杂度智能路由到不同等级的模型。
//
// 三级模型策略:
//   Tier1: 本地小模型 (Ollama 7B) — 低成本、低延迟
//   Tier2: 中等模型 (Qwen-Plus) — 平衡性价比
//   Tier3: 旗舰大模型 (Qwen-Max) — 最高质量
//
// 路由规则 (design.md §8.3.3):
//   简单问答 → Tier1
//   中等难度苏格拉底对话 → Tier2
//   复杂推理、跨学科 → Tier3
//
// Reference: design.md section 8.3.3

// Compile-time check: ModelRouter implements LLMProvider.
var _ LLMProvider = (*ModelRouter)(nil)

// TaskComplexity represents the estimated complexity of a task.
type TaskComplexity string

const (
	TaskComplexityLow    TaskComplexity = "low"
	TaskComplexityMedium TaskComplexity = "medium"
	TaskComplexityHigh   TaskComplexity = "high"
)

// ModelRouter routes LLM calls to the most appropriate provider
// based on task complexity. It defaults to the fallback provider
// when no specific routing context is available.
type ModelRouter struct {
	Tier1    LLMProvider // Local small model (e.g., Ollama 7B)
	Tier2    LLMProvider // Mid-range model (e.g., Qwen-Plus)
	Tier3    LLMProvider // Flagship model (e.g., Qwen-Max)
	Fallback LLMProvider // Default provider when no routing context
}

// NewModelRouter creates a model router.
// Any tier can be nil — requests fall through to the next available tier.
// fallback is required and used when no tier-specific context is available.
func NewModelRouter(tier1, tier2, tier3, fallback LLMProvider) *ModelRouter {
	return &ModelRouter{
		Tier1:    tier1,
		Tier2:    tier2,
		Tier3:    tier3,
		Fallback: fallback,
	}
}

// Name returns the router identifier.
func (r *ModelRouter) Name() string { return "router" }

// Route selects the appropriate provider for the given complexity.
func (r *ModelRouter) Route(complexity TaskComplexity) LLMProvider {
	switch complexity {
	case TaskComplexityLow:
		if r.Tier1 != nil {
			return r.Tier1
		}
		// Fall through
	case TaskComplexityMedium:
		if r.Tier2 != nil {
			return r.Tier2
		}
		// Fall through
	case TaskComplexityHigh:
		if r.Tier3 != nil {
			return r.Tier3
		}
		// Fall through
	}
	return r.Fallback
}

// -- LLMProvider Interface (delegates to Fallback) ----------------
// When used as a plain LLMProvider (without explicit routing),
// the ModelRouter delegates all calls to the Fallback provider.
// This allows the router to be used transparently wherever LLMProvider is expected.

// Chat delegates to the fallback provider.
func (r *ModelRouter) Chat(ctx context.Context, messages []ChatMessage, opts *ChatOptions) (string, error) {
	provider := r.Fallback
	log.Printf("[Router] Chat → %s (default)", provider.Name())
	return provider.Chat(ctx, messages, opts)
}

// StreamChat delegates to the fallback provider.
func (r *ModelRouter) StreamChat(ctx context.Context, messages []ChatMessage, opts *ChatOptions, onToken func(token string)) (string, error) {
	provider := r.Fallback
	log.Printf("[Router] StreamChat → %s (default)", provider.Name())
	return provider.StreamChat(ctx, messages, opts, onToken)
}

// Embed delegates to the fallback provider.
func (r *ModelRouter) Embed(ctx context.Context, text string) ([]float64, error) {
	return r.Fallback.Embed(ctx, text)
}

// EmbedBatch delegates to the fallback provider.
func (r *ModelRouter) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	return r.Fallback.EmbedBatch(ctx, texts)
}

// -- Routed Chat Methods -----------------------------------------
// These methods allow callers to explicitly specify complexity for routing.

// ChatRouted sends a chat request via the provider selected for the given complexity.
func (r *ModelRouter) ChatRouted(ctx context.Context, complexity TaskComplexity, messages []ChatMessage, opts *ChatOptions) (string, error) {
	provider := r.Route(complexity)
	log.Printf("[Router] ChatRouted(%s) → %s", complexity, provider.Name())
	return provider.Chat(ctx, messages, opts)
}

// StreamChatRouted sends a streaming chat request via the selected provider.
func (r *ModelRouter) StreamChatRouted(ctx context.Context, complexity TaskComplexity, messages []ChatMessage, opts *ChatOptions, onToken func(token string)) (string, error) {
	provider := r.Route(complexity)
	log.Printf("[Router] StreamChatRouted(%s) → %s", complexity, provider.Name())
	return provider.StreamChat(ctx, messages, opts, onToken)
}
