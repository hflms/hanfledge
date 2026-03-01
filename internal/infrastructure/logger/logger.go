// Package logger provides structured logging for the Hanfledge application
// using Go's log/slog package. It wraps slog with convenience helpers for
// component-scoped loggers and fatal-level logging.
//
// Usage:
//
//	logger.Init("debug")                         // call once at startup
//	log := logger.L("KA-RAG")                    // per-component logger
//	log.Info("slicing document", "doc", name)     // structured fields
//	log.Warn("embedding failed", "err", err)
//	logger.Fatal("server failed to start", "err", err)  // logs + os.Exit(1)
package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// -- Package State --------------------------------------------------------

var (
	// defaultLogger is the package-level logger, initialized by Init().
	defaultLogger *slog.Logger
	initOnce      sync.Once
)

// -- Initialization -------------------------------------------------------

// Init configures the global slog logger based on the given level string.
// Valid levels: "debug", "info", "warn", "error". The level string is
// case-insensitive. If the level is unrecognized, it defaults to Info.
//
// In debug mode, the text handler uses a human-readable format.
// In other modes, the JSON handler is used for machine-parseable output.
//
// Init is safe to call multiple times; only the first call takes effect.
func Init(level string) {
	initOnce.Do(func() {
		lvl := parseLevel(level)

		opts := &slog.HandlerOptions{
			Level: lvl,
		}

		var handler slog.Handler
		if lvl == slog.LevelDebug {
			// Human-readable text output for development
			handler = slog.NewTextHandler(os.Stdout, opts)
		} else {
			// JSON output for production (log aggregation friendly)
			handler = slog.NewJSONHandler(os.Stdout, opts)
		}

		defaultLogger = slog.New(handler)
		slog.SetDefault(defaultLogger)
	})
}

// -- Component Logger -----------------------------------------------------

// L returns a child logger with the given component name as a structured
// attribute. Each subsystem should call this once and reuse the logger:
//
//	var log = logger.L("Coach")
//
// The component name appears as "component":"Coach" in structured output.
func L(component string) *slog.Logger {
	ensureInit()
	return defaultLogger.With("component", component)
}

// -- Convenience Wrappers -------------------------------------------------

// These wrap the default logger for callers that don't need a component scope.

// Debug logs at Debug level on the default logger.
func Debug(msg string, args ...any) {
	ensureInit()
	defaultLogger.Debug(msg, args...)
}

// Info logs at Info level on the default logger.
func Info(msg string, args ...any) {
	ensureInit()
	defaultLogger.Info(msg, args...)
}

// Warn logs at Warn level on the default logger.
func Warn(msg string, args ...any) {
	ensureInit()
	defaultLogger.Warn(msg, args...)
}

// Error logs at Error level on the default logger.
func Error(msg string, args ...any) {
	ensureInit()
	defaultLogger.Error(msg, args...)
}

// Fatal logs at Error level and then calls os.Exit(1).
// Use this only for unrecoverable startup failures.
func Fatal(msg string, args ...any) {
	ensureInit()
	defaultLogger.Error(msg, args...)
	os.Exit(1)
}

// InfoContext logs at Info level with context.
func InfoContext(ctx context.Context, msg string, args ...any) {
	ensureInit()
	defaultLogger.InfoContext(ctx, msg, args...)
}

// WarnContext logs at Warn level with context.
func WarnContext(ctx context.Context, msg string, args ...any) {
	ensureInit()
	defaultLogger.WarnContext(ctx, msg, args...)
}

// ErrorContext logs at Error level with context.
func ErrorContext(ctx context.Context, msg string, args ...any) {
	ensureInit()
	defaultLogger.ErrorContext(ctx, msg, args...)
}

// -- Helpers --------------------------------------------------------------

// parseLevel converts a level string to a slog.Level.
func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// ensureInit guarantees a default logger exists even if Init() was never called.
func ensureInit() {
	if defaultLogger == nil {
		Init("info")
	}
}

// -- Level Mapping Guide --------------------------------------------------
//
// GIN_MODE → slog level mapping (set in main.go):
//   "debug"   → logger.Init("debug")   — all logs emitted
//   "test"    → logger.Init("info")    — debug suppressed
//   "release" → logger.Init("info")    — debug suppressed, JSON output
//
// Emoji prefix convention preserved in msg strings for visual scanning:
//   Fatal:  "server failed to start"       (no emoji, slog level is sufficient)
//   Error:  "KA-RAG pipeline failed"
//   Warn:   "embedding chunk failed"
//   Info:   "document processing complete"
//   Debug:  "CoT reasoning trace"          (suppressed in production)
