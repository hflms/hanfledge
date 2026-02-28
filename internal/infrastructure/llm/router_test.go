package llm

import (
	"context"
	"fmt"
	"testing"
)

// ── MockLLMProvider ─────────────────────────────────────────
// Hand-written mock implementing the LLMProvider interface for testing.

type MockLLMProvider struct {
	ProviderName    string
	ChatResponse    string
	ChatError       error
	EmbedResponse   []float64
	EmbedError      error
	StreamResponses []string // tokens to emit
}

func (m *MockLLMProvider) Name() string { return m.ProviderName }

func (m *MockLLMProvider) Chat(ctx context.Context, messages []ChatMessage, opts *ChatOptions) (string, error) {
	return m.ChatResponse, m.ChatError
}

func (m *MockLLMProvider) StreamChat(ctx context.Context, messages []ChatMessage, opts *ChatOptions, onToken func(token string)) (string, error) {
	if m.ChatError != nil {
		return "", m.ChatError
	}
	full := ""
	for _, tok := range m.StreamResponses {
		if onToken != nil {
			onToken(tok)
		}
		full += tok
	}
	if full == "" {
		full = m.ChatResponse
	}
	return full, nil
}

func (m *MockLLMProvider) Embed(ctx context.Context, text string) ([]float64, error) {
	return m.EmbedResponse, m.EmbedError
}

func (m *MockLLMProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	if m.EmbedError != nil {
		return nil, m.EmbedError
	}
	result := make([][]float64, len(texts))
	for i := range texts {
		result[i] = m.EmbedResponse
	}
	return result, nil
}

// ── ModelRouter.Name Tests ──────────────────────────────────

func TestModelRouter_Name(t *testing.T) {
	fb := &MockLLMProvider{ProviderName: "fallback"}
	r := NewModelRouter(nil, nil, nil, fb)

	if r.Name() != "router" {
		t.Errorf("Name() = %q, want %q", r.Name(), "router")
	}
}

// ── ModelRouter.Route Tests ─────────────────────────────────

func TestModelRouter_Route(t *testing.T) {
	tier1 := &MockLLMProvider{ProviderName: "tier1"}
	tier2 := &MockLLMProvider{ProviderName: "tier2"}
	tier3 := &MockLLMProvider{ProviderName: "tier3"}
	fb := &MockLLMProvider{ProviderName: "fallback"}

	r := NewModelRouter(tier1, tier2, tier3, fb)

	tests := []struct {
		name       string
		complexity TaskComplexity
		expected   string
	}{
		{"低复杂度→Tier1", TaskComplexityLow, "tier1"},
		{"中复杂度→Tier2", TaskComplexityMedium, "tier2"},
		{"高复杂度→Tier3", TaskComplexityHigh, "tier3"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			provider := r.Route(tc.complexity)
			if provider.Name() != tc.expected {
				t.Errorf("Route(%q) = %q, want %q",
					tc.complexity, provider.Name(), tc.expected)
			}
		})
	}
}

func TestModelRouter_Route_FallbackOnNilTier(t *testing.T) {
	fb := &MockLLMProvider{ProviderName: "fallback"}

	// 只有 fallback，没有任何 tier
	r := NewModelRouter(nil, nil, nil, fb)

	tests := []struct {
		name       string
		complexity TaskComplexity
	}{
		{"低复杂度回退", TaskComplexityLow},
		{"中复杂度回退", TaskComplexityMedium},
		{"高复杂度回退", TaskComplexityHigh},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			provider := r.Route(tc.complexity)
			if provider.Name() != "fallback" {
				t.Errorf("Route(%q) = %q, want fallback",
					tc.complexity, provider.Name())
			}
		})
	}
}

func TestModelRouter_Route_PartialTiers(t *testing.T) {
	tier2 := &MockLLMProvider{ProviderName: "tier2"}
	fb := &MockLLMProvider{ProviderName: "fallback"}

	// 只有 tier2 和 fallback
	r := NewModelRouter(nil, tier2, nil, fb)

	if r.Route(TaskComplexityLow).Name() != "fallback" {
		t.Errorf("Low without tier1 should fallback")
	}
	if r.Route(TaskComplexityMedium).Name() != "tier2" {
		t.Errorf("Medium should use tier2")
	}
	if r.Route(TaskComplexityHigh).Name() != "fallback" {
		t.Errorf("High without tier3 should fallback")
	}
}

func TestModelRouter_Route_UnknownComplexity(t *testing.T) {
	fb := &MockLLMProvider{ProviderName: "fallback"}
	r := NewModelRouter(nil, nil, nil, fb)

	// 未知复杂度应回退到 fallback
	provider := r.Route(TaskComplexity("unknown"))
	if provider.Name() != "fallback" {
		t.Errorf("Route(unknown) = %q, want fallback", provider.Name())
	}
}

