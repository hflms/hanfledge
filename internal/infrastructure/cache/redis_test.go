package cache

import (
	"math"
	"testing"
)

// ============================
// L2 Semantic Cache + Cosine Similarity Tests
// ============================
//
// Tests cover pure functions that don't require a Redis connection:
// - CosineSimilarity: vector math
// - PromptHash: deterministic hashing
// - embeddingHash: embedding → key mapping
// - truncateStr: log helper

// -- CosineSimilarity Tests -------------------------------------------

func TestCosineSimilarity_IdenticalVectors(t *testing.T) {
	a := []float64{1.0, 2.0, 3.0}
	sim := CosineSimilarity(a, a)
	if math.Abs(sim-1.0) > 1e-9 {
		t.Errorf("identical vectors should have similarity 1.0, got %.10f", sim)
	}
}

func TestCosineSimilarity_OrthogonalVectors(t *testing.T) {
	a := []float64{1.0, 0.0, 0.0}
	b := []float64{0.0, 1.0, 0.0}
	sim := CosineSimilarity(a, b)
	if math.Abs(sim) > 1e-9 {
		t.Errorf("orthogonal vectors should have similarity 0, got %.10f", sim)
	}
}

func TestCosineSimilarity_OppositeVectors(t *testing.T) {
	a := []float64{1.0, 2.0, 3.0}
	b := []float64{-1.0, -2.0, -3.0}
	sim := CosineSimilarity(a, b)
	if math.Abs(sim-(-1.0)) > 1e-9 {
		t.Errorf("opposite vectors should have similarity -1.0, got %.10f", sim)
	}
}

func TestCosineSimilarity_DifferentLengths(t *testing.T) {
	a := []float64{1.0, 2.0}
	b := []float64{1.0, 2.0, 3.0}
	sim := CosineSimilarity(a, b)
	if sim != 0 {
		t.Errorf("mismatched dimensions should return 0, got %f", sim)
	}
}

func TestCosineSimilarity_EmptyVectors(t *testing.T) {
	sim := CosineSimilarity(nil, nil)
	if sim != 0 {
		t.Errorf("nil vectors should return 0, got %f", sim)
	}

	sim2 := CosineSimilarity([]float64{}, []float64{})
	if sim2 != 0 {
		t.Errorf("empty vectors should return 0, got %f", sim2)
	}
}

func TestCosineSimilarity_ZeroVector(t *testing.T) {
	a := []float64{0, 0, 0}
	b := []float64{1.0, 2.0, 3.0}
	sim := CosineSimilarity(a, b)
	if sim != 0 {
		t.Errorf("zero vector should return 0, got %f", sim)
	}
}

func TestCosineSimilarity_HighSimilarity(t *testing.T) {
	// Two very similar but not identical vectors
	a := []float64{0.5, 0.3, 0.8, 0.1}
	b := []float64{0.51, 0.29, 0.81, 0.09}
	sim := CosineSimilarity(a, b)
	if sim < 0.99 {
		t.Errorf("very similar vectors should have high similarity, got %f", sim)
	}
}

func TestCosineSimilarity_UnitVectors(t *testing.T) {
	// Pre-normalized vectors: cos(45°) = √2/2 ≈ 0.7071
	a := []float64{1.0, 0.0}
	b := []float64{math.Sqrt(2) / 2, math.Sqrt(2) / 2}
	sim := CosineSimilarity(a, b)
	expected := math.Sqrt(2) / 2
	if math.Abs(sim-expected) > 1e-9 {
		t.Errorf("expected sim=%.6f, got %.6f", expected, sim)
	}
}

func TestCosineSimilarity_LargeDimension(t *testing.T) {
	// Simulate 1024-dim embeddings (bge-m3 output)
	n := 1024
	a := make([]float64, n)
	b := make([]float64, n)
	for i := 0; i < n; i++ {
		a[i] = float64(i) / float64(n)
		b[i] = float64(i) / float64(n) // identical
	}
	sim := CosineSimilarity(a, b)
	if math.Abs(sim-1.0) > 1e-9 {
		t.Errorf("identical 1024-dim vectors should have similarity 1.0, got %.10f", sim)
	}

	// Slightly perturb b
	b[0] += 0.001
	sim2 := CosineSimilarity(a, b)
	if sim2 >= 1.0 || sim2 < 0.999 {
		t.Errorf("slightly perturbed 1024-dim vectors should have very high similarity, got %.10f", sim2)
	}
}

