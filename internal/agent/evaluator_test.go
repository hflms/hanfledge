package agent

import (
	"strings"
	"testing"
	"time"
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
	if cfg.MaxInterval != 5*time.Minute {
		t.Errorf("MaxInterval = %v, want 5m", cfg.MaxInterval)
	}
	if cfg.NotifyBuffer != 64 {
		t.Errorf("NotifyBuffer = %d, want 64", cfg.NotifyBuffer)
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

// -- Notify Tests ---------------------------------------------

func TestNotify_SendsToChannel(t *testing.T) {
	cfg := DefaultEvalConfig()
	cfg.NotifyBuffer = 4
	e := NewRAGASEvaluator(nil, nil, cfg)

	e.Notify(42)

	select {
	case id := <-e.evalCh:
		if id != 42 {
			t.Errorf("Notify sent %d, want 42", id)
		}
	default:
		t.Error("Notify should have sent to evalCh")
	}
}

func TestNotify_NonBlockingWhenFull(t *testing.T) {
	cfg := DefaultEvalConfig()
	cfg.NotifyBuffer = 2
	e := NewRAGASEvaluator(nil, nil, cfg)

	// Fill the channel
	e.Notify(1)
	e.Notify(2)

	// This should NOT block — it should drop silently
	done := make(chan struct{})
	go func() {
		e.Notify(3)
		close(done)
	}()

	select {
	case <-done:
		// Success — Notify returned without blocking
	case <-time.After(1 * time.Second):
		t.Fatal("Notify blocked when channel was full")
	}

	// Verify channel still has exactly 2 items (original ones)
	if len(e.evalCh) != 2 {
		t.Errorf("channel length = %d, want 2", len(e.evalCh))
	}
}

func TestNotify_MultipleSends(t *testing.T) {
	cfg := DefaultEvalConfig()
	cfg.NotifyBuffer = 10
	e := NewRAGASEvaluator(nil, nil, cfg)

	for i := uint(1); i <= 5; i++ {
		e.Notify(i)
	}

	if len(e.evalCh) != 5 {
		t.Errorf("channel length = %d, want 5", len(e.evalCh))
	}
}

// -- drainNotifications Tests ---------------------------------

func TestDrainNotifications_EmptiesChannel(t *testing.T) {
	cfg := DefaultEvalConfig()
	cfg.NotifyBuffer = 10
	e := NewRAGASEvaluator(nil, nil, cfg)

	// Fill channel with 5 notifications
	for i := uint(1); i <= 5; i++ {
		e.evalCh <- i
	}

	e.drainNotifications()

	if len(e.evalCh) != 0 {
		t.Errorf("after drain, channel length = %d, want 0", len(e.evalCh))
	}
}

func TestDrainNotifications_EmptyChannelNoop(t *testing.T) {
	cfg := DefaultEvalConfig()
	cfg.NotifyBuffer = 4
	e := NewRAGASEvaluator(nil, nil, cfg)

	// Should not block on empty channel
	done := make(chan struct{})
	go func() {
		e.drainNotifications()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("drainNotifications blocked on empty channel")
	}
}

// -- backoff Tests --------------------------------------------

func TestBackoff_Doubles(t *testing.T) {
	cfg := DefaultEvalConfig()
	cfg.MaxInterval = 5 * time.Minute
	e := NewRAGASEvaluator(nil, nil, cfg)

	next := e.backoff(30 * time.Second)
	if next != 60*time.Second {
		t.Errorf("backoff(30s) = %v, want 60s", next)
	}
}

func TestBackoff_CapsAtMax(t *testing.T) {
	cfg := DefaultEvalConfig()
	cfg.MaxInterval = 5 * time.Minute
	e := NewRAGASEvaluator(nil, nil, cfg)

	next := e.backoff(3 * time.Minute)
	if next != 5*time.Minute {
		t.Errorf("backoff(3m) = %v, want 5m (max)", next)
	}
}

func TestBackoff_AlreadyAtMax(t *testing.T) {
	cfg := DefaultEvalConfig()
	cfg.MaxInterval = 5 * time.Minute
	e := NewRAGASEvaluator(nil, nil, cfg)

	next := e.backoff(5 * time.Minute)
	if next != 5*time.Minute {
		t.Errorf("backoff(5m) = %v, want 5m (should stay at max)", next)
	}
}

func TestBackoff_Progression(t *testing.T) {
	cfg := DefaultEvalConfig()
	cfg.PollInterval = 30 * time.Second
	cfg.MaxInterval = 5 * time.Minute
	e := NewRAGASEvaluator(nil, nil, cfg)

	expected := []time.Duration{
		60 * time.Second,
		120 * time.Second,
		240 * time.Second,
		5 * time.Minute, // 480s would exceed 300s max
		5 * time.Minute, // stays at max
	}

	current := cfg.PollInterval
	for i, want := range expected {
		current = e.backoff(current)
		if current != want {
			t.Errorf("step %d: backoff = %v, want %v", i, current, want)
		}
	}
}

// -- NewRAGASEvaluator Tests ----------------------------------

func TestNewRAGASEvaluator_DefaultBuffer(t *testing.T) {
	cfg := EvalConfig{
		BatchSize:    5,
		PollInterval: 10 * time.Second,
		MaxInterval:  1 * time.Minute,
		NotifyBuffer: 0, // should default to 64
	}
	e := NewRAGASEvaluator(nil, nil, cfg)
	if cap(e.evalCh) != 64 {
		t.Errorf("evalCh capacity = %d, want 64 (default)", cap(e.evalCh))
	}
}

func TestNewRAGASEvaluator_CustomBuffer(t *testing.T) {
	cfg := DefaultEvalConfig()
	cfg.NotifyBuffer = 128
	e := NewRAGASEvaluator(nil, nil, cfg)
	if cap(e.evalCh) != 128 {
		t.Errorf("evalCh capacity = %d, want 128", cap(e.evalCh))
	}
}
