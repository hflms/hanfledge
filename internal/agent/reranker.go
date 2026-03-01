package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/hflms/hanfledge/internal/infrastructure/llm"
	"github.com/hflms/hanfledge/internal/infrastructure/logger"
)

var slogRerank = logger.L("Reranker")

// ============================
// Cross-Encoder Reranker (§8.1.1 Stage 2)
// ============================
//
// 职责：对 RRF 粗排后的 Top-N 候选切片执行深度语义重排。
// 将候选切片与原始查询组成 [Query, Chunk] 对，通过 LLM 评分，
// 输出精排后的 Top-K 最相关知识块。
//
// 设计参考 (design.md §8.1.1):
//
//	Stage 2 — 精重排 (Cross-Encoder Rerank)：将候选切片与原始查询
//	组成 [Query, Chunk] 对，送入交叉编码器进行深度语义匹配，输出精排后的
//	Top-5~10 最相关知识块。
//
// 实现方式：LLM-as-judge — 使用现有 LLMProvider.Chat() 接口，
// 对每批 query-chunk 对打分（0-10），避免引入独立的 reranker 服务。
// 评分结果归一化到 [0.0, 1.0] 后替换 RRF 分数。
//
// 批量策略（P3-2 优化）：
//   - 将多个 chunk 合并为一批发送给 LLM，减少 LLM 调用次数
//   - 每批默认 5 个 chunk，LLM 返回 JSON 数组评分
//   - 批量解析失败时，自动降级为逐个评分
//
// 降级策略：
//   - LLM 不可用时：跳过重排，直接返回原始 RRF 排序结果
//   - 批量评分解析失败时：自动降级为逐个评分
//   - 单个 chunk 评分失败时：保留该 chunk 的 RRF 分数（不剔除）
//   - 全部评分失败时：回退到 RRF 排序

// -- Constants --------------------------------------------------------

// defaultRerankTopK is the number of chunks to keep after reranking.
// design.md §8.1.1 specifies Top-5~10.
const defaultRerankTopK = 5

// defaultBatchSize is the number of chunks per batch LLM call.
// Balances between fewer API calls (larger batches) and reliable JSON parsing
// (smaller batches). 5 chunks per batch = 4 calls for 20 chunks instead of 20.
const defaultBatchSize = 5

// rerankScorePrompt is the system prompt template for single-chunk LLM-based relevance scoring.
// The LLM is instructed to output ONLY a numeric score (0-10).
const rerankScorePrompt = `你是一位学术文献相关性评估专家。请评估以下"检索片段"与"学生查询"的语义相关度。

评分标准（0-10 分）：
- 0-2：完全不相关或主题无关
- 3-4：主题相关但未直接回答查询
- 5-6：部分相关，包含有用的背景信息
- 7-8：高度相关，直接涉及查询的核心概念
- 9-10：完美匹配，精确回答查询问题

【学生查询】
%s

【检索片段】
%s

请仅输出一个 0 到 10 之间的整数评分，不要输出任何解释或其他文字。
评分：`

// rerankBatchPrompt is the prompt template for batch scoring multiple chunks at once.
// The LLM returns a JSON array of integer scores corresponding to each chunk.
const rerankBatchPrompt = `你是一位学术文献相关性评估专家。请评估以下多个"检索片段"与"学生查询"的语义相关度。

评分标准（0-10 分）：
- 0-2：完全不相关或主题无关
- 3-4：主题相关但未直接回答查询
- 5-6：部分相关，包含有用的背景信息
- 7-8：高度相关，直接涉及查询的核心概念
- 9-10：完美匹配，精确回答查询问题

【学生查询】
%s

%s

请严格按照 JSON 数组格式输出每个片段的评分（0-10 整数），顺序与上面片段编号一致。
示例输出：[7, 3, 9, 5, 2]
评分：`

// -- CrossEncoderReranker Struct --------------------------------------

// CrossEncoderReranker uses an LLM to re-score retrieved chunks against
// the original query, providing more accurate relevance ranking than
// the initial bi-encoder + RRF fusion.
type CrossEncoderReranker struct {
	llm       llm.LLMProvider
	topK      int
	batchSize int
}

// NewCrossEncoderReranker creates a new reranker with default top-K and batch size.
func NewCrossEncoderReranker(llmClient llm.LLMProvider) *CrossEncoderReranker {
	return &CrossEncoderReranker{
		llm:       llmClient,
		topK:      defaultRerankTopK,
		batchSize: defaultBatchSize,
	}
}

// NewCrossEncoderRerankerWithTopK creates a reranker with custom top-K and default batch size.
func NewCrossEncoderRerankerWithTopK(llmClient llm.LLMProvider, topK int) *CrossEncoderReranker {
	if topK <= 0 {
		topK = defaultRerankTopK
	}
	return &CrossEncoderReranker{
		llm:       llmClient,
		topK:      topK,
		batchSize: defaultBatchSize,
	}
}

// -- Reranking --------------------------------------------------------

// rerankResult holds a chunk with its cross-encoder relevance score.
type rerankResult struct {
	chunk   RetrievedChunk
	ceScore float64 // cross-encoder score [0.0, 1.0]
	scored  bool    // whether the LLM successfully scored this chunk
}

