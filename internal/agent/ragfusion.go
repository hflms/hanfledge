package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hflms/hanfledge/internal/infrastructure/llm"
	"github.com/hflms/hanfledge/internal/infrastructure/logger"
)

var slogFusion = logger.L("RAGFusion")

// ============================
// RAG-Fusion Query Expansion (§8.1.2)
// ============================
//
// 职责：将学生的原始查询扩展为 3-5 个变体查询，从不同学术视角重写，
// 提升多路检索的召回率。每个变体独立检索后通过 RRF 合并。
//
// Example:
//   "为啥抛物线变了" →
//     1. "二次函数图像平移变换的数学原理"
//     2. "抛物线顶点式参数 a, h, k 对图形的影响"
//     3. "二次函数 y=a(x-h)²+k 的图像变化规律"

// -- Constants --------------------------------------------------------

// defaultVariantCount is the number of query variants the expander generates.
const defaultVariantCount = 3

// queryExpansionPrompt is the system prompt template for LLM-based query expansion.
// The LLM is instructed to output numbered variants, one per line.
const queryExpansionPrompt = `你是一位学科知识检索专家。你的任务是将学生提出的口语化问题改写为 %d 个不同视角的学术化检索查询。

要求：
1. 每个查询独占一行，以数字编号开头（如 "1. ..."）
2. 从不同的学术视角或知识维度改写，提升检索覆盖面
3. 使用精确的学术术语替换口语化表达
4. 保持与原始问题相同的学科领域
5. 不要输出任何额外说明，只输出编号查询列表

学生原始问题：%s`

// -- QueryExpander Struct ---------------------------------------------

// QueryExpander 使用 LLM 将学生查询扩展为多个学术化变体。
type QueryExpander struct {
	llm          llm.LLMProvider
	variantCount int
}

// NewQueryExpander 创建查询扩展器。
func NewQueryExpander(llmClient llm.LLMProvider) *QueryExpander {
	return &QueryExpander{
		llm:          llmClient,
		variantCount: defaultVariantCount,
	}
}

// ExpandQuery 将原始查询扩展为多个变体查询。
// 返回的切片总是包含原始查询作为第一个元素，后续为 LLM 生成的变体。
// 如果 LLM 调用失败，仍返回仅含原始查询的切片（graceful degradation）。
func (e *QueryExpander) ExpandQuery(ctx context.Context, originalQuery string) []string {
	// Always include the original query as the first variant
	queries := []string{originalQuery}

	if e.llm == nil {
		return queries
	}

	prompt := fmt.Sprintf(queryExpansionPrompt, e.variantCount, originalQuery)
	messages := []llm.ChatMessage{
		{Role: "user", Content: prompt},
	}

	slogFusion.Info("[DEBUG] calling LLM for query expansion", "provider", e.llm.Name(), "query", originalQuery)
	// 使用独立的短超时，防止阻塞整个管道 (main context 只有 3 分钟)
	// qwen3.5-plus 等推理模型的 thinking 阶段需要更多时间
	expCtx, expCancel := context.WithTimeout(ctx, 30*time.Second)
	defer expCancel()
	response, err := e.llm.Chat(expCtx, messages, &llm.ChatOptions{
		Temperature: 0.7,
		MaxTokens:   512,
	})
	if err != nil {
		slogFusion.Warn("query expansion LLM call failed (15s timeout), using original query only", "err", err, "provider", e.llm.Name())
		return queries
	}
	slogFusion.Info("[DEBUG] query expansion LLM call succeeded", "response_len", len(response))

	// Parse numbered lines from response
	variants := parseNumberedLines(response)
	if len(variants) == 0 {
		slogFusion.Warn("no variants parsed from LLM response")
		return queries
	}

	queries = append(queries, variants...)
	slogFusion.Info("expanded query",
		"total", len(queries), "generated", len(variants))

	return queries
}

// -- Helpers ----------------------------------------------------------

// parseNumberedLines extracts content from numbered lines in LLM output.
// Expected format: "1. query text\n2. query text\n..."
// Lines without a number prefix are skipped.
func parseNumberedLines(text string) []string {
	var results []string
	lines := strings.Split(strings.TrimSpace(text), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Strip numbered prefix: "1. ", "2. ", "1、", "2、", etc.
		cleaned := stripNumberPrefix(line)
		if cleaned != "" {
			results = append(results, cleaned)
		}
	}

	return results
}

// stripNumberPrefix removes leading number + delimiter from a line.
// Supports formats: "1. text", "1、text", "1) text", "1: text".
func stripNumberPrefix(line string) string {
	// Find first digit
	if len(line) == 0 || line[0] < '0' || line[0] > '9' {
		return ""
	}

	// Skip digits
	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	if i >= len(line) {
		return ""
	}

	// Skip delimiter: ".", "、", ")", ":"
	rest := line[i:]
	for _, delim := range []string{".", "、", ")", ":", "．"} {
		if strings.HasPrefix(rest, delim) {
			return strings.TrimSpace(rest[len(delim):])
		}
	}

	return ""
}
