package lms

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSCORMAdapter(t *testing.T) {
	tests := []struct {
		name    string
		cfg     LMSConfig
		wantErr bool
	}{
		{
			name: "valid configuration",
			cfg: LMSConfig{
				SCORMEndpoint: "http://example.com/scorm",
				SCORMAPIKey:   "secret-key",
			},
			wantErr: false,
		},
		{
			name: "missing endpoint",
			cfg: LMSConfig{
				SCORMAPIKey: "secret-key",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, err := NewSCORMAdapter(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewSCORMAdapter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && adapter == nil {
				t.Errorf("NewSCORMAdapter() returned nil adapter when expecting valid one")
			}
		})
	}
}

func TestSCORMAdapter_Type(t *testing.T) {
	adapter := &SCORMAdapter{}
	if got := adapter.Type(); got != AdapterSCORM {
		t.Errorf("SCORMAdapter.Type() = %v, want %v", got, AdapterSCORM)
	}
}

func TestSCORMAdapter_LaunchURL(t *testing.T) {
	adapter := &SCORMAdapter{
		endpoint: "http://example.com/scorm",
		apiKey:   "test-key",
	}

	req := LaunchRequest{
		UserID:     "user1",
		CourseID:   "course1",
		ActivityID: "activity1",
	}

	resp, err := adapter.LaunchURL(context.Background(), req)
	if err != nil {
		t.Fatalf("LaunchURL() unexpected error: %v", err)
	}

	if resp == nil {
		t.Fatalf("LaunchURL() returned nil response")
	}

	if resp.URL != "http://example.com/scorm" {
		t.Errorf("LaunchURL() URL = %v, want %v", resp.URL, "http://example.com/scorm")
	}

	if resp.Method != "POST" {
		t.Errorf("LaunchURL() Method = %v, want %v", resp.Method, "POST")
	}

	expectedSessionID := "scorm_user1_activity1"
	if resp.SessionID != expectedSessionID {
		t.Errorf("LaunchURL() SessionID = %v, want %v", resp.SessionID, expectedSessionID)
	}

	if resp.FormData["user_id"] != "user1" {
		t.Errorf("LaunchURL() FormData[user_id] = %v, want %v", resp.FormData["user_id"], "user1")
	}

	if resp.FormData["api_key"] != "test-key" {
		t.Errorf("LaunchURL() FormData[api_key] = %v, want %v", resp.FormData["api_key"], "test-key")
	}
}

func TestSCORMAdapter_Validate(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		wantErr  bool
	}{
		{
			name:     "valid endpoint",
			endpoint: "http://example.com/scorm",
			wantErr:  false,
		},
		{
			name:     "empty endpoint",
			endpoint: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := &SCORMAdapter{endpoint: tt.endpoint}
			err := adapter.Validate(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSCORMAdapter_ReportScore(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		expectedScore := ScoreReport{
			UserID:     "user123",
			CourseID:   "course456",
			ActivityID: "act789",
			Score:      0.85,
			MaxScore:   1.0,
			Status:     "completed",
			Timestamp:  time.Now().Truncate(time.Second),
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))

			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var receivedScore ScoreReport
			err = json.Unmarshal(body, &receivedScore)
			require.NoError(t, err)

			assert.Equal(t, expectedScore.UserID, receivedScore.UserID)
			assert.Equal(t, expectedScore.CourseID, receivedScore.CourseID)
			assert.Equal(t, expectedScore.ActivityID, receivedScore.ActivityID)
			assert.InDelta(t, expectedScore.Score, receivedScore.Score, 0.001)
			assert.InDelta(t, expectedScore.MaxScore, receivedScore.MaxScore, 0.001)
			assert.Equal(t, expectedScore.Status, receivedScore.Status)
			assert.True(t, expectedScore.Timestamp.Equal(receivedScore.Timestamp))

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		adapter, err := NewSCORMAdapter(LMSConfig{
			Type:          AdapterSCORM,
			SCORMEndpoint: server.URL,
			SCORMAPIKey:   "test-api-key",
		})
		require.NoError(t, err)

		err = adapter.ReportScore(context.Background(), expectedScore)
		assert.NoError(t, err)
	})

	t.Run("server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		adapter, err := NewSCORMAdapter(LMSConfig{
			Type:          AdapterSCORM,
			SCORMEndpoint: server.URL,
		})
		require.NoError(t, err)

		err = adapter.ReportScore(context.Background(), ScoreReport{})
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "failed with status: 500 Internal Server Error"))
	})
}
