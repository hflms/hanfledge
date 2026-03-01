package asr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// -- DefaultASRConfig ---------------------------------------------

func TestDefaultASRConfig(t *testing.T) {
	cfg := DefaultASRConfig()

	if cfg.Provider != "whisper" {
		t.Errorf("expected provider=whisper, got %q", cfg.Provider)
	}
	if cfg.WhisperURL != "http://localhost:9000" {
		t.Errorf("expected WhisperURL=http://localhost:9000, got %q", cfg.WhisperURL)
	}
	if cfg.ModelSize != "large-v3" {
		t.Errorf("expected ModelSize=large-v3, got %q", cfg.ModelSize)
	}
	if cfg.APIKey != "" {
		t.Errorf("expected empty APIKey, got %q", cfg.APIKey)
	}
}

// -- DefaultTranscribeConfig --------------------------------------

func TestDefaultTranscribeConfig(t *testing.T) {
	cfg := DefaultTranscribeConfig()

	if cfg.Language != "zh-CN" {
		t.Errorf("expected Language=zh-CN, got %q", cfg.Language)
	}
	if cfg.SampleRate != 16000 {
		t.Errorf("expected SampleRate=16000, got %d", cfg.SampleRate)
	}
	if cfg.Format != "webm" {
		t.Errorf("expected Format=webm, got %q", cfg.Format)
	}
	if !cfg.EnablePunctuation {
		t.Error("expected EnablePunctuation=true")
	}
}

// -- NewWhisperProvider -------------------------------------------

func TestNewWhisperProvider(t *testing.T) {
	cfg := DefaultASRConfig()
	p := NewWhisperProvider(cfg)

	if p == nil {
		t.Fatal("expected non-nil WhisperProvider")
	}
	if p.config.Provider != "whisper" {
		t.Errorf("expected provider=whisper, got %q", p.config.Provider)
	}
	if p.client == nil {
		t.Error("expected non-nil http.Client")
	}
	if p.client.Timeout != 120*time.Second {
		t.Errorf("expected timeout=120s, got %v", p.client.Timeout)
	}
}

// -- Transcribe (with httptest mock) ------------------------------

func TestTranscribe_Success(t *testing.T) {
	// Mock Whisper API server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/asr" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// Verify Content-Type is multipart
		ct := r.Header.Get("Content-Type")
		if len(ct) == 0 {
			t.Error("expected Content-Type header")
		}

		resp := whisperResponse{
			Text:     "你好世界",
			Language: "zh",
			Duration: 2.5,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := ASRConfig{
		Provider:   "whisper",
		WhisperURL: srv.URL,
		ModelSize:  "base",
	}
	p := NewWhisperProvider(cfg)

	tcfg := DefaultTranscribeConfig()
	audio := []byte("fake audio data")

	result, err := p.Transcribe(context.Background(), audio, tcfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Text != "你好世界" {
		t.Errorf("expected text=你好世界, got %q", result.Text)
	}
	if result.Language != "zh" {
		t.Errorf("expected language=zh, got %q", result.Language)
	}
	if result.Duration != 2.5 {
		t.Errorf("expected duration=2.5, got %f", result.Duration)
	}
	if result.Confidence != 1.0 {
		t.Errorf("expected confidence=1.0, got %f", result.Confidence)
	}
}

func TestTranscribe_WithAPIKey(t *testing.T) {
	var receivedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		resp := whisperResponse{Text: "test", Language: "en", Duration: 1.0}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := ASRConfig{
		Provider:   "whisper",
		WhisperURL: srv.URL,
		APIKey:     "test-key-123",
		ModelSize:  "base",
	}
	p := NewWhisperProvider(cfg)

	_, err := p.Transcribe(context.Background(), []byte("audio"), DefaultTranscribeConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedAuth != "Bearer test-key-123" {
		t.Errorf("expected Authorization=Bearer test-key-123, got %q", receivedAuth)
	}
}

func TestTranscribe_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("model not loaded"))
	}))
	defer srv.Close()

	cfg := ASRConfig{WhisperURL: srv.URL, ModelSize: "base"}
	p := NewWhisperProvider(cfg)

	_, err := p.Transcribe(context.Background(), []byte("audio"), DefaultTranscribeConfig())
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestTranscribe_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second) // Simulate slow server
		json.NewEncoder(w).Encode(whisperResponse{Text: "late"})
	}))
	defer srv.Close()

	cfg := ASRConfig{WhisperURL: srv.URL, ModelSize: "base"}
	p := NewWhisperProvider(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := p.Transcribe(ctx, []byte("audio"), DefaultTranscribeConfig())
	if err == nil {
		t.Error("expected error when context is cancelled")
	}
}

// -- StreamTranscribe ---------------------------------------------

func TestStreamTranscribe_ProcessesChunks(t *testing.T) {
	// Mock Whisper server that returns text for any audio
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := whisperResponse{Text: "chunk", Language: "zh", Duration: 0.5}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := ASRConfig{WhisperURL: srv.URL, ModelSize: "base"}
	p := NewWhisperProvider(cfg)

	audioStream := make(chan []byte, 5)
	tcfg := DefaultTranscribeConfig()

	events, err := p.StreamTranscribe(context.Background(), audioStream, tcfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Send enough data to trigger a batch (batchSize = 32000)
	bigChunk := make([]byte, 32000)
	audioStream <- bigChunk
	close(audioStream)

	// Collect events
	var received []TranscribeEvent
	for ev := range events {
		received = append(received, ev)
	}

	if len(received) == 0 {
		t.Error("expected at least one transcribe event")
	}
}

// -- ASRProvider interface check -----------------------------------

func TestWhisperProviderImplementsASRProvider(t *testing.T) {
	var _ ASRProvider = (*WhisperProvider)(nil)
}
