package weknora

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hflms/hanfledge/internal/infrastructure/logger"
)

var slog = logger.L("WeKnora")

// Client is an HTTP client for the WeKnora knowledge base service API.
type Client struct {
	baseURL    string
	apiKey     string // Global API Key (fallback)
	userToken  string // Per-user Bearer token (takes precedence)
	httpClient *http.Client
}

// NewClient creates a new WeKnora API client.
//
// Parameters:
//   - baseURL: WeKnora API base URL (e.g., "http://localhost:9380/api/v1")
//   - apiKey: API Key for authentication
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// WithToken returns a shallow clone of this Client that uses the given
// user-specific Bearer token for all API calls. The original client is
// not modified.
func (c *Client) WithToken(token string) *Client {
	return &Client{
		baseURL:    c.baseURL,
		apiKey:     c.apiKey,
		userToken:  token,
		httpClient: c.httpClient,
	}
}

// -- Authentication APIs ---------------------------------------------------

// Register creates a new user account in WeKnora.
// This is called without authentication (public endpoint).
func (c *Client) Register(ctx context.Context, req *RegisterRequest) (*RegisterResponse, error) {
	var resp RegisterResponse
	if err := c.doPostNoAuth(ctx, "/auth/register", req, &resp); err != nil {
		return nil, fmt.Errorf("register: %w", err)
	}
	return &resp, nil
}

// Login authenticates a user and returns tokens.
// This is called without authentication (public endpoint).
func (c *Client) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	var resp LoginResponse
	if err := c.doPostNoAuth(ctx, "/auth/login", req, &resp); err != nil {
		return nil, fmt.Errorf("login: %w", err)
	}
	return &resp, nil
}

// RefreshToken exchanges a refresh token for a new access token.
func (c *Client) RefreshToken(ctx context.Context, refreshToken string) (*LoginResponse, error) {
	var resp LoginResponse
	if err := c.doPostNoAuth(ctx, "/auth/refresh", &RefreshRequest{RefreshToken: refreshToken}, &resp); err != nil {
		return nil, fmt.Errorf("refresh token: %w", err)
	}
	return &resp, nil
}

// -- Knowledge Base APIs --------------------------------------------------

// ListKnowledgeBases returns all knowledge bases from WeKnora.
func (c *Client) ListKnowledgeBases(ctx context.Context) ([]KnowledgeBase, error) {
	var resp ListKBResponse
	if err := c.doGet(ctx, "/knowledge-bases", &resp); err != nil {
		return nil, fmt.Errorf("list knowledge bases: %w", err)
	}
	return resp.Data, nil
}

// GetKnowledgeBase returns a single knowledge base by ID.
func (c *Client) GetKnowledgeBase(ctx context.Context, id string) (*KnowledgeBase, error) {
	var kb KnowledgeBase
	if err := c.doGet(ctx, "/knowledge-bases/"+id, &kb); err != nil {
		return nil, fmt.Errorf("get knowledge base %s: %w", id, err)
	}
	return &kb, nil
}

// -- Knowledge (Document/File) APIs ----------------------------------------

// ListKnowledge returns knowledge entries (files/documents) in a knowledge base.
func (c *Client) ListKnowledge(ctx context.Context, kbID string, page, pageSize int) (*KnowledgeListResponse, error) {
	path := fmt.Sprintf("/knowledge-bases/%s/knowledge?page=%d&page_size=%d", kbID, page, pageSize)
	var resp KnowledgeListResponse
	if err := c.doGet(ctx, path, &resp); err != nil {
		return nil, fmt.Errorf("list knowledge for kb %s: %w", kbID, err)
	}
	return &resp, nil
}

// -- Session & Retrieval APIs ----------------------------------------------

// CreateSession creates a new conversation session in WeKnora.
func (c *Client) CreateSession(ctx context.Context, req *CreateSessionRequest) (*Session, error) {
	var session Session
	if err := c.doPost(ctx, "/sessions", req, &session); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return &session, nil
}

// Retrieve performs a retrieval-only search within a session's knowledge bases.
func (c *Client) Retrieve(ctx context.Context, sessionID string, req *RetrievalRequest) (*RetrievalResponse, error) {
	path := fmt.Sprintf("/sessions/%s/retrieval", sessionID)
	var resp RetrievalResponse
	if err := c.doPost(ctx, path, req, &resp); err != nil {
		return nil, fmt.Errorf("retrieve session %s: %w", sessionID, err)
	}
	return &resp, nil
}

// -- Health Check ----------------------------------------------------------

// Ping checks if the WeKnora service is reachable.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/auth/validate", nil)
	if err != nil {
		return fmt.Errorf("create ping request: %w", err)
	}
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("weknora unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("weknora auth failed (status %d): check WEKNORA_API_KEY", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("weknora ping failed with status %d", resp.StatusCode)
	}
	return nil
}

// -- Internal HTTP helpers -------------------------------------------------

// setAuth adds the authorization header. Per-user token takes precedence over global API Key.
func (c *Client) setAuth(req *http.Request) {
	if c.userToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.userToken)
	} else if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
}

// doGet performs an authenticated GET request and decodes the JSON response.
func (c *Client) doGet(ctx context.Context, path string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	c.setAuth(req)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		slog.Error("WeKnora API error", "status", resp.StatusCode, "path", path, "body", string(body))
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// doPost performs an authenticated POST request with JSON body and decodes the response.
func (c *Client) doPost(ctx context.Context, path string, body interface{}, out interface{}) error {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		slog.Error("WeKnora API error", "status", resp.StatusCode, "path", path, "body", string(respBody))
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// doPostNoAuth performs a POST request without authentication (for public endpoints like login/register).
func (c *Client) doPostNoAuth(ctx context.Context, path string, body interface{}, out interface{}) error {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		slog.Error("WeKnora API error", "status", resp.StatusCode, "path", path, "body", string(respBody))
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}
