package federated

import (
	"math"
	"testing"
)

// -- ClipGradient -------------------------------------------------

func TestClipGradient_NoClipNeeded(t *testing.T) {
	dp := NewDPMechanism(1.0, 1e-5, 10.0)
	gradients := []float64{1.0, 2.0, 3.0} // L2 norm ≈ 3.74 < 10.0

	clipped := dp.ClipGradient(gradients)

	for i, g := range gradients {
		if math.Abs(clipped[i]-g) > 1e-9 {
			t.Errorf("index %d: expected %f (no clip), got %f", i, g, clipped[i])
		}
	}
}

func TestClipGradient_ClipsLargeGradient(t *testing.T) {
	dp := NewDPMechanism(1.0, 1e-5, 1.0)
	gradients := []float64{3.0, 4.0} // L2 norm = 5.0, should be clipped to norm=1.0

	clipped := dp.ClipGradient(gradients)

	// After clipping, L2 norm should be ≈ 1.0
	var norm float64
	for _, g := range clipped {
		norm += g * g
	}
	norm = math.Sqrt(norm)

	if math.Abs(norm-1.0) > 1e-9 {
		t.Errorf("expected clipped norm=1.0, got %f", norm)
	}

	// Direction should be preserved (proportional scaling)
	ratio := clipped[0] / clipped[1]
	expected := 3.0 / 4.0
	if math.Abs(ratio-expected) > 1e-9 {
		t.Errorf("expected ratio=%f, got %f (direction not preserved)", expected, ratio)
	}
}

func TestClipGradient_EmptyInput(t *testing.T) {
	dp := NewDPMechanism(1.0, 1e-5, 1.0)
	result := dp.ClipGradient(nil)
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}

	result = dp.ClipGradient([]float64{})
	if result != nil {
		t.Errorf("expected nil for empty input, got %v", result)
	}
}

func TestClipGradient_DoesNotMutateOriginal(t *testing.T) {
	dp := NewDPMechanism(1.0, 1e-5, 1.0)
	original := []float64{3.0, 4.0}
	originalCopy := make([]float64, len(original))
	copy(originalCopy, original)

	dp.ClipGradient(original)

	for i := range original {
		if original[i] != originalCopy[i] {
			t.Errorf("original modified at index %d: %f -> %f", i, originalCopy[i], original[i])
		}
	}
}

// -- AddGaussianNoise ---------------------------------------------

func TestAddGaussianNoise_ProducesNoisyOutput(t *testing.T) {
	dp := NewDPMechanism(1.0, 1e-5, 1.0)
	gradients := []float64{1.0, 2.0, 3.0}

	noisy := dp.AddGaussianNoise(gradients, 100)

	if len(noisy) != len(gradients) {
		t.Fatalf("expected %d elements, got %d", len(gradients), len(noisy))
	}

	// With Gaussian noise, at least one element should differ
	allSame := true
	for i := range gradients {
		if math.Abs(noisy[i]-gradients[i]) > 1e-15 {
			allSame = false
			break
		}
	}
	if allSame {
		t.Error("expected noisy output to differ from input")
	}
}

func TestAddGaussianNoise_EmptyInput(t *testing.T) {
	dp := NewDPMechanism(1.0, 1e-5, 1.0)
	result := dp.AddGaussianNoise(nil, 100)
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}
}

func TestAddGaussianNoise_DoesNotMutateOriginal(t *testing.T) {
	dp := NewDPMechanism(1.0, 1e-5, 1.0)
	original := []float64{1.0, 2.0, 3.0}
	originalCopy := make([]float64, len(original))
	copy(originalCopy, original)

	dp.AddGaussianNoise(original, 100)

	for i := range original {
		if original[i] != originalCopy[i] {
			t.Errorf("original modified at index %d", i)
		}
	}
}

// -- Sanitize -----------------------------------------------------

