package embedding

import (
	"testing"
)

// -- DefaultLoRAConfig --------------------------------------------

func TestDefaultLoRAConfig(t *testing.T) {
	cfg := DefaultLoRAConfig()

	if cfg.BaseModel != "Qwen/Qwen2.5-7B-Instruct" {
		t.Errorf("expected BaseModel=Qwen/Qwen2.5-7B-Instruct, got %q", cfg.BaseModel)
	}
	if cfg.LoRARank != 64 {
		t.Errorf("expected LoRARank=64, got %d", cfg.LoRARank)
	}
	if cfg.LoRAAlpha != 128 {
		t.Errorf("expected LoRAAlpha=128, got %d", cfg.LoRAAlpha)
	}
	if cfg.LoRADropout != 0.05 {
		t.Errorf("expected LoRADropout=0.05, got %f", cfg.LoRADropout)
	}
	if cfg.LearningRate != 2e-4 {
		t.Errorf("expected LearningRate=2e-4, got %f", cfg.LearningRate)
	}
	if cfg.NumEpochs != 3 {
		t.Errorf("expected NumEpochs=3, got %d", cfg.NumEpochs)
	}
	if cfg.BatchSize != 4 {
		t.Errorf("expected BatchSize=4, got %d", cfg.BatchSize)
	}
	if cfg.GradAccumSteps != 4 {
		t.Errorf("expected GradAccumSteps=4, got %d", cfg.GradAccumSteps)
	}
	if cfg.MaxSeqLen != 2048 {
		t.Errorf("expected MaxSeqLen=2048, got %d", cfg.MaxSeqLen)
	}
	if cfg.Quantization != "4bit" {
		t.Errorf("expected Quantization=4bit, got %q", cfg.Quantization)
	}
	if len(cfg.TargetModules) != 7 {
		t.Errorf("expected 7 target modules, got %d", len(cfg.TargetModules))
	}
}

// -- buildDistillSystemPrompt ------------------------------------

func TestBuildDistillSystemPrompt(t *testing.T) {
	tests := []struct {
		skillID  string
		contains string
	}{
		{"general_concept_socratic", "苏格拉底式提问"},
		{"general_assessment_fallacy", "谬误侦探"},
		{"general_review_roleplay", "角色扮演"},
		{"general_practice_quiz", "自动出题"},
		{"unknown_skill", "AI 学习导师"},
	}

	for _, tt := range tests {
		t.Run(tt.skillID, func(t *testing.T) {
			prompt := buildDistillSystemPrompt(tt.skillID)
			if !containsSubstr(prompt, tt.contains) {
				t.Errorf("prompt for skill %q should contain %q, got %q", tt.skillID, tt.contains, prompt)
			}
		})
	}
}

func TestBuildDistillSystemPrompt_AlwaysHasBase(t *testing.T) {
	skills := []string{
		"general_concept_socratic",
		"general_assessment_fallacy",
		"general_review_roleplay",
		"general_practice_quiz",
		"random",
	}

	base := "你是一位专业的 AI 学习导师"
	for _, skillID := range skills {
		prompt := buildDistillSystemPrompt(skillID)
		if !containsSubstr(prompt, base) {
			t.Errorf("prompt for %q should contain base prompt", skillID)
		}
	}
}

// -- parseDistillScore --------------------------------------------

func TestParseDistillScore(t *testing.T) {
	tests := []struct {
		name string
		text string
		want float64
	}{
		{"integer", "8", 8.0},
		{"float", "7.5", 7.5},
		{"with_text", "8.5 分", 8.5},
		{"zero", "0", 0.0},
		{"ten", "10", 10.0},
		{"over_ten_clamped", "15", 10.0},
		{"negative_clamped", "-3", 0.0},
		{"unparseable", "很好", 5.0}, // default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDistillScore(tt.text)
			if got != tt.want {
				t.Errorf("parseDistillScore(%q) = %f, want %f", tt.text, got, tt.want)
			}
		})
	}
}

// -- estimateTokenCount -------------------------------------------

func TestEstimateTokenCount(t *testing.T) {
	tests := []struct {
		name string
		text string
		min  int
		max  int
	}{
		{"empty", "", 0, 0},
		{"chinese_only", "你好世界", 2, 4},
		{"english_only", "hello world test", 3, 5},
		{"mixed", "你好 hello 世界", 3, 6},
		{"single_char", "a", 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateTokenCount(tt.text)
			if got < tt.min || got > tt.max {
				t.Errorf("estimateTokenCount(%q) = %d, want in [%d, %d]", tt.text, got, tt.min, tt.max)
			}
		})
	}
}

func TestEstimateTokenCount_NeverZeroForNonEmpty(t *testing.T) {
	// Should return at least 1 for any non-empty text
	texts := []string{"a", "x", "!", "1"}
	for _, text := range texts {
		got := estimateTokenCount(text)
		if got < 1 {
			t.Errorf("estimateTokenCount(%q) = %d, expected >= 1", text, got)
		}
	}
}

// -- NewDistillationPipeline --------------------------------------

func TestNewDistillationPipeline(t *testing.T) {
	p := NewDistillationPipeline(nil, nil)

	if p == nil {
		t.Fatal("expected non-nil DistillationPipeline")
	}
	if p.OutputDir != "data/distillation" {
		t.Errorf("expected OutputDir=data/distillation, got %q", p.OutputDir)
	}
	if p.MinRAGASScore != 0.85 {
		t.Errorf("expected MinRAGASScore=0.85, got %f", p.MinRAGASScore)
	}
	if p.MaxSamplesPerSkill != 5000 {
		t.Errorf("expected MaxSamplesPerSkill=5000, got %d", p.MaxSamplesPerSkill)
	}
}

// -- Helpers -------------------------------------------------------

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