// ── ModelRouter delegation Tests ────────────────────────────

func TestModelRouter_Chat_DelegatesToFallback(t *testing.T) {
	fb := &MockLLMProvider{
		ProviderName: "fallback",
		ChatResponse: "你好",
	}
	r := NewModelRouter(nil, nil, nil, fb)

	resp, err := r.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "test"}}, nil)
	if err != nil {
		t.Errorf("Chat() returned error: %v", err)
	}
	if resp != "你好" {
		t.Errorf("Chat() = %q, want %q", resp, "你好")
	}
}

func TestModelRouter_Embed_DelegatesToFallback(t *testing.T) {
	fb := &MockLLMProvider{
		ProviderName:  "fallback",
		EmbedResponse: []float64{0.1, 0.2, 0.3},
	}
	r := NewModelRouter(nil, nil, nil, fb)

	vec, err := r.Embed(context.Background(), "test")
	if err != nil {
		t.Errorf("Embed() returned error: %v", err)
	}
	if len(vec) != 3 {
		t.Errorf("Embed() returned %d dimensions, want 3", len(vec))
	}
}

func TestModelRouter_ChatRouted(t *testing.T) {
	tier1 := &MockLLMProvider{ProviderName: "tier1", ChatResponse: "tier1回复"}
	tier3 := &MockLLMProvider{ProviderName: "tier3", ChatResponse: "tier3回复"}
	fb := &MockLLMProvider{ProviderName: "fallback", ChatResponse: "fallback回复"}

	r := NewModelRouter(tier1, nil, tier3, fb)

	resp, err := r.ChatRouted(context.Background(), TaskComplexityLow,
		[]ChatMessage{{Role: "user", Content: "简单问题"}}, nil)
	if err != nil {
		t.Errorf("ChatRouted() error: %v", err)
	}
	if resp != "tier1回复" {
		t.Errorf("ChatRouted(low) = %q, want %q", resp, "tier1回复")
	}

	resp, err = r.ChatRouted(context.Background(), TaskComplexityHigh,
		[]ChatMessage{{Role: "user", Content: "复杂问题"}}, nil)
	if err != nil {
		t.Errorf("ChatRouted() error: %v", err)
	}
	if resp != "tier3回复" {
		t.Errorf("ChatRouted(high) = %q, want %q", resp, "tier3回复")
	}
}

func TestModelRouter_ChatRouted_Error(t *testing.T) {
	fb := &MockLLMProvider{
		ProviderName: "fallback",
		ChatError:    fmt.Errorf("LLM 连接失败"),
	}
	r := NewModelRouter(nil, nil, nil, fb)

	_, err := r.ChatRouted(context.Background(), TaskComplexityLow,
		[]ChatMessage{{Role: "user", Content: "test"}}, nil)
	if err == nil {
		t.Errorf("ChatRouted() should return error when provider fails")
	}
}

func TestModelRouter_StreamChatRouted(t *testing.T) {
	tier2 := &MockLLMProvider{
		ProviderName:    "tier2",
		StreamResponses: []string{"你", "好", "世", "界"},
	}
	fb := &MockLLMProvider{ProviderName: "fallback"}
	r := NewModelRouter(nil, tier2, nil, fb)

	var tokens []string
	resp, err := r.StreamChatRouted(context.Background(), TaskComplexityMedium,
		[]ChatMessage{{Role: "user", Content: "test"}}, nil,
		func(token string) {
			tokens = append(tokens, token)
		})

	if err != nil {
		t.Errorf("StreamChatRouted() error: %v", err)
	}
	if resp != "你好世界" {
		t.Errorf("StreamChatRouted() = %q, want %q", resp, "你好世界")
	}
	if len(tokens) != 4 {
		t.Errorf("onToken called %d times, want 4", len(tokens))
	}
}

// ── Compile-time interface check ────────────────────────────

func TestMockLLMProvider_ImplementsInterface(t *testing.T) {
	var _ LLMProvider = (*MockLLMProvider)(nil)
}

// ── TaskContext Complexity Estimation Tests ─────────────────

