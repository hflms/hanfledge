package handler

import (
	"context"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hflms/hanfledge/internal/delivery/http/middleware"
	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/cache"
	"github.com/hflms/hanfledge/internal/infrastructure/logger"
	"github.com/hflms/hanfledge/internal/infrastructure/storage"
	"github.com/hflms/hanfledge/internal/repository"
	"github.com/hflms/hanfledge/internal/usecase"
	"github.com/ledongthuc/pdf"
)

var slogCourse = logger.L("Course")

// parseCourseID converts a string course ID param to uint.
func parseCourseID(s string) (uint, error) {
	id, err := strconv.ParseUint(s, 10, 64)
	return uint(id), err
}

// parseDocID converts a string document ID param to uint.
func parseDocID(s string) (uint, error) {
	id, err := strconv.ParseUint(s, 10, 64)
	return uint(id), err
}

// CourseHandler handles course and material management.
type CourseHandler struct {
	Courses repository.CourseRepository
	Docs    repository.DocumentRepository
	KARAG   *usecase.KARAGEngine
	Cache   *cache.RedisCache   // nil if Redis unavailable
	Storage storage.FileStorage // File storage backend
}

// NewCourseHandler creates a new CourseHandler.
func NewCourseHandler(courses repository.CourseRepository, docs repository.DocumentRepository, karag *usecase.KARAGEngine, redisCache *cache.RedisCache, fs storage.FileStorage) *CourseHandler {
	return &CourseHandler{Courses: courses, Docs: docs, KARAG: karag, Cache: redisCache, Storage: fs}
}

// ListCourses returns courses for the authenticated teacher.
//
//	@Summary      课程列表
//	@Description  返回当前教师的课程列表（支持分页和学校筛选）
//	@Tags         Courses
//	@Produce      json
//	@Security     BearerAuth
//	@Param        school_id  query     int  false  "学校 ID"
//	@Param        page       query     int  false  "页码"  default(1)
//	@Param        limit      query     int  false  "每页数量"  default(20)
//	@Success      200        {object}  PaginatedResponse
//	@Failure      500        {object}  ErrorResponse
//	@Router       /courses [get]
func (h *CourseHandler) ListCourses(c *gin.Context) {
	userID := middleware.GetUserID(c)
	p := ParsePagination(c)

	var schoolID uint
	if s := c.Query("school_id"); s != "" {
		if id, err := parseCourseID(s); err == nil {
			schoolID = id
		}
	}

	courses, total, err := h.Courses.ListByTeacher(c.Request.Context(), userID, schoolID, p.Offset, p.Limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询课程失败"})
		return
	}
	c.JSON(http.StatusOK, NewPaginatedResponse(courses, total, p))
}

// CreateCourseRequest represents the request body for creating a course.
type CreateCourseRequest struct {
	SchoolID    uint   `json:"school_id" binding:"required"`
	Title       string `json:"title" binding:"required"`
	Subject     string `json:"subject" binding:"required"`
	GradeLevel  int    `json:"grade_level" binding:"required"`
	Description string `json:"description"`
}

// CreateCourse creates a new course.
//
//	@Summary      创建课程
//	@Description  为当前教师创建一门新课程
//	@Tags         Courses
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        body  body      CreateCourseRequest  true  "课程信息"
//	@Success      201   {object}  model.Course
//	@Failure      400   {object}  ErrorResponse
//	@Failure      500   {object}  ErrorResponse
//	@Router       /courses [post]
func (h *CourseHandler) CreateCourse(c *gin.Context) {
	var req CreateCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求数据格式错误: " + err.Error()})
		return
	}

	userID := middleware.GetUserID(c)
	desc := req.Description

	course := model.Course{
		SchoolID:    req.SchoolID,
		TeacherID:   userID,
		Title:       req.Title,
		Subject:     req.Subject,
		GradeLevel:  req.GradeLevel,
		Description: &desc,
		Status:      model.CourseStatusDraft,
	}

	if err := h.Courses.Create(c.Request.Context(), &course); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建课程失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, course)
}

