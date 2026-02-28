package agent

import (
	"testing"

	"github.com/hflms/hanfledge/internal/domain/model"
)

// ── truncate Tests ──────────────────────────────────────────

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{"短于上限", "hello", 10, "hello"},
		{"等于上限", "hello", 5, "hello"},
		{"超过上限", "hello world", 5, "hello..."},
		{"中文截断", "你好世界测试", 4, "你好世界..."},
		{"空字符串", "", 5, ""},
		{"上限为0", "hello", 0, "..."},
		{"上限为1", "hello", 1, "h..."},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := truncate(tc.input, tc.maxLen)
			if result != tc.expected {
				t.Errorf("truncate(%q, %d) = %q, want %q",
					tc.input, tc.maxLen, result, tc.expected)
			}
		})
	}
}

// ── scaffoldDirection Tests ─────────────────────────────────

func TestScaffoldDirection(t *testing.T) {
	tests := []struct {
		name     string
		old      model.ScaffoldLevel
		new_     model.ScaffoldLevel
		expected string
	}{
		{"high→medium衰减", ScaffoldHigh, ScaffoldMedium, "fade"},
		{"high→low衰减", ScaffoldHigh, ScaffoldLow, "fade"},
		{"medium→low衰减", ScaffoldMedium, ScaffoldLow, "fade"},
		{"low→medium增强", ScaffoldLow, ScaffoldMedium, "strengthen"},
		{"low→high增强", ScaffoldLow, ScaffoldHigh, "strengthen"},
		{"medium→high增强", ScaffoldMedium, ScaffoldHigh, "strengthen"},
		{"same→same增强", ScaffoldHigh, ScaffoldHigh, "strengthen"}, // 相同视为 strengthen
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := scaffoldDirection(tc.old, tc.new_)
			if result != tc.expected {
				t.Errorf("scaffoldDirection(%q, %q) = %q, want %q",
					tc.old, tc.new_, result, tc.expected)
			}
		})
	}
}

// ── inferCorrectness Tests ──────────────────────────────────

func TestInferCorrectness(t *testing.T) {
	o := &AgentOrchestrator{}

	tests := []struct {
		name     string
		tc       *TurnContext
		expected bool
	}{
		{
			"有审查结果且深度分数高",
			&TurnContext{
				Review: &ReviewResult{DepthScore: 0.8},
			},
			true,
		},
		{
			"有审查结果且深度分数刚好0.6",
			&TurnContext{
				Review: &ReviewResult{DepthScore: 0.6},
			},
			true,
		},
		{
			"有审查结果但深度分数低",
			&TurnContext{
				Review: &ReviewResult{DepthScore: 0.3},
			},
			false,
		},
		{
			"无审查结果默认正确",
			&TurnContext{
				Review: nil,
			},
			true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := o.inferCorrectness(tc.tc)
			if result != tc.expected {
				t.Errorf("inferCorrectness() = %t, want %t", result, tc.expected)
			}
		})
	}
}