func TestSanitize_AppliesBothClipAndNoise(t *testing.T) {
	dp := NewDPMechanism(1.0, 1e-5, 1.0)
	gradients := []float64{3.0, 4.0} // Large gradient, norm = 5.0

	sanitized := dp.Sanitize(gradients, 100)

	if len(sanitized) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(sanitized))
	}

	// The output should be different from the original (clipped + noised)
	if sanitized[0] == 3.0 && sanitized[1] == 4.0 {
		t.Error("expected sanitized output to differ from input")
	}
}

// -- Encrypt / Decrypt round-trip ---------------------------------

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	enc, err := NewGradientEncryptor(key)
	if err != nil {
		t.Fatalf("NewGradientEncryptor failed: %v", err)
	}

	gradients := []float64{1.5, -2.3, 0.0, 3.14159, -99.99}

	ciphertext, err := enc.Encrypt(gradients)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if len(ciphertext) == 0 {
		t.Fatal("expected non-empty ciphertext")
	}

	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if len(decrypted) != len(gradients) {
		t.Fatalf("expected %d elements, got %d", len(gradients), len(decrypted))
	}

	for i := range gradients {
		if math.Abs(decrypted[i]-gradients[i]) > 1e-15 {
			t.Errorf("index %d: expected %f, got %f", i, gradients[i], decrypted[i])
		}
	}
}

func TestEncryptDecrypt_LargeVector(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 10)
	}

	enc, err := NewGradientEncryptor(key)
	if err != nil {
		t.Fatalf("NewGradientEncryptor failed: %v", err)
	}

	// 1024-dim vector (bge-m3 embedding dimension)
	gradients := make([]float64, 1024)
	for i := range gradients {
		gradients[i] = float64(i) * 0.001
	}

	ciphertext, err := enc.Encrypt(gradients)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	for i := range gradients {
		if math.Abs(decrypted[i]-gradients[i]) > 1e-15 {
			t.Errorf("index %d: expected %f, got %f", i, gradients[i], decrypted[i])
		}
	}
}

// -- NewGradientEncryptor errors ----------------------------------

func TestNewGradientEncryptor_InvalidKeySize(t *testing.T) {
	_, err := NewGradientEncryptor([]byte("short"))
	if err == nil {
		t.Error("expected error for short key")
	}

	_, err = NewGradientEncryptor(make([]byte, 16))
	if err == nil {
		t.Error("expected error for 16-byte key")
	}
}

func TestEncrypt_EmptyGradient(t *testing.T) {
	key := make([]byte, 32)
	enc, _ := NewGradientEncryptor(key)

	_, err := enc.Encrypt(nil)
	if err == nil {
		t.Error("expected error for empty gradients")
	}

	_, err = enc.Encrypt([]float64{})
	if err == nil {
		t.Error("expected error for empty gradients")
	}
}

func TestDecrypt_EmptyCiphertext(t *testing.T) {
	key := make([]byte, 32)
	enc, _ := NewGradientEncryptor(key)

	_, err := enc.Decrypt(nil)
	if err == nil {
		t.Error("expected error for nil ciphertext")
	}

	_, err = enc.Decrypt([]byte{})
	if err == nil {
		t.Error("expected error for empty ciphertext")
	}
}

func TestDecrypt_CorruptedCiphertext(t *testing.T) {
	key := make([]byte, 32)
	enc, _ := NewGradientEncryptor(key)

	gradients := []float64{1.0, 2.0, 3.0}
	ciphertext, err := enc.Encrypt(gradients)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Corrupt the ciphertext
	ciphertext[len(ciphertext)-1] ^= 0xFF

	_, err = enc.Decrypt(ciphertext)
	if err == nil {
		t.Error("expected error for corrupted ciphertext")
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	for i := range key2 {
		key2[i] = 0xFF
	}

	enc1, _ := NewGradientEncryptor(key1)
	enc2, _ := NewGradientEncryptor(key2)

	gradients := []float64{1.0, 2.0, 3.0}
	ciphertext, err := enc1.Encrypt(gradients)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = enc2.Decrypt(ciphertext)
	if err == nil {
		t.Error("expected error when decrypting with wrong key")
	}
}
