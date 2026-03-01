package federated

import (
	"math"
	"testing"
	"time"
)

// -- NewFedAvgServer ----------------------------------------------

func TestNewFedAvgServer(t *testing.T) {
	cfg := DefaultFedConfig()
	s := NewFedAvgServer(cfg)

	if s == nil {
		t.Fatal("expected non-nil FedAvgServer")
	}
	if s.currentRound != 0 {
		t.Errorf("expected currentRound=0, got %d", s.currentRound)
	}
	if s.dpMechanism == nil {
		t.Error("expected non-nil dpMechanism")
	}
	if len(s.schools) != 0 {
		t.Errorf("expected 0 schools, got %d", len(s.schools))
	}
	if len(s.pendingUpdates) != 0 {
		t.Errorf("expected 0 pending updates, got %d", len(s.pendingUpdates))
	}
}

// -- DefaultFedConfig ---------------------------------------------

func TestDefaultFedConfig(t *testing.T) {
	cfg := DefaultFedConfig()

	if cfg.MinSchools != 2 {
		t.Errorf("expected MinSchools=2, got %d", cfg.MinSchools)
	}
	if cfg.MaxRoundDuration != 10*time.Minute {
		t.Errorf("expected MaxRoundDuration=10m, got %v", cfg.MaxRoundDuration)
	}
	if cfg.DPEpsilon != 1.0 {
		t.Errorf("expected DPEpsilon=1.0, got %f", cfg.DPEpsilon)
	}
	if cfg.DPDelta != 1e-5 {
		t.Errorf("expected DPDelta=1e-5, got %e", cfg.DPDelta)
	}
	if cfg.ClipNorm != 1.0 {
		t.Errorf("expected ClipNorm=1.0, got %f", cfg.ClipNorm)
	}
	if cfg.LearningRate != 0.01 {
		t.Errorf("expected LearningRate=0.01, got %f", cfg.LearningRate)
	}
}

// -- RegisterSchool -----------------------------------------------

func TestRegisterSchool_Success(t *testing.T) {
	s := NewFedAvgServer(DefaultFedConfig())

	err := s.RegisterSchool("school-001", "Test School")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(s.schools) != 1 {
		t.Errorf("expected 1 school, got %d", len(s.schools))
	}

	school := s.schools["school-001"]
	if school.Name != "Test School" {
		t.Errorf("expected name=Test School, got %q", school.Name)
	}
	if !school.Active {
		t.Error("expected school to be active")
	}
}

func TestRegisterSchool_EmptyID(t *testing.T) {
	s := NewFedAvgServer(DefaultFedConfig())
	err := s.RegisterSchool("", "Test")
	if err == nil {
		t.Error("expected error for empty school ID")
	}
}

