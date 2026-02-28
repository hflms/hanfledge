package handler

import (
	"context"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hflms/hanfledge/internal/delivery/http/middleware"
	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/cache"
	"github.com/hflms/hanfledge/internal/infrastructure/storage"
	"github.com/hflms/hanfledge/internal/usecase"
	"github.com/ledongthuc/pdf"
	"gorm.io/gorm"
)

// CourseHandler handles course and material management.
type CourseHandler struct {
	DB      *gorm.DB
	KARAG   *usecase.KARAGEngine
	Cache   *cache.RedisCache   // nil if Redis unavailable
	Storage storage.FileStorage // File storage backend
}

// NewCourseHandler creates a new CourseHandler.
func NewCourseHandler(db *gorm.DB, karag *usecase.KARAGEngine, redisCache *cache.RedisCache, fs storage.FileStorage) *CourseHandler {
	return &CourseHandler{DB: db, KARAG: karag, Cache: redisCache, Storage: fs}
}

// ListCourses returns courses for the authenticated teacher.
// GET /api/v1/courses?school_id=X
func (h *CourseHandler) ListCourses(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var courses []model.Course
	query := h.DB.Preload("Chapters.KnowledgePoints")

	if schoolID := c.Query("school_id"); schoolID != "" {
		query = query.Where("school_id = ?", schoolID)
	}
	// Teachers see only their courses; admins see all
	query = query.Where("teacher_id = ?", userID)

	if err := query.Find(&courses).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询课程失败"})
		return
	}
	c.JSON(http.StatusOK, courses)
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
// POST /api/v1/courses
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

	if err := h.DB.Create(&course).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建课程失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, course)
}

// UploadMaterial handles PDF upload and triggers the KA-RAG pipeline.
// POST /api/v1/courses/:id/materials
func (h *CourseHandler) UploadMaterial(c *gin.Context) {
	courseID := c.Param("id")

	// Verify course exists
	var course model.Course
	if err := h.DB.First(&course, courseID).Error; err != nil {
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
	storageKey := filepath.Join(courseID, uuid.New().String()+".pdf")
	if err := h.Storage.Upload(c.Request.Context(), storageKey, file, "application/pdf"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
		return
	}

	// Resolve local path for PDF text extraction
	// For local storage, URL returns the filesystem path; for OSS, extractPDFText
	// would need to download first (future enhancement).
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
	h.DB.Create(&doc)

	// Extract text from PDF
	rawText, pageCount, err := extractPDFText(filePath)
	if err != nil {
		h.DB.Model(&doc).Updates(map[string]interface{}{
			"status": model.DocStatusFailed,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "PDF 解析失败: " + err.Error()})
		return
	}
	h.DB.Model(&doc).Update("page_count", pageCount)

	// Trigger KA-RAG pipeline asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		if err := h.KARAG.ProcessDocument(ctx, &doc, rawText); err != nil {
			log.Printf("❌ KA-RAG pipeline failed for doc %d: %v", doc.ID, err)
			h.DB.Model(&doc).Update("status", model.DocStatusFailed)
			return
		}

		// Invalidate L2 semantic cache for this course (§8.1.3)
		// Course materials changed → cached responses may be stale
		if h.Cache != nil {
			if err := h.Cache.InvalidateSemanticCacheByCourse(ctx, doc.CourseID); err != nil {
				log.Printf("⚠️  [Cache] Invalidate L2 cache for course=%d failed: %v", doc.CourseID, err)
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
// GET /api/v1/courses/:id/outline
func (h *CourseHandler) GetOutline(c *gin.Context) {
	courseID := c.Param("id")

	var course model.Course
	if err := h.DB.Preload("Chapters", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	}).Preload("Chapters.KnowledgePoints.MountedSkills").
		First(&course, courseID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "课程不存在"})
		return
	}

	// Also include document processing status
	var docs []model.Document
	h.DB.Where("course_id = ?", courseID).Find(&docs)

	c.JSON(http.StatusOK, gin.H{
		"course":    course,
		"documents": docs,
	})
}

// GetDocumentStatus returns the processing status of uploaded documents.
// GET /api/v1/courses/:id/documents
func (h *CourseHandler) GetDocumentStatus(c *gin.Context) {
	courseID := c.Param("id")

	var docs []model.Document
	h.DB.Where("course_id = ?", courseID).Order("created_at DESC").Find(&docs)

	c.JSON(http.StatusOK, docs)
}

