package usecase

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// ── hybridSlice Tests ───────────────────────────────────────

func TestHybridSlice_BasicSplitting(t *testing.T) {
	e := &KARAGEngine{}

	// 构造两段足够长的文本（每段 >= 20 runes）
	para1 := strings.Repeat("光合作用是植物利用光能将二氧化碳和水转化为有机物的过程", 1)
	para2 := strings.Repeat("细胞呼吸是生物体通过分解有机物释放能量的代谢过程", 1)
	text := para1 + "\n\n" + para2

	chunks := e.hybridSlice(text)

	if len(chunks) == 0 {
		t.Fatalf("hybridSlice() returned 0 chunks")
	}

	// 每个 chunk 都应 >= 20 个 rune
	for i, c := range chunks {
		if utf8.RuneCountInString(c) < 20 {
			t.Errorf("chunk[%d] has %d runes, want >= 20: %q",
				i, utf8.RuneCountInString(c), c)
		}
	}
}

func TestHybridSlice_MaxChunkSize(t *testing.T) {
	e := &KARAGEngine{}

	// 创建一个非常长的段落（>500 chars），应被按段分割
	longPara := strings.Repeat("这是一段很长的教材内容用于测试分块功能。", 30) // ~450 chars
	anotherPara := strings.Repeat("另一段教材内容也很长需要独立成块。", 30)
	text := longPara + "\n\n" + anotherPara

	chunks := e.hybridSlice(text)

	if len(chunks) < 2 {
		t.Errorf("hybridSlice() should split into at least 2 chunks for long text, got %d", len(chunks))
	}
}

func TestHybridSlice_FilterShortChunks(t *testing.T) {
	e := &KARAGEngine{}

	// 短段落（< 20 runes）应被过滤
	text := "短文\n\n这是一段足够长的教材内容用于测试短文本过滤功能的正确性。"

	chunks := e.hybridSlice(text)

	for _, c := range chunks {
		if utf8.RuneCountInString(c) < 20 {
			t.Errorf("Short chunk should be filtered: %q (%d runes)",
				c, utf8.RuneCountInString(c))
		}
	}
}

func TestHybridSlice_EmptyInput(t *testing.T) {
	e := &KARAGEngine{}

	chunks := e.hybridSlice("")
	if len(chunks) != 0 {
		t.Errorf("hybridSlice(\"\") returned %d chunks, want 0", len(chunks))
	}
}

func TestHybridSlice_OnlyWhitespace(t *testing.T) {
	e := &KARAGEngine{}

	chunks := e.hybridSlice("   \n\n   \n\n   ")
	if len(chunks) != 0 {
		t.Errorf("hybridSlice(whitespace) returned %d chunks, want 0", len(chunks))
	}
}

func TestHybridSlice_FallbackWhenAllFiltered(t *testing.T) {
	e := &KARAGEngine{}

	// 所有段落都短于 20 runes，应 fallback 返回全部
	text := "第一段短文\n\n第二段短文"

	chunks := e.hybridSlice(text)

	// Fallback: 当过滤后为空但原始有内容时返回全部
	if len(chunks) == 0 {
		t.Errorf("hybridSlice() with only short chunks should fallback, got 0 chunks")
	}
}

func TestHybridSlice_PreserveParagraphBoundary(t *testing.T) {
	e := &KARAGEngine{}

	para1 := "第一章：光合作用是植物的重要代谢过程，通过叶绿体进行。"
	para2 := "第二章：细胞呼吸分为有氧呼吸和无氧呼吸两种基本类型。"
	text := para1 + "\n\n" + para2

	chunks := e.hybridSlice(text)

	// 两段都够长，应该在同一个 chunk 或者分两个 chunk
	if len(chunks) == 0 {
		t.Fatalf("hybridSlice() returned 0 chunks")
	}
}

// ── formatVector Tests ──────────────────────────────────────

func TestFormatVector(t *testing.T) {
	tests := []struct {
		name     string
		input    []float64
		expected string
	}{
		{
			"三维向量",
			[]float64{0.1, 0.2, 0.3},
			"[0.100000,0.200000,0.300000]",
		},
		{
			"空向量",
			[]float64{},
			"[]",
		},
		{
			"单维向量",
			[]float64{1.0},
			"[1.000000]",
		},
		{
			"负值向量",
			[]float64{-0.5, 0.0, 0.5},
			"[-0.500000,0.000000,0.500000]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := formatVector(tc.input)
			if result != tc.expected {
				t.Errorf("formatVector() = %q, want %q", result, tc.expected)
			}
		})
	}
}

func TestFormatVector_PgvectorFormat(t *testing.T) {
	vec := []float64{0.1, 0.2}
	result := formatVector(vec)

	// 应以 [ 开头，] 结尾
	if !strings.HasPrefix(result, "[") || !strings.HasSuffix(result, "]") {
		t.Errorf("formatVector() = %q, should be wrapped in []", result)
	}
}

// ── extractJSON Tests ───────────────────────────────────────

func TestExtractJSON_KARAGEngine(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			"纯JSON",
			`{"chapters": []}`,
			`{"chapters": []}`,
		},
		{
			"Markdown json代码块",
			"```json\n{\"chapters\": []}\n```",
			`{"chapters": []}`,
		},
		{
			"Markdown普通代码块",
			"```\n{\"chapters\": []}\n```",
			`{"chapters": []}`,
		},
		{
			"前后有空白",
			"  \n{\"key\": \"value\"}\n  ",
			`{"key": "value"}`,
		},
		{
			"无代码块标记",
			`{"data": 123}`,
			`{"data": 123}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractJSON(tc.input)
			if result != tc.expected {
				t.Errorf("extractJSON(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}
