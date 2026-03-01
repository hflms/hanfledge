package asr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/hflms/hanfledge/internal/infrastructure/logger"
)

var slogASR = logger.L("ASR")

// ============================
// ASR 语音识别基础设施
// ============================

// -- Interfaces ------------------------------------------------

// ASRProvider defines the interface for speech-to-text services.
type ASRProvider interface {
	// Transcribe converts audio data to text.
	Transcribe(ctx context.Context, audio []byte, config TranscribeConfig) (*TranscribeResult, error)
	// StreamTranscribe handles streaming audio input.
	StreamTranscribe(ctx context.Context, audioStream <-chan []byte, config TranscribeConfig) (<-chan TranscribeEvent, error)
}

// -- Configuration Types ---------------------------------------

// TranscribeConfig holds ASR request configuration.
type TranscribeConfig struct {
	Language          string // "zh-CN", "en-US"
	SampleRate        int    // Audio sample rate (16000, 44100)
	Format            string // "pcm", "wav", "mp3", "webm"
	EnablePunctuation bool
}

// ASRConfig holds ASR service configuration.
type ASRConfig struct {
	Provider   string // "whisper" | "dashscope" | "local"
	WhisperURL string // Whisper API endpoint
	APIKey     string
	ModelSize  string // "tiny" | "base" | "small" | "medium" | "large-v3"
}

// -- Result Types ----------------------------------------------

// TranscribeResult holds the final transcription result.
type TranscribeResult struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
	Duration   float64 `json:"duration_seconds"`
	Language   string  `json:"language"`
}

// TranscribeEvent represents a streaming transcription event.
type TranscribeEvent struct {
	Type       string  `json:"type"` // "partial" | "final" | "error"
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
	IsFinal    bool    `json:"is_final"`
}

// -- Default Configurations ------------------------------------

// DefaultASRConfig returns the default ASR configuration.
// Uses Whisper on localhost:9000 with the large-v3 model.
func DefaultASRConfig() ASRConfig {
	return ASRConfig{
		Provider:   "whisper",
		WhisperURL: "http://localhost:9000",
		APIKey:     "",
		ModelSize:  "large-v3",
	}
}

// DefaultTranscribeConfig returns the default transcription configuration.
// Configured for Chinese with 16kHz sample rate and webm format.
func DefaultTranscribeConfig() TranscribeConfig {
	return TranscribeConfig{
		Language:          "zh-CN",
		SampleRate:        16000,
		Format:            "webm",
		EnablePunctuation: true,
	}
}

// -- Whisper Provider ------------------------------------------

// whisperResponse represents the JSON response from Whisper API.
type whisperResponse struct {
	Text     string  `json:"text"`
	Language string  `json:"language"`
	Duration float64 `json:"duration"`
}

// WhisperProvider implements ASRProvider using a Whisper-compatible API.
type WhisperProvider struct {
	config ASRConfig
	client *http.Client
}

// NewWhisperProvider creates a new WhisperProvider with the given configuration.
func NewWhisperProvider(config ASRConfig) *WhisperProvider {
	slogASR.Info("initializing whisper provider", "url", config.WhisperURL, "model", config.ModelSize)
	return &WhisperProvider{
		config: config,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Transcribe converts audio data to text by sending it to the Whisper API.
// The audio is uploaded as multipart form data to the /asr endpoint.
func (w *WhisperProvider) Transcribe(ctx context.Context, audio []byte, config TranscribeConfig) (*TranscribeResult, error) {
	slogASR.Debug("transcribing audio", "bytes", len(audio), "format", config.Format, "lang", config.Language)

	// Build multipart form body
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add audio file part
	part, err := writer.CreateFormFile("audio_file", fmt.Sprintf("audio.%s", config.Format))
	if err != nil {
		return nil, fmt.Errorf("creating form file: %w", err)
	}
	if _, err := part.Write(audio); err != nil {
		return nil, fmt.Errorf("writing audio data: %w", err)
	}

	// Add configuration fields
	if err := writer.WriteField("language", config.Language); err != nil {
		return nil, fmt.Errorf("writing language field: %w", err)
	}
	if err := writer.WriteField("output", "json"); err != nil {
		return nil, fmt.Errorf("writing output field: %w", err)
	}
	if config.EnablePunctuation {
		if err := writer.WriteField("word_timestamps", "true"); err != nil {
			return nil, fmt.Errorf("writing word_timestamps field: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("closing multipart writer: %w", err)
	}

	// Build HTTP request
	url := fmt.Sprintf("%s/asr", w.config.WhisperURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	if w.config.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", w.config.APIKey))
	}

	// Send request
	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request to Whisper API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Whisper API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var whisperResp whisperResponse
	if err := json.NewDecoder(resp.Body).Decode(&whisperResp); err != nil {
		return nil, fmt.Errorf("decoding Whisper response: %w", err)
	}

	result := &TranscribeResult{
		Text:       whisperResp.Text,
		Confidence: 1.0, // Whisper doesn't return confidence by default
		Duration:   whisperResp.Duration,
		Language:   whisperResp.Language,
	}

	slogASR.Info("transcription complete", "chars", len(result.Text), "duration_s", result.Duration)
	return result, nil
}

// StreamTranscribe handles streaming audio input by batching chunks and
// sending them to the Whisper API. Results are returned through an events channel.
func (w *WhisperProvider) StreamTranscribe(ctx context.Context, audioStream <-chan []byte, config TranscribeConfig) (<-chan TranscribeEvent, error) {
	slogASR.Debug("starting stream transcription", "format", config.Format, "lang", config.Language)

	events := make(chan TranscribeEvent, 16)

	go func() {
		defer close(events)

		var buffer []byte
		const batchSize = 32000 // ~1 second of 16kHz 16-bit audio

		for {
			select {
			case <-ctx.Done():
				slogASR.Debug("stream transcription cancelled")
				return

			case chunk, ok := <-audioStream:
				if !ok {
					// Channel closed — process remaining buffer
					if len(buffer) > 0 {
						w.processChunk(ctx, buffer, config, events, true)
					}
					return
				}

				buffer = append(buffer, chunk...)

				// Process when buffer reaches batch size
				if len(buffer) >= batchSize {
					batch := make([]byte, len(buffer))
					copy(batch, buffer)
					buffer = buffer[:0]

					w.processChunk(ctx, batch, config, events, false)
				}
			}
		}
	}()

	return events, nil
}

// processChunk transcribes a single audio chunk and sends the result to the events channel.
func (w *WhisperProvider) processChunk(ctx context.Context, chunk []byte, config TranscribeConfig, events chan<- TranscribeEvent, isFinal bool) {
	result, err := w.Transcribe(ctx, chunk, config)
	if err != nil {
		slogASR.Warn("chunk transcription failed", "err", err)
		select {
		case events <- TranscribeEvent{
			Type: "error",
			Text: err.Error(),
		}:
		case <-ctx.Done():
		}
		return
	}

	if result.Text == "" {
		return
	}

	eventType := "partial"
	if isFinal {
		eventType = "final"
	}

	select {
	case events <- TranscribeEvent{
		Type:       eventType,
		Text:       result.Text,
		Confidence: result.Confidence,
		IsFinal:    isFinal,
	}:
	case <-ctx.Done():
	}
}
