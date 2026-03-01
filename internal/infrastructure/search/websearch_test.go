package search

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// -- DefaultSearchConfig ------------------------------------------

func TestDefaultSearchConfig(t *testing.T) {
	cfg := DefaultSearchConfig()

	if cfg.Provider != "searxng" {
		t.Errorf("expected provider=searxng, got %q", cfg.Provider)
	}
	if cfg.BaseURL != "http://localhost:8888" {
		t.Errorf("expected baseURL=http://localhost:8888, got %q", cfg.BaseURL)
	}
	if cfg.MaxResults != 10 {
		t.Errorf("expected maxResults=10, got %d", cfg.MaxResults)
	}
	if cfg.Timeout != 10*time.Second {
		t.Errorf("expected timeout=10s, got %v", cfg.Timeout)
	}
	if !cfg.SafeSearch {
		t.Error("expected SafeSearch=true")
	}
	if !cfg.EduFilter {
		t.Error("expected EduFilter=true")
	}
}

// -- truncateQuery ------------------------------------------------

func TestTruncateQuery(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short", "hello", 10, "hello"},
		{"exact", "hello", 5, "hello"},
		{"truncate", "hello world", 5, "hello..."},
		{"empty", "", 5, ""},
		{"chinese", "你好世界测试字符串", 4, "你好世界..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateQuery(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateQuery(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

// -- NewDynamicConnector ------------------------------------------

func TestNewDynamicConnector_SearXNG(t *testing.T) {
	cfg := DefaultSearchConfig()
	dc := NewDynamicConnector(cfg)

	if dc == nil {
		t.Fatal("expected non-nil DynamicConnector")
	}
	if dc.config.Provider != "searxng" {
		t.Errorf("expected provider=searxng, got %q", dc.config.Provider)
	}
	if dc.searcher == nil {
		t.Error("expected non-nil searcher")
	}
}

func TestNewDynamicConnector_UnknownProviderFallsToSearXNG(t *testing.T) {
	cfg := DefaultSearchConfig()
	cfg.Provider = "unknown"
	dc := NewDynamicConnector(cfg)

	if dc == nil {
		t.Fatal("expected non-nil DynamicConnector")
	}
	// Should still create a searcher (fallback to SearXNG)
	if dc.searcher == nil {
		t.Error("expected non-nil searcher for unknown provider")
	}
}

// -- FormatAsContext ----------------------------------------------

func TestFormatAsContext_Empty(t *testing.T) {
	dc := NewDynamicConnector(DefaultSearchConfig())
	result := dc.FormatAsContext(nil)
	if result != "" {
		t.Errorf("expected empty string for nil results, got %q", result)
	}

	result = dc.FormatAsContext([]SearchResult{})
	if result != "" {
		t.Errorf("expected empty string for empty results, got %q", result)
	}
}

func TestFormatAsContext_WithResults(t *testing.T) {
	dc := NewDynamicConnector(DefaultSearchConfig())
	results := []SearchResult{
		{Title: "Title A", URL: "http://a.com", Snippet: "Snippet A", Score: 0.9},
		{Title: "Title B", URL: "http://b.com", Snippet: "", Score: 0.5},
	}

	ctx := dc.FormatAsContext(results)

	// Should contain numbered entries
	if len(ctx) == 0 {
		t.Fatal("expected non-empty context")
	}
	// Check for presence of key content
	for _, sub := range []string{"[1]", "Title A", "http://a.com", "Snippet A", "[2]", "Title B"} {
		if !containsStr(ctx, sub) {
			t.Errorf("expected context to contain %q", sub)
		}
	}
}

// -- EnhancePrompt ------------------------------------------------

func TestEnhancePrompt_NoResults(t *testing.T) {
	dc := NewDynamicConnector(DefaultSearchConfig())
	original := "You are a helpful tutor."

	enhanced := dc.EnhancePrompt(original, nil)
	if enhanced != original {
		t.Errorf("expected unchanged prompt with no results, got %q", enhanced)
	}
}

func TestEnhancePrompt_WithResults(t *testing.T) {
	dc := NewDynamicConnector(DefaultSearchConfig())
	original := "You are a helpful tutor."
	results := []SearchResult{
		{Title: "Test", URL: "http://test.com", Snippet: "A test result", Score: 0.8},
	}

	enhanced := dc.EnhancePrompt(original, results)

	if enhanced == original {
		t.Error("expected enhanced prompt to differ from original")
	}
	if !containsStr(enhanced, original) {
		t.Error("expected enhanced prompt to contain original prompt")
	}
	if !containsStr(enhanced, "网络搜索补充材料") {
		t.Error("expected enhanced prompt to contain web search section header")
	}
	if !containsStr(enhanced, "Test") {
		t.Error("expected enhanced prompt to contain search result title")
	}
}

// -- SearXNG Search (with httptest) --------------------------------

func TestSearXNGSearch_Success(t *testing.T) {
	// Mock SearXNG server
	mockResp := searxngResponse{
		Results: []searxngResult{
			{Title: "Result 1", URL: "http://r1.com", Content: "Content 1", Score: 0.9, Engine: "google"},
			{Title: "Result 2", URL: "http://r2.com", Content: "Content 2", Score: 0.7, Engine: "bing"},
			{Title: "Result 3", URL: "http://r3.com", Content: "Content 3", Score: 0.5, Engine: "duckduckgo"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		q := r.URL.Query().Get("q")
		if q != "test query" {
			t.Errorf("unexpected query: %s", q)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResp)
	}))
	defer srv.Close()

	searcher := NewSearXNGSearcher(srv.URL, 5*time.Second)
	results, err := searcher.Search(context.Background(), "test query", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Title != "Result 1" {
		t.Errorf("expected first result title=Result 1, got %q", results[0].Title)
	}
	if results[0].Source != "searxng" {
		t.Errorf("expected source=searxng, got %q", results[0].Source)
	}
}

func TestSearXNGSearch_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	searcher := NewSearXNGSearcher(srv.URL, 5*time.Second)
	_, err := searcher.Search(context.Background(), "test", 5)
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestSearXNGSearch_ScoreClamp(t *testing.T) {
	mockResp := searxngResponse{
		Results: []searxngResult{
			{Title: "High", URL: "http://h.com", Content: "C", Score: 5.0},
			{Title: "Low", URL: "http://l.com", Content: "C", Score: -1.0},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(mockResp)
	}))
	defer srv.Close()

	searcher := NewSearXNGSearcher(srv.URL, 5*time.Second)
	results, err := searcher.Search(context.Background(), "test", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if results[0].Score != 1.0 {
		t.Errorf("expected score clamped to 1.0, got %f", results[0].Score)
	}
	if results[1].Score != 0.0 {
		t.Errorf("expected score clamped to 0.0, got %f", results[1].Score)
	}
}

// -- SearchAndRank ------------------------------------------------

// mockSearcher implements WebSearcher for testing.
type mockSearcher struct {
	results []SearchResult
	err     error
}

func (m *mockSearcher) Search(_ context.Context, _ string, _ int) ([]SearchResult, error) {
	return m.results, m.err
}

func TestSearchAndRank_SortsAndTruncates(t *testing.T) {
	dc := &DynamicConnector{
		searcher: &mockSearcher{
			results: []SearchResult{
				{Title: "C", Score: 0.3},
				{Title: "A", Score: 0.9},
				{Title: "B", Score: 0.6},
			},
		},
		config: SearchConfig{MaxResults: 2},
	}

	results, err := dc.SearchAndRank(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results after truncation, got %d", len(results))
	}
	if results[0].Title != "A" || results[1].Title != "B" {
		t.Errorf("expected sorted order [A, B], got [%s, %s]", results[0].Title, results[1].Title)
	}
}

func TestSearchAndRank_EmptyResults(t *testing.T) {
	dc := &DynamicConnector{
		searcher: &mockSearcher{results: nil},
		config:   SearchConfig{MaxResults: 5},
	}

	results, err := dc.SearchAndRank(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// -- Helpers -------------------------------------------------------

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstring(s, sub))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
