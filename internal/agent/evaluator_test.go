package agent

import (
	"strings"
	"testing"
)

// ============================
// Phase G: RAGAS Evaluator Tests
// ============================

// -- extractJSON Tests ----------------------------------------

func TestExtractJSON_PureJSON(t *testing.T) {
	input := `{"faithfulness": 0.8, "actionability": 0.7, "answer_restraint": 0.9, "context_precision": 0.6, "context_recall": 0.5}`
	result := extractJSON(input)
	if !strings.HasPrefix(result, "{") || !strings.HasSuffix(result, "}") {
		t.Errorf("extractJSON should return JSON object, got %q", result)
	}
}

func TestExtractJSON_MarkdownCodeBlock(t *testing.T) {
	input := "```json\n{\"faithfulness\": 0.8}\n```"
	result := extractJSON(input)
	if !strings.HasPrefix(result, "{") || !strings.HasSuffix(result, "}") {
		t.Errorf("extractJSON should extract from code block, got %q", result)
	}
}

func TestExtractJSON_SurroundingText(t *testing.T) {
	input := `以下是评估结果：
{"faithfulness": 0.8, "actionability": 0.7}
评估完成。`
	result := extractJSON(input)
	if result != `{"faithfulness": 0.8, "actionability": 0.7}` {
		t.Errorf("extractJSON should extract embedded JSON, got %q", result)
	}
}

func TestExtractJSON_NoJSON(t *testing.T) {
	input := "no json here"
	result := extractJSON(input)
	// When no { is found, returns original string
	if result != input {
		t.Errorf("extractJSON with no JSON should return original, got %q", result)
	}
}

func TestExtractJSON_NestedBraces(t *testing.T) {
	input := `{"a": {"b": 1}, "c": 2}`
	result := extractJSON(input)
	if result != input {
		t.Errorf("extractJSON should handle nested braces, got %q", result)
	}
}

func TestExtractJSON_EmptyString(t *testing.T) {
	result := extractJSON("")
	if result != "" {
		t.Errorf("extractJSON of empty string should be empty, got %q", result)
	}
}

// -- clampScore Tests -----------------------------------------

func TestClampScore(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected float64
	}{
		{"within range", 0.5, 0.5},
		{"at zero", 0.0, 0.0},
		{"at one", 1.0, 1.0},
		{"below zero", -0.1, 0.0},
		{"above one", 1.5, 1.0},
		{"very negative", -100.0, 0.0},
		{"very high", 999.0, 1.0},
		{"small positive", 0.001, 0.001},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := clampScore(tc.input)
			if result != tc.expected {
				t.Errorf("clampScore(%f) = %f, want %f", tc.input, result, tc.expected)
			}
		})
	}
}

// -- parseEvalScores Tests ------------------------------------

func TestParseEvalScores_ValidJSON(t *testing.T) {
	input := `{"faithfulness": 0.85, "actionability": 0.7, "answer_restraint": 0.9, "context_precision": 0.6, "context_recall": 0.75}`
	scores, err := parseEvalScores(input)
	if err != nil {
		t.Fatalf("parseEvalScores failed: %v", err)
	}
	if scores.Faithfulness != 0.85 {
		t.Errorf("Faithfulness = %f, want 0.85", scores.Faithfulness)
	}
	if scores.Actionability != 0.7 {
		t.Errorf("Actionability = %f, want 0.7", scores.Actionability)
	}
	if scores.AnswerRestraint != 0.9 {
		t.Errorf("AnswerRestraint = %f, want 0.9", scores.AnswerRestraint)
	}
	if scores.ContextPrecision != 0.6 {
		t.Errorf("ContextPrecision = %f, want 0.6", scores.ContextPrecision)
	}
	if scores.ContextRecall != 0.75 {
		t.Errorf("ContextRecall = %f, want 0.75", scores.ContextRecall)
	}
}

func TestParseEvalScores_WithCodeBlock(t *testing.T) {
	input := "```json\n{\"faithfulness\": 0.8, \"actionability\": 0.6, \"answer_restraint\": 0.7, \"context_precision\": 0.5, \"context_recall\": 0.9}\n```"
	scores, err := parseEvalScores(input)
	if err != nil {
		t.Fatalf("parseEvalScores with code block failed: %v", err)
	}
	if scores.Faithfulness != 0.8 {
		t.Errorf("Faithfulness = %f, want 0.8", scores.Faithfulness)
	}
}

