package weknora

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ============================
// WeKnora Client Unit Tests
// ============================

func TestNewClient(t *testing.T) {
	c := NewClient("http://localhost:9380/api/v1", "test-key")
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
	if c.baseURL != "http://localhost:9380/api/v1" {
		t.Errorf("baseURL = %q, want %q", c.baseURL, "http://localhost:9380/api/v1")
	}
	if c.apiKey != "test-key" {
		t.Errorf("apiKey = %q, want %q", c.apiKey, "test-key")
	}
	if c.httpClient == nil {
		t.Error("httpClient is nil")
	}
}

func TestListKnowledgeBases(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/knowledge-bases" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}

		resp := ListKBResponse{
			Data: []KnowledgeBase{
				{ID: "kb-1", Name: "数学知识库", Description: "中学数学教材"},
				{ID: "kb-2", Name: "物理知识库", Description: "高中物理"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := NewClient(srv.URL+"/api/v1", "test-key")
	kbs, err := c.ListKnowledgeBases(t.Context())
	if err != nil {
		t.Fatalf("ListKnowledgeBases error: %v", err)
	}
	if len(kbs) != 2 {
		t.Fatalf("len(kbs) = %d, want 2", len(kbs))
	}
	if kbs[0].ID != "kb-1" {
		t.Errorf("kbs[0].ID = %q, want %q", kbs[0].ID, "kb-1")
	}
	if kbs[1].Name != "物理知识库" {
		t.Errorf("kbs[1].Name = %q, want %q", kbs[1].Name, "物理知识库")
	}
}

func TestGetKnowledgeBase(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/knowledge-bases/kb-123" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		kb := KnowledgeBase{
			ID:          "kb-123",
			Name:        "测试知识库",
			Description: "用于测试",
			FileCount:   5,
			ChunkCount:  120,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(kb)
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := NewClient(srv.URL+"/api/v1", "test-key")
	kb, err := c.GetKnowledgeBase(t.Context(), "kb-123")
	if err != nil {
		t.Fatalf("GetKnowledgeBase error: %v", err)
	}
	if kb.ID != "kb-123" {
		t.Errorf("kb.ID = %q, want %q", kb.ID, "kb-123")
	}
	if kb.FileCount != 5 {
		t.Errorf("kb.FileCount = %d, want 5", kb.FileCount)
	}
}

func TestListKnowledge(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") != "1" {
			t.Errorf("unexpected page: %s", r.URL.Query().Get("page"))
		}
		if r.URL.Query().Get("page_size") != "10" {
			t.Errorf("unexpected page_size: %s", r.URL.Query().Get("page_size"))
		}

		resp := KnowledgeListResponse{
			Data: []Knowledge{
				{ID: "k-1", FileName: "教材.pdf", Status: "completed"},
			},
			Total: 1,
			Page:  1,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := NewClient(srv.URL+"/api/v1", "test-key")
	resp, err := c.ListKnowledge(t.Context(), "kb-1", 1, 10)
	if err != nil {
		t.Fatalf("ListKnowledge error: %v", err)
	}
	if resp.Total != 1 {
		t.Errorf("total = %d, want 1", resp.Total)
	}
	if resp.Data[0].FileName != "教材.pdf" {
		t.Errorf("data[0].FileName = %q, want %q", resp.Data[0].FileName, "教材.pdf")
	}
}

func TestPing_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"valid":true}`))
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := NewClient(srv.URL+"/api/v1", "test-key")
	if err := c.Ping(t.Context()); err != nil {
		t.Fatalf("Ping error: %v", err)
	}
}

func TestPing_AuthFailure(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := NewClient(srv.URL+"/api/v1", "bad-key")
	err := c.Ping(t.Context())
	if err == nil {
		t.Fatal("expected auth error, got nil")
	}
}

func TestAPIError_Handling(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"code":500,"message":"internal error"}`))
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := NewClient(srv.URL+"/api/v1", "test-key")
	_, err := c.ListKnowledgeBases(t.Context())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