// UploadMaterial handles PDF upload and triggers the KA-RAG pipeline.
//
//	@Summary      上传教学材料
//	@Description  上传 PDF 文件并异步触发 KA-RAG 知识抽取管线
//	@Tags         Courses
//	@Accept       multipart/form-data
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id    path      int   true  "课程 ID"
//	@Param        file  formData  file  true  "PDF 文件"
//	@Success      202   {object}  map[string]interface{}
//	@Failure      400   {object}  ErrorResponse
//	@Failure      500   {object}  ErrorResponse
//	@Router       /courses/{id}/materials [post]
func (h *CourseHandler) UploadMaterial(c *gin.Context) {
	courseID, err := parseCourseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的课程 ID"})
		return
	}

	// Verify course exists
	course, err := h.Courses.FindByID(c.Request.Context(), courseID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "课程不存在"})
		return
	}

	// Get uploaded file
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请上传文件"})
		return
	}
	defer file.Close()

	// Validate file type
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".pdf") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "仅支持 PDF 格式"})
		return
	}

	// Validate file size (50 MB max)
	if header.Size > MaxFileSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件大小不能超过 50 MB"})
		return
	}

	// Save file via storage backend
	courseIDStr := c.Param("id")
	storageKey := filepath.Join(courseIDStr, uuid.New().String()+".pdf")
	if err := h.Storage.Upload(c.Request.Context(), storageKey, file, "application/pdf"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
		return
	}

	// Resolve local path for PDF text extraction
	filePath, _ := h.Storage.URL(c.Request.Context(), storageKey)

	// Create document record
	doc := model.Document{
		CourseID: course.ID,
		FileName: header.Filename,
		FilePath: storageKey,
		FileSize: header.Size,
		MimeType: "application/pdf",
		Status:   model.DocStatusUploaded,
	}
	if err := h.Docs.Create(c.Request.Context(), &doc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建文档记录失败"})
		return
	}

	// Extract text from PDF
	rawText, pageCount, err := extractPDFText(filePath)
	if err != nil {
		if updateErr := h.Docs.UpdateStatus(context.Background(), doc.ID, model.DocStatusFailed); updateErr != nil {
			slogCourse.Warn("failed to update doc status to failed", "doc_id", doc.ID, "err", updateErr)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "PDF 解析失败: " + err.Error()})
		return
	}
	if updateErr := h.Docs.UpdateFields(c.Request.Context(), doc.ID, map[string]interface{}{"page_count": pageCount}); updateErr != nil {
		slogCourse.Warn("failed to update page_count", "doc_id", doc.ID, "err", updateErr)
	}

	// Trigger KA-RAG pipeline asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		if err := h.KARAG.ProcessDocument(ctx, &doc, rawText); err != nil {
			slogCourse.Error("ka-rag pipeline failed", "doc_id", doc.ID, "err", err)
			if updateErr := h.Docs.UpdateStatus(ctx, doc.ID, model.DocStatusFailed); updateErr != nil {
				slogCourse.Warn("failed to update doc status", "doc_id", doc.ID, "err", updateErr)
			}
			return
		}

		// Invalidate L2 semantic cache for this course (§8.1.3)
		if h.Cache != nil {
			if err := h.Cache.InvalidateSemanticCacheByCourse(ctx, doc.CourseID); err != nil {
				slogCourse.Warn("invalidate L2 cache failed", "course_id", doc.CourseID, "err", err)
			}
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message":     "文件已上传，正在后台处理知识图谱...",
		"document":    doc,
		"page_count":  pageCount,
		"text_length": len(rawText),
	})
}

// GetOutline returns the AI-generated course outline (chapters + knowledge points).
//
//	@Summary      获取课程大纲
//	@Description  返回 AI 生成的课程大纲（章节 + 知识点）及文档状态
//	@Tags         Courses
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id  path      int  true  "课程 ID"
//	@Success      200 {object}  map[string]interface{}
//	@Failure      400 {object}  ErrorResponse
//	@Failure      404 {object}  ErrorResponse
//	@Router       /courses/{id}/outline [get]
func (h *CourseHandler) GetOutline(c *gin.Context) {
	courseID, err := parseCourseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的课程 ID"})
		return
	}

	course, err := h.Courses.FindWithOutline(c.Request.Context(), courseID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "课程不存在"})
		return
	}

	// Also include document processing status
	docs, _ := h.Docs.FindByCourseID(c.Request.Context(), courseID)

	c.JSON(http.StatusOK, gin.H{
		"course":    course,
		"documents": docs,
	})
}

