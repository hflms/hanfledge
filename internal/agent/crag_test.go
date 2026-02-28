package agent

import (
	"strings"
	"testing"
)

// ============================
// CRAG Quality Gateway Tests
// ============================

// -- EvaluateRelevance Tests ------------------------------------------

func TestQualityGateway_EvaluateRelevance_HighQuality(t *testing.T) {
	gw := NewQualityGateway() // threshold = 0.4

	chunks := []RetrievedChunk{
		{Content: "chunk 1", Score: 0.80, ChunkIndex: 1},
		{Content: "chunk 2", Score: 0.65, ChunkIndex: 2},
		{Content: "chunk 3", Score: 0.50, ChunkIndex: 3},
	}

	result := gw.EvaluateRelevance(chunks, "test query")

	if !result.Passed {
		t.Errorf("expected Passed=true, got false (avg=%.4f)", result.AvgScore)
	}
	if result.ChunkCount != 3 {
		t.Errorf("expected ChunkCount=3, got %d", result.ChunkCount)
	}
	// Expected avg: (0.80 + 0.65 + 0.50) / 3 ≈ 0.65
	expectedAvg := 0.65
	if abs(result.AvgScore-expectedAvg) > 0.0001 {
		t.Errorf("expected avg=%.4f, got %.4f", expectedAvg, result.AvgScore)
	}
}

func TestQualityGateway_EvaluateRelevance_LowQuality(t *testing.T) {
	gw := NewQualityGateway() // threshold = 0.4

	chunks := []RetrievedChunk{
		{Content: "weak chunk 1", Score: 0.20, ChunkIndex: 1},
		{Content: "weak chunk 2", Score: 0.15, ChunkIndex: 2},
		{Content: "weak chunk 3", Score: 0.30, ChunkIndex: 3},
	}

	result := gw.EvaluateRelevance(chunks, "irrelevant query")

	if result.Passed {
		t.Errorf("expected Passed=false, got true (avg=%.4f)", result.AvgScore)
	}
	// Expected avg: (0.20 + 0.15 + 0.30) / 3 ≈ 0.2167
	if result.AvgScore >= 0.4 {
		t.Errorf("avg score %.4f should be below threshold 0.4", result.AvgScore)
	}
}

func TestQualityGateway_EvaluateRelevance_EmptyChunks(t *testing.T) {
	gw := NewQualityGateway()

	result := gw.EvaluateRelevance(nil, "query")

	if result.Passed {
		t.Error("expected Passed=false for empty chunks")
	}
	if result.AvgScore != 0 {
		t.Errorf("expected AvgScore=0, got %.4f", result.AvgScore)
	}
	if result.ChunkCount != 0 {
		t.Errorf("expected ChunkCount=0, got %d", result.ChunkCount)
	}
}

func TestQualityGateway_EvaluateRelevance_ExactlyAtThreshold(t *testing.T) {
	gw := NewQualityGatewayWithThreshold(0.50)

	chunks := []RetrievedChunk{
		{Content: "chunk", Score: 0.50, ChunkIndex: 1},
	}

	result := gw.EvaluateRelevance(chunks, "query")

	if !result.Passed {
		t.Error("expected Passed=true when score exactly equals threshold")
	}
}

func TestQualityGateway_EvaluateRelevance_SingleChunk(t *testing.T) {
	gw := NewQualityGateway()

	chunks := []RetrievedChunk{
		{Content: "good chunk", Score: 0.80, ChunkIndex: 1},
	}

	result := gw.EvaluateRelevance(chunks, "query")

	if !result.Passed {
		t.Error("expected Passed=true for high-scoring single chunk")
	}
	if result.ChunkCount != 1 {
		t.Errorf("expected ChunkCount=1, got %d", result.ChunkCount)
	}
}

// -- CustomThreshold Tests --------------------------------------------

func TestQualityGateway_CustomThreshold(t *testing.T) {
	gw := NewQualityGatewayWithThreshold(0.90) // very high threshold

	chunks := []RetrievedChunk{
		{Content: "chunk", Score: 0.70, ChunkIndex: 1},
		{Content: "chunk", Score: 0.75, ChunkIndex: 2},
	}

	result := gw.EvaluateRelevance(chunks, "query")

	if result.Passed {
		t.Error("expected Passed=false with high threshold")
	}
}

// -- HandleFallback Tests ---------------------------------------------

func TestQualityGateway_HandleFallback(t *testing.T) {
	gw := NewQualityGateway()
	original := "你是一位 AI 学习教练"

	result := gw.HandleFallback(original)

	if !strings.HasPrefix(result, original) {
		t.Error("fallback should preserve original prompt")
	}
	if !strings.Contains(result, "相关度较低") {
		t.Error("fallback should contain low-confidence caveat in Chinese")
	}
	if !strings.Contains(result, "自身的知识储备") {
		t.Error("fallback should instruct LLM to rely on internal knowledge")
	}
}

// -- truncateForLog Tests ---------------------------------------------

func TestTruncateForLog_Short(t *testing.T) {
	got := truncateForLog("hello", 10)
	if got != "hello" {
		t.Errorf("expected %q, got %q", "hello", got)
	}
}

func TestTruncateForLog_Long(t *testing.T) {
	got := truncateForLog("这是一个很长的中文字符串需要被截断", 5)
	runes := []rune(got)
	// 5 chars + "..."
	if len(runes) != 8 {
		t.Errorf("expected 8 runes, got %d: %q", len(runes), got)
	}
	if !strings.HasSuffix(got, "...") {
		t.Error("expected truncated string to end with ...")
	}
}

func TestTruncateForLog_ExactLength(t *testing.T) {
	got := truncateForLog("12345", 5)
	if got != "12345" {
		t.Errorf("expected %q, got %q", "12345", got)
	}
}

// -- Helper -----------------------------------------------------------

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
