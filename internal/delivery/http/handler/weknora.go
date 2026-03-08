package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/delivery/http/middleware"
	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/logger"
	"github.com/hflms/hanfledge/internal/infrastructure/weknora"
	"gorm.io/gorm"
)

var slogWeKnora = logger.L("WeKnoraHandler")

// WeKnoraHandler handles WeKnora knowledge base integration endpoints.
type WeKnoraHandler struct {
	client   *weknora.Client       // Global client (fallback)
	tokenMgr *weknora.TokenManager // Per-user token management (nil = use global)
	db       *gorm.DB
}

// NewWeKnoraHandler creates a new WeKnoraHandler.
func NewWeKnoraHandler(client *weknora.Client, tokenMgr *weknora.TokenManager, db *gorm.DB) *WeKnoraHandler {
	return &WeKnoraHandler{client: client, tokenMgr: tokenMgr, db: db}
}

// getUserClient returns a WeKnora client authenticated for the current user.
// Falls back to the global client if TokenManager is not available.
func (h *WeKnoraHandler) getUserClient(c *gin.Context) *weknora.Client {
	if h.tokenMgr == nil {
		return h.client
	}
	userID := middleware.GetUserID(c)
	userClient, err := h.tokenMgr.GetClientForUser(c.Request.Context(), userID)
	if err != nil {
		slogWeKnora.Warn("failed to get per-user WeKnora client, falling back to global", "user_id", userID, "error", err)
		return h.client
	}
	return userClient
}

// ── WeKnora Proxy Endpoints ──────────────────────────────────

// ListKnowledgeBases proxies the knowledge base list from WeKnora.
//
//	@Summary      获取 WeKnora 知识库列表
//	@Description  代理获取 WeKnora 中可用的全部知识库
//	@Tags         WeKnora
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200 {array}   weknora.KnowledgeBase
//	@Failure      500 {object}  ErrorResponse
//	@Router       /weknora/knowledge-bases [get]
func (h *WeKnoraHandler) ListKnowledgeBases(c *gin.Context) {
	client := h.getUserClient(c)
	kbs, err := client.ListKnowledgeBases(c.Request.Context())
	if err != nil {
		slogWeKnora.Error("failed to list WeKnora knowledge bases", "error", err)
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "failed to fetch knowledge bases from WeKnora: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": kbs})
}

// GetKnowledgeBase proxies a single knowledge base detail from WeKnora.
//
//	@Summary      获取 WeKnora 知识库详情
//	@Description  代理获取指定知识库的详细信息
//	@Tags         WeKnora
//	@Produce      json
//	@Security     BearerAuth
//	@Param        kb_id path string true "WeKnora 知识库 ID"
//	@Success      200 {object}  weknora.KnowledgeBase
//	@Failure      404 {object}  ErrorResponse
//	@Failure      500 {object}  ErrorResponse
//	@Router       /weknora/knowledge-bases/{kb_id} [get]
func (h *WeKnoraHandler) GetKnowledgeBase(c *gin.Context) {
	kbID := c.Param("kb_id")
	if kbID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "kb_id is required"})
		return
	}

	client := h.getUserClient(c)
	kb, err := client.GetKnowledgeBase(c.Request.Context(), kbID)
	if err != nil {
		slogWeKnora.Error("failed to get WeKnora knowledge base", "kb_id", kbID, "error", err)
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "failed to fetch knowledge base from WeKnora: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, kb)
}

