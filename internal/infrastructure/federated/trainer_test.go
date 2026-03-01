package federated

import (
	"math"
	"testing"
)

// -- Helper: create a valid 32-byte key ---------------------------

func testKey() []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return key
}

// -- NewLocalTrainer ----------------------------------------------

func TestNewLocalTrainer_Success(t *testing.T) {
	cfg := DefaultLocalTrainerConfig("school-001")
	trainer, err := NewLocalTrainer(cfg, testKey())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trainer == nil {
		t.Fatal("expected non-nil trainer")
	}
	if trainer.config.SchoolID != "school-001" {
		t.Errorf("expected SchoolID=school-001, got %q", trainer.config.SchoolID)
	}
}

func TestNewLocalTrainer_InvalidKey(t *testing.T) {
	cfg := DefaultLocalTrainerConfig("school-001")
	_, err := NewLocalTrainer(cfg, []byte("short"))
	if err == nil {
		t.Error("expected error for invalid encryption key")
	}
}

// -- DefaultLocalTrainerConfig ------------------------------------

func TestDefaultLocalTrainerConfig(t *testing.T) {
	cfg := DefaultLocalTrainerConfig("test-school")

	if cfg.SchoolID != "test-school" {
		t.Errorf("expected SchoolID=test-school, got %q", cfg.SchoolID)
	}
	if cfg.BatchSize != 32 {
		t.Errorf("expected BatchSize=32, got %d", cfg.BatchSize)
	}
	if cfg.Epochs != 3 {
		t.Errorf("expected Epochs=3, got %d", cfg.Epochs)
	}
	if cfg.LearningRate != 0.001 {
		t.Errorf("expected LearningRate=0.001, got %f", cfg.LearningRate)
	}
	if cfg.ModelDim != 1024 {
		t.Errorf("expected ModelDim=1024, got %d", cfg.ModelDim)
	}
}

// -- AddTrainingPairs ---------------------------------------------

func TestAddTrainingPairs(t *testing.T) {
	cfg := DefaultLocalTrainerConfig("school-001")
	trainer, _ := NewLocalTrainer(cfg, testKey())

	pairs1 := []TrainingPair{
		{Query: "什么是力", Passage: "力是物体之间的相互作用", Score: 0.9, KPID: 1},
	}
	trainer.AddTrainingPairs(pairs1)

	if len(trainer.pairs) != 1 {
		t.Errorf("expected 1 pair, got %d", len(trainer.pairs))
	}

	pairs2 := []TrainingPair{
		{Query: "光合作用", Passage: "植物利用光能", Score: 0.8, KPID: 2},
		{Query: "细胞分裂", Passage: "有丝分裂", Score: 0.7, KPID: 3},
	}
	trainer.AddTrainingPairs(pairs2)

	if len(trainer.pairs) != 3 {
		t.Errorf("expected 3 pairs after second add, got %d", len(trainer.pairs))
	}
}

// -- Train --------------------------------------------------------

func TestTrain_NoPairsError(t *testing.T) {
	cfg := DefaultLocalTrainerConfig("school-001")
	trainer, _ := NewLocalTrainer(cfg, testKey())

	_, err := trainer.Train()
	if err == nil {
		t.Error("expected error when training with no pairs")
	}
}

func TestTrain_ProducesGradients(t *testing.T) {
	cfg := DefaultLocalTrainerConfig("school-001")
	cfg.ModelDim = 16 // Small dim for testing
	cfg.Epochs = 2
	cfg.BatchSize = 2
	trainer, _ := NewLocalTrainer(cfg, testKey())

	pairs := []TrainingPair{
		{Query: "什么是力", Passage: "力是物体之间的相互作用", Score: 0.9, KPID: 1},
		{Query: "光合作用", Passage: "植物利用光能合成有机物", Score: 0.8, KPID: 2},
		{Query: "DNA", Passage: "脱氧核糖核酸", Score: 0.7, KPID: 3},
	}
	trainer.AddTrainingPairs(pairs)

	gradients, err := trainer.Train()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(gradients) != 16 {
		t.Fatalf("expected 16-dim gradient, got %d", len(gradients))
	}

	// At least some gradients should be non-zero
	hasNonZero := false
	for _, g := range gradients {
		if math.Abs(g) > 1e-15 {
			hasNonZero = true
			break
		}
	}
	if !hasNonZero {
		t.Error("expected at least some non-zero gradients")
	}
}

