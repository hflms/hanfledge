package federated

import (
	"crypto/sha256"
	"fmt"
	"log"
	"sync"
	"time"
)

// ============================
// Federated Averaging Server (§8.4.2)
// ============================
//
// 职责：中央聚合服务器，接收各学校节点的加密梯度更新，
// 执行 Federated Averaging 算法聚合梯度，注入差分隐私噪声，
// 并将全局模型权重分发回各学校。
//
// 事件主题: system.federated.ready — 节点梯度就绪信号
//
// Reference: design.md §8.4.2

// -- Data Types ---------------------------------------------------

// GradientUpdate represents an encrypted gradient update from a school node.
type GradientUpdate struct {
	SchoolID    string    `json:"school_id"`
	RoundID     int       `json:"round_id"`
	Gradients   []float64 `json:"gradients"`    // encrypted gradient values
	SampleCount int       `json:"sample_count"` // number of local training samples
	Timestamp   time.Time `json:"timestamp"`
	Checksum    string    `json:"checksum"` // SHA-256 for integrity verification
}

// GlobalWeights represents the aggregated model weights.
type GlobalWeights struct {
	RoundID     int       `json:"round_id"`
	Weights     []float64 `json:"weights"`
	SchoolCount int       `json:"school_count"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SchoolNode represents a registered school in the federation.
type SchoolNode struct {
	SchoolID     string    `json:"school_id"`
	Name         string    `json:"name"`
	RegisteredAt time.Time `json:"registered_at"`
	LastSeen     time.Time `json:"last_seen"`
	Active       bool      `json:"active"`
}

// -- Configuration ------------------------------------------------

// FedConfig holds configuration for the federated learning server.
type FedConfig struct {
	MinSchools       int           // Minimum schools required to start aggregation
	MaxRoundDuration time.Duration // Max wait time per round
	DPEpsilon        float64       // Differential privacy epsilon (privacy budget)
	DPDelta          float64       // Differential privacy delta
	ClipNorm         float64       // Gradient clipping norm for DP
	LearningRate     float64       // Global learning rate
}

// DefaultFedConfig returns sensible defaults for federated learning.
func DefaultFedConfig() FedConfig {
	return FedConfig{
		MinSchools:       2,
		MaxRoundDuration: 10 * time.Minute,
		DPEpsilon:        1.0,
		DPDelta:          1e-5,
		ClipNorm:         1.0,
		LearningRate:     0.01,
	}
}

// -- FedAvg Server ------------------------------------------------

// FedAvgServer is the central aggregation server that receives encrypted
// gradient updates from school nodes and produces global model weights
// using the Federated Averaging algorithm with differential privacy.
type FedAvgServer struct {
	config         FedConfig
	mu             sync.RWMutex
	currentRound   int
	pendingUpdates map[string]*GradientUpdate // schoolID -> update
	globalWeights  *GlobalWeights
	schools        map[string]*SchoolNode
	dpMechanism    *DPMechanism
}

// NewFedAvgServer creates a new FedAvg aggregation server with the given configuration.
func NewFedAvgServer(config FedConfig) *FedAvgServer {
	log.Printf("🌐 [FedRAG] Initializing FedAvg server (minSchools=%d, ε=%.2f, δ=%.0e)",
		config.MinSchools, config.DPEpsilon, config.DPDelta)

	return &FedAvgServer{
		config:         config,
		currentRound:   0,
		pendingUpdates: make(map[string]*GradientUpdate),
		schools:        make(map[string]*SchoolNode),
		dpMechanism:    NewDPMechanism(config.DPEpsilon, config.DPDelta, config.ClipNorm),
	}
}

// -- School Registration ------------------------------------------

// RegisterSchool registers a new school node in the federation.
func (s *FedAvgServer) RegisterSchool(schoolID, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if schoolID == "" {
		return fmt.Errorf("register school: school ID must not be empty")
	}
	if name == "" {
		return fmt.Errorf("register school: name must not be empty")
	}

	if _, exists := s.schools[schoolID]; exists {
		return fmt.Errorf("register school: school %q already registered", schoolID)
	}

	now := time.Now()
	s.schools[schoolID] = &SchoolNode{
		SchoolID:     schoolID,
		Name:         name,
		RegisteredAt: now,
		LastSeen:     now,
		Active:       true,
	}

	log.Printf("🏫 [FedRAG] School registered: %s (%s), total=%d", schoolID, name, len(s.schools))
	return nil
}

// -- Gradient Submission ------------------------------------------

// SubmitGradient validates and stores a gradient update from a school node.
// Returns an error if the school is not registered, the round ID is stale,
// or the checksum does not match.
func (s *FedAvgServer) SubmitGradient(update *GradientUpdate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if update == nil {
		return fmt.Errorf("submit gradient: update must not be nil")
	}

	// Verify school is registered
	school, exists := s.schools[update.SchoolID]
	if !exists {
		return fmt.Errorf("submit gradient: school %q not registered", update.SchoolID)
	}

	// Verify round ID
	if update.RoundID < s.currentRound {
		return fmt.Errorf("submit gradient: stale round %d (current=%d)", update.RoundID, s.currentRound)
	}

	// Verify gradients are non-empty
	if len(update.Gradients) == 0 {
		return fmt.Errorf("submit gradient: empty gradients from school %q", update.SchoolID)
	}

	// Verify sample count
	if update.SampleCount <= 0 {
		return fmt.Errorf("submit gradient: invalid sample count %d from school %q", update.SampleCount, update.SchoolID)
	}

	// Verify checksum integrity
	if !s.verifyChecksum(update) {
		return fmt.Errorf("submit gradient: checksum mismatch from school %q", update.SchoolID)
	}

	// Store the update
	s.pendingUpdates[update.SchoolID] = update
	school.LastSeen = time.Now()

	log.Printf("🔒 [FedRAG] Gradient received from school %q (round=%d, dims=%d, samples=%d), pending=%d/%d",
		update.SchoolID, update.RoundID, len(update.Gradients), update.SampleCount,
		len(s.pendingUpdates), s.activeSchoolCount())

	return nil
}

// -- Aggregation --------------------------------------------------

// TryAggregate attempts to run Federated Averaging if enough schools have
// submitted gradient updates. Returns the new global weights on success,
// or nil if not enough updates are available yet.
func (s *FedAvgServer) TryAggregate() (*GlobalWeights, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	activeCount := s.activeSchoolCount()

	if len(s.pendingUpdates) < s.config.MinSchools {
		log.Printf("🌐 [FedRAG] Not enough updates for aggregation: %d/%d (min=%d)",
			len(s.pendingUpdates), activeCount, s.config.MinSchools)
		return nil, nil
	}

	// Collect updates
	updates := make([]*GradientUpdate, 0, len(s.pendingUpdates))
	for _, u := range s.pendingUpdates {
		updates = append(updates, u)
	}

	// Verify dimension consistency
	dim := len(updates[0].Gradients)
	for _, u := range updates[1:] {
		if len(u.Gradients) != dim {
			return nil, fmt.Errorf("aggregate: dimension mismatch: school %q has %d, expected %d",
				u.SchoolID, len(u.Gradients), dim)
		}
	}

	log.Printf("🌐 [FedRAG] Starting aggregation round %d with %d schools (dim=%d)",
		s.currentRound, len(updates), dim)

	// Step 1: FedAvg — weighted average by sample count
	aggregated := s.fedAvg(updates)

	// Step 2: Apply differential privacy (clip + Gaussian noise)
	totalSamples := 0
	for _, u := range updates {
		totalSamples += u.SampleCount
	}
	sanitized := s.dpMechanism.Sanitize(aggregated, totalSamples)

	// Step 3: Apply learning rate
	for i := range sanitized {
		sanitized[i] *= s.config.LearningRate
	}

	// Build global weights
	s.currentRound++
	s.globalWeights = &GlobalWeights{
		RoundID:     s.currentRound,
		Weights:     sanitized,
		SchoolCount: len(updates),
		UpdatedAt:   time.Now(),
	}

	// Clear pending updates for next round
	s.pendingUpdates = make(map[string]*GradientUpdate)

	log.Printf("🌐 [FedRAG] Aggregation round %d complete: %d schools, %d dims, %d total samples",
		s.currentRound, len(updates), dim, totalSamples)

	return s.globalWeights, nil
}

// GetGlobalWeights returns the current global model weights.
func (s *FedAvgServer) GetGlobalWeights() *GlobalWeights {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.globalWeights
}

// -- FedAvg Algorithm ---------------------------------------------

// fedAvg computes the weighted average of gradient updates,
// weighted by each school's sample count.
func (s *FedAvgServer) fedAvg(updates []*GradientUpdate) []float64 {
	if len(updates) == 0 {
		return nil
	}

	dim := len(updates[0].Gradients)
	result := make([]float64, dim)

	// Total sample count across all schools
	totalSamples := 0
	for _, u := range updates {
		totalSamples += u.SampleCount
	}

	if totalSamples == 0 {
		return result
	}

	// Weighted average: w_i = n_i / N
	for _, u := range updates {
		weight := float64(u.SampleCount) / float64(totalSamples)
		for j, g := range u.Gradients {
			result[j] += weight * g
		}
	}

	return result
}

// -- Checksum Verification ----------------------------------------

// computeChecksum computes a SHA-256 checksum for a gradient vector.
func computeChecksum(gradients []float64) string {
	h := sha256.New()
	for _, g := range gradients {
		fmt.Fprintf(h, "%.15e", g)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// verifyChecksum verifies the integrity of a gradient update.
func (s *FedAvgServer) verifyChecksum(update *GradientUpdate) bool {
	expected := computeChecksum(update.Gradients)
	return expected == update.Checksum
}

// -- Helpers ------------------------------------------------------

// activeSchoolCount returns the number of active schools.
func (s *FedAvgServer) activeSchoolCount() int {
	count := 0
	for _, school := range s.schools {
		if school.Active {
			count++
		}
	}
	return count
}
