package agent

import (
	"context"
	"fmt"
	"strings"
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

// mockBatchRerankLLM supports both batch and single scoring.
// When a prompt contains "【片段 1】" (batch format), it returns a JSON array.
// Otherwise, it falls back to single-score behavior.
type mockBatchRerankLLM struct {
	// batchResponse is returned for batch prompts (JSON array like "[9, 5, 2]")
	batchResponse string
	// batchErr forces batch calls to fail (triggers fallback to individual scoring)
	batchErr error
	// singleScores maps chunk content → individual score
	singleScores map[string]string
	// defaultScore for unmatched single scoring
	defaultScore string
	// callCount tracks number of Chat calls
	callCount int
}

func (m *mockBatchRerankLLM) Name() string { return "mock-batch-reranker" }
func (m *mockBatchRerankLLM) Chat(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatOptions) (string, error) {
	m.callCount++
	prompt := messages[0].Content

	// Detect batch prompt by presence of "【片段 1】"
	if containsSubstring(prompt, "【片段 1】") {
		if m.batchErr != nil {
			return "", m.batchErr
		}
		return m.batchResponse, nil
	}

	// Single scoring
	for content, score := range m.singleScores {
		if containsSubstring(prompt, content) {
			return score, nil
		}
	}
	return m.defaultScore, nil
}
func (m *mockBatchRerankLLM) StreamChat(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatOptions, onToken func(string)) (string, error) {
	return m.Chat(ctx, messages, opts)
}
func (m *mockBatchRerankLLM) Embed(ctx context.Context, text string) ([]float64, error) {
	return nil, nil
}
func (m *mockBatchRerankLLM) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	return nil, nil
}

// containsSubstring checks if haystack contains needle (simple substring match).
func containsSubstring(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && strings.Contains(haystack, needle)
}

// -- Rerank Tests (single chunk / no batch) ----------------------------

