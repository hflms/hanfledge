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

func TestSCORMAdapter_ReportScore(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		expectedScore := ScoreReport{
			UserID:     "user123",
			CourseID:   "course456",
			ActivityID: "act789",
			Score:      0.85,
			MaxScore:   1.0,
			Status:     "completed",
			Timestamp:  time.Now().Truncate(time.Second), // Truncate for JSON matching
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
