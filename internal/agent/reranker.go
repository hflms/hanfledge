package agent

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"

	"github.com/hflms/hanfledge/internal/infrastructure/llm"
)

// ============================
// Cross-Encoder Reranker (§8.1.1 Stage 2)
// ============================
//
// 职责：对 RRF 粗排后的 Top-N 候选切片执行深度语义重排。
// 将每个候选切片与原始查询组成 [Query, Chunk] 对，通过 LLM 评分，
// 输出精排后的 Top-K 最相关知识块。
//
// 设计参考 (design.md §8.1.1):
//   Stage 2 — 精重排 (Cross-Encoder Rerank)：将候选切片逐一与原始查询
//   组成 [Query, Chunk] 对，送入交叉编码器进行深度语义匹配，输出精排后的
//   Top-5~10 最相关知识块。
//
// 实现方式：LLM-as-judge — 使用现有 LLMProvider.Chat() 接口，
// 让 LLM 对每个 query-chunk 对打分（0-10），避免引入独立的 reranker 服务。
// 评分结果归一化到 [0.0, 1.0] 后替换 RRF 分数。
//
// 降级策略：
//   - LLM 不可用时：跳过重排，直接返回原始 RRF 排序结果
//   - 单个 chunk 评分失败时：保留该 chunk 的 RRF 分数（不剔除）
//   - 全部评分失败时：回退到 RRF 排序

// -- Constants --------------------------------------------------------

// defaultRerankTopK is the number of chunks to keep after reranking.
// design.md §8.1.1 specifies Top-5~10.
const defaultRerankTopK = 5

// rerankScorePrompt is the system prompt template for LLM-based relevance scoring.
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

// -- CrossEncoderReranker Struct --------------------------------------

// CrossEncoderReranker uses an LLM to re-score retrieved chunks against
// the original query, providing more accurate relevance ranking than
// the initial bi-encoder + RRF fusion.
type CrossEncoderReranker struct {
	llm  llm.LLMProvider
	topK int
}

// NewCrossEncoderReranker creates a new reranker with default top-K.
func NewCrossEncoderReranker(llmClient llm.LLMProvider) *CrossEncoderReranker {
	return &CrossEncoderReranker{
		llm:  llmClient,
		topK: defaultRerankTopK,
	}
}

// NewCrossEncoderRerankerWithTopK creates a reranker with custom top-K.
func NewCrossEncoderRerankerWithTopK(llmClient llm.LLMProvider, topK int) *CrossEncoderReranker {
	if topK <= 0 {
		topK = defaultRerankTopK
	}
	return &CrossEncoderReranker{
		llm:  llmClient,
		topK: topK,
	}
}

// -- Reranking --------------------------------------------------------

// rerankResult holds a chunk with its cross-encoder relevance score.
type rerankResult struct {
	chunk   RetrievedChunk
	ceScore float64 // cross-encoder score [0.0, 1.0]
	scored  bool    // whether the LLM successfully scored this chunk
}

// Rerank scores each chunk against the query using the LLM and returns
// the top-K chunks sorted by cross-encoder relevance score.
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

	log.Printf("🔬 [Reranker] Scoring %d chunks against query (topK=%d)", len(chunks), r.topK)

	// Score each chunk
	results := make([]rerankResult, len(chunks))
	scoredCount := 0

	for i, chunk := range chunks {
		score, err := r.scoreChunk(ctx, query, chunk.Content)
		if err != nil {
			log.Printf("⚠️  [Reranker] Scoring failed for chunk %d (index=%d): %v",
				i, chunk.ChunkIndex, err)
			results[i] = rerankResult{
				chunk:   chunk,
				ceScore: chunk.Score, // preserve RRF score as fallback
				scored:  false,
			}
			continue
		}
		results[i] = rerankResult{
			chunk:   chunk,
			ceScore: score,
			scored:  true,
		}
		scoredCount++
	}

	// If no chunks were successfully scored, return original order
	if scoredCount == 0 {
		log.Printf("⚠️  [Reranker] All scoring failed — falling back to RRF order")
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

	log.Printf("   → Reranked: %d/%d scored, top-%d selected (best=%.2f, worst=%.2f)",
		scoredCount, len(chunks), topK, reranked[0].Score,
		reranked[topK-1].Score)

	return reranked
}

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
