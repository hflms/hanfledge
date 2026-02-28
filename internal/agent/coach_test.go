package agent

import (
	"testing"
)

// ── estimateTokens Tests ────────────────────────────────────

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		minToken int
		maxToken int
	}{
		{"空字符串", "", 1, 1},           // +1 base
		{"纯英文", "hello world", 3, 5}, // 11 chars / 4 ≈ 2, + 1
		{"纯中文", "你好世界", 2, 4},        // 4 chars / 1.5 ≈ 2, + 1
		{"中英混合", "Hello你好World世界", 4, 8},
		{"长文本", "这是一段较长的中文文本用于测试token估算功能的准确性。", 10, 25},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := estimateTokens(tc.input)
			if result < tc.minToken || result > tc.maxToken {
				t.Errorf("estimateTokens(%q) = %d, want in [%d, %d]",
					tc.input, result, tc.minToken, tc.maxToken)
			}
		})
	}
}

func TestEstimateTokens_PositiveResult(t *testing.T) {
	// 任何输入都应返回 >= 1（base token）
	inputs := []string{"", "a", "你", "Hello World 你好世界"}
	for _, input := range inputs {
		result := estimateTokens(input)
		if result < 1 {
			t.Errorf("estimateTokens(%q) = %d, should be >= 1", input, result)
		}
	}
}

func TestEstimateTokens_ChineseMoreTokensPerChar(t *testing.T) {
	// 相同字节数下，中文应产生更多 token
	chinese := "你好你好你好你好" // 8 中文字
	english := "abcdefgh" // 8 英文字
	chineseTokens := estimateTokens(chinese)
	englishTokens := estimateTokens(english)

	if chineseTokens <= englishTokens {
		t.Errorf("中文 (%d tokens) 应比等长英文 (%d tokens) 有更多 token",
			chineseTokens, englishTokens)
	}
}