// ListKnowledge proxies the knowledge/file list within a knowledge base.
//
//	@Summary      获取知识库中的文件列表
//	@Description  代理获取指定知识库中的文件/知识条目列表
//	@Tags         WeKnora
//	@Produce      json
//	@Security     BearerAuth
//	@Param        kb_id     path  string true  "WeKnora 知识库 ID"
//	@Param        page      query int    false "页码" default(1)
//	@Param        page_size query int    false "每页数量" default(10)
//	@Success      200 {object}  weknora.KnowledgeListResponse
//	@Failure      500 {object}  ErrorResponse
//	@Router       /weknora/knowledge-bases/{kb_id}/knowledge [get]
func (h *WeKnoraHandler) ListKnowledge(c *gin.Context) {
	kbID := c.Param("kb_id")
	if kbID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "kb_id is required"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	client := h.getUserClient(c)
	resp, err := client.ListKnowledge(c.Request.Context(), kbID, page, pageSize)
	if err != nil {
		slogWeKnora.Error("failed to list knowledge", "kb_id", kbID, "error", err)
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "failed to fetch knowledge from WeKnora: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ── Course ↔ WeKnora Binding Endpoints ───────────────────────

// BindKBRequest represents the request to bind a WeKnora KB to a course.
type BindKBRequest struct {
	KBID   string `json:"kb_id" binding:"required"`
	KBName string `json:"kb_name" binding:"required"`
}

// BindKnowledgeBase binds a WeKnora knowledge base to a course.
//
//	@Summary      绑定知识库到课程
//	@Description  将 WeKnora 知识库关联到指定课程的教材大纲
//	@Tags         WeKnora
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id   path int           true "课程 ID"
//	@Param        body body BindKBRequest  true "绑定请求"
//	@Success      201  {object} model.WeKnoraKBRef
//	@Failure      400  {object} ErrorResponse
//	@Failure      409  {object} ErrorResponse
//	@Failure      500  {object} ErrorResponse
//	@Router       /courses/{id}/weknora-refs [post]
func (h *WeKnoraHandler) BindKnowledgeBase(c *gin.Context) {
	courseID, err := parseCourseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid course ID"})
		return
	}

	var req BindKBRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	userID := middleware.GetUserID(c)

	// Check for duplicate binding
	var existing model.WeKnoraKBRef
	if result := h.db.Where("course_id = ? AND kb_id = ?", courseID, req.KBID).First(&existing); result.Error == nil {
		c.JSON(http.StatusConflict, ErrorResponse{Error: "knowledge base already bound to this course"})
		return
	}

	ref := model.WeKnoraKBRef{
		CourseID:  courseID,
		KBID:      req.KBID,
		KBName:    req.KBName,
		AddedByID: userID,
	}

	if result := h.db.Create(&ref); result.Error != nil {
		slogWeKnora.Error("failed to bind knowledge base", "course_id", courseID, "kb_id", req.KBID, "error", result.Error)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to bind knowledge base"})
		return
	}

	c.JSON(http.StatusCreated, ref)
}

// ListBoundKnowledgeBases returns all WeKnora KBs bound to a course.
//
//	@Summary      获取课程绑定的知识库列表
//	@Description  返回指定课程已绑定的所有 WeKnora 知识库
//	@Tags         WeKnora
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id path int true "课程 ID"
//	@Success      200 {array}   model.WeKnoraKBRef
//	@Failure      400 {object}  ErrorResponse
//	@Failure      500 {object}  ErrorResponse
//	@Router       /courses/{id}/weknora-refs [get]
func (h *WeKnoraHandler) ListBoundKnowledgeBases(c *gin.Context) {
	courseID, err := parseCourseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid course ID"})
		return
	}

	var refs []model.WeKnoraKBRef
	if result := h.db.Where("course_id = ?", courseID).Order("created_at DESC").Find(&refs); result.Error != nil {
		slogWeKnora.Error("failed to list bound knowledge bases", "course_id", courseID, "error", result.Error)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to list bound knowledge bases"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": refs})
}

// UnbindKnowledgeBase removes a WeKnora KB binding from a course.
//
//	@Summary      解绑知识库
//	@Description  从课程中移除 WeKnora 知识库绑定
//	@Tags         WeKnora
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id     path int true "课程 ID"
//	@Param        ref_id path int true "绑定记录 ID"
//	@Success      200 {object}  map[string]interface{}
//	@Failure      400 {object}  ErrorResponse
//	@Failure      404 {object}  ErrorResponse
//	@Router       /courses/{id}/weknora-refs/{ref_id} [delete]
func (h *WeKnoraHandler) UnbindKnowledgeBase(c *gin.Context) {
	courseID, err := parseCourseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid course ID"})
		return
	}

	refID, err := strconv.ParseUint(c.Param("ref_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid ref_id"})
		return
	}

	result := h.db.Where("id = ? AND course_id = ?", refID, courseID).Delete(&model.WeKnoraKBRef{})
	if result.Error != nil {
		slogWeKnora.Error("failed to unbind knowledge base", "ref_id", refID, "error", result.Error)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to unbind knowledge base"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "binding not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "knowledge base unbound successfully"})
}

// ── WeKnora Search ───────────────────────────────────────────

