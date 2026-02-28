package handler

import (
	"testing"

	"github.com/hflms/hanfledge/internal/domain/model"
)

// ── isValidTrapType Tests ───────────────────────────────────

func TestIsValidTrapType(t *testing.T) {
	tests := []struct {
		name     string
		input    model.TrapType
		expected bool
	}{
		{"conceptual有效", model.TrapTypeConceptual, true},
		{"procedural有效", model.TrapTypeProcedural, true},
		{"intuitive有效", model.TrapTypeIntuit, true},
		{"transfer有效", model.TrapTypeTransfer, true},
		{"空字符串无效", "", false},
		{"随机字符串无效", "unknown", false},
		{"大写无效", "CONCEPTUAL", false},
		{"混合大小写无效", "Conceptual", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := isValidTrapType(tc.input)
			if result != tc.expected {
				t.Errorf("isValidTrapType(%q) = %v, want %v",
					tc.input, result, tc.expected)
			}
		})
	}
}

// ── isValidLinkType Tests ───────────────────────────────────

func TestIsValidLinkType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"analogy有效", "analogy", true},
		{"shared_model有效", "shared_model", true},
		{"application有效", "application", true},
		{"空字符串无效", "", false},
		{"随机字符串无效", "other", false},
		{"大写无效", "ANALOGY", false},
		{"带空格无效", " analogy ", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := isValidLinkType(tc.input)
			if result != tc.expected {
				t.Errorf("isValidLinkType(%q) = %v, want %v",
					tc.input, result, tc.expected)
			}
		})
	}
}