func TestReranker_BasicReranking(t *testing.T) {
	// Use batch mock that returns correct JSON array
	mock := &mockBatchRerankLLM{
		batchResponse: "[9, 5, 2]",
		singleScores: map[string]string{
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

	// Batch prompt lists chunks in input order: 不太(1), 高度(2), 中等(3)
	// Batch scores: [9, 5, 2] → 不太=0.9, 高度=0.5, 中等=0.2
	// Sorted by score: 不太(0.9), 高度(0.5), 中等(0.2)
	// Note: batch assigns scores by position, not content
	if result[0].Score < 0.85 || result[0].Score > 0.95 {
		t.Errorf("expected highest score ~0.9, got %.2f", result[0].Score)
	}
}

func TestReranker_TopK_Truncation(t *testing.T) {
	mock := &mockBatchRerankLLM{
		batchResponse: "[7, 7, 7, 7, 7]",
		defaultScore:  "7",
	}
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
	mock := &mockBatchRerankLLM{
		batchResponse: "[7, 7]",
		defaultScore:  "7",
	}
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

	// Single chunk → uses scoreChunk directly (no batch)
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

func TestReranker_DefaultBatchSize(t *testing.T) {
	reranker := NewCrossEncoderReranker(nil)
	if reranker.batchSize != defaultBatchSize {
		t.Errorf("expected default batchSize=%d, got %d", defaultBatchSize, reranker.batchSize)
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

func TestReranker_PreservesChunkMetadata(t *testing.T) {
	mock := &mockRerankLLM{defaultScore: "7"}
	reranker := NewCrossEncoderRerankerWithTopK(mock, 5)

	// Single chunk → scoreChunk path (no batch)
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

// -- Batch Scoring Tests ----------------------------------------------

func TestReranker_BatchScoring_ReducesLLMCalls(t *testing.T) {
	// 3 chunks with batchSize=5 → 1 batch call
	mock := &mockBatchRerankLLM{
		batchResponse: "[7, 8, 6]",
		defaultScore:  "5",
	}
	reranker := NewCrossEncoderRerankerWithTopK(mock, 5)

	chunks := []RetrievedChunk{
		{Content: "chunk 1", ChunkIndex: 1},
		{Content: "chunk 2", ChunkIndex: 2},
		{Content: "chunk 3", ChunkIndex: 3},
	}

	reranker.Rerank(context.Background(), "query", chunks)

	// Should make exactly 1 batch call instead of 3 individual calls
	if mock.callCount != 1 {
		t.Errorf("expected 1 LLM call (batch), got %d", mock.callCount)
	}
}

func TestReranker_BatchScoring_MultipleBatches(t *testing.T) {
	// 8 chunks with batchSize=5 → 1 batch of 5 + 1 batch of 3 = 2 calls
	mock := &mockBatchRerankLLM{
		defaultScore: "7",
	}
	// Return appropriate-length JSON arrays based on batch content
	mock.batchResponse = "[7, 7, 7, 7, 7]" // will be used for first batch

	// Need a smarter mock — let's use a custom one
	callIdx := 0
	responses := []string{"[7, 7, 7, 7, 7]", "[8, 8, 8]"}
	customMock := &mockBatchRerankLLM{defaultScore: "5"}
	customMock.batchResponse = "" // won't be used directly

	// Override with a counting mock
	counterMock := &countingBatchMock{
		responses: responses,
	}

	reranker := NewCrossEncoderRerankerWithTopK(counterMock, 10)
	_ = callIdx
	_ = customMock

	chunks := make([]RetrievedChunk, 8)
	for i := range chunks {
		chunks[i] = RetrievedChunk{Content: fmt.Sprintf("chunk %d", i+1), ChunkIndex: i + 1}
	}

	result := reranker.Rerank(context.Background(), "query", chunks)

	if len(result) != 8 {
		t.Fatalf("expected 8 results, got %d", len(result))
	}
	// 2 batch calls: batch of 5 + batch of 3
	if counterMock.callCount != 2 {
		t.Errorf("expected 2 LLM calls (2 batches), got %d", counterMock.callCount)
	}
}

// countingBatchMock returns pre-configured responses in order.
type countingBatchMock struct {
	responses []string
	callCount int
}

func (m *countingBatchMock) Name() string { return "mock-counting-batch" }
func (m *countingBatchMock) Chat(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatOptions) (string, error) {
	idx := m.callCount
	m.callCount++
	if idx < len(m.responses) {
		return m.responses[idx], nil
	}
	return "5", nil
}
func (m *countingBatchMock) StreamChat(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatOptions, onToken func(string)) (string, error) {
	return m.Chat(ctx, messages, opts)
}
func (m *countingBatchMock) Embed(ctx context.Context, text string) ([]float64, error) {
	return nil, nil
}
func (m *countingBatchMock) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	return nil, nil
}

func TestReranker_BatchScoring_FallbackToIndividual(t *testing.T) {
	// Batch fails → falls back to individual scoring
	mock := &mockBatchRerankLLM{
		batchErr:     fmt.Errorf("batch parse error"),
		defaultScore: "7",
		singleScores: map[string]string{
			"chunk A": "9",
			"chunk B": "3",
		},
	}
	reranker := NewCrossEncoderRerankerWithTopK(mock, 5)

	chunks := []RetrievedChunk{
		{Content: "chunk A", ChunkIndex: 1},
		{Content: "chunk B", ChunkIndex: 2},
	}

	result := reranker.Rerank(context.Background(), "query", chunks)

	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}
	// Chunk A should be first (score 9 > 3)
	if result[0].ChunkIndex != 1 {
		t.Errorf("expected chunk A first (higher score), got chunk %d", result[0].ChunkIndex)
	}
	// 1 batch call (failed) + 2 individual calls = 3
	if mock.callCount != 3 {
		t.Errorf("expected 3 LLM calls (1 failed batch + 2 individual), got %d", mock.callCount)
	}
}

func TestReranker_BatchScoring_SingleChunk_UsesScoreChunk(t *testing.T) {
	// Single chunk in a batch → uses scoreChunk directly (simpler prompt)
	mock := &mockRerankLLM{defaultScore: "8"}
	reranker := NewCrossEncoderRerankerWithTopK(mock, 5)

	chunks := []RetrievedChunk{
		{Content: "single chunk", ChunkIndex: 1, Score: 0.03},
	}

	result := reranker.Rerank(context.Background(), "query", chunks)

	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	// Should use single scoring prompt (1 call)
	if mock.callCount != 1 {
		t.Errorf("expected 1 LLM call, got %d", mock.callCount)
	}
	if result[0].Score < 0.75 || result[0].Score > 0.85 {
		t.Errorf("expected score ~0.8, got %.3f", result[0].Score)
	}
}

func TestReranker_BatchScoring_WrongCount_FallsBack(t *testing.T) {
	// Batch returns wrong number of scores → falls back to individual
	mock := &mockBatchRerankLLM{
		batchResponse: "[7, 8]", // only 2 scores for 3 chunks
		singleScores: map[string]string{
			"chunk 1": "9",
			"chunk 2": "5",
			"chunk 3": "2",
		},
		defaultScore: "5",
	}
	reranker := NewCrossEncoderRerankerWithTopK(mock, 5)

	chunks := []RetrievedChunk{
		{Content: "chunk 1", ChunkIndex: 1},
		{Content: "chunk 2", ChunkIndex: 2},
		{Content: "chunk 3", ChunkIndex: 3},
	}

	result := reranker.Rerank(context.Background(), "query", chunks)

	if len(result) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result))
	}
	// 1 batch (wrong count) + 3 individual = 4 calls
	if mock.callCount != 4 {
		t.Errorf("expected 4 LLM calls (1 failed batch + 3 individual), got %d", mock.callCount)
	}
}

// -- parseBatchScores Tests -------------------------------------------

func TestParseBatchScores_ValidJSON(t *testing.T) {
	scores, err := parseBatchScores("[7, 3, 9, 5, 2]", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []float64{0.7, 0.3, 0.9, 0.5, 0.2}
	for i, s := range scores {
		if abs(s-expected[i]) > 0.001 {
			t.Errorf("score[%d] = %.3f, want %.3f", i, s, expected[i])
		}
	}
}

func TestParseBatchScores_WithPrefix(t *testing.T) {
	scores, err := parseBatchScores("评分：[8, 6, 4]", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []float64{0.8, 0.6, 0.4}
	for i, s := range scores {
		if abs(s-expected[i]) > 0.001 {
			t.Errorf("score[%d] = %.3f, want %.3f", i, s, expected[i])
		}
	}
}

func TestParseBatchScores_WithSurroundingText(t *testing.T) {
	scores, err := parseBatchScores("以下是评分结果：\n[9, 7, 3]\n以上是我的评估。", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scores) != 3 {
		t.Fatalf("expected 3 scores, got %d", len(scores))
	}
	if abs(scores[0]-0.9) > 0.001 {
		t.Errorf("expected 0.9, got %.3f", scores[0])
	}
}

func TestParseBatchScores_Clamping(t *testing.T) {
	scores, err := parseBatchScores("[15, -3, 5]", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scores[0] != 1.0 {
		t.Errorf("expected 1.0 (clamped 15), got %.3f", scores[0])
	}
	if scores[1] != 0.0 {
		t.Errorf("expected 0.0 (clamped -3), got %.3f", scores[1])
	}
	if abs(scores[2]-0.5) > 0.001 {
		t.Errorf("expected 0.5, got %.3f", scores[2])
	}
}

func TestParseBatchScores_WrongCount(t *testing.T) {
	_, err := parseBatchScores("[7, 8]", 3)
	if err == nil {
		t.Error("expected error for wrong count, got nil")
	}
}

func TestParseBatchScores_InvalidJSON(t *testing.T) {
	_, err := parseBatchScores("not json at all", 3)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestParseBatchScores_FloatScores(t *testing.T) {
	scores, err := parseBatchScores("[7.5, 3.2, 9.8]", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if abs(scores[0]-0.75) > 0.001 {
		t.Errorf("expected 0.75, got %.3f", scores[0])
	}
}

func TestParseBatchScores_Empty(t *testing.T) {
	_, err := parseBatchScores("", 3)
	if err == nil {
		t.Error("expected error for empty response, got nil")
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
