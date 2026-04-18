package embedding

import (
	"context"
	"math"
	"testing"

	"github.com/hflms/hanfledge/internal/infrastructure/llm"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type mockLLMProvider struct{}

func (m *mockLLMProvider) Name() string { return "mock" }
func (m *mockLLMProvider) Chat(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatOptions) (string, error) {
	return "", nil
}
func (m *mockLLMProvider) StreamChat(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatOptions, onToken func(token string)) (string, error) {
	return "", nil
}
func (m *mockLLMProvider) Embed(ctx context.Context, text string) ([]float64, error) { return nil, nil }
func (m *mockLLMProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	return nil, nil
}

// -- cosineSimilarity ---------------------------------------------

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a    []float64
		b    []float64
		want float64
		tol  float64
	}{
		{"identical", []float64{1, 2, 3}, []float64{1, 2, 3}, 1.0, 1e-9},
		{"opposite", []float64{1, 0, 0}, []float64{-1, 0, 0}, -1.0, 1e-9},
		{"orthogonal", []float64{1, 0}, []float64{0, 1}, 0.0, 1e-9},
		{"scaled", []float64{2, 4, 6}, []float64{1, 2, 3}, 1.0, 1e-9},
		{"partial", []float64{1, 1, 0}, []float64{1, 0, 1}, 0.5, 1e-9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cosineSimilarity(tt.a, tt.b)
			if math.Abs(got-tt.want) > tt.tol {
				t.Errorf("cosineSimilarity(%v, %v) = %f, want %f (tol=%e)", tt.a, tt.b, got, tt.want, tt.tol)
			}
		})
	}
}

func TestCosineSimilarity_EdgeCases(t *testing.T) {
	// Empty vectors
	if got := cosineSimilarity(nil, nil); got != 0 {
		t.Errorf("expected 0 for nil vectors, got %f", got)
	}
	if got := cosineSimilarity([]float64{}, []float64{}); got != 0 {
		t.Errorf("expected 0 for empty vectors, got %f", got)
	}

	// Mismatched dimensions
	if got := cosineSimilarity([]float64{1, 2}, []float64{1, 2, 3}); got != 0 {
		t.Errorf("expected 0 for mismatched dimensions, got %f", got)
	}

	// Zero vector
	if got := cosineSimilarity([]float64{0, 0, 0}, []float64{1, 2, 3}); got != 0 {
		t.Errorf("expected 0 for zero vector, got %f", got)
	}
}

// -- DefaultInfoNCEConfig -----------------------------------------

func TestDefaultInfoNCEConfig(t *testing.T) {
	cfg := DefaultInfoNCEConfig()

	if cfg.Temperature != 0.07 {
		t.Errorf("expected Temperature=0.07, got %f", cfg.Temperature)
	}
	if cfg.NegativeSamples != 7 {
		t.Errorf("expected NegativeSamples=7, got %d", cfg.NegativeSamples)
	}
}

// -- NewFineTunePipeline ------------------------------------------

func TestNewFineTunePipeline(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	mockProvider := &mockLLMProvider{}

	p := NewFineTunePipeline(db, mockProvider)

	if p == nil {
		t.Fatal("expected non-nil FineTunePipeline")
	}
	if p.DB != db {
		t.Errorf("expected DB to be set correctly")
	}
	if p.LLM != mockProvider {
		t.Errorf("expected LLM to be set correctly")
	}
	if p.OutputDir != "data/embedding-finetune" {
		t.Errorf("expected OutputDir=data/embedding-finetune, got %q", p.OutputDir)
	}
	if p.MinPairs != 100 {
		t.Errorf("expected MinPairs=100, got %d", p.MinPairs)
	}
	if p.MinScore != 0.6 {
		t.Errorf("expected MinScore=0.6, got %f", p.MinScore)
	}
	if p.BatchSize != 32 {
		t.Errorf("expected BatchSize=32, got %d", p.BatchSize)
	}
}

// -- computeStats -------------------------------------------------

func TestComputeStats_Empty(t *testing.T) {
	p := NewFineTunePipeline(nil, nil)
	stats := p.computeStats(nil, nil)

	if stats.TotalPairs != 0 {
		t.Errorf("expected 0 total pairs, got %d", stats.TotalPairs)
	}
}

func TestComputeStats_WithData(t *testing.T) {
	p := NewFineTunePipeline(nil, nil)

	pairs := []TrainingPair{
		{Query: "什么是光合作用", Passage: "光合作用是植物利用光能将二氧化碳和水转化为有机物的过程", CourseID: 1, Score: 0.9},
		{Query: "牛顿定律", Passage: "牛顿第一定律：物体在没有外力作用时保持静止或匀速直线运动", CourseID: 2, Score: 0.8},
		{Query: "DNA结构", Passage: "DNA是由两条反向平行的多核苷酸链组成的双螺旋结构", CourseID: 1, Score: 0.7},
	}

	stats := p.computeStats(pairs, []uint{1, 2})

	if stats.TotalPairs != 3 {
		t.Errorf("expected 3 total pairs, got %d", stats.TotalPairs)
	}
	if stats.CourseCount != 2 {
		t.Errorf("expected 2 courses, got %d", stats.CourseCount)
	}
	if stats.AvgQueryLen <= 0 {
		t.Errorf("expected positive AvgQueryLen, got %f", stats.AvgQueryLen)
	}
	if stats.AvgPassageLen <= 0 {
		t.Errorf("expected positive AvgPassageLen, got %f", stats.AvgPassageLen)
	}
	// Passage should generally be longer than query
	if stats.AvgPassageLen < stats.AvgQueryLen {
		t.Logf("note: AvgPassageLen (%.1f) < AvgQueryLen (%.1f)", stats.AvgPassageLen, stats.AvgQueryLen)
	}
}