// Rerank scores chunks against the query using the LLM in batches
// and returns the top-K chunks sorted by cross-encoder relevance score.
//
// Batch strategy:
//   - Chunks are grouped into batches of batchSize (default 5)
//   - Each batch is sent as a single LLM call requesting JSON array scores
//   - If batch parsing fails, the batch falls back to individual scoring
//
// Graceful degradation:
//   - If LLM is nil, returns the original chunks unchanged
//   - If all scoring fails, returns the original chunks unchanged
//   - If some scoring fails, unscored chunks are appended after scored ones
//
// The Score field on returned RetrievedChunk is updated to the normalized
// cross-encoder score [0.0, 1.0] for scored chunks.
func (r *CrossEncoderReranker) Rerank(ctx context.Context, query string, chunks []RetrievedChunk) []RetrievedChunk {
	if r.llm == nil || len(chunks) == 0 {
		return chunks
	}

	slogRerank.Info("scoring chunks against query",
		"chunks", len(chunks), "topK", r.topK, "batchSize", r.batchSize)

	results := make([]rerankResult, len(chunks))
	scoredCount := 0
	llmCalls := 0

	// Process in batches
	for batchStart := 0; batchStart < len(chunks); batchStart += r.batchSize {
		batchEnd := batchStart + r.batchSize
		if batchEnd > len(chunks) {
			batchEnd = len(chunks)
		}
		batch := chunks[batchStart:batchEnd]

		// Try batch scoring first
		scores, err := r.scoreBatch(ctx, query, batch)
		if err == nil && len(scores) == len(batch) {
			llmCalls++
			for i, score := range scores {
				results[batchStart+i] = rerankResult{
					chunk:   batch[i],
					ceScore: score,
					scored:  true,
				}
				scoredCount++
			}
			continue
		}

		// Batch failed — fall back to individual scoring for this batch
		if err != nil {
			slogRerank.Warn("batch scoring failed, falling back to individual", "err", err)
		} else {
			slogRerank.Warn("batch returned mismatched scores, falling back",
				"got", len(scores), "expected", len(batch))
		}

		for i, chunk := range batch {
			score, scoreErr := r.scoreChunk(ctx, query, chunk.Content)
			llmCalls++
			if scoreErr != nil {
				slogRerank.Warn("individual scoring failed",
					"chunk", batchStart+i, "chunkIndex", chunk.ChunkIndex, "err", scoreErr)
				results[batchStart+i] = rerankResult{
					chunk:   chunk,
					ceScore: chunk.Score, // preserve RRF score as fallback
					scored:  false,
				}
				continue
			}
			results[batchStart+i] = rerankResult{
				chunk:   chunk,
				ceScore: score,
				scored:  true,
			}
			scoredCount++
		}
	}

	// If no chunks were successfully scored, return original order
	if scoredCount == 0 {
		slogRerank.Warn("all scoring failed, falling back to RRF order")
		return chunks
	}

	// Sort: scored chunks first (by ceScore desc), then unscored (by original RRF score desc)
	sort.SliceStable(results, func(i, j int) bool {
		// Scored chunks always rank above unscored
		if results[i].scored && !results[j].scored {
			return true
		}
		if !results[i].scored && results[j].scored {
			return false
		}
		// Within same category, sort by score descending
		return results[i].ceScore > results[j].ceScore
	})

	// Take top-K
	topK := r.topK
	if topK > len(results) {
		topK = len(results)
	}

	reranked := make([]RetrievedChunk, topK)
	for i := 0; i < topK; i++ {
		reranked[i] = results[i].chunk
		if results[i].scored {
			reranked[i].Score = results[i].ceScore // replace with cross-encoder score
		}
	}

	slogRerank.Debug("reranked results",
		"scored", scoredCount, "total", len(chunks), "topK", topK,
		"llmCalls", llmCalls, "bestScore", reranked[0].Score,
		"worstScore", reranked[topK-1].Score)

	return reranked
}

// -- Batch Scoring ----------------------------------------------------

