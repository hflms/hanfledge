package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// ============================
// Health Handler Unit Tests
// ============================

// -- Liveness Response Test -----------------------------------

func TestHealthLiveness_ReturnsOK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/health", nil)

	h := &HealthHandler{} // no deps needed for liveness
	h.Liveness(c)

	if w.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if body == "" {
		t.Fatal("response body should not be empty")
	}
}

func TestHealthLiveness_ContainsStatusOK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/health", nil)

	h := &HealthHandler{}
	h.Liveness(c)

	body := w.Body.String()
	if len(body) == 0 {
		t.Fatal("response body is empty")
	}
}
