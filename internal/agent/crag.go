package agent

import (
	"log"
)

// ============================
// CRAG Quality Gateway (§8.1.2)
// ============================
//
// 职责：在 RRF 融合排序之后，评估检索结果与查询的相关性质量。
// 如果平均相关性低于阈值，标记为低质量并触发回退策略。
//
// CRAG (Corrective RAG) 的核心思想：
//   检索结果不够好时，不应该强行用低质量上下文生成回答，
//   而是应该有"纠正"机制（如改用 Web 搜索、提示 LLM 仅用内在知识等）。

// -- Constants --------------------------------------------------------

// defaultRelevanceThreshold is the minimum average relevance score for chunks
// to be considered "high quality" retrieval. Below this threshold, the
// gateway flags the result for fallback handling.
//
// When Cross-Encoder reranking is active (§8.1.1 Stage 2), chunk scores are
// normalized to [0.0, 1.0] from LLM-based 0-10 ratings. A threshold of 0.4
// corresponds to a 4/10 average rating, meaning most chunks are at least
// "partially relevant with useful background information".
//
// When Cross-Encoder is disabled (fallback to RRF scores), RRF scores are
// typically in [0.01, 0.03] — both will fall below this threshold, triggering
// the CRAG fallback caveat. This is acceptable since without reranking the
// quality signal is weaker.
const defaultRelevanceThreshold = 0.4

// -- QualityGateway Struct --------------------------------------------

// QualityGateway evaluates the quality of retrieved chunks after RRF merge.
// If the average relevance score is below the threshold, it flags the result
// for fallback handling.
type QualityGateway struct {
	threshold float64
}

// NewQualityGateway 创建质量网关。
func NewQualityGateway() *QualityGateway {
	return &QualityGateway{
		threshold: defaultRelevanceThreshold,
	}
}

// NewQualityGatewayWithThreshold 创建自定义阈值的质量网关。
func NewQualityGatewayWithThreshold(threshold float64) *QualityGateway {
	return &QualityGateway{
		threshold: threshold,
	}
}

// -- Quality Evaluation -----------------------------------------------

// RelevanceResult holds the quality evaluation outcome.
type RelevanceResult struct {
	// AvgScore is the average RRF score of the chunks.
	AvgScore float64
	// Passed indicates whether the chunks meet the quality threshold.
	Passed bool
	// ChunkCount is the number of chunks evaluated.
	ChunkCount int
}

// EvaluateRelevance computes the average RRF score of the merged chunks
// and checks whether it exceeds the quality threshold.
//
// Returns:
//   - RelevanceResult with evaluation details
//
// If chunks is empty, it returns Passed=false (no context available).
func (g *QualityGateway) EvaluateRelevance(chunks []RetrievedChunk, query string) RelevanceResult {
	if len(chunks) == 0 {
		log.Printf("⚠️  [CRAG] No chunks to evaluate — flagging as low quality")
		return RelevanceResult{
			AvgScore:   0,
			Passed:     false,
			ChunkCount: 0,
		}
	}

	// Compute average RRF score
	var totalScore float64
	for _, c := range chunks {
		totalScore += c.Score
	}
	avgScore := totalScore / float64(len(chunks))

	passed := avgScore >= g.threshold

	if passed {
		log.Printf("✅ [CRAG] Quality check passed: avg_score=%.4f (threshold=%.4f, chunks=%d)",
			avgScore, g.threshold, len(chunks))
	} else {
		log.Printf("⚠️  [CRAG] Quality check FAILED: avg_score=%.4f < threshold=%.4f (chunks=%d) — query=%q",
			avgScore, g.threshold, len(chunks), truncateForLog(query, 80))
	}

	return RelevanceResult{
		AvgScore:   avgScore,
		Passed:     passed,
		ChunkCount: len(chunks),
	}
}

// -- Fallback --------------------------------------------------------

// HandleFallback is called when the quality check fails.
// Currently it logs a warning and appends a caveat to the system prompt.
// Future: integrate web search (Dynamic Connector) as described in §8.1.2.
func (g *QualityGateway) HandleFallback(systemPrompt string) string {
	caveat := "\n\n【注意：检索到的参考材料与学生问题的相关度较低。" +
		"请主要依靠你自身的知识储备来回答，" +
		"同时提醒学生该问题可能超出当前课程材料的覆盖范围。】\n"

	log.Printf("🔄 [CRAG] Fallback activated — appending low-confidence caveat to prompt")
	return systemPrompt + caveat
}

// -- Helpers ----------------------------------------------------------

// truncateForLog truncates a string for log output.
func truncateForLog(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