// GetDocumentStatus returns the processing status of uploaded documents.
//
//	@Summary      文档处理状态
//	@Description  返回课程下所有上传文档的处理状态
//	@Tags         Courses
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id  path      int  true  "课程 ID"
//	@Success      200 {array}   model.Document
//	@Failure      400 {object}  ErrorResponse
//	@Failure      500 {object}  ErrorResponse
//	@Router       /courses/{id}/documents [get]
func (h *CourseHandler) GetDocumentStatus(c *gin.Context) {
	courseID, err := parseCourseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的课程 ID"})
		return
	}

	docs, err := h.Docs.FindByCourseIDOrdered(c.Request.Context(), courseID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询文档失败"})
		return
	}

	c.JSON(http.StatusOK, docs)
}

// SearchRequest represents the semantic search request body.
type SearchRequest struct {
	Query string `json:"query" binding:"required"`
	TopK  int    `json:"top_k"`
}

// SearchCourse performs semantic search on course documents.
//
//	@Summary      课程语义搜索
//	@Description  在课程文档中进行语义检索，返回相关知识片段
//	@Tags         Courses
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id    path      int            true  "课程 ID"
//	@Param        body  body      SearchRequest  true  "搜索请求"
//	@Success      200   {object}  map[string]interface{}
//	@Failure      400   {object}  ErrorResponse
//	@Failure      500   {object}  ErrorResponse
//	@Router       /courses/{id}/search [post]
func (h *CourseHandler) SearchCourse(c *gin.Context) {
	courseID, err := parseCourseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的课程 ID"})
		return
	}

	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供查询内容"})
		return
	}

	if req.TopK <= 0 || req.TopK > 20 {
		req.TopK = 5
	}

	// Verify course exists
	course, err := h.Courses.FindByID(c.Request.Context(), courseID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "课程不存在"})
		return
	}

	results, err := h.KARAG.SemanticSearch(c.Request.Context(), course.ID, req.Query, req.TopK)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "检索失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"query":   req.Query,
		"results": results,
		"count":   len(results),
	})
}

// -- Document Management ----------------------------------------

// MaxFileSize is the maximum allowed upload size (50 MB).
const MaxFileSize = 50 * 1024 * 1024

// DeleteDocument removes a document and its associated chunks and file.
//
//	@Summary      删除文档
//	@Description  删除文档及其关联的分块和存储文件
//	@Tags         Courses
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id      path      int  true  "课程 ID"
//	@Param        doc_id  path      int  true  "文档 ID"
//	@Success      200     {object}  map[string]string
//	@Failure      400     {object}  ErrorResponse
//	@Failure      404     {object}  ErrorResponse
//	@Router       /courses/{id}/documents/{doc_id} [delete]
func (h *CourseHandler) DeleteDocument(c *gin.Context) {
	courseID, err := parseCourseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的课程 ID"})
		return
	}
	docID, err := parseDocID(c.Param("doc_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的文档 ID"})
		return
	}

	ctx := c.Request.Context()
	doc, err := h.Docs.FindByIDAndCourseID(ctx, docID, courseID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文档不存在"})
		return
	}

	// Don't allow deleting documents that are currently processing
	if doc.Status == model.DocStatusProcessing {
		c.JSON(http.StatusConflict, gin.H{"error": "文档正在处理中，无法删除"})
		return
	}

	// Delete chunks first (cascade)
	if err := h.Docs.DeleteChunksByDocumentID(ctx, doc.ID); err != nil {
		slogCourse.Warn("failed to delete chunks", "doc_id", doc.ID, "err", err)
	}

	// Delete the file from storage
	if doc.FilePath != "" {
		if err := h.Storage.Delete(ctx, doc.FilePath); err != nil {
			slogCourse.Warn("delete file failed", "path", doc.FilePath, "err", err)
		}
	}

	// Delete the document record
	if err := h.Docs.Delete(ctx, doc); err != nil {
		slogCourse.Warn("failed to delete doc", "doc_id", doc.ID, "err", err)
	}

	// Invalidate L2 semantic cache for this course
	if h.Cache != nil {
		if err := h.Cache.InvalidateSemanticCacheByCourse(ctx, doc.CourseID); err != nil {
			slogCourse.Warn("invalidate L2 cache failed", "course_id", doc.CourseID, "err", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "文档已删除"})
}

