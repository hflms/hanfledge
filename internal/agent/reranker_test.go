package agent

import (
	"context"
	"fmt"
	"testing"

	"github.com/hflms/hanfledge/internal/infrastructure/llm"
)

// ============================
// Cross-Encoder Reranker Tests
// ============================

// -- Mock LLM for Reranker Tests --------------------------------------

type mockRerankLLM struct {
	// scores maps chunk content → score response (e.g., "8")
	scores map[string]string
	// defaultScore is returned when content is not in the scores map
	defaultScore string
	// err is returned for all calls if non-nil
	err error
	// callCount tracks number of Chat calls
	callCount int
}

func (m *mockRerankLLM) Name() string { return "mock-reranker" }
func (m *mockRerankLLM) Chat(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatOptions) (string, error) {
	m.callCount++
	if m.err != nil {
		return "", m.err
	}
	// Extract chunk content from the prompt to determine which score to return
	prompt := messages[0].Content
	for content, score := range m.scores {
		if containsSubstring(prompt, content) {
			return score, nil
		}
	}
	return m.defaultScore, nil
}
func (m *mockRerankLLM) StreamChat(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatOptions, onToken func(string)) (string, error) {
	return m.Chat(ctx, messages, opts)
}
func (m *mockRerankLLM) Embed(ctx context.Context, text string) ([]float64, error) {
	return nil, nil
}
func (m *mockRerankLLM) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	return nil, nil
}

// containsSubstring checks if haystack contains needle (simple substring match).
func containsSubstring(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && findSubstring(haystack, needle)
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// -- Rerank Tests -----------------------------------------------------

func TestReranker_BasicReranking(t *testing.T) {
	mock := &mockRerankLLM{
		scores: map[string]string{
			"高度相关的内容": "9",
			"中等相关的内容": "5",
			"不太相关的内容": "2",
		},
		defaultScore: "5",
	}
	reranker := NewCrossEncoderRerankerWithTopK(mock, 3)

	chunks := []RetrievedChunk{
		{Content: "不太相关的内容", Source: "semantic", Score: 0.030, ChunkIndex: 1},
		{Content: "高度相关的内容", Source: "semantic", Score: 0.020, ChunkIndex: 2},
		{Content: "中等相关的内容", Source: "graph", Score: 0.025, ChunkIndex: 3},
	}

	result := reranker.Rerank(context.Background(), "测试查询", chunks)

	if len(result) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result))
	}

	// Highest scoring chunk should be first (9/10 = 0.9)
	if result[0].ChunkIndex != 2 {
		t.Errorf("expected chunk 2 (highest score) first, got chunk %d", result[0].ChunkIndex)
	}
	// Score should be normalized to [0, 1]
	if result[0].Score < 0.85 || result[0].Score > 0.95 {
		t.Errorf("expected score ~0.9, got %.2f", result[0].Score)
	}

	// Second should be medium (5/10 = 0.5)
	if result[1].ChunkIndex != 3 {
		t.Errorf("expected chunk 3 (medium score) second, got chunk %d", result[1].ChunkIndex)
	}

	// Lowest should be last (2/10 = 0.2)
	if result[2].ChunkIndex != 1 {
		t.Errorf("expected chunk 1 (lowest score) last, got chunk %d", result[2].ChunkIndex)
	}
}

func TestReranker_TopK_Truncation(t *testing.T) {
	mock := &mockRerankLLM{defaultScore: "7"}
	reranker := NewCrossEncoderRerankerWithTopK(mock, 2)

	chunks := []RetrievedChunk{
		{Content: "chunk 1", ChunkIndex: 1},
		{Content: "chunk 2", ChunkIndex: 2},
		{Content: "chunk 3", ChunkIndex: 3},
		{Content: "chunk 4", ChunkIndex: 4},
		{Content: "chunk 5", ChunkIndex: 5},
	}

	result := reranker.Rerank(context.Background(), "query", chunks)

	if len(result) != 2 {
		t.Errorf("expected 2 results (topK=2), got %d", len(result))
	}
}

func TestReranker_TopK_LargerThanInput(t *testing.T) {
	mock := &mockRerankLLM{defaultScore: "7"}
	reranker := NewCrossEncoderRerankerWithTopK(mock, 100)

	chunks := []RetrievedChunk{
		{Content: "chunk 1", ChunkIndex: 1},
		{Content: "chunk 2", ChunkIndex: 2},
	}

	result := reranker.Rerank(context.Background(), "query", chunks)

	if len(result) != 2 {
		t.Errorf("expected 2 results (all chunks), got %d", len(result))
	}
}

func TestReranker_NilLLM_GracefulDegradation(t *testing.T) {
	reranker := NewCrossEncoderReranker(nil)

	chunks := []RetrievedChunk{
		{Content: "chunk 1", Score: 0.030, ChunkIndex: 1},
		{Content: "chunk 2", Score: 0.020, ChunkIndex: 2},
	}

	result := reranker.Rerank(context.Background(), "query", chunks)

	// Should return chunks unchanged
	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}
	if result[0].Score != 0.030 {
		t.Errorf("expected original score 0.030, got %.3f", result[0].Score)
	}
}

