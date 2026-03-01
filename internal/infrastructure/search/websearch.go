package search

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ============================
// Dynamic Connector — Web Search Fallback (§1.1 + §8.1.2)
// ============================
//
// 当 CRAG 质量网关判定检索结果质量不足时，Dynamic Connector 作为
// "知识安全网" 接入外部搜索引擎（SearXNG / Google / Bing），
// 补充教育相关的上下文信息，确保系统始终能给出有价值的回答。

// -- Types --------------------------------------------------------

// SearchResult represents a single web search result.
type SearchResult struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`  // relevance score [0,1]
	Source  string  `json:"source"` // "google", "bing", "searxng", etc.
}

// SearchConfig holds web search connector configuration.
type SearchConfig struct {
	Provider   string        // "searxng" | "google" | "bing"
	BaseURL    string        // SearXNG instance URL or API endpoint
	APIKey     string        // API key (for Google/Bing)
	MaxResults int           // Max results to fetch (default 10)
	Timeout    time.Duration // HTTP request timeout
	SafeSearch bool          // Enable safe search (educational context)
	EduFilter  bool          // Filter for educational content
}

// WebSearcher defines the interface for web search providers.
type WebSearcher interface {
	Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error)
}

// -- Default Config -----------------------------------------------

// DefaultSearchConfig returns a SearchConfig with sensible defaults
// for a local SearXNG instance.
func DefaultSearchConfig() SearchConfig {
	return SearchConfig{
		Provider:   "searxng",
		BaseURL:    "http://localhost:8888",
		MaxResults: 10,
		Timeout:    10 * time.Second,
		SafeSearch: true,
		EduFilter:  true,
	}
}

// -- SearXNG Searcher ---------------------------------------------

// SearXNGSearcher implements WebSearcher using a SearXNG instance.
type SearXNGSearcher struct {
	baseURL string
	client  *http.Client
}

// searxngResponse represents the JSON response from SearXNG /search endpoint.
type searxngResponse struct {
	Results []searxngResult `json:"results"`
}

// searxngResult represents a single result item from SearXNG.
type searxngResult struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
	Engine  string  `json:"engine"`
}

// NewSearXNGSearcher creates an HTTP-based SearXNG search client.
func NewSearXNGSearcher(baseURL string, timeout time.Duration) *SearXNGSearcher {
	return &SearXNGSearcher{
		baseURL: strings.TrimRight(baseURL, "/"),
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Search calls the SearXNG JSON API and returns parsed search results.
func (s *SearXNGSearcher) Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	// Build request URL
	params := url.Values{}
	params.Set("q", query)
	params.Set("format", "json")
	params.Set("categories", "general")
	params.Set("language", "zh-CN")
	if maxResults > 0 {
		params.Set("pageno", "1")
	}

	reqURL := fmt.Sprintf("%s/search?%s", s.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("构建搜索请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	log.Printf("🌐 [Search] Querying SearXNG: query=%q maxResults=%d", query, maxResults)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("SearXNG 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("SearXNG 返回非 200 状态码: %d, body=%s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 SearXNG 响应失败: %w", err)
	}

	var searxResp searxngResponse
	if err := json.Unmarshal(body, &searxResp); err != nil {
		return nil, fmt.Errorf("解析 SearXNG JSON 失败: %w", err)
	}

	// Convert to SearchResult slice, capping at maxResults
	limit := maxResults
	if limit <= 0 || limit > len(searxResp.Results) {
		limit = len(searxResp.Results)
	}

	results := make([]SearchResult, 0, limit)
	for i := 0; i < limit; i++ {
		r := searxResp.Results[i]
		score := r.Score
		// Normalize score to [0,1] if needed — SearXNG scores can vary
		if score > 1.0 {
			score = 1.0
		}
		if score < 0 {
			score = 0
		}
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Content,
			Score:   score,
			Source:  "searxng",
		})
	}

	log.Printf("🔍 [Search] SearXNG returned %d results (requested %d)", len(results), maxResults)
	return results, nil
}

// -- Dynamic Connector --------------------------------------------

// DynamicConnector orchestrates web search fallback and result processing.
// It acts as the "knowledge safety net" described in §1.1, triggered when
// CRAG quality gateway determines retrieval results are insufficient.
type DynamicConnector struct {
	searcher WebSearcher
	config   SearchConfig
}

// NewDynamicConnector creates a DynamicConnector with the given config.
// It initializes the appropriate WebSearcher based on config.Provider.
func NewDynamicConnector(config SearchConfig) *DynamicConnector {
	var searcher WebSearcher

	switch config.Provider {
	case "searxng":
		searcher = NewSearXNGSearcher(config.BaseURL, config.Timeout)
	default:
		// Default to SearXNG; other providers (Google, Bing) can be added later
		log.Printf("⚠️  [Search] Unknown provider %q, falling back to SearXNG", config.Provider)
		searcher = NewSearXNGSearcher(config.BaseURL, config.Timeout)
	}

	return &DynamicConnector{
		searcher: searcher,
		config:   config,
	}
}

// SearchAndRank performs a web search, sorts results by relevance score
// (descending), and truncates to the configured maximum number of results.
func (dc *DynamicConnector) SearchAndRank(ctx context.Context, query string) ([]SearchResult, error) {
	log.Printf("🌐 [Search] Dynamic Connector triggered for query: %q", truncateQuery(query, 80))

	results, err := dc.searcher.Search(ctx, query, dc.config.MaxResults)
	if err != nil {
		return nil, fmt.Errorf("web search failed: %w", err)
	}

	if len(results) == 0 {
		log.Printf("⚠️  [Search] No results returned for query: %q", truncateQuery(query, 80))
		return results, nil
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Truncate to max results
	if dc.config.MaxResults > 0 && len(results) > dc.config.MaxResults {
		results = results[:dc.config.MaxResults]
	}

	log.Printf("🔍 [Search] Ranked %d results, top score=%.4f", len(results), results[0].Score)
	return results, nil
}

// FormatAsContext formats search results into a context string suitable
// for injection into an LLM prompt. Each result is numbered with title,
// source URL, and snippet.
func (dc *DynamicConnector) FormatAsContext(results []SearchResult) string {
	if len(results) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("以下是从网络搜索获取的补充参考资料：\n\n")

	for i, r := range results {
		sb.WriteString("[" + strconv.Itoa(i+1) + "] " + r.Title + "\n")
		sb.WriteString("    来源: " + r.URL + "\n")
		if r.Snippet != "" {
			sb.WriteString("    摘要: " + r.Snippet + "\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// EnhancePrompt wraps search results into the system prompt with clear
// delineation, so the LLM can distinguish between retrieved course
// materials and web search supplements.
func (dc *DynamicConnector) EnhancePrompt(systemPrompt string, results []SearchResult) string {
	if len(results) == 0 {
		return systemPrompt
	}

	contextStr := dc.FormatAsContext(results)

	enhanced := systemPrompt +
		"\n\n" +
		"━━━━━━━━━━ 网络搜索补充材料 ━━━━━━━━━━\n" +
		"【注意：以下内容来自网络搜索，非课程原始材料。请批判性地使用这些信息，" +
		"优先参考课程内容，仅在课程材料不足时引用网络搜索结果。】\n\n" +
		contextStr +
		"━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"

	return enhanced
}

// -- Helpers -------------------------------------------------------

// truncateQuery truncates a query string for log output.
func truncateQuery(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
