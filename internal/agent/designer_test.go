package agent

import (
	"testing"
)

// ── rrfMerge Tests ──────────────────────────────────────────

func TestRRFMerge_BasicMerge(t *testing.T) {
	semantic := []RetrievedChunk{
		{Content: "语义结果1", Source: "semantic", Score: 0.9, ChunkIndex: 1},
		{Content: "语义结果2", Source: "semantic", Score: 0.8, ChunkIndex: 2},
		{Content: "语义结果3", Source: "semantic", Score: 0.7, ChunkIndex: 3},
	}
	graph := []RetrievedChunk{
		{Content: "图谱结果1", Source: "graph", Score: 0.85, ChunkIndex: 4},
		{Content: "图谱结果2", Source: "graph", Score: 0.75, ChunkIndex: 5},
	}

	result := rrfMerge(semantic, graph, 10)

	if len(result) != 5 {
		t.Errorf("rrfMerge() returned %d results, want 5", len(result))
	}
}

func TestRRFMerge_Deduplication(t *testing.T) {
	// 相同 ChunkIndex 出现在两个列表中应被去重
	semantic := []RetrievedChunk{
		{Content: "共享片段", Source: "semantic", Score: 0.9, ChunkIndex: 1},
		{Content: "仅语义", Source: "semantic", Score: 0.8, ChunkIndex: 2},
	}
	graph := []RetrievedChunk{
		{Content: "共享片段", Source: "graph", Score: 0.85, ChunkIndex: 1}, // 与 semantic 重复
		{Content: "仅图谱", Source: "graph", Score: 0.75, ChunkIndex: 3},
	}

	result := rrfMerge(semantic, graph, 10)

	// ChunkIndex=1 出现在两个列表中，应去重为 1 个
	if len(result) != 3 {
		t.Errorf("rrfMerge() returned %d results, want 3 (dedup)", len(result))
	}

	// 重复的 chunk 应标记为 "hybrid"
	for _, r := range result {
		if r.ChunkIndex == 1 && r.Source != "hybrid" {
			t.Errorf("ChunkIndex=1 should be source='hybrid', got %q", r.Source)
		}
	}
}

func TestRRFMerge_HybridScoreHigher(t *testing.T) {
	// 同时出现在两个列表中的文档应有更高的 RRF 分数
	semantic := []RetrievedChunk{
		{Content: "共享", Source: "semantic", Score: 0.9, ChunkIndex: 1},
		{Content: "仅语义", Source: "semantic", Score: 0.95, ChunkIndex: 2},
	}
	graph := []RetrievedChunk{
		{Content: "共享", Source: "graph", Score: 0.85, ChunkIndex: 1},
		{Content: "仅图谱", Source: "graph", Score: 0.9, ChunkIndex: 3},
	}

	result := rrfMerge(semantic, graph, 10)

	// ChunkIndex=1 (hybrid) 应排在第一位，因为 RRF 分数最高
	if len(result) == 0 {
		t.Fatalf("rrfMerge() returned empty result")
	}
	if result[0].ChunkIndex != 1 {
		t.Errorf("Expected hybrid chunk (index=1) to rank first, got index=%d", result[0].ChunkIndex)
	}
}

func TestRRFMerge_TopN_Truncation(t *testing.T) {
	semantic := make([]RetrievedChunk, 10)
	for i := 0; i < 10; i++ {
		semantic[i] = RetrievedChunk{Content: "chunk", Source: "semantic", ChunkIndex: i}
	}

	result := rrfMerge(semantic, nil, 3)
	if len(result) != 3 {
		t.Errorf("rrfMerge(topN=3) returned %d, want 3", len(result))
	}
}

func TestRRFMerge_TopN_LargerThanInput(t *testing.T) {
	semantic := []RetrievedChunk{
		{Content: "a", Source: "semantic", ChunkIndex: 1},
	}

	result := rrfMerge(semantic, nil, 100)
	if len(result) != 1 {
		t.Errorf("rrfMerge(topN=100, input=1) returned %d, want 1", len(result))
	}
}

func TestRRFMerge_EmptyInputs(t *testing.T) {
	result := rrfMerge(nil, nil, 10)
	if len(result) != 0 {
		t.Errorf("rrfMerge(nil, nil) returned %d, want 0", len(result))
	}
}

func TestRRFMerge_SortedByScore(t *testing.T) {
	semantic := []RetrievedChunk{
		{Content: "s1", Source: "semantic", ChunkIndex: 1},
		{Content: "s2", Source: "semantic", ChunkIndex: 2},
		{Content: "s3", Source: "semantic", ChunkIndex: 3},
	}

	result := rrfMerge(semantic, nil, 10)

	// 排名靠前的 chunk 应有更高的 RRF 分数
	for i := 0; i < len(result)-1; i++ {
		if result[i].Score < result[i+1].Score {
			t.Errorf("Results not sorted by RRF score descending: index %d (%.6f) < index %d (%.6f)",
				i, result[i].Score, i+1, result[i+1].Score)
		}
	}
}