// SearchRequest represents the semantic search request body.
type SearchRequest struct {
	Query string `json:"query" binding:"required"`
	TopK  int    `json:"top_k"`
}

// SearchCourse performs semantic search on course documents.
// POST /api/v1/courses/:id/search
func (h *CourseHandler) SearchCourse(c *gin.Context) {
	courseID := c.Param("id")

	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供查询内容"})
		return
	}

	if req.TopK <= 0 || req.TopK > 20 {
		req.TopK = 5
	}

	// Verify course exists
	var course model.Course
	if err := h.DB.First(&course, courseID).Error; err != nil {
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
// DELETE /api/v1/courses/:id/documents/:doc_id
func (h *CourseHandler) DeleteDocument(c *gin.Context) {
	courseID := c.Param("id")
	docID := c.Param("doc_id")

	var doc model.Document
	if err := h.DB.Where("id = ? AND course_id = ?", docID, courseID).First(&doc).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文档不存在"})
		return
	}

	// Don't allow deleting documents that are currently processing
	if doc.Status == model.DocStatusProcessing {
		c.JSON(http.StatusConflict, gin.H{"error": "文档正在处理中，无法删除"})
		return
	}

	// Delete chunks first (cascade)
	h.DB.Where("document_id = ?", doc.ID).Delete(&model.DocumentChunk{})

	// Delete the file from storage
	if doc.FilePath != "" {
		if err := h.Storage.Delete(c.Request.Context(), doc.FilePath); err != nil {
			log.Printf("⚠️  [Storage] Delete file %s failed: %v", doc.FilePath, err)
		}
	}

	// Delete the document record
	h.DB.Delete(&doc)

	// Invalidate L2 semantic cache for this course
	if h.Cache != nil {
		ctx := c.Request.Context()
		if err := h.Cache.InvalidateSemanticCacheByCourse(ctx, doc.CourseID); err != nil {
			log.Printf("⚠️  [Cache] Invalidate L2 cache for course=%d failed: %v", doc.CourseID, err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "文档已删除"})
}

// RetryDocument retriggers the KA-RAG pipeline for a failed document.
// POST /api/v1/courses/:id/documents/:doc_id/retry
func (h *CourseHandler) RetryDocument(c *gin.Context) {
	courseID := c.Param("id")
	docID := c.Param("doc_id")

	var doc model.Document
	if err := h.DB.Where("id = ? AND course_id = ?", docID, courseID).First(&doc).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文档不存在"})
		return
	}

	if doc.Status != model.DocStatusFailed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只能重试失败的文档"})
		return
	}

	// Verify file still exists in storage
	exists, err := h.Storage.Exists(c.Request.Context(), doc.FilePath)
	if err != nil || !exists {
		c.JSON(http.StatusGone, gin.H{"error": "原始文件已丢失，请重新上传"})
		return
	}

	// Resolve local path for PDF text extraction
	filePath, _ := h.Storage.URL(c.Request.Context(), doc.FilePath)

	// Re-extract text
	rawText, pageCount, err := extractPDFText(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "PDF 解析失败: " + err.Error()})
		return
	}

	// Delete old chunks before reprocessing
	h.DB.Where("document_id = ?", doc.ID).Delete(&model.DocumentChunk{})

	// Update status to processing
	h.DB.Model(&doc).Updates(map[string]interface{}{
		"status":     model.DocStatusProcessing,
		"page_count": pageCount,
	})

	// Trigger KA-RAG pipeline asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		if err := h.KARAG.ProcessDocument(ctx, &doc, rawText); err != nil {
			log.Printf("❌ KA-RAG retry failed for doc %d: %v", doc.ID, err)
			h.DB.Model(&doc).Update("status", model.DocStatusFailed)
			return
		}

		// Invalidate L2 semantic cache
		if h.Cache != nil {
			if err := h.Cache.InvalidateSemanticCacheByCourse(ctx, doc.CourseID); err != nil {
				log.Printf("⚠️  [Cache] Invalidate L2 cache for course=%d failed: %v", doc.CourseID, err)
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
