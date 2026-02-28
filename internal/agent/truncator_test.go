package agent

import (
	"testing"
)

// ============================
// Token Truncator Tests (§8.2.2)
// ============================

func TestDefaultTokenTruncator(t *testing.T) {
	tr := DefaultTokenTruncator()
	if tr.MaxOutputTokens != 1024 {
		t.Errorf("MaxOutputTokens = %d, want 1024", tr.MaxOutputTokens)
	}
	if tr.MaxChunks != 5 {
		t.Errorf("MaxChunks = %d, want 5", tr.MaxChunks)
	}
	if tr.PageSize != 5 {
		t.Errorf("PageSize = %d, want 5", tr.PageSize)
	}
}

func TestTruncateChunks_NoTruncation(t *testing.T) {
	tr := DefaultTokenTruncator()
	chunks := []RetrievedChunk{
		{Content: "短内容1", Source: "semantic", Score: 0.9},
		{Content: "短内容2", Source: "graph", Score: 0.8},
	}

	result := tr.TruncateChunks(chunks)
	if result.Truncated {
		t.Error("expected no truncation for small input")
	}
	if len(result.Data) != 2 {
		t.Errorf("len(Data) = %d, want 2", len(result.Data))
	}
	if result.TotalPages != 1 {
		t.Errorf("TotalPages = %d, want 1", result.TotalPages)
	}
}

func TestTruncateChunks_Empty(t *testing.T) {
	tr := DefaultTokenTruncator()
	result := tr.TruncateChunks(nil)
	if result.Truncated {
		t.Error("empty input should not be truncated")
	}
	if len(result.Data) != 0 {
		t.Errorf("len(Data) = %d, want 0", len(result.Data))
	}
}

func TestTruncateChunks_ExceedsMaxChunks(t *testing.T) {
	tr := &TokenTruncator{
		MaxOutputTokens: 100000, // 很高的 token 限制
		MaxChunks:       3,
		PageSize:        3,
	}

	chunks := make([]RetrievedChunk, 10)
	for i := range chunks {
		chunks[i] = RetrievedChunk{Content: "短", Source: "semantic", Score: float64(10-i) / 10}
	}

	result := tr.TruncateChunks(chunks)
	if !result.Truncated {
		t.Error("expected truncation when exceeding MaxChunks")
	}
	if len(result.Data) > 3 {
		t.Errorf("len(Data) = %d, want <= 3", len(result.Data))
	}
	if result.TotalItems != 10 {
		t.Errorf("TotalItems = %d, want 10", result.TotalItems)
	}
}

func TestTruncateChunks_ExceedsMaxTokens(t *testing.T) {
	tr := &TokenTruncator{
		MaxOutputTokens: 10, // 非常低的 token 限制
		MaxChunks:       100,
		PageSize:        5,
	}

	chunks := []RetrievedChunk{
		{Content: "这是一段很长的中文内容，包含许多知识点和详细说明，远远超过了Token限制", Source: "semantic", Score: 0.9},
		{Content: "第二段同样很长的内容，包含更多的细节和解释说明", Source: "graph", Score: 0.8},
		{Content: "第三段内容", Source: "semantic", Score: 0.7},
	}

	result := tr.TruncateChunks(chunks)
	if !result.Truncated {
		t.Error("expected truncation when exceeding MaxOutputTokens")
	}
	if result.Message == "" {
		t.Error("truncated result should have a message")
	}
}

func TestTruncateChunks_Pagination(t *testing.T) {
	tr := &TokenTruncator{
		MaxOutputTokens: 100000,
		MaxChunks:       2,
		PageSize:        2,
	}

	chunks := make([]RetrievedChunk, 7)
	for i := range chunks {
		chunks[i] = RetrievedChunk{Content: "短", Source: "semantic"}
	}

	result := tr.TruncateChunks(chunks)
	if result.TotalPages != 4 { // ceil(7/2) = 4
		t.Errorf("TotalPages = %d, want 4", result.TotalPages)
	}
	if result.Page != 1 {
		t.Errorf("Page = %d, want 1", result.Page)
	}
}