// WeKnoraSearchRequest represents a search request against bound WeKnora KBs.
type WeKnoraSearchRequest struct {
	Query string `json:"query" binding:"required"`
	TopK  int    `json:"top_k"`
}

// SearchKnowledgeBase searches within the WeKnora knowledge bases bound to a course.
//
//	@Summary      在知识库中检索
//	@Description  在课程绑定的 WeKnora 知识库中进行语义检索
//	@Tags         WeKnora
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id   path int                   true "课程 ID"
//	@Param        body body WeKnoraSearchRequest   true "检索请求"
//	@Success      200  {object} weknora.RetrievalResponse
//	@Failure      400  {object} ErrorResponse
//	@Failure      500  {object} ErrorResponse
//	@Router       /courses/{id}/weknora-search [post]
func (h *WeKnoraHandler) SearchKnowledgeBase(c *gin.Context) {
	courseID, err := parseCourseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid course ID"})
		return
	}

	var req WeKnoraSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	if req.TopK <= 0 {
		req.TopK = 5
	}

	// Get bound knowledge bases for this course
	var refs []model.WeKnoraKBRef
	if result := h.db.Where("course_id = ?", courseID).Find(&refs); result.Error != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to query bound knowledge bases"})
		return
	}

	if len(refs) == 0 {
		c.JSON(http.StatusOK, gin.H{"results": []interface{}{}, "total": 0, "message": "no WeKnora knowledge bases bound to this course"})
		return
	}

	// Use per-user client
	client := h.getUserClient(c)

	// Create a session with all bound KBs
	kbIDs := make([]string, len(refs))
	for i, ref := range refs {
		kbIDs[i] = ref.KBID
	}

	session, err := client.CreateSession(c.Request.Context(), &weknora.CreateSessionRequest{
		KnowledgeBases: kbIDs,
	})
	if err != nil {
		slogWeKnora.Error("failed to create WeKnora session", "error", err)
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "failed to create WeKnora retrieval session"})
		return
	}

	// Perform retrieval
	retrievalResp, err := client.Retrieve(c.Request.Context(), session.ID, &weknora.RetrievalRequest{
		Question: req.Query,
		TopK:     req.TopK,
	})
	if err != nil {
		slogWeKnora.Error("failed to retrieve from WeKnora", "session_id", session.ID, "error", err)
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "retrieval failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, retrievalResp)
}

// CreateKnowledgeBase creates a new knowledge base in WeKnora.
//
//	@Summary      创建 WeKnora 知识库
//	@Description  在 WeKnora 中创建新的知识库
//	@Tags         WeKnora
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        request body weknora.CreateKBRequest true "创建知识库请求"
//	@Success      201 {object}  weknora.KnowledgeBase
//	@Failure      400 {object}  ErrorResponse
//	@Failure      500 {object}  ErrorResponse
//	@Router       /weknora/knowledge-bases [post]
func (h *WeKnoraHandler) CreateKnowledgeBase(c *gin.Context) {
	slogWeKnora.Info("CreateKnowledgeBase called")
	var req weknora.CreateKBRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	client := h.getUserClient(c)
	kb, err := client.CreateKnowledgeBase(c.Request.Context(), &req)
	if err != nil {
		slogWeKnora.Error("failed to create knowledge base", "error", err)
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "failed to create knowledge base: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, kb)
}

// DeleteKnowledgeBase deletes a knowledge base from WeKnora.
//
//	@Summary      删除 WeKnora 知识库
//	@Description  从 WeKnora 中删除指定知识库
//	@Tags         WeKnora
//	@Produce      json
//	@Security     BearerAuth
//	@Param        kb_id path string true "WeKnora 知识库 ID"
//	@Success      200 {object}  map[string]string
//	@Failure      500 {object}  ErrorResponse
//	@Router       /weknora/knowledge-bases/{kb_id} [delete]
func (h *WeKnoraHandler) DeleteKnowledgeBase(c *gin.Context) {
	kbID := c.Param("kb_id")
	client := h.getUserClient(c)

	if err := client.DeleteKnowledgeBase(c.Request.Context(), kbID); err != nil {
		slogWeKnora.Error("failed to delete knowledge base", "kb_id", kbID, "error", err)
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "failed to delete knowledge base: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "knowledge base deleted successfully"})
}
