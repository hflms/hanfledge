package llm

import (
	"context"
	"log"
	"strings"
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

// -- TaskContext — 基于上下文的复杂度评分 (§8.3.3) --------

// TaskContext 描述一次 LLM 调用的任务上下文，用于复杂度评分和模型路由。
type TaskContext struct {
	UserInput         string  // 学生输入
	SkillID           string  // 当前活跃技能 ID
	Mastery           float64 // 当前 KP 掌握度 [0,1]
	TurnCount         int     // 本会话已进行的对话轮数
	ChunkCount        int     // RAG 检索到的文档片段数
	GraphNodeCount    int     // 知识图谱上下文节点数
	HasMisconceptions bool    // 是否涉及谬误检测
	IsCrossDiscipline bool    // 是否涉及跨学科
}

// complexityKeywords 复杂推理关键词（中文）。
var complexityKeywords = []string{
	"为什么", "解释", "比较", "分析", "推导", "证明",
	"区别", "联系", "本质", "原理", "矛盾", "辩证",
}

// simpleQAKeywords 简单问答关键词。
var simpleQAKeywords = []string{
	"是什么", "定义", "公式", "记住", "背诵", "列举",
}

// EstimateComplexity 基于 TaskContext 评估任务复杂度 (design.md §8.3.3)。
//
// 评分规则:
//   - 基础分: 0.3 (中等)
//   - 输入长度 > 100 字: +0.1
//   - 包含复杂推理关键词: +0.15
//   - 包含简单问答关键词: -0.15
//   - 谬误侦探/角色扮演技能: +0.1
//   - 跨学科: +0.2
//   - 掌握度低 (< 0.3): +0.1 (需要更细致引导)
//   - RAG 检索量大 (> 5 chunks): +0.1
//   - 对话轮数 > 8: +0.05 (深度对话)
//
// 最终映射:
//
//	score < 0.3  → low
//	score < 0.6  → medium
//	score >= 0.6 → high
func (tc *TaskContext) EstimateComplexity() TaskComplexity {
	score := tc.ComplexityScore()
	if score < 0.3 {
		return TaskComplexityLow
	}
	if score < 0.6 {
		return TaskComplexityMedium
	}
	return TaskComplexityHigh
}

// ComplexityScore 返回 [0, 1] 范围的复杂度分数，用于测试和可观察性。
func (tc *TaskContext) ComplexityScore() float64 {
	score := 0.3 // 基础分

	// 输入长度
	inputRunes := []rune(tc.UserInput)
	if len(inputRunes) > 100 {
		score += 0.1
	}

	// 关键词分析
	lower := strings.ToLower(tc.UserInput)
	for _, kw := range complexityKeywords {
		if strings.Contains(lower, kw) {
			score += 0.15
			break
		}
	}
	for _, kw := range simpleQAKeywords {
		if strings.Contains(lower, kw) {
			score -= 0.15
			break
		}
	}

	// 技能复杂度
	complexSkills := map[string]bool{
		"general_assessment_fallacy": true,
		"fallacy-detective":          true,
		"general_review_roleplay":    true,
		"role-play":                  true,
		"general_assessment_quiz":    true,
	}
	if complexSkills[tc.SkillID] {
		score += 0.1
	}

	// 跨学科
	if tc.IsCrossDiscipline {
		score += 0.2
	}

	// 谬误检测
	if tc.HasMisconceptions {
		score += 0.05
	}

	// 低掌握度需要更细致引导
	if tc.Mastery < 0.3 && tc.Mastery > 0 {
		score += 0.1
	}

	// RAG 检索量大
	if tc.ChunkCount > 5 {
		score += 0.1
	}

	// 深度对话
	if tc.TurnCount > 8 {
		score += 0.05
	}

	// 限制到 [0, 1]
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	return score
}

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

// -- TaskContext-Based Routing (§8.3.3) --------------------------
// These methods accept a TaskContext and automatically estimate complexity.

// ChatWithContext routes a chat request based on TaskContext complexity estimation.
func (r *ModelRouter) ChatWithContext(ctx context.Context, tc *TaskContext, messages []ChatMessage, opts *ChatOptions) (string, error) {
	complexity := tc.EstimateComplexity()
	score := tc.ComplexityScore()
	provider := r.Route(complexity)
	log.Printf("[Router] ChatWithContext(score=%.2f → %s) → %s", score, complexity, provider.Name())
	return provider.Chat(ctx, messages, opts)
}

// StreamChatWithContext routes a streaming chat request based on TaskContext.
func (r *ModelRouter) StreamChatWithContext(ctx context.Context, tc *TaskContext, messages []ChatMessage, opts *ChatOptions, onToken func(token string)) (string, error) {
	complexity := tc.EstimateComplexity()
	score := tc.ComplexityScore()
	provider := r.Route(complexity)
	log.Printf("[Router] StreamChatWithContext(score=%.2f → %s) → %s", score, complexity, provider.Name())
	return provider.StreamChat(ctx, messages, opts, onToken)
}