// scoreBatch sends multiple [query, chunk] pairs to the LLM in a single call.
// Returns normalized scores [0.0, 1.0] for each chunk in the batch.
func (r *CrossEncoderReranker) scoreBatch(ctx context.Context, query string, batch []RetrievedChunk) ([]float64, error) {
	if len(batch) == 1 {
		// Single chunk — use simpler single-scoring prompt for reliability
		score, err := r.scoreChunk(ctx, query, batch[0].Content)
		if err != nil {
			return nil, err
		}
		return []float64{score}, nil
	}

	// Build numbered chunk list
	var sb strings.Builder
	for i, chunk := range batch {
		truncated := truncateContent(chunk.Content, 500)
		fmt.Fprintf(&sb, "【片段 %d】\n%s\n\n", i+1, truncated)
	}

	prompt := fmt.Sprintf(rerankBatchPrompt, query, sb.String())
	messages := []llm.ChatMessage{
		{Role: "user", Content: prompt},
	}

	// Allow more tokens for JSON array output
	maxTokens := len(batch)*8 + 16
	response, err := r.llm.Chat(ctx, messages, &llm.ChatOptions{
		Temperature: 0.0,
		MaxTokens:   maxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM batch chat failed: %w", err)
	}

	scores, err := parseBatchScores(response, len(batch))
	if err != nil {
		return nil, fmt.Errorf("parse batch scores from %q: %w", response, err)
	}

	return scores, nil
}

// -- Single Scoring (fallback) ----------------------------------------

// scoreChunk sends a single [query, chunk] pair to the LLM for relevance scoring.
// Returns a normalized score in [0.0, 1.0].
func (r *CrossEncoderReranker) scoreChunk(ctx context.Context, query, chunkContent string) (float64, error) {
	// Truncate chunk content to prevent excessive token usage
	truncated := truncateContent(chunkContent, 500)

	prompt := fmt.Sprintf(rerankScorePrompt, query, truncated)
	messages := []llm.ChatMessage{
		{Role: "user", Content: prompt},
	}

	response, err := r.llm.Chat(ctx, messages, &llm.ChatOptions{
		Temperature: 0.0, // deterministic scoring
		MaxTokens:   8,   // only need a single number
	})
	if err != nil {
		return 0, fmt.Errorf("LLM chat failed: %w", err)
	}

	// Parse numeric score from response
	score, err := parseScore(response)
	if err != nil {
		return 0, fmt.Errorf("parse score from %q: %w", response, err)
	}

	return score, nil
}

// -- Helpers ----------------------------------------------------------

// truncateContent truncates text to maxChars characters (rune-aware).
func truncateContent(s string, maxChars int) string {
	runes := []rune(s)
	if len(runes) <= maxChars {
		return s
	}
	return string(runes[:maxChars]) + "..."
}

// parseScore extracts a numeric score (0-10) from LLM output and normalizes to [0.0, 1.0].
// Handles common LLM output quirks: leading/trailing whitespace, trailing punctuation,
// "评分：7" format, etc.
func parseScore(response string) (float64, error) {
	s := strings.TrimSpace(response)

	// Handle "评分：7" or "分数：8" format
	for _, prefix := range []string{"评分：", "评分:", "分数：", "分数:", "Score:", "score:"} {
		if strings.HasPrefix(s, prefix) {
			s = strings.TrimSpace(s[len(prefix):])
			break
		}
	}

	// Strip trailing non-numeric characters (periods, "分", etc.)
	s = strings.TrimRight(s, " .分/。")

	// Try to parse as float (handles "7", "7.5", etc.)
	score, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("cannot parse %q as number: %w", s, err)
	}

	// Clamp to [0, 10]
	if score < 0 {
		score = 0
	}
	if score > 10 {
		score = 10
	}

	// Normalize to [0.0, 1.0]
	return score / 10.0, nil
}

// parseBatchScores extracts a JSON array of integer scores from LLM output.
// Expected format: [7, 3, 9, 5, 2]
// Each score is clamped to [0, 10] and normalized to [0.0, 1.0].
// Returns error if the array length doesn't match expected count.
func parseBatchScores(response string, expected int) ([]float64, error) {
	s := strings.TrimSpace(response)

	// Handle "评分：[7, 3, 9]" prefix
	for _, prefix := range []string{"评分：", "评分:", "分数：", "分数:", "Score:", "score:"} {
		if strings.HasPrefix(s, prefix) {
			s = strings.TrimSpace(s[len(prefix):])
			break
		}
	}

	// Find JSON array boundaries — LLM may output extra text before/after
	start := strings.IndexByte(s, '[')
	end := strings.LastIndexByte(s, ']')
	if start < 0 || end < 0 || end <= start {
		return nil, fmt.Errorf("no JSON array found in response")
	}
	s = s[start : end+1]

	// Parse as JSON array of numbers
	var rawScores []json.Number
	if err := json.Unmarshal([]byte(s), &rawScores); err != nil {
		// Try float64 array directly
		var floatScores []float64
		if err2 := json.Unmarshal([]byte(s), &floatScores); err2 != nil {
			return nil, fmt.Errorf("cannot parse JSON array: %w", err)
		}
		// Validate length
		if len(floatScores) != expected {
			return nil, fmt.Errorf("expected %d scores, got %d", expected, len(floatScores))
		}
		// Normalize
		result := make([]float64, len(floatScores))
		for i, score := range floatScores {
			result[i] = clampAndNormalize(score)
		}
		return result, nil
	}

	if len(rawScores) != expected {
		return nil, fmt.Errorf("expected %d scores, got %d", expected, len(rawScores))
	}

	result := make([]float64, len(rawScores))
	for i, raw := range rawScores {
		score, err := raw.Float64()
		if err != nil {
			return nil, fmt.Errorf("invalid score at index %d: %w", i, err)
		}
		result[i] = clampAndNormalize(score)
	}

	return result, nil
}

// clampAndNormalize clamps a raw score to [0, 10] and normalizes to [0.0, 1.0].
func clampAndNormalize(score float64) float64 {
	if score < 0 {
		score = 0
	}
	if score > 10 {
		score = 10
	}
	return score / 10.0
}