func TestRegisterSchool_EmptyName(t *testing.T) {
	s := NewFedAvgServer(DefaultFedConfig())
	err := s.RegisterSchool("school-001", "")
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestRegisterSchool_Duplicate(t *testing.T) {
	s := NewFedAvgServer(DefaultFedConfig())

	if err := s.RegisterSchool("school-001", "School A"); err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	err := s.RegisterSchool("school-001", "School B")
	if err == nil {
		t.Error("expected error for duplicate school ID")
	}
}

// -- SubmitGradient -----------------------------------------------

func TestSubmitGradient_Success(t *testing.T) {
	s := NewFedAvgServer(DefaultFedConfig())
	s.RegisterSchool("school-001", "School A")

	gradients := []float64{0.1, 0.2, 0.3}
	update := &GradientUpdate{
		SchoolID:    "school-001",
		RoundID:     0,
		Gradients:   gradients,
		SampleCount: 100,
		Timestamp:   time.Now(),
		Checksum:    computeChecksum(gradients),
	}

	err := s.SubmitGradient(update)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(s.pendingUpdates) != 1 {
		t.Errorf("expected 1 pending update, got %d", len(s.pendingUpdates))
	}
}

func TestSubmitGradient_NilUpdate(t *testing.T) {
	s := NewFedAvgServer(DefaultFedConfig())
	err := s.SubmitGradient(nil)
	if err == nil {
		t.Error("expected error for nil update")
	}
}

func TestSubmitGradient_UnregisteredSchool(t *testing.T) {
	s := NewFedAvgServer(DefaultFedConfig())

	update := &GradientUpdate{
		SchoolID:    "unknown",
		RoundID:     0,
		Gradients:   []float64{0.1},
		SampleCount: 10,
		Checksum:    computeChecksum([]float64{0.1}),
	}

	err := s.SubmitGradient(update)
	if err == nil {
		t.Error("expected error for unregistered school")
	}
}

func TestSubmitGradient_StaleRound(t *testing.T) {
	s := NewFedAvgServer(DefaultFedConfig())
	s.RegisterSchool("school-001", "School A")
	s.currentRound = 5

	update := &GradientUpdate{
		SchoolID:    "school-001",
		RoundID:     3, // stale
		Gradients:   []float64{0.1},
		SampleCount: 10,
		Checksum:    computeChecksum([]float64{0.1}),
	}

	err := s.SubmitGradient(update)
	if err == nil {
		t.Error("expected error for stale round")
	}
}

func TestSubmitGradient_EmptyGradients(t *testing.T) {
	s := NewFedAvgServer(DefaultFedConfig())
	s.RegisterSchool("school-001", "School A")

	update := &GradientUpdate{
		SchoolID:    "school-001",
		RoundID:     0,
		Gradients:   nil,
		SampleCount: 10,
	}

	err := s.SubmitGradient(update)
	if err == nil {
		t.Error("expected error for empty gradients")
	}
}

func TestSubmitGradient_InvalidSampleCount(t *testing.T) {
	s := NewFedAvgServer(DefaultFedConfig())
	s.RegisterSchool("school-001", "School A")

	update := &GradientUpdate{
		SchoolID:    "school-001",
		RoundID:     0,
		Gradients:   []float64{0.1},
		SampleCount: 0,
		Checksum:    computeChecksum([]float64{0.1}),
	}

	err := s.SubmitGradient(update)
	if err == nil {
		t.Error("expected error for zero sample count")
	}
}

func TestSubmitGradient_BadChecksum(t *testing.T) {
	s := NewFedAvgServer(DefaultFedConfig())
	s.RegisterSchool("school-001", "School A")

	update := &GradientUpdate{
		SchoolID:    "school-001",
		RoundID:     0,
		Gradients:   []float64{0.1},
		SampleCount: 10,
		Checksum:    "wrong-checksum",
	}

	err := s.SubmitGradient(update)
	if err == nil {
		t.Error("expected error for bad checksum")
	}
}

// -- TryAggregate -------------------------------------------------

func TestTryAggregate_NotEnoughSchools(t *testing.T) {
	cfg := DefaultFedConfig()
	cfg.MinSchools = 2
	s := NewFedAvgServer(cfg)
	s.RegisterSchool("school-001", "School A")

	gradients := []float64{0.1, 0.2}
	s.pendingUpdates["school-001"] = &GradientUpdate{
		SchoolID:    "school-001",
		Gradients:   gradients,
		SampleCount: 100,
	}

	weights, err := s.TryAggregate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if weights != nil {
		t.Error("expected nil weights when not enough schools")
	}
}

func TestTryAggregate_SuccessfulAggregation(t *testing.T) {
	cfg := DefaultFedConfig()
	cfg.MinSchools = 2
	cfg.DPEpsilon = 100.0 // High epsilon = low noise for test predictability
	cfg.ClipNorm = 100.0  // High clip norm to not affect results
	cfg.LearningRate = 1.0
	s := NewFedAvgServer(cfg)

	s.RegisterSchool("school-001", "School A")
	s.RegisterSchool("school-002", "School B")

	g1 := []float64{1.0, 2.0, 3.0}
	g2 := []float64{3.0, 2.0, 1.0}

	s.pendingUpdates["school-001"] = &GradientUpdate{
		SchoolID:    "school-001",
		Gradients:   g1,
		SampleCount: 100,
	}
	s.pendingUpdates["school-002"] = &GradientUpdate{
		SchoolID:    "school-002",
		Gradients:   g2,
		SampleCount: 100,
	}

	weights, err := s.TryAggregate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if weights == nil {
		t.Fatal("expected non-nil weights")
	}

	if weights.RoundID != 1 {
		t.Errorf("expected RoundID=1, got %d", weights.RoundID)
	}
	if weights.SchoolCount != 2 {
		t.Errorf("expected SchoolCount=2, got %d", weights.SchoolCount)
	}
	if len(weights.Weights) != 3 {
		t.Errorf("expected 3 weight dimensions, got %d", len(weights.Weights))
	}

	// After aggregation, pending updates should be cleared
	if len(s.pendingUpdates) != 0 {
		t.Errorf("expected 0 pending updates after aggregation, got %d", len(s.pendingUpdates))
	}
}

func TestTryAggregate_DimensionMismatch(t *testing.T) {
	cfg := DefaultFedConfig()
	cfg.MinSchools = 2
	s := NewFedAvgServer(cfg)

	s.RegisterSchool("school-001", "School A")
	s.RegisterSchool("school-002", "School B")

	s.pendingUpdates["school-001"] = &GradientUpdate{
		SchoolID:    "school-001",
		Gradients:   []float64{1.0, 2.0},
		SampleCount: 100,
	}
	s.pendingUpdates["school-002"] = &GradientUpdate{
		SchoolID:    "school-002",
		Gradients:   []float64{1.0, 2.0, 3.0}, // different dim
		SampleCount: 100,
	}

	_, err := s.TryAggregate()
	if err == nil {
		t.Error("expected error for dimension mismatch")
	}
}

// -- fedAvg -------------------------------------------------------

func TestFedAvg_WeightedAverage(t *testing.T) {
	s := NewFedAvgServer(DefaultFedConfig())

	updates := []*GradientUpdate{
		{SchoolID: "A", Gradients: []float64{1.0, 0.0}, SampleCount: 100},
		{SchoolID: "B", Gradients: []float64{0.0, 1.0}, SampleCount: 100},
	}

	result := s.fedAvg(updates)

	// Equal weights (100/200 = 0.5 each) → average
	if math.Abs(result[0]-0.5) > 1e-9 {
		t.Errorf("expected result[0]=0.5, got %f", result[0])
	}
	if math.Abs(result[1]-0.5) > 1e-9 {
		t.Errorf("expected result[1]=0.5, got %f", result[1])
	}
}

func TestFedAvg_UnequalWeights(t *testing.T) {
	s := NewFedAvgServer(DefaultFedConfig())

	updates := []*GradientUpdate{
		{SchoolID: "A", Gradients: []float64{10.0}, SampleCount: 300},
		{SchoolID: "B", Gradients: []float64{0.0}, SampleCount: 100},
	}

	result := s.fedAvg(updates)

	// A gets 300/400 = 0.75 weight, B gets 100/400 = 0.25 weight
	// result = 0.75 * 10.0 + 0.25 * 0.0 = 7.5
	if math.Abs(result[0]-7.5) > 1e-9 {
		t.Errorf("expected result=7.5, got %f", result[0])
	}
}

func TestFedAvg_Empty(t *testing.T) {
	s := NewFedAvgServer(DefaultFedConfig())
	result := s.fedAvg(nil)
	if result != nil {
		t.Errorf("expected nil for empty input, got %v", result)
	}
}

// -- computeChecksum ----------------------------------------------

func TestComputeChecksum_Deterministic(t *testing.T) {
	gradients := []float64{1.0, 2.0, 3.0}

	c1 := computeChecksum(gradients)
	c2 := computeChecksum(gradients)

	if c1 != c2 {
		t.Errorf("checksum should be deterministic: %q != %q", c1, c2)
	}
}

func TestComputeChecksum_DifferentInput(t *testing.T) {
	c1 := computeChecksum([]float64{1.0, 2.0})
	c2 := computeChecksum([]float64{1.0, 3.0})

	if c1 == c2 {
		t.Error("different inputs should produce different checksums")
	}
}

// -- GetGlobalWeights ---------------------------------------------

func TestGetGlobalWeights_InitiallyNil(t *testing.T) {
	s := NewFedAvgServer(DefaultFedConfig())
	if s.GetGlobalWeights() != nil {
		t.Error("expected nil global weights initially")
	}
}
