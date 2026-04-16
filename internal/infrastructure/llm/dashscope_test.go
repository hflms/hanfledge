package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDashScopeClient_Chat_ParseError(t *testing.T) {
	// Start a mock server that returns invalid JSON
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{invalid_json`))
	}))
	defer ts.Close()

	client := NewDashScopeClient(DashScopeConfig{
		APIKey:        "test-key",
		CompatBaseURL: ts.URL,
	})

	_, err := client.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hello"}}, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	expectedPrefix := "dashscope parse response failed:"
	if len(err.Error()) < len(expectedPrefix) || err.Error()[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("expected error starting with %q, got %q", expectedPrefix, err.Error())
	}
}

func TestDashScopeClient_Chat_EmptyResponse(t *testing.T) {
	// Start a mock server that returns valid JSON but empty choices
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices": []}`))
	}))
	defer ts.Close()

	client := NewDashScopeClient(DashScopeConfig{
		APIKey:        "test-key",
		CompatBaseURL: ts.URL,
	})

	_, err := client.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hello"}}, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	expectedErr := "dashscope returned empty response"
	if err.Error() != expectedErr {
		t.Errorf("expected error %q, got %q", expectedErr, err.Error())
	}
}

func TestDashScopeClient_Chat_Success(t *testing.T) {
	// Start a mock server that returns valid JSON with choices
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices": [{"message": {"content": "world"}}]}`))
	}))
	defer ts.Close()

	client := NewDashScopeClient(DashScopeConfig{
		APIKey:        "test-key",
		CompatBaseURL: ts.URL,
	})

	content, err := client.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hello"}}, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if content != "world" {
		t.Errorf("expected content 'world', got %q", content)
	}
}

func TestDashScopeClient_Chat_HTTPError(t *testing.T) {
	// Start a mock server that returns 500 Internal Server Error
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`internal server error`))
	}))
	defer ts.Close()

	client := NewDashScopeClient(DashScopeConfig{
		APIKey:        "test-key",
		CompatBaseURL: ts.URL,
	})

	_, err := client.Chat(context.Background(), []ChatMessage{{Role: "user", Content: "hello"}}, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	expectedPrefix := "dashscope chat failed:"
	if len(err.Error()) < len(expectedPrefix) || err.Error()[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("expected error starting with %q, got %q", expectedPrefix, err.Error())
	}
}