func TestCosineSimilarity_SingleDimension(t *testing.T) {
	a := []float64{5.0}
	b := []float64{3.0}
	sim := CosineSimilarity(a, b)
	if math.Abs(sim-1.0) > 1e-9 {
		t.Errorf("same-direction 1D vectors should have similarity 1.0, got %f", sim)
	}

	c := []float64{-3.0}
	sim2 := CosineSimilarity(a, c)
	if math.Abs(sim2-(-1.0)) > 1e-9 {
		t.Errorf("opposite-direction 1D vectors should have similarity -1.0, got %f", sim2)
	}
}

// -- PromptHash Tests -------------------------------------------------

func TestPromptHash_Deterministic(t *testing.T) {
	h1 := PromptHash("system", "input", []string{"user", "assistant"}, []string{"hello", "world"})
	h2 := PromptHash("system", "input", []string{"user", "assistant"}, []string{"hello", "world"})
	if h1 != h2 {
		t.Errorf("same inputs should produce same hash: %s != %s", h1, h2)
	}
}

func TestPromptHash_DifferentSystemPrompt(t *testing.T) {
	h1 := PromptHash("system A", "input", nil, nil)
	h2 := PromptHash("system B", "input", nil, nil)
	if h1 == h2 {
		t.Error("different system prompts should produce different hashes")
	}
}

func TestPromptHash_DifferentUserInput(t *testing.T) {
	h1 := PromptHash("system", "input A", nil, nil)
	h2 := PromptHash("system", "input B", nil, nil)
	if h1 == h2 {
		t.Error("different user inputs should produce different hashes")
	}
}

func TestPromptHash_DifferentHistory(t *testing.T) {
	h1 := PromptHash("system", "input", []string{"user"}, []string{"hello"})
	h2 := PromptHash("system", "input", []string{"user"}, []string{"hi"})
	if h1 == h2 {
		t.Error("different history should produce different hashes")
	}
}

func TestPromptHash_EmptyInputs(t *testing.T) {
	h := PromptHash("", "", nil, nil)
	if h == "" {
		t.Error("hash of empty inputs should not be empty")
	}
	if len(h) != 64 {
		t.Errorf("SHA-256 hex hash should be 64 chars, got %d", len(h))
	}
}

func TestPromptHash_HistoryOrderMatters(t *testing.T) {
	h1 := PromptHash("sys", "usr", []string{"user", "assistant"}, []string{"a", "b"})
	h2 := PromptHash("sys", "usr", []string{"assistant", "user"}, []string{"b", "a"})
	if h1 == h2 {
		t.Error("different history order should produce different hashes")
	}
}

func TestPromptHash_ChineseContent(t *testing.T) {
	h := PromptHash("你是AI学习教练", "什么是二次函数", []string{"user"}, []string{"请解释抛物线"})
	if h == "" {
		t.Error("Chinese content should produce a valid hash")
	}
	if len(h) != 64 {
		t.Errorf("SHA-256 hex hash should be 64 chars, got %d", len(h))
	}
}

// -- embeddingHash Tests ----------------------------------------------

func TestEmbeddingHash_Deterministic(t *testing.T) {
	emb := []float64{0.1, 0.2, 0.3, 0.4}
	h1 := embeddingHash(emb)
	h2 := embeddingHash(emb)
	if h1 != h2 {
		t.Errorf("same embedding should produce same hash: %s != %s", h1, h2)
	}
}

func TestEmbeddingHash_DifferentEmbeddings(t *testing.T) {
	h1 := embeddingHash([]float64{0.1, 0.2, 0.3})
	h2 := embeddingHash([]float64{0.1, 0.2, 0.4})
	if h1 == h2 {
		t.Error("different embeddings should produce different hashes")
	}
}

func TestEmbeddingHash_Length(t *testing.T) {
	h := embeddingHash([]float64{1.0, 2.0, 3.0})
	// SHA-256 first 16 bytes → 32 hex chars
	if len(h) != 32 {
		t.Errorf("expected 32-char hex hash, got %d chars: %s", len(h), h)
	}
}