func TestTrain_DeterministicGivenSameData(t *testing.T) {
	pairs := []TrainingPair{
		{Query: "力", Passage: "力是作用", Score: 0.9, KPID: 1},
	}

	cfg := DefaultLocalTrainerConfig("school-001")
	cfg.ModelDim = 8
	cfg.Epochs = 1
	cfg.BatchSize = 10

	t1, _ := NewLocalTrainer(cfg, testKey())
	t1.AddTrainingPairs(pairs)
	g1, _ := t1.Train()

	t2, _ := NewLocalTrainer(cfg, testKey())
	t2.AddTrainingPairs(pairs)
	g2, _ := t2.Train()

	for i := range g1 {
		if math.Abs(g1[i]-g2[i]) > 1e-15 {
			t.Errorf("index %d: gradients differ: %f vs %f", i, g1[i], g2[i])
		}
	}
}

// -- PrepareUpdate ------------------------------------------------

func TestPrepareUpdate_NoGradientsError(t *testing.T) {
	cfg := DefaultLocalTrainerConfig("school-001")
	trainer, _ := NewLocalTrainer(cfg, testKey())

	_, err := trainer.PrepareUpdate(0)
	if err == nil {
		t.Error("expected error when no gradients available")
	}
}

func TestPrepareUpdate_Success(t *testing.T) {
	cfg := DefaultLocalTrainerConfig("school-001")
	cfg.ModelDim = 8
	cfg.Epochs = 1
	trainer, _ := NewLocalTrainer(cfg, testKey())

	pairs := []TrainingPair{
		{Query: "Q1", Passage: "P1", Score: 0.9, KPID: 1},
	}
	trainer.AddTrainingPairs(pairs)
	trainer.Train()

	update, err := trainer.PrepareUpdate(5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if update.SchoolID != "school-001" {
		t.Errorf("expected SchoolID=school-001, got %q", update.SchoolID)
	}
	if update.RoundID != 5 {
		t.Errorf("expected RoundID=5, got %d", update.RoundID)
	}
	if len(update.Gradients) != 8 {
		t.Errorf("expected 8-dim gradients, got %d", len(update.Gradients))
	}
	if update.SampleCount != 1 {
		t.Errorf("expected SampleCount=1, got %d", update.SampleCount)
	}
	if update.Checksum == "" {
		t.Error("expected non-empty checksum")
	}

	// Verify checksum is valid
	if computeChecksum(update.Gradients) != update.Checksum {
		t.Error("checksum does not match computed value")
	}
}

// -- ApplyGlobalWeights -------------------------------------------

func TestApplyGlobalWeights_NilError(t *testing.T) {
	cfg := DefaultLocalTrainerConfig("school-001")
	trainer, _ := NewLocalTrainer(cfg, testKey())

	err := trainer.ApplyGlobalWeights(nil)
	if err == nil {
		t.Error("expected error for nil weights")
	}
}

func TestApplyGlobalWeights_DimensionMismatch(t *testing.T) {
	cfg := DefaultLocalTrainerConfig("school-001")
	cfg.ModelDim = 8
	trainer, _ := NewLocalTrainer(cfg, testKey())

	weights := &GlobalWeights{
		RoundID: 1,
		Weights: make([]float64, 16), // wrong dim
	}

	err := trainer.ApplyGlobalWeights(weights)
	if err == nil {
		t.Error("expected error for dimension mismatch")
	}
}

func TestApplyGlobalWeights_Success(t *testing.T) {
	cfg := DefaultLocalTrainerConfig("school-001")
	cfg.ModelDim = 4
	cfg.Epochs = 1
	trainer, _ := NewLocalTrainer(cfg, testKey())

	// Train first to populate gradients
	trainer.AddTrainingPairs([]TrainingPair{
		{Query: "Q", Passage: "P", Score: 0.9, KPID: 1},
	})
	trainer.Train()

	weights := &GlobalWeights{
		RoundID: 1,
		Weights: []float64{0.1, 0.2, 0.3, 0.4},
	}

	err := trainer.ApplyGlobalWeights(weights)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After applying, gradients should be blended
	// (0.9 * local + 0.1 * global + tiny perturbation)
	if len(trainer.gradients) != 4 {
		t.Errorf("expected 4-dim gradients, got %d", len(trainer.gradients))
	}
}

func TestApplyGlobalWeights_InitializesNilGradients(t *testing.T) {
	cfg := DefaultLocalTrainerConfig("school-001")
	cfg.ModelDim = 4
	trainer, _ := NewLocalTrainer(cfg, testKey())
	// Don't train — gradients are nil

	weights := &GlobalWeights{
		RoundID: 1,
		Weights: []float64{0.1, 0.2, 0.3, 0.4},
	}

	err := trainer.ApplyGlobalWeights(weights)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(trainer.gradients) != 4 {
		t.Errorf("expected gradients to be initialized with dim=4, got %d", len(trainer.gradients))
	}
}

// -- simpleHash ---------------------------------------------------

func TestSimpleHash_Deterministic(t *testing.T) {
	h1 := simpleHash("hello")
	h2 := simpleHash("hello")
	if h1 != h2 {
		t.Errorf("expected same hash, got %d != %d", h1, h2)
	}
}

func TestSimpleHash_DifferentStrings(t *testing.T) {
	h1 := simpleHash("hello")
	h2 := simpleHash("world")
	if h1 == h2 {
		t.Error("expected different hashes for different strings")
	}
}

func TestSimpleHash_EmptyString(t *testing.T) {
	h := simpleHash("")
	if h != 0 {
		t.Errorf("expected 0 for empty string, got %d", h)
	}
}

// -- End-to-end: Train → PrepareUpdate → SubmitGradient -----------

func TestEndToEnd_TrainAndSubmit(t *testing.T) {
	// Set up server
	cfg := DefaultFedConfig()
	cfg.MinSchools = 1
	cfg.DPEpsilon = 100.0
	cfg.ClipNorm = 100.0
	server := NewFedAvgServer(cfg)
	server.RegisterSchool("school-001", "School A")

	// Set up trainer
	tcfg := DefaultLocalTrainerConfig("school-001")
	tcfg.ModelDim = 8
	tcfg.Epochs = 1
	trainer, err := NewLocalTrainer(tcfg, testKey())
	if err != nil {
		t.Fatalf("create trainer failed: %v", err)
	}

	// Add data and train
	trainer.AddTrainingPairs([]TrainingPair{
		{Query: "Q1", Passage: "P1", Score: 0.9, KPID: 1},
		{Query: "Q2", Passage: "P2", Score: 0.8, KPID: 2},
	})

	_, err = trainer.Train()
	if err != nil {
		t.Fatalf("train failed: %v", err)
	}

	// Prepare and submit update
	update, err := trainer.PrepareUpdate(0)
	if err != nil {
		t.Fatalf("prepare update failed: %v", err)
	}

	err = server.SubmitGradient(update)
	if err != nil {
		t.Fatalf("submit gradient failed: %v", err)
	}

	// Try aggregation (should succeed with MinSchools=1)
	weights, err := server.TryAggregate()
	if err != nil {
		t.Fatalf("aggregate failed: %v", err)
	}
	if weights == nil {
		t.Fatal("expected non-nil weights")
	}

	// Apply global weights back to trainer
	err = trainer.ApplyGlobalWeights(weights)
	if err != nil {
		t.Fatalf("apply global weights failed: %v", err)
	}
}