func TestReranker_EmptyChunks(t *testing.T) {
	mock := &mockRerankLLM{defaultScore: "7"}
	reranker := NewCrossEncoderReranker(mock)

	result := reranker.Rerank(context.Background(), "query", nil)

	if len(result) != 0 {
		t.Errorf("expected 0 results for nil chunks, got %d", len(result))
	}
}

func TestReranker_AllScoringFails_FallbackToRRF(t *testing.T) {
	mock := &mockRerankLLM{err: fmt.Errorf("LLM service unavailable")}
	reranker := NewCrossEncoderRerankerWithTopK(mock, 5)

	chunks := []RetrievedChunk{
		{Content: "chunk 1", Score: 0.030, ChunkIndex: 1},
		{Content: "chunk 2", Score: 0.025, ChunkIndex: 2},
		{Content: "chunk 3", Score: 0.020, ChunkIndex: 3},
	}

	result := reranker.Rerank(context.Background(), "query", chunks)

	// Should return original chunks with original order
	if len(result) != 3 {
		t.Fatalf("expected 3 results (fallback), got %d", len(result))
	}
	// Original order preserved
	if result[0].ChunkIndex != 1 {
		t.Errorf("expected chunk 1 first (original order), got chunk %d", result[0].ChunkIndex)
	}
	// Original scores preserved
	if result[0].Score != 0.030 {
		t.Errorf("expected original score 0.030, got %.3f", result[0].Score)
	}
}

func TestReranker_PartialScoringFailure(t *testing.T) {
	callNum := 0
	// Custom mock that fails on the 2nd call
	mock := &mockRerankLLM{
		scores: map[string]string{
			"good chunk": "9",
			"bad chunk":  "", // will trigger error
		},
		defaultScore: "7",
	}
	_ = callNum

	reranker := NewCrossEncoderRerankerWithTopK(mock, 5)

	chunks := []RetrievedChunk{
		{Content: "good chunk", Score: 0.030, ChunkIndex: 1},
		{Content: "another good chunk", Score: 0.025, ChunkIndex: 2},
	}

	result := reranker.Rerank(context.Background(), "query", chunks)

	// Should still return results (partial success)
	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}
	// Successfully scored chunks should rank first
	if result[0].Score < 0.5 {
		t.Errorf("expected high score for scored chunk, got %.2f", result[0].Score)
	}
}

func TestReranker_ScoresReplaceRRFScores(t *testing.T) {
	mock := &mockRerankLLM{defaultScore: "8"}
	reranker := NewCrossEncoderRerankerWithTopK(mock, 5)

	chunks := []RetrievedChunk{
		{Content: "chunk", Score: 0.025, ChunkIndex: 1}, // RRF score
	}

	result := reranker.Rerank(context.Background(), "query", chunks)

	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	// Score should be normalized cross-encoder score (8/10 = 0.8), not RRF 0.025
	if result[0].Score < 0.75 || result[0].Score > 0.85 {
		t.Errorf("expected score ~0.8 (cross-encoder), got %.3f", result[0].Score)
	}
}

func TestReranker_DefaultTopK(t *testing.T) {
	reranker := NewCrossEncoderReranker(nil) // nil LLM to check config only
	if reranker.topK != defaultRerankTopK {
		t.Errorf("expected default topK=%d, got %d", defaultRerankTopK, reranker.topK)
	}
}

func TestReranker_InvalidTopK_Defaults(t *testing.T) {
	reranker := NewCrossEncoderRerankerWithTopK(nil, 0)
	if reranker.topK != defaultRerankTopK {
		t.Errorf("expected default topK=%d for topK=0, got %d", defaultRerankTopK, reranker.topK)
	}

	reranker2 := NewCrossEncoderRerankerWithTopK(nil, -5)
	if reranker2.topK != defaultRerankTopK {
		t.Errorf("expected default topK=%d for topK=-5, got %d", defaultRerankTopK, reranker2.topK)
	}
}

func TestReranker_LLMCallCount(t *testing.T) {
	mock := &mockRerankLLM{defaultScore: "7"}
	reranker := NewCrossEncoderRerankerWithTopK(mock, 5)

	chunks := []RetrievedChunk{
		{Content: "chunk 1", ChunkIndex: 1},
		{Content: "chunk 2", ChunkIndex: 2},
		{Content: "chunk 3", ChunkIndex: 3},
	}

	reranker.Rerank(context.Background(), "query", chunks)

	// Should make exactly one LLM call per chunk
	if mock.callCount != 3 {
		t.Errorf("expected 3 LLM calls, got %d", mock.callCount)
	}
}

