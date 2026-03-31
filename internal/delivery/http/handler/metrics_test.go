package handler

import (
	"net/http"
	"testing"
)

// ============================
// Metrics Handler Unit Tests
// ============================

// -- Constructor Tests ----------------------------------------

func TestNewMetricsHandler(t *testing.T) {
	h := NewMetricsHandler(nil)
	if h == nil {
		t.Fatal("NewMetricsHandler returned nil")
	}
	if h.cache != nil {
		t.Error("expected nil cache")
	}
}

// -- GetCacheMetrics Tests ------------------------------------

func TestGetCacheMetrics_NilCache(t *testing.T) {
	h := NewMetricsHandler(nil)
	w, c := newTestContextWithQuery("GET", "/api/v1/metrics/cache", 0)

	h.GetCacheMetrics(c)

	assertStatus(t, w, http.StatusOK)
	assertBodyContains(t, w, `"enabled":false`)
	assertBodyContains(t, w, "Redis cache not available")
}

// -- InvalidateCache Tests ------------------------------------

func TestInvalidateCache_NilCache(t *testing.T) {
	h := NewMetricsHandler(nil)
	w, c := newTestContextWithQuery("POST", "/api/v1/metrics/cache/invalidate?pattern=session:*", 0)

	h.InvalidateCache(c)

	assertStatus(t, w, http.StatusBadRequest)
	assertBodyContains(t, w, "Redis cache not available")
}

func TestInvalidateCache_NilCache_EmptyPattern(t *testing.T) {
	h := NewMetricsHandler(nil)
	w, c := newTestContextWithQuery("POST", "/api/v1/metrics/cache/invalidate", 0)

	h.InvalidateCache(c)

	// nil cache check comes before pattern check
	assertStatus(t, w, http.StatusBadRequest)
	assertBodyContains(t, w, "Redis cache not available")
}
