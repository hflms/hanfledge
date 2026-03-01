package handler

import (
	"testing"
	"time"
)

// ============================
// Session Handler Unit Tests
// ============================

// -- WebSocket Constants Tests --------------------------------

func TestWSPongWait(t *testing.T) {
	expected := 60 * time.Second
	if wsPongWait != expected {
		t.Errorf("wsPongWait = %v, want %v", wsPongWait, expected)
	}
}

func TestWSPingInterval(t *testing.T) {
	expected := 30 * time.Second
	if wsPingInterval != expected {
		t.Errorf("wsPingInterval = %v, want %v", wsPingInterval, expected)
	}
}

func TestWSWriteWait(t *testing.T) {
	expected := 10 * time.Second
	if wsWriteWait != expected {
		t.Errorf("wsWriteWait = %v, want %v", wsWriteWait, expected)
	}
}

func TestWSPingIntervalLessThanPongWait(t *testing.T) {
	if wsPingInterval >= wsPongWait {
		t.Errorf("wsPingInterval (%v) must be < wsPongWait (%v)", wsPingInterval, wsPongWait)
	}
}

// -- truncateStr Tests ----------------------------------------

func TestTruncateStr(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{"empty string", "", 10, ""},
		{"shorter than limit", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"exceeds limit", "hello world", 5, "hello..."},
		{"zero max length", "hello", 0, "..."},
		{"single char within limit", "a", 1, "a"},
		{"single char exceeds limit", "ab", 1, "a..."},
		{"chinese text within limit", "你好世界", 4, "你好世界"},
		{"chinese text exceeds limit", "你好世界欢迎", 4, "你好世界..."},
		{"chinese text exact", "你好", 2, "你好"},
		{"mixed content", "hello你好", 5, "hello..."},
		{"emoji handling", "👋🌍🎉", 2, "👋🌍..."},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := truncateStr(tc.input, tc.maxLen)
			if result != tc.expected {
				t.Errorf("truncateStr(%q, %d) = %q, want %q",
					tc.input, tc.maxLen, result, tc.expected)
			}
		})
	}
}

// -- SessionHandler Constructor Test --------------------------

func TestNewSessionHandler(t *testing.T) {
	h := NewSessionHandler(nil, nil, nil, nil)
	if h == nil {
		t.Fatal("NewSessionHandler returned nil")
	}
	if h.DB != nil {
		t.Error("expected nil DB")
	}
	if h.Orchestrator != nil {
		t.Error("expected nil Orchestrator")
	}
	if h.InjectionGuard != nil {
		t.Error("expected nil InjectionGuard")
	}
	if h.ASR != nil {
		t.Error("expected nil ASR")
	}
}