func TestReranker_PreservesChunkMetadata(t *testing.T) {
	mock := &mockRerankLLM{defaultScore: "7"}
	reranker := NewCrossEncoderRerankerWithTopK(mock, 5)

	chunks := []RetrievedChunk{
		{Content: "some content", Source: "hybrid", Score: 0.03, ChunkIndex: 42},
	}

	result := reranker.Rerank(context.Background(), "query", chunks)

	if result[0].Content != "some content" {
		t.Errorf("expected content preserved, got %q", result[0].Content)
	}
	if result[0].Source != "hybrid" {
		t.Errorf("expected source preserved, got %q", result[0].Source)
	}
	if result[0].ChunkIndex != 42 {
		t.Errorf("expected ChunkIndex preserved, got %d", result[0].ChunkIndex)
	}
}

// -- parseScore Tests -------------------------------------------------

func TestParseScore_Integer(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"7", 0.7},
		{"0", 0.0},
		{"10", 1.0},
		{"5", 0.5},
	}
	for _, tt := range tests {
		got, err := parseScore(tt.input)
		if err != nil {
			t.Errorf("parseScore(%q) error: %v", tt.input, err)
			continue
		}
		if abs(got-tt.expected) > 0.001 {
			t.Errorf("parseScore(%q) = %.3f, want %.3f", tt.input, got, tt.expected)
		}
	}
}

func TestParseScore_WithWhitespace(t *testing.T) {
	got, err := parseScore("  8  ")
	if err != nil {
		t.Fatalf("parseScore with whitespace error: %v", err)
	}
	if abs(got-0.8) > 0.001 {
		t.Errorf("expected 0.8, got %.3f", got)
	}
}

func TestParseScore_WithPrefix(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"评分：7", 0.7},
		{"评分:8", 0.8},
		{"分数：9", 0.9},
		{"Score:6", 0.6},
		{"score:4", 0.4},
	}
	for _, tt := range tests {
		got, err := parseScore(tt.input)
		if err != nil {
			t.Errorf("parseScore(%q) error: %v", tt.input, err)
			continue
		}
		if abs(got-tt.expected) > 0.001 {
			t.Errorf("parseScore(%q) = %.3f, want %.3f", tt.input, got, tt.expected)
		}
	}
}

func TestParseScore_WithTrailingChars(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"7分", 0.7},
		{"8.", 0.8},
		{"9。", 0.9},
	}
	for _, tt := range tests {
		got, err := parseScore(tt.input)
		if err != nil {
			t.Errorf("parseScore(%q) error: %v", tt.input, err)
			continue
		}
		if abs(got-tt.expected) > 0.001 {
			t.Errorf("parseScore(%q) = %.3f, want %.3f", tt.input, got, tt.expected)
		}
	}
}

func TestParseScore_Float(t *testing.T) {
	got, err := parseScore("7.5")
	if err != nil {
		t.Fatalf("parseScore float error: %v", err)
	}
	if abs(got-0.75) > 0.001 {
		t.Errorf("expected 0.75, got %.3f", got)
	}
}

func TestParseScore_ClampAbove10(t *testing.T) {
	got, err := parseScore("15")
	if err != nil {
		t.Fatalf("parseScore clamp error: %v", err)
	}
	if got != 1.0 {
		t.Errorf("expected 1.0 (clamped), got %.3f", got)
	}
}

func TestParseScore_ClampBelowZero(t *testing.T) {
	got, err := parseScore("-3")
	if err != nil {
		t.Fatalf("parseScore clamp error: %v", err)
	}
	if got != 0.0 {
		t.Errorf("expected 0.0 (clamped), got %.3f", got)
	}
}

func TestParseScore_InvalidInput(t *testing.T) {
	invalidInputs := []string{
		"这是一个非常好的文档，相关性很高",
		"very relevant",
		"",
		"abc",
	}
	for _, input := range invalidInputs {
		_, err := parseScore(input)
		if err == nil {
			t.Errorf("parseScore(%q) expected error, got nil", input)
		}
	}
}

// -- truncateContent Tests --------------------------------------------

func TestTruncateContent_Short(t *testing.T) {
	got := truncateContent("hello", 10)
	if got != "hello" {
		t.Errorf("expected %q, got %q", "hello", got)
	}
}

func TestTruncateContent_Long(t *testing.T) {
	got := truncateContent("这是一个很长的中文字符串需要被截断处理", 5)
	runes := []rune(got)
	// 5 chars + "..." (3 bytes but 3 runes)
	if len(runes) != 8 {
		t.Errorf("expected 8 runes, got %d: %q", len(runes), got)
	}
}

func TestTruncateContent_ExactLength(t *testing.T) {
	got := truncateContent("12345", 5)
	if got != "12345" {
		t.Errorf("expected %q, got %q", "12345", got)
	}
}

func TestTruncateContent_Empty(t *testing.T) {
	got := truncateContent("", 5)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}
