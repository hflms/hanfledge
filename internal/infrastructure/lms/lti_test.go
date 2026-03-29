package lms

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func setupMockServers(t *testing.T) (*httptest.Server, *httptest.Server) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected token request to be POST, got %s", r.Method)
		}

		err := r.ParseForm()
		if err != nil {
			t.Errorf("Failed to parse form: %v", err)
		}

		if r.FormValue("grant_type") != "client_credentials" {
			t.Errorf("Expected grant_type to be client_credentials")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"access_token": "mock-access-token",
			"token_type":   "Bearer",
		})
	}))

	scoreServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected score request to be POST, got %s", r.Method)
		}

		if r.Header.Get("Authorization") != "Bearer mock-access-token" {
			t.Errorf("Expected Bearer mock-access-token, got %s", r.Header.Get("Authorization"))
		}

		if r.Header.Get("Content-Type") != "application/vnd.ims.lis.v1.score+json" {
			t.Errorf("Expected correct Content-Type, got %s", r.Header.Get("Content-Type"))
		}

		var score agsScore
		if err := json.NewDecoder(r.Body).Decode(&score); err != nil {
			t.Errorf("Failed to decode score payload: %v", err)
		}

		if score.ScoreGiven != 0.95 {
			t.Errorf("Expected ScoreGiven 0.95, got %f", score.ScoreGiven)
		}

		w.WriteHeader(http.StatusOK)
	}))

	return tokenServer, scoreServer
}

func TestLTI13Adapter_ReportScore(t *testing.T) {
	testPrivateKey := generateTestPrivateKeyPEM(t)
	tokenServer, scoreServer := setupMockServers(t)
	defer tokenServer.Close()
	defer scoreServer.Close()

	cfg := LMSConfig{
		Type:        AdapterLTI13,
		ClientID:    "test-client-id",
		PlatformURL: "https://test.platform.com",
		TokenURL:    tokenServer.URL,
		PrivateKey:  testPrivateKey,
	}

	adapter, err := NewLTI13Adapter(cfg)
	if err != nil {
		t.Fatalf("failed to create LTI13Adapter: %v", err)
	}

	req := ScoreReport{
		UserID:     "user-123",
		CourseID:   "course-456",
		ActivityID: scoreServer.URL,
		Score:      0.95,
		MaxScore:   1.0,
		Comment:    "Great job!",
		Timestamp:  time.Now(),
		Status:     "completed",
	}

	ctx := context.Background()
	err = adapter.ReportScore(ctx, req)
	if err != nil {
		t.Errorf("ReportScore failed: %v", err)
	}
}

func TestLTI13Adapter_ReportScore_InvalidKey(t *testing.T) {
	tokenServer, scoreServer := setupMockServers(t)
	defer tokenServer.Close()
	defer scoreServer.Close()

	cfg := LMSConfig{
		Type:        AdapterLTI13,
		ClientID:    "test-client-id",
		PlatformURL: "https://test.platform.com",
		TokenURL:    tokenServer.URL,
		PrivateKey:  "invalid-private-key",
	}

	adapter, err := NewLTI13Adapter(cfg)
	if err != nil {
		t.Fatalf("failed to create LTI13Adapter: %v", err)
	}

	req := ScoreReport{
		UserID:     "user-123",
		CourseID:   "course-456",
		ActivityID: scoreServer.URL,
		Score:      0.95,
		MaxScore:   1.0,
		Comment:    "Great job!",
		Timestamp:  time.Now(),
		Status:     "completed",
	}

	ctx := context.Background()
	err = adapter.ReportScore(ctx, req)
	if err == nil {
		t.Errorf("Expected ReportScore to fail with invalid private key")
	}
}
