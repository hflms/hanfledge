package lms

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestXAPIAdapter_ReportScore(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/statements" {
			t.Errorf("expected path /statements, got %s", r.URL.Path)
		}

		user, pass, ok := r.BasicAuth()
		if !ok || user != "test-key" || pass != "test-secret" {
			t.Errorf("expected basic auth with test-key and test-secret")
		}

		if r.Header.Get("X-Experience-API-Version") != "1.0.3" {
			t.Errorf("expected X-Experience-API-Version 1.0.3")
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode JSON body: %v", err)
		}

		actor, ok := payload["actor"].(map[string]interface{})
		if !ok {
			t.Fatal("missing actor")
		}
		account, _ := actor["account"].(map[string]interface{})
		if account["name"] != "user-123" {
			t.Errorf("expected actor account name user-123, got %v", account["name"])
		}

		verb, _ := payload["verb"].(map[string]interface{})
		if verb["id"] != "http://adlnet.gov/expapi/verbs/completed" {
			t.Errorf("expected verb completed, got %v", verb["id"])
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`["stmt-123"]`))
	}))
	defer ts.Close()

	cfg := LMSConfig{
		Type:         AdapterXAPI,
		XAPIEndpoint: ts.URL,
		XAPIKey:      "test-key",
		XAPISecret:   "test-secret",
	}

	adapter, err := NewXAPIAdapter(cfg)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	req := ScoreReport{
		UserID:     "user-123",
		CourseID:   "course-456",
		ActivityID: "activity-789",
		Score:      0.95,
		MaxScore:   1.0,
		Status:     "completed",
		Timestamp:  time.Now(),
	}

	err = adapter.ReportScore(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
