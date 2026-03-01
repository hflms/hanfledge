package federated

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	randv2 "math/rand/v2"
)

// ============================
// Differential Privacy & Gradient Encryption (§8.4.2)
// ============================
//
// 职责：
//   1. 差分隐私机制 — 梯度裁剪 + 高斯噪声注入 (DP-SGD)
//   2. 梯度加密 — AES-256-GCM 加密/解密梯度数据
//
// Reference: design.md §8.4.2

// -- Differential Privacy -----------------------------------------

// DPMechanism implements differential privacy mechanisms for gradient sanitization.
// It applies L2 norm clipping followed by calibrated Gaussian noise injection (DP-SGD).
type DPMechanism struct {
	Epsilon  float64 // Privacy budget (smaller = more private)
	Delta    float64 // Failure probability
	ClipNorm float64 // L2 norm clipping threshold
}

// NewDPMechanism creates a new differential privacy mechanism.
func NewDPMechanism(epsilon, delta, clipNorm float64) *DPMechanism {
	return &DPMechanism{
		Epsilon:  epsilon,
		Delta:    delta,
		ClipNorm: clipNorm,
	}
}

// ClipGradient applies L2 norm clipping to a gradient vector.
// If the L2 norm exceeds ClipNorm, the gradient is scaled down proportionally.
// Returns a new slice; the original is not modified.
func (dp *DPMechanism) ClipGradient(gradients []float64) []float64 {
	if len(gradients) == 0 {
		return nil
	}

	// Compute L2 norm
	var norm float64
	for _, g := range gradients {
		norm += g * g
	}
	norm = math.Sqrt(norm)

	clipped := make([]float64, len(gradients))
	copy(clipped, gradients)

	// Scale down if norm exceeds the clip threshold
	if norm > dp.ClipNorm && norm > 0 {
		scale := dp.ClipNorm / norm
		for i := range clipped {
			clipped[i] *= scale
		}
	}

	return clipped
}

// AddGaussianNoise injects calibrated Gaussian noise into a gradient vector
// to satisfy (ε, δ)-differential privacy.
//
// The noise standard deviation is computed as:
//
//	σ = ClipNorm * sqrt(2 * ln(1.25 / δ)) / ε
//
// Returns a new slice; the original is not modified.
func (dp *DPMechanism) AddGaussianNoise(gradients []float64, sampleCount int) []float64 {
	if len(gradients) == 0 {
		return nil
	}

	// Compute noise scale: σ = C * sqrt(2 * ln(1.25/δ)) / ε
	sigma := dp.ClipNorm * math.Sqrt(2.0*math.Log(1.25/dp.Delta)) / dp.Epsilon

	// Scale by inverse of sample count for per-sample privacy
	if sampleCount > 0 {
		sigma /= float64(sampleCount)
	}

	noisy := make([]float64, len(gradients))
	for i, g := range gradients {
		noise := randv2.NormFloat64() * sigma
		noisy[i] = g + noise
	}

	return noisy
}

// Sanitize applies both gradient clipping and Gaussian noise in one step.
// This is the primary entry point for DP-SGD sanitization.
func (dp *DPMechanism) Sanitize(gradients []float64, sampleCount int) []float64 {
	clipped := dp.ClipGradient(gradients)
	return dp.AddGaussianNoise(clipped, sampleCount)
}

// -- Gradient Encryption ------------------------------------------

// GradientEncryptor handles gradient encryption/decryption for secure
// transmission between school nodes and the aggregation server using AES-256-GCM.
type GradientEncryptor struct {
	key []byte // AES-256 key (32 bytes)
}

// NewGradientEncryptor creates a new encryptor with the given AES-256 key.
// The key must be exactly 32 bytes.
func NewGradientEncryptor(key []byte) (*GradientEncryptor, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("new gradient encryptor: key must be 32 bytes, got %d", len(key))
	}

	return &GradientEncryptor{key: key}, nil
}

// Encrypt encrypts a gradient vector using AES-256-GCM.
// Each float64 is encoded as 8 bytes (IEEE 754 little-endian) before encryption.
func (e *GradientEncryptor) Encrypt(gradients []float64) ([]byte, error) {
	if len(gradients) == 0 {
		return nil, fmt.Errorf("encrypt: empty gradient vector")
	}

	// Serialize float64 slice to bytes
	plaintext := make([]byte, len(gradients)*8)
	for i, g := range gradients {
		binary.LittleEndian.PutUint64(plaintext[i*8:], math.Float64bits(g))
	}

	// Create AES cipher
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("encrypt: create cipher failed: %w", err)
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("encrypt: create GCM failed: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("encrypt: generate nonce failed: %w", err)
	}

	// Encrypt: nonce is prepended to ciphertext
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts a ciphertext produced by Encrypt back into a gradient vector.
func (e *GradientEncryptor) Decrypt(ciphertext []byte) ([]float64, error) {
	if len(ciphertext) == 0 {
		return nil, fmt.Errorf("decrypt: empty ciphertext")
	}

	// Create AES cipher
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("decrypt: create cipher failed: %w", err)
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("decrypt: create GCM failed: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("decrypt: ciphertext too short")
	}

	// Extract nonce and decrypt
	nonce, encryptedData := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: decryption failed: %w", err)
	}

	// Deserialize bytes back to float64 slice
	if len(plaintext)%8 != 0 {
		return nil, fmt.Errorf("decrypt: invalid plaintext length %d (not multiple of 8)", len(plaintext))
	}

	gradients := make([]float64, len(plaintext)/8)
	for i := range gradients {
		gradients[i] = math.Float64frombits(binary.LittleEndian.Uint64(plaintext[i*8:]))
	}

	return gradients, nil
}
