package federated

import (
	"context"
	"fmt"
	"math"
	randv2 "math/rand/v2"
	"time"

	"gorm.io/gorm"

	"github.com/hflms/hanfledge/internal/infrastructure/logger"
)

var slogTrainer = logger.L("Trainer")

// ============================
// Local Trainer (§8.4.2)
// ============================
//
// 职责：在每个学校节点本地运行训练，收集"学生提问 ↔ 教材段落"
// 配对数据，产生梯度更新，加密后提交给联邦聚合服务器。
//
// Reference: design.md §8.4.2

// -- Training Data Types ------------------------------------------

// TrainingPair represents a student-query to textbook-passage pair
// used as local training data at each school node.
type TrainingPair struct {
	Query   string  `json:"query"`   // student query
	Passage string  `json:"passage"` // textbook passage
	Score   float64 `json:"score"`   // relevance score [0,1]
	KPID    uint    `json:"kp_id"`   // knowledge point ID
}

// -- Configuration ------------------------------------------------

// LocalTrainerConfig holds configuration for local training at a school node.
type LocalTrainerConfig struct {
	SchoolID     string  // school identifier
	BatchSize    int     // training batch size
	Epochs       int     // number of local training epochs
	LearningRate float64 // local learning rate
	ModelDim     int     // embedding dimension (1024 for bge-m3)
}

// DefaultLocalTrainerConfig returns sensible defaults for local training.
func DefaultLocalTrainerConfig(schoolID string) LocalTrainerConfig {
	return LocalTrainerConfig{
		SchoolID:     schoolID,
		BatchSize:    32,
		Epochs:       3,
		LearningRate: 0.001,
		ModelDim:     1024,
	}
}

// -- Local Trainer ------------------------------------------------

// LocalTrainer runs training at each school node. It collects local
// training data, produces gradient updates, encrypts them, and prepares
// them for submission to the federated aggregation server.
type LocalTrainer struct {
	config    LocalTrainerConfig
	encryptor *GradientEncryptor
	pairs     []TrainingPair
	gradients []float64
}

// NewLocalTrainer creates a new local trainer for a school node.
func NewLocalTrainer(config LocalTrainerConfig, encryptionKey []byte) (*LocalTrainer, error) {
	enc, err := NewGradientEncryptor(encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("new local trainer: %w", err)
	}

	slogTrainer.Info("local trainer initialized",
		"schoolID", config.SchoolID, "dim", config.ModelDim, "epochs", config.Epochs, "lr", config.LearningRate)

	return &LocalTrainer{
		config:    config,
		encryptor: enc,
	}, nil
}

// -- Training Data Collection -------------------------------------

// AddTrainingPairs appends local training data to the trainer.
func (t *LocalTrainer) AddTrainingPairs(pairs []TrainingPair) {
	t.pairs = append(t.pairs, pairs...)
	slogTrainer.Debug("added training pairs",
		"count", len(pairs), "schoolID", t.config.SchoolID, "total", len(t.pairs))
}

// trainingRow maps to a row from the training data collection query.
type trainingRow struct {
	Query   string
	Passage string
	Score   float64
	KPID    uint
}

