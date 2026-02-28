package agent

import (
	"fmt"
	"log"
)

// ============================
// Token Truncation Middleware
// ============================
//
// Reference: design.md §8.2.2 — Token Efficiency & Pagination
//
// 大模型存在严重的"Lost in the Middle"问题——当上下文窗口中间塞入过多信息时，
// 模型倾向于忽略中间部分。TokenTruncator 拦截过长的技能输出并强制分页。

// TruncatedOutput 截断后的输出，包含分页元信息。
type TruncatedOutput struct {
	Data       []RetrievedChunk `json:"data"`
	Truncated  bool             `json:"truncated"`
	TotalItems int              `json:"total_items"`
	TotalPages int              `json:"total_pages"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
	Message    string           `json:"message,omitempty"`
}

// TokenTruncator 中间件：拦截过长的技能/检索输出并强制分页。
// 当检索到的材料过多时，截断并附加分页元信息，
// 引导 Agent 采用"多次小范围查询"策略。
type TokenTruncator struct {
	MaxOutputTokens int // 单次输出的最大 Token 数（默认 1024）
	MaxChunks       int // 单次输出的最大片段数（默认 5）
	PageSize        int // 每页片段数（默认 5）
}

// DefaultTokenTruncator 返回使用默认配置的 TokenTruncator。
func DefaultTokenTruncator() *TokenTruncator {
	return &TokenTruncator{
		MaxOutputTokens: 1024,
		MaxChunks:       5,
		PageSize:        5,
	}
}

// TruncateChunks 检查检索结果是否过长，必要时截断并返回分页元信息。
// 截断策略：
//  1. 如果片段数超过 MaxChunks，截断到 MaxChunks
//  2. 如果所有片段的总 Token 数超过 MaxOutputTokens，逐个裁剪
//  3. 返回 TruncatedOutput，包含分页信息供 Agent 使用
func (t *TokenTruncator) TruncateChunks(chunks []RetrievedChunk) TruncatedOutput {
	if len(chunks) == 0 {
		return TruncatedOutput{
			Data:       chunks,
			Truncated:  false,
			TotalItems: 0,
			TotalPages: 1,
			Page:       1,
			PageSize:   t.pageSize(),
		}
	}

	totalTokens := 0
	for _, c := range chunks {
		totalTokens += estimateTokens(c.Content)
	}

	// Case 1: 片段数和 Token 数都在限制内 → 无需截断
	if len(chunks) <= t.maxChunks() && totalTokens <= t.maxOutputTokens() {
		return TruncatedOutput{
			Data:       chunks,
			Truncated:  false,
			TotalItems: len(chunks),
			TotalPages: 1,
			Page:       1,
			PageSize:   t.pageSize(),
		}
	}

	// Case 2: 需要截断
	truncated := make([]RetrievedChunk, 0, t.maxChunks())
	accTokens := 0

	for i, chunk := range chunks {
		if i >= t.maxChunks() {
			break
		}

		chunkTokens := estimateTokens(chunk.Content)

		// 如果加上这个片段会超出 Token 限制
		if accTokens+chunkTokens > t.maxOutputTokens() {
			// 尝试截断这个片段的内容
			remaining := t.maxOutputTokens() - accTokens
			if remaining > 50 { // 至少保留 50 tokens 才值得截断
				chunk.Content = truncateToTokens(chunk.Content, remaining)
				truncated = append(truncated, chunk)
			}
			break
		}

		truncated = append(truncated, chunk)
		accTokens += chunkTokens
	}

	totalPages := (len(chunks) + t.pageSize() - 1) / t.pageSize()

	log.Printf("✂️  [Truncator] Truncated: %d→%d chunks, %d→%d tokens, %d pages",
		len(chunks), len(truncated), totalTokens, accTokens, totalPages)

	return TruncatedOutput{
		Data:       truncated,
		Truncated:  true,
		TotalItems: len(chunks),
		TotalPages: totalPages,
		Page:       1,
		PageSize:   t.pageSize(),
		Message:    fmt.Sprintf("结果已截断。共 %d 个片段，当前显示前 %d 个。", len(chunks), len(truncated)),
	}
}

// TruncateSystemPrompt 截断系统 Prompt 到指定 Token 上限。
// 用于防止过长的上下文材料污染 LLM 注意力窗口。
func (t *TokenTruncator) TruncateSystemPrompt(prompt string, maxTokens int) (string, bool) {
	tokens := estimateTokens(prompt)
	if tokens <= maxTokens {
		return prompt, false
	}

	truncated := truncateToTokens(prompt, maxTokens)
	truncated += "\n\n[...上下文已截断，请基于已提供的材料回答...]"

	log.Printf("✂️  [Truncator] System prompt truncated: %d→%d tokens", tokens, maxTokens)
	return truncated, true
}

// -- Internal helpers ------------------------------------------------

func (t *TokenTruncator) maxOutputTokens() int {
	if t.MaxOutputTokens <= 0 {
		return 1024
	}
	return t.MaxOutputTokens
}

func (t *TokenTruncator) maxChunks() int {
	if t.MaxChunks <= 0 {
		return 5
	}
	return t.MaxChunks
}

func (t *TokenTruncator) pageSize() int {
	if t.PageSize <= 0 {
		return 5
	}
	return t.PageSize
}

// truncateToTokens 将文本截断到指定的 Token 数。
// 使用 rune 级别的粗略估算（中文 ~1.5 char/token, 英文 ~4 char/token）。
func truncateToTokens(text string, maxTokens int) string {
	runes := []rune(text)
	accTokens := 0

	for i, r := range runes {
		if r > 127 {
			// 中文字符约 1.5 char/token → 每个字 0.67 token
			accTokens += 67 // 乘以 100 避免浮点
		} else {
			accTokens += 25 // 英文 4 char/token → 每个字 0.25 token
		}
		// 转换回实际 token 数进行比较
		if accTokens/100 >= maxTokens {
			return string(runes[:i])
		}
	}

	return text
}
