package weknora

import "time"

// ============================
// WeKnora API 数据类型定义
// ============================

// KnowledgeBase 知识库信息。
type KnowledgeBase struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	FileCount   int       `json:"file_count"`
	TokenCount  int       `json:"token_count"`
	ChunkCount  int       `json:"chunk_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Knowledge 知识库中的知识条目（文件/文档）。
type Knowledge struct {
	ID          string `json:"id"`
	KBName      string `json:"kb_name"`
	FileName    string `json:"file_name"`
	FileType    string `json:"file_type"`
	FileSize    int64  `json:"file_size"`
	Status      string `json:"status"`
	ChunkCount  int    `json:"chunk_count"`
	TokenCount  int    `json:"token_count"`
	ProcessedAt string `json:"processed_at"`
}

// Chunk 知识分块。
type Chunk struct {
	ID         string  `json:"id"`
	Content    string  `json:"content"`
	FileName   string  `json:"file_name"`
	PageNumber int     `json:"page_number"`
	Score      float64 `json:"score,omitempty"`
}

// -- 请求类型 --

// SearchRequest 检索请求。
type SearchRequest struct {
	KnowledgeBaseIDs []string `json:"knowledge_base_ids"`
	Query            string   `json:"query"`
	TopK             int      `json:"top_k,omitempty"`
}

// CreateSessionRequest 创建会话请求。
type CreateSessionRequest struct {
	KnowledgeBases []string `json:"knowledge_bases,omitempty"`
	AgentID        string   `json:"agent_id,omitempty"`
}

// RetrievalRequest 纯检索请求（不生成回答）。
type RetrievalRequest struct {
	Question string `json:"question"`
	TopK     int    `json:"top_k,omitempty"`
}

// -- 响应类型 --

// ListKBResponse 知识库列表响应。
type ListKBResponse struct {
	Data []KnowledgeBase `json:"data"`
}

// KnowledgeListResponse 知识列表响应。
type KnowledgeListResponse struct {
	Data  []Knowledge `json:"data"`
	Total int         `json:"total"`
	Page  int         `json:"page"`
}

// SearchResult 检索结果条目。
type SearchResult struct {
	ChunkID    string  `json:"chunk_id"`
	Content    string  `json:"content"`
	Score      float64 `json:"score"`
	FileName   string  `json:"file_name"`
	KBName     string  `json:"kb_name"`
	PageNumber int     `json:"page_number"`
}

// RetrievalResponse 检索响应。
type RetrievalResponse struct {
	Results []SearchResult `json:"results"`
	Total   int            `json:"total"`
}

// Session WeKnora 会话对象。
type Session struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// APIError WeKnora API 错误响应。
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ── 认证相关类型 ─────────────────────────────────────────────

// LoginRequest WeKnora 登录请求。
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse WeKnora 登录响应。
type LoginResponse struct {
	Success      bool      `json:"success"`
	Message      string    `json:"message"`
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token"`
	User         *WKUser   `json:"user,omitempty"`
	Tenant       *WKTenant `json:"tenant,omitempty"`
}

// RegisterRequest WeKnora 注册请求。
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Username string `json:"username"`
}

// RegisterResponse WeKnora 注册响应。
type RegisterResponse struct {
	Success bool    `json:"success"`
	Message string  `json:"message"`
	User    *WKUser `json:"user,omitempty"`
}

// RefreshRequest WeKnora 刷新 Token 请求。
type RefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// WKUser WeKnora 用户信息。
type WKUser struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

// WKTenant WeKnora 租户信息。
type WKTenant struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// CreateKBRequest 创建知识库请求。
type CreateKBRequest struct {
	Name           string `json:"name" binding:"required"`
	Description    string `json:"description"`
	EmbeddingModel string `json:"embedding_model"`
}

// CreateKBResponse 创建知识库响应。
type CreateKBResponse struct {
	Success bool           `json:"success"`
	Message string         `json:"message"`
	Data    *KnowledgeBase `json:"data"`
}

// DeleteKBResponse 删除知识库响应。
type DeleteKBResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}
