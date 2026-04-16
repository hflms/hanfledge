package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDashScopeClient_Chat_ParseResponseFailed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{invalid-json`))
	}))
	defer server.Close()

	client := &DashScopeClient{
		APIKey:        "test-api-key",
		ChatModel:     "test-model",
		CompatBaseURL: server.URL,
		HTTPClient:    &http.Client{},
	}

	ctx := context.Background()
	messages := []ChatMessage{{Role: "user", Content: "hello"}}

	_, err := client.Chat(ctx, messages, nil)

	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	expectedErrMsg := "dashscope parse response failed"
	if !strings.Contains(err.Error(), expectedErrMsg) {
		t.Errorf("expected error to contain %q, got: %v", expectedErrMsg, err)
	}
}