// RetryDocument retriggers the KA-RAG pipeline for a failed document.
//
//	@Summary      重试文档处理
//	@Description  重新触发 KA-RAG 管线处理失败的文档
//	@Tags         Courses
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id      path      int  true  "课程 ID"
//	@Param        doc_id  path      int  true  "文档 ID"
//	@Success      202     {object}  map[string]interface{}
//	@Failure      400     {object}  ErrorResponse
//	@Failure      404     {object}  ErrorResponse
//	@Failure      409     {object}  ErrorResponse
//	@Router       /courses/{id}/documents/{doc_id}/retry [post]
func (h *CourseHandler) RetryDocument(c *gin.Context) {
	courseID, err := parseCourseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的课程 ID"})
		return
	}
	docID, err := parseDocID(c.Param("doc_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的文档 ID"})
		return
	}

	ctx := c.Request.Context()
	doc, err := h.Docs.FindByIDAndCourseID(ctx, docID, courseID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文档不存在"})
		return
	}

	if doc.Status != model.DocStatusFailed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只能重试失败的文档"})
		return
	}

	// Verify file still exists in storage
	exists, err := h.Storage.Exists(ctx, doc.FilePath)
	if err != nil || !exists {
		c.JSON(http.StatusGone, gin.H{"error": "原始文件已丢失，请重新上传"})
		return
	}

	// Resolve local path for PDF text extraction
	filePath, _ := h.Storage.URL(ctx, doc.FilePath)

	// Re-extract text
	rawText, pageCount, err := extractPDFText(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "PDF 解析失败: " + err.Error()})
		return
	}

	// Delete old chunks before reprocessing
	if err := h.Docs.DeleteChunksByDocumentID(ctx, doc.ID); err != nil {
		slogCourse.Warn("failed to delete old chunks", "doc_id", doc.ID, "err", err)
	}

	// Update status to processing
	if err := h.Docs.UpdateFields(ctx, doc.ID, map[string]interface{}{
		"status":     model.DocStatusProcessing,
		"page_count": pageCount,
	}); err != nil {
		slogCourse.Warn("failed to update doc to processing", "doc_id", doc.ID, "err", err)
	}

	// Trigger KA-RAG pipeline asynchronously
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		if err := h.KARAG.ProcessDocument(bgCtx, doc, rawText); err != nil {
			slogCourse.Error("ka-rag retry failed", "doc_id", doc.ID, "err", err)
			if updateErr := h.Docs.UpdateStatus(bgCtx, doc.ID, model.DocStatusFailed); updateErr != nil {
				slogCourse.Warn("failed to update doc status", "doc_id", doc.ID, "err", updateErr)
			}
			return
		}

		// Invalidate L2 semantic cache
		if h.Cache != nil {
			if err := h.Cache.InvalidateSemanticCacheByCourse(bgCtx, doc.CourseID); err != nil {
				slogCourse.Warn("invalidate L2 cache failed", "course_id", doc.CourseID, "err", err)
			}
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message":    "文档重新处理已启动",
		"document":   doc,
		"page_count": pageCount,
	})
}

// extractPDFText extracts all text content from a PDF file.
func extractPDFText(filePath string) (string, int, error) {
	f, r, err := pdf.Open(filePath)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	totalPages := r.NumPage()
	var textBuilder strings.Builder

	for i := 1; i <= totalPages; i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		text, err := p.GetPlainText(nil)
		if err != nil {
			continue // Skip problematic pages
		}
		textBuilder.WriteString(text)
		textBuilder.WriteString("\n\n")
	}

	return textBuilder.String(), totalPages, nil
}
