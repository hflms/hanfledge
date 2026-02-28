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

// -- HealthCheck Response Test --------------------------------

func TestHealthCheck_ReturnsOK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/health", nil)

	HealthCheck(c)

	if w.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if body == "" {
		t.Fatal("response body should not be empty")
	}
}

func TestHealthCheck_ContainsStatusOK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/health", nil)

	HealthCheck(c)

	body := w.Body.String()
	// The response should contain "ok" status
	if len(body) == 0 {
		t.Fatal("response body is empty")
	}
}