// CollectTrainingPairs collects query-passage training pairs from the database
// for the given school. It joins student interactions with document chunks
// to produce local training data.
func (t *LocalTrainer) CollectTrainingPairs(ctx context.Context, db *gorm.DB, schoolID string) ([]TrainingPair, error) {
	slogTrainer.Debug("collecting training pairs from DB", "schoolID", schoolID)

	// Query student interactions paired with relevant document chunks
	// Using raw SQL scanning to avoid GORM model dependencies outside this package
	query := `
		SELECT
			i.content AS query,
			dc.content AS passage,
			COALESCE(i.relevance_score, 0.5) AS score,
			COALESCE(ss.current_kp, 0) AS kp_id
		FROM interactions i
		JOIN student_sessions ss ON ss.id = i.session_id
		JOIN learning_activities la ON la.id = ss.activity_id
		JOIN document_chunks dc ON dc.course_id = la.course_id
		JOIN schools s ON s.id = la.school_id
		WHERE i.role = 'student'
			AND s.external_id = ?
			AND LENGTH(i.content) > 10
			AND LENGTH(dc.content) > 50
		ORDER BY i.created_at DESC
		LIMIT 5000
	`

	rows, err := db.WithContext(ctx).Raw(query, schoolID).Rows()
	if err != nil {
		return nil, fmt.Errorf("collect training pairs: query failed: %w", err)
	}
	defer rows.Close()

	var pairs []TrainingPair
	for rows.Next() {
		var r trainingRow
		if err := rows.Scan(&r.Query, &r.Passage, &r.Score, &r.KPID); err != nil {
			slogTrainer.Warn("skipping row scan error", "error", err)
			continue
		}
		pairs = append(pairs, TrainingPair{
			Query:   r.Query,
			Passage: r.Passage,
			Score:   r.Score,
			KPID:    r.KPID,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("collect training pairs: row iteration failed: %w", err)
	}

	slogTrainer.Info("collected training pairs", "count", len(pairs), "schoolID", schoolID)
	return pairs, nil
}

// -- Local Training -----------------------------------------------

// Train simulates local training on the collected training pairs and produces
// gradient updates. In a production setting, this would run actual backpropagation
// on the local embedding model; here we simulate gradient computation based on
// the training data statistics.
func (t *LocalTrainer) Train() ([]float64, error) {
	if len(t.pairs) == 0 {
		return nil, fmt.Errorf("train: no training pairs available for school %q", t.config.SchoolID)
	}

	slogTrainer.Info("starting local training",
		"schoolID", t.config.SchoolID, "pairs", len(t.pairs), "epochs", t.config.Epochs)

	dim := t.config.ModelDim
	gradients := make([]float64, dim)

	// Simulate training over epochs
	for epoch := 0; epoch < t.config.Epochs; epoch++ {
		epochGrad := make([]float64, dim)

		// Process in batches
		for batchStart := 0; batchStart < len(t.pairs); batchStart += t.config.BatchSize {
			batchEnd := batchStart + t.config.BatchSize
			if batchEnd > len(t.pairs) {
				batchEnd = len(t.pairs)
			}
			batch := t.pairs[batchStart:batchEnd]

			// Simulate batch gradient computation
			batchGrad := t.computeBatchGradient(batch, dim)
			for i := range epochGrad {
				epochGrad[i] += batchGrad[i]
			}
		}

		// Average over batches
		numBatches := float64((len(t.pairs) + t.config.BatchSize - 1) / t.config.BatchSize)
		for i := range epochGrad {
			epochGrad[i] /= numBatches
		}

		// Accumulate epoch gradients
		for i := range gradients {
			gradients[i] += epochGrad[i]
		}

		slogTrainer.Debug("epoch complete",
			"schoolID", t.config.SchoolID, "epoch", epoch+1, "totalEpochs", t.config.Epochs)
	}

	// Average over epochs
	for i := range gradients {
		gradients[i] /= float64(t.config.Epochs)
	}

	t.gradients = gradients
	slogTrainer.Info("local training complete",
		"schoolID", t.config.SchoolID, "dims", dim)

	return gradients, nil
}

// computeBatchGradient simulates gradient computation for a mini-batch.
// Uses training pair scores to modulate gradient magnitudes, producing
// a deterministic-yet-data-dependent gradient signal.
func (t *LocalTrainer) computeBatchGradient(batch []TrainingPair, dim int) []float64 {
	grad := make([]float64, dim)

	for _, pair := range batch {
		// Use content features to seed gradient direction
		queryHash := simpleHash(pair.Query)
		passageHash := simpleHash(pair.Passage)

		for d := 0; d < dim; d++ {
			// Data-dependent gradient component
			signal := math.Sin(float64(queryHash+uint64(d))) * math.Cos(float64(passageHash+uint64(d)))
			// Scale by relevance score and learning rate
			grad[d] += signal * pair.Score * t.config.LearningRate
		}
	}

	// Average over batch
	batchSize := float64(len(batch))
	if batchSize > 0 {
		for i := range grad {
			grad[i] /= batchSize
		}
	}

	return grad
}

// simpleHash computes a simple hash of a string for gradient seeding.
func simpleHash(s string) uint64 {
	var h uint64
	for _, c := range s {
		h = h*31 + uint64(c)
	}
	return h
}

// -- Update Preparation -------------------------------------------

// PrepareUpdate encrypts the local gradients and builds a GradientUpdate
// ready for submission to the aggregation server.
func (t *LocalTrainer) PrepareUpdate(roundID int) (*GradientUpdate, error) {
	if len(t.gradients) == 0 {
		return nil, fmt.Errorf("prepare update: no gradients available (run Train first)")
	}

	// Encrypt gradients for secure transmission
	encrypted, err := t.encryptor.Encrypt(t.gradients)
	if err != nil {
		return nil, fmt.Errorf("prepare update: encryption failed: %w", err)
	}

	// Decrypt to get back the float64 values for the update structure
	// (The GradientUpdate carries the values; in a real system these would
	// travel encrypted on the wire and be decrypted server-side.)
	decrypted, err := t.encryptor.Decrypt(encrypted)
	if err != nil {
		return nil, fmt.Errorf("prepare update: verification decrypt failed: %w", err)
	}

	checksum := computeChecksum(decrypted)

	update := &GradientUpdate{
		SchoolID:    t.config.SchoolID,
		RoundID:     roundID,
		Gradients:   decrypted,
		SampleCount: len(t.pairs),
		Timestamp:   time.Now(),
		Checksum:    checksum,
	}

	slogTrainer.Debug("prepared update",
		"schoolID", t.config.SchoolID, "round", roundID, "dims", len(decrypted), "samples", len(t.pairs))

	return update, nil
}

// -- Global Weights Application -----------------------------------

// ApplyGlobalWeights applies the received global model weights from the
// aggregation server. In a production setting, this would update the local
// embedding model parameters.
func (t *LocalTrainer) ApplyGlobalWeights(weights *GlobalWeights) error {
	if weights == nil {
		return fmt.Errorf("apply global weights: weights must not be nil")
	}

	if len(weights.Weights) != t.config.ModelDim {
		return fmt.Errorf("apply global weights: dimension mismatch: got %d, expected %d",
			len(weights.Weights), t.config.ModelDim)
	}

	// Apply global weight update to local gradients
	// In production, this would modify the local model parameters
	if t.gradients == nil {
		t.gradients = make([]float64, t.config.ModelDim)
	}

	for i := range t.gradients {
		// Blend local gradients with global weights using momentum
		t.gradients[i] = 0.9*t.gradients[i] + 0.1*weights.Weights[i]
	}

	// Add small perturbation to break symmetry across schools
	for i := range t.gradients {
		t.gradients[i] += randv2.NormFloat64() * 1e-6
	}

	slogTrainer.Info("applied global weights",
		"round", weights.RoundID, "schoolID", t.config.SchoolID)

	return nil
}