func TestEmbeddingHash_Empty(t *testing.T) {
	h := embeddingHash([]float64{})
	if h == "" {
		t.Error("empty embedding should still produce a hash (of empty string)")
	}
}

func TestEmbeddingHash_PrecisionTruncation(t *testing.T) {
	// Values differing only beyond 4 decimal places should hash the same
	h1 := embeddingHash([]float64{0.12345})
	h2 := embeddingHash([]float64{0.12349})
	if h1 != h2 {
		t.Errorf("values differing beyond 4 decimals should hash same (%.4f == %.4f): %s != %s",
			0.12345, 0.12349, h1, h2)
	}

	// Values differing at 4th decimal should hash differently
	h3 := embeddingHash([]float64{0.1234})
	h4 := embeddingHash([]float64{0.1235})
	if h3 == h4 {
		t.Error("values differing at 4th decimal should hash differently")
	}
}

// -- truncateStr Tests ------------------------------------------------

func TestTruncateStr_Short(t *testing.T) {
	got := truncateStr("hello", 10)
	if got != "hello" {
		t.Errorf("expected %q, got %q", "hello", got)
	}
}

func TestTruncateStr_Long(t *testing.T) {
	got := truncateStr("hello world foo", 5)
	if got != "hello..." {
		t.Errorf("expected %q, got %q", "hello...", got)
	}
}

func TestTruncateStr_ExactLength(t *testing.T) {
	got := truncateStr("12345", 5)
	if got != "12345" {
		t.Errorf("expected %q, got %q", "12345", got)
	}
}

func TestTruncateStr_Chinese(t *testing.T) {
	got := truncateStr("你好世界测试", 3)
	if got != "你好世..." {
		t.Errorf("expected %q, got %q", "你好世...", got)
	}
}

func TestTruncateStr_ZeroMax(t *testing.T) {
	got := truncateStr("hello", 0)
	// maxLen <= 0 returns original string
	if got != "hello" {
		t.Errorf("expected %q with maxLen=0, got %q", "hello", got)
	}
}

func TestTruncateStr_Empty(t *testing.T) {
	got := truncateStr("", 10)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

// -- SemanticCacheEntry / Key Helpers Tests ----------------------------

func TestSemanticEntryKey(t *testing.T) {
	key := semanticEntryKey("abc123")
	expected := "semantic:entry:abc123"
	if key != expected {
		t.Errorf("expected %q, got %q", expected, key)
	}
}

func TestSemanticIndexKey(t *testing.T) {
	key := semanticIndexKey(42)
	expected := "semantic:index:42"
	if key != expected {
		t.Errorf("expected %q, got %q", expected, key)
	}
}

func TestOutputCacheKey(t *testing.T) {
	key := outputCacheKey("deadbeef")
	expected := "output:deadbeef"
	if key != expected {
		t.Errorf("expected %q, got %q", expected, key)
	}
}

func TestSessionHistoryKey(t *testing.T) {
	key := sessionHistoryKey(100)
	expected := "session:100:history"
	if key != expected {
		t.Errorf("expected %q, got %q", expected, key)
	}
}

func TestSessionStateKey(t *testing.T) {
	key := sessionStateKey(200)
	expected := "session:200:state"
	if key != expected {
		t.Errorf("expected %q, got %q", expected, key)
	}
}

// -- Threshold Constants Tests ----------------------------------------

func TestSemanticSimilarityThreshold(t *testing.T) {
	// Per design.md §8.1.3: threshold should be 0.95
	if semanticSimilarityThreshold != 0.95 {
		t.Errorf("expected threshold 0.95, got %f", semanticSimilarityThreshold)
	}
}

func TestSemanticMaxEntries(t *testing.T) {
	if semanticMaxEntries != 200 {
		t.Errorf("expected max entries 200, got %d", semanticMaxEntries)
	}
}

func TestCacheTTLValues(t *testing.T) {
	// L2 semantic: 2 hours
	if semanticCacheTTL.Hours() != 2 {
		t.Errorf("expected semantic TTL 2h, got %v", semanticCacheTTL)
	}
	// L3 output: 1 hour
	if outputCacheTTL.Hours() != 1 {
		t.Errorf("expected output TTL 1h, got %v", outputCacheTTL)
	}
}