func TestTruncateSystemPrompt_NoTruncation(t *testing.T) {
	tr := DefaultTokenTruncator()
	prompt := "短内容"
	result, truncated := tr.TruncateSystemPrompt(prompt, 1000)
	if truncated {
		t.Error("short prompt should not be truncated")
	}
	if result != prompt {
		t.Errorf("untruncated prompt should be unchanged")
	}
}

func TestTruncateSystemPrompt_Truncated(t *testing.T) {
	tr := DefaultTokenTruncator()
	// 创建一个很长的 prompt
	longPrompt := ""
	for i := 0; i < 500; i++ {
		longPrompt += "这是一段很长的系统提示内容，用于测试截断功能。"
	}

	result, truncated := tr.TruncateSystemPrompt(longPrompt, 100)
	if !truncated {
		t.Error("long prompt should be truncated")
	}
	if len(result) >= len(longPrompt) {
		t.Error("truncated result should be shorter than original")
	}
	// 检查截断后缀
	if !contains(result, "上下文已截断") {
		t.Error("truncated prompt should contain truncation notice")
	}
}

func TestTruncateToTokens(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		maxTokens int
		wantShort bool
	}{
		{"short text", "hello", 100, false},
		{"chinese truncate", "这是一段测试文本用于验证截断功能是否正常工作", 5, true},
		{"english truncate", "This is a test string for token truncation", 3, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateToTokens(tt.text, tt.maxTokens)
			if tt.wantShort && len(result) >= len(tt.text) {
				t.Errorf("expected truncation for %q with max %d tokens", tt.text, tt.maxTokens)
			}
			if !tt.wantShort && result != tt.text {
				t.Errorf("expected no truncation, got %q", result)
			}
		})
	}
}

func TestTruncateChunks_ZeroConfig(t *testing.T) {
	tr := &TokenTruncator{} // 全部使用默认值
	chunks := []RetrievedChunk{{Content: "test"}}
	result := tr.TruncateChunks(chunks)
	if result.Truncated {
		t.Error("single chunk should not be truncated with defaults")
	}
}

// -- CoT Reasoning Strip Tests (§8.2.3) ---

func TestStripReasoningBlock_NoBlock(t *testing.T) {
	input := "这是一段普通回复，没有推理块。"
	clean, reasoning := stripReasoningBlock(input)
	if clean != input {
		t.Errorf("clean = %q, want %q", clean, input)
	}
	if reasoning != "" {
		t.Errorf("reasoning should be empty, got %q", reasoning)
	}
}

func TestStripReasoningBlock_WithBlock(t *testing.T) {
	input := `<reasoning>
学生的核心问题是理解牛顿第二定律。
我应该使用苏格拉底式引导来启发思考。
</reasoning>

好的，让我们一起来思考一下力和加速度的关系。如果一辆车的质量增加了一倍，在相同的力作用下，加速度会怎样变化呢？`

	clean, reasoning := stripReasoningBlock(input)

	if contains(clean, "<reasoning>") || contains(clean, "</reasoning>") {
		t.Error("clean output should not contain reasoning tags")
	}
	if reasoning == "" {
		t.Error("reasoning should not be empty")
	}
	if !contains(reasoning, "牛顿第二定律") {
		t.Error("reasoning should contain the thinking content")
	}
	if !contains(clean, "加速度") {
		t.Error("clean output should contain the student-facing response")
	}
}

func TestStripReasoningBlock_MultipleBlocks(t *testing.T) {
	input := `<reasoning>第一个推理</reasoning>
回复1
<reasoning>第二个推理</reasoning>
回复2`

	clean, reasoning := stripReasoningBlock(input)

	// regex 匹配第一个（非贪婪）
	if reasoning == "" {
		t.Error("should extract reasoning")
	}
	if contains(clean, "<reasoning>") {
		t.Error("all reasoning blocks should be stripped")
	}
}

func TestStripReasoningBlock_EmptyBlock(t *testing.T) {
	input := `<reasoning>
</reasoning>
正式回复内容`

	clean, _ := stripReasoningBlock(input)
	if !contains(clean, "正式回复内容") {
		t.Error("clean should contain the reply")
	}
}

// -- Helper --

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
