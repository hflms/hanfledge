package logger

import (
	"context"
	"log/slog"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected slog.Level
	}{
		{"debug", "debug", slog.LevelDebug},
		{"info", "info", slog.LevelInfo},
		{"warn", "warn", slog.LevelWarn},
		{"warning", "warning", slog.LevelWarn},
		{"error", "error", slog.LevelError},
		{"empty", "", slog.LevelInfo},
		{"invalid", "unknown", slog.LevelInfo},
		{"spaces", "  debug  ", slog.LevelDebug},
		{"uppercase", "ERROR", slog.LevelError},
		{"mixed case", "WaRn", slog.LevelWarn},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseLevel(tt.input)
			if result != tt.expected {
				t.Errorf("parseLevel(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestInit(t *testing.T) {
	// Calling Init shouldn't panic
	// Note: We test "debug" first in our local sequence just to cover the branch
	// that handles text format, if possible.
	// However, since Go tests run in no particular order unless specified,
	// and Init uses sync.Once, it might have already been initialized.
	// We can at least ensure we don't panic and we have a logger.
	Init("debug")
	if defaultLogger == nil {
		t.Fatal("Expected defaultLogger to be initialized, got nil")
	}

	// Calling it multiple times shouldn't panic or change anything
	Init("error")
	if defaultLogger == nil {
		t.Fatal("Expected defaultLogger to remain initialized, got nil")
	}
}

func TestEnsureInit(t *testing.T) {
	// Call ensureInit to ensure it handles the already-initialized case
	// gracefully without panics.
	ensureInit()
	if defaultLogger == nil {
		t.Fatal("Expected defaultLogger to be initialized, got nil")
	}
}

func TestComponentLogger(t *testing.T) {
	// This relies on Init or ensureInit having run
	log := L("test-component")
	if log == nil {
		t.Fatal("Expected component logger to be initialized, got nil")
	}
}

func TestConvenienceWrappers(t *testing.T) {
	// These shouldn't panic when called
	Debug("test debug message")
	Info("test info message")
	Warn("test warn message")
	Error("test error message")

	ctx := context.Background()
	InfoContext(ctx, "test info context")
	WarnContext(ctx, "test warn context")
	ErrorContext(ctx, "test error context")

	// Note: We cannot test Fatal easily because it calls os.Exit(1).
	// We will skip testing Fatal directly here to avoid aborting the test runner.
}