func TestTaskContext_EstimateComplexity(t *testing.T) {
	tests := []struct {
		name     string
		ctx      TaskContext
		expected TaskComplexity
	}{
		{
			name:     "简单问答→low",
			ctx:      TaskContext{UserInput: "光合作用的定义是什么"},
			expected: TaskComplexityLow,
		},
		{
			name:     "默认中等复杂度→medium",
			ctx:      TaskContext{UserInput: "请帮我理解这个概念"},
			expected: TaskComplexityMedium,
		},
		{
			name: "复杂推理关键词+跨学科→high",
			ctx: TaskContext{
				UserInput:         "为什么植物在夜晚不进行光合作用？请分析其本质原理并比较动植物代谢差异",
				SkillID:           "general_assessment_fallacy",
				IsCrossDiscipline: true,
			},
			expected: TaskComplexityHigh,
		},
		{
			name: "跨学科→high",
			ctx: TaskContext{
				UserInput:         "请解释一下",
				IsCrossDiscipline: true,
				ChunkCount:        8,
			},
			expected: TaskComplexityHigh,
		},
		{
			name: "低掌握度+谬误侦探→medium→high",
			ctx: TaskContext{
				UserInput:         "我不太确定",
				Mastery:           0.2,
				SkillID:           "general_assessment_fallacy",
				HasMisconceptions: true,
				TurnCount:         10,
			},
			expected: TaskComplexityHigh,
		},
		{
			name: "简单关键词降级",
			ctx: TaskContext{
				UserInput: "列举光合作用的公式",
			},
			expected: TaskComplexityLow,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.ctx.EstimateComplexity()
			if result != tc.expected {
				t.Errorf("EstimateComplexity() = %q (score=%.2f), want %q",
					result, tc.ctx.ComplexityScore(), tc.expected)
			}
		})
	}
}

func TestTaskContext_ComplexityScore_Range(t *testing.T) {
	// 确保分数始终在 [0, 1] 范围内
	extremeCases := []TaskContext{
		{}, // 全空
		{
			UserInput:         "为什么比较分析推导证明区别联系本质原理矛盾辩证解释" + string(make([]rune, 200)),
			SkillID:           "general_assessment_fallacy",
			Mastery:           0.1,
			TurnCount:         20,
			ChunkCount:        10,
			GraphNodeCount:    5,
			HasMisconceptions: true,
			IsCrossDiscipline: true,
		},
	}

	for i, tc := range extremeCases {
		score := tc.ComplexityScore()
		if score < 0 || score > 1 {
			t.Errorf("case %d: ComplexityScore() = %.2f, want [0, 1]", i, score)
		}
	}
}

// ── ChatWithContext / StreamChatWithContext Tests ────────────

func TestModelRouter_ChatWithContext(t *testing.T) {
	tier1 := &MockLLMProvider{ProviderName: "tier1", ChatResponse: "简单回复"}
	tier3 := &MockLLMProvider{ProviderName: "tier3", ChatResponse: "复杂回复"}
	fb := &MockLLMProvider{ProviderName: "fallback", ChatResponse: "默认"}

	r := NewModelRouter(tier1, nil, tier3, fb)

	// 简单问答 → tier1
	resp, err := r.ChatWithContext(context.Background(), &TaskContext{
		UserInput: "列举光合作用的公式",
	}, []ChatMessage{{Role: "user", Content: "test"}}, nil)

	if err != nil {
		t.Errorf("ChatWithContext() error: %v", err)
	}
	if resp != "简单回复" {
		t.Errorf("ChatWithContext(simple) = %q, want %q", resp, "简单回复")
	}

	// 复杂推理 → tier3
	resp, err = r.ChatWithContext(context.Background(), &TaskContext{
		UserInput:         "为什么这两个学科之间存在联系？请分析本质原理",
		IsCrossDiscipline: true,
		SkillID:           "general_assessment_fallacy",
	}, []ChatMessage{{Role: "user", Content: "test"}}, nil)

	if err != nil {
		t.Errorf("ChatWithContext() error: %v", err)
	}
	if resp != "复杂回复" {
		t.Errorf("ChatWithContext(complex) = %q, want %q", resp, "复杂回复")
	}
}

func TestModelRouter_StreamChatWithContext(t *testing.T) {
	tier2 := &MockLLMProvider{
		ProviderName:    "tier2",
		StreamResponses: []string{"中", "等"},
	}
	fb := &MockLLMProvider{ProviderName: "fallback"}
	r := NewModelRouter(nil, tier2, nil, fb)

	var tokens []string
	resp, err := r.StreamChatWithContext(context.Background(), &TaskContext{
		UserInput: "请帮我理解这个概念的特点",
	}, []ChatMessage{{Role: "user", Content: "test"}}, nil,
		func(token string) { tokens = append(tokens, token) })

	if err != nil {
		t.Errorf("StreamChatWithContext() error: %v", err)
	}
	if resp != "中等" {
		t.Errorf("StreamChatWithContext() = %q, want %q", resp, "中等")
	}
	if len(tokens) != 2 {
		t.Errorf("onToken called %d times, want 2", len(tokens))
	}
}