func TestParseEvalScores_ClampsOutOfRange(t *testing.T) {
	input := `{"faithfulness": 1.5, "actionability": -0.2, "answer_restraint": 0.5, "context_precision": 2.0, "context_recall": -1.0}`
	scores, err := parseEvalScores(input)
	if err != nil {
		t.Fatalf("parseEvalScores failed: %v", err)
	}
	if scores.Faithfulness != 1.0 {
		t.Errorf("Faithfulness should be clamped to 1.0, got %f", scores.Faithfulness)
	}
	if scores.Actionability != 0.0 {
		t.Errorf("Actionability should be clamped to 0.0, got %f", scores.Actionability)
	}
	if scores.ContextPrecision != 1.0 {
		t.Errorf("ContextPrecision should be clamped to 1.0, got %f", scores.ContextPrecision)
	}
	if scores.ContextRecall != 0.0 {
		t.Errorf("ContextRecall should be clamped to 0.0, got %f", scores.ContextRecall)
	}
}

func TestParseEvalScores_InvalidJSON(t *testing.T) {
	input := "this is not json at all"
	_, err := parseEvalScores(input)
	if err == nil {
		t.Error("parseEvalScores should fail on invalid JSON")
	}
}

func TestParseEvalScores_PartialFields(t *testing.T) {
	// Missing fields default to 0 in Go's JSON unmarshaling
	input := `{"faithfulness": 0.8}`
	scores, err := parseEvalScores(input)
	if err != nil {
		t.Fatalf("parseEvalScores failed: %v", err)
	}
	if scores.Faithfulness != 0.8 {
		t.Errorf("Faithfulness = %f, want 0.8", scores.Faithfulness)
	}
	if scores.Actionability != 0.0 {
		t.Errorf("Actionability should default to 0.0, got %f", scores.Actionability)
	}
}

func TestParseEvalScores_WithSurroundingText(t *testing.T) {
	input := `好的，以下是我的评估结果：

{"faithfulness": 0.9, "actionability": 0.8, "answer_restraint": 0.7, "context_precision": 0.6, "context_recall": 0.5}

希望对您有帮助！`

	scores, err := parseEvalScores(input)
	if err != nil {
		t.Fatalf("parseEvalScores with surrounding text failed: %v", err)
	}
	if scores.Faithfulness != 0.9 {
		t.Errorf("Faithfulness = %f, want 0.9", scores.Faithfulness)
	}
}

// -- buildEvalPrompt Tests ------------------------------------

func TestBuildEvalPrompt_ContainsAllParts(t *testing.T) {
	result := buildEvalPrompt("coach response", "student question", "general_concept_socratic", "high")

	if !strings.Contains(result, "student question") {
		t.Error("prompt should contain student input")
	}
	if !strings.Contains(result, "coach response") {
		t.Error("prompt should contain coach content")
	}
	if !strings.Contains(result, "general_concept_socratic") {
		t.Error("prompt should contain skill ID")
	}
	if !strings.Contains(result, "high") {
		t.Error("prompt should contain scaffold level")
	}
}

func TestBuildEvalPrompt_EmptyInputs(t *testing.T) {
	// Should not panic with empty inputs
	result := buildEvalPrompt("", "", "", "")
	if result == "" {
		t.Error("prompt should not be empty even with empty inputs")
	}
}

// -- DefaultEvalConfig Tests ----------------------------------

func TestDefaultEvalConfig(t *testing.T) {
	cfg := DefaultEvalConfig()
	if cfg.BatchSize != 10 {
		t.Errorf("BatchSize = %d, want 10", cfg.BatchSize)
	}
	if cfg.PollInterval.Seconds() != 30 {
		t.Errorf("PollInterval = %v, want 30s", cfg.PollInterval)
	}
}

// -- ragasSystemPrompt Tests ----------------------------------

func TestRagasSystemPrompt_ContainsDimensions(t *testing.T) {
	dimensions := []string{
		"faithfulness",
		"actionability",
		"answer_restraint",
		"context_precision",
		"context_recall",
	}
	for _, dim := range dimensions {
		if !strings.Contains(ragasSystemPrompt, dim) {
			t.Errorf("ragasSystemPrompt should contain dimension %q", dim)
		}
	}
}
