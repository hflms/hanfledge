package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/delivery/http/middleware"
	"github.com/hflms/hanfledge/internal/domain/model"
	neo4jRepo "github.com/hflms/hanfledge/internal/repository/neo4j"
	"gorm.io/gorm"
)

// KnowledgeGraphHandler handles knowledge graph enrichment operations.
type KnowledgeGraphHandler struct {
	DB    *gorm.DB
	Neo4j *neo4jRepo.Client
}

// NewKnowledgeGraphHandler creates a new KnowledgeGraphHandler.
func NewKnowledgeGraphHandler(db *gorm.DB, neo4jClient *neo4jRepo.Client) *KnowledgeGraphHandler {
	return &KnowledgeGraphHandler{DB: db, Neo4j: neo4jClient}
}

// ── Misconception CRUD ──────────────────────────────────────

// CreateMisconceptionRequest 创建误区请求。
type CreateMisconceptionRequest struct {
	Description string         `json:"description" binding:"required"`
	TrapType    model.TrapType `json:"trap_type" binding:"required"`
	Severity    float64        `json:"severity"`
}

// CreateMisconception creates a misconception linked to a knowledge point.
// POST /api/v1/knowledge-points/:id/misconceptions
func (h *KnowledgeGraphHandler) CreateMisconception(c *gin.Context) {
	kpID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的知识点 ID"})
		return
	}

	// Verify KP exists
	var kp model.KnowledgePoint
	if err := h.DB.First(&kp, kpID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "知识点不存在"})
		return
	}

	var req CreateMisconceptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求数据格式错误: " + err.Error()})
		return
	}

	// Validate trap type
	if !isValidTrapType(req.TrapType) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的误区类型，可选: conceptual, procedural, intuitive, transfer"})
		return
	}

	if req.Severity <= 0 || req.Severity > 1 {
		req.Severity = 0.5
	}

	// Create in PostgreSQL
	misconception := model.Misconception{
		KPID:        uint(kpID),
		Description: req.Description,
		TrapType:    req.TrapType,
		Severity:    req.Severity,
	}
	if err := h.DB.Create(&misconception).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建误区失败: " + err.Error()})
		return
	}

	// Sync to Neo4j: create Misconception node + HAS_TRAP relation
	misconception.Neo4jNodeID = fmt.Sprintf("misconception_%d", misconception.ID)
	h.DB.Model(&misconception).Update("neo4j_node_id", misconception.Neo4jNodeID)

	if h.Neo4j != nil {
		if err := h.Neo4j.CreateMisconceptionNode(
			c.Request.Context(),
			uint(kpID), misconception.ID,
			req.Description, string(req.TrapType),
		); err != nil {
			// Non-fatal: PG record exists, Neo4j sync failed
			c.JSON(http.StatusCreated, gin.H{
				"misconception": misconception,
				"warning":       "Neo4j 同步失败，请稍后重试: " + err.Error(),
			})
			return
		}
	}

	c.JSON(http.StatusCreated, misconception)
}

// ListMisconceptions returns all misconceptions for a knowledge point.
// GET /api/v1/knowledge-points/:id/misconceptions
func (h *KnowledgeGraphHandler) ListMisconceptions(c *gin.Context) {
	kpID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的知识点 ID"})
		return
	}

	var misconceptions []model.Misconception
	if err := h.DB.Where("kp_id = ?", kpID).Order("severity DESC").Find(&misconceptions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询误区失败"})
		return
	}

	c.JSON(http.StatusOK, misconceptions)
}

// DeleteMisconception removes a misconception from both PG and Neo4j.
// DELETE /api/v1/knowledge-points/:id/misconceptions/:misconception_id
func (h *KnowledgeGraphHandler) DeleteMisconception(c *gin.Context) {
	kpID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的知识点 ID"})
		return
	}

	mID, err := strconv.ParseUint(c.Param("misconception_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的误区 ID"})
		return
	}

	// Verify misconception exists and belongs to this KP
	var misconception model.Misconception
	if err := h.DB.Where("id = ? AND kp_id = ?", mID, kpID).First(&misconception).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "误区不存在"})
		return
	}

	// Delete from Neo4j first (non-fatal)
	if h.Neo4j != nil {
		_ = h.Neo4j.DeleteMisconceptionNode(c.Request.Context(), uint(mID))
	}

	// Delete from PostgreSQL
	if err := h.DB.Delete(&misconception).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除误区失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "误区已删除"})
}

// ── Cross-Disciplinary Links ────────────────────────────────

// CreateCrossLinkRequest 创建跨学科联结请求。
type CreateCrossLinkRequest struct {
	ToKPID   uint    `json:"to_kp_id" binding:"required"`
	LinkType string  `json:"link_type" binding:"required"`
	Weight   float64 `json:"weight"`
}

// CreateCrossLink creates a cross-disciplinary link between two KPs.
// POST /api/v1/knowledge-points/:id/cross-links
func (h *KnowledgeGraphHandler) CreateCrossLink(c *gin.Context) {
	fromKPID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的知识点 ID"})
		return
	}

	var req CreateCrossLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求数据格式错误: " + err.Error()})
		return
	}

	// Validate link type
	if !isValidLinkType(req.LinkType) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的联结类型，可选: analogy, shared_model, application"})
		return
	}

	if req.Weight <= 0 || req.Weight > 1 {
		req.Weight = 1.0
	}

	// Verify both KPs exist
	var fromKP, toKP model.KnowledgePoint
	if err := h.DB.First(&fromKP, fromKPID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "源知识点不存在"})
		return
	}
	if err := h.DB.First(&toKP, req.ToKPID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "目标知识点不存在"})
		return
	}

	// Prevent self-link
	if uint(fromKPID) == req.ToKPID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能创建自身到自身的联结"})
		return
	}

	// Create in PostgreSQL
	crossLink := model.CrossLink{
		FromKPID: uint(fromKPID),
		ToKPID:   req.ToKPID,
		LinkType: req.LinkType,
		Weight:   req.Weight,
	}
	if err := h.DB.Create(&crossLink).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建跨学科联结失败: " + err.Error()})
		return
	}

	// Sync to Neo4j
	if h.Neo4j != nil {
		if err := h.Neo4j.CreateCrossLink(
			c.Request.Context(),
			uint(fromKPID), req.ToKPID,
			req.LinkType, req.Weight,
		); err != nil {
			c.JSON(http.StatusCreated, gin.H{
				"cross_link": crossLink,
				"warning":    "Neo4j 同步失败: " + err.Error(),
			})
			return
		}
	}

	c.JSON(http.StatusCreated, crossLink)
}

// ListCrossLinks returns all cross-disciplinary links for a knowledge point.
// GET /api/v1/knowledge-points/:id/cross-links
func (h *KnowledgeGraphHandler) ListCrossLinks(c *gin.Context) {
	kpID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的知识点 ID"})
		return
	}

	// Query from PG (bidirectional)
	var links []model.CrossLink
	if err := h.DB.Where("from_kp_id = ? OR to_kp_id = ?", kpID, kpID).
		Find(&links).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询跨学科联结失败"})
		return
	}

	// Enrich with KP titles
	type CrossLinkResponse struct {
		model.CrossLink
		FromKPTitle string `json:"from_kp_title"`
		ToKPTitle   string `json:"to_kp_title"`
	}
	var response []CrossLinkResponse
	for _, link := range links {
		var fromKP, toKP model.KnowledgePoint
		h.DB.Select("title").First(&fromKP, link.FromKPID)
		h.DB.Select("title").First(&toKP, link.ToKPID)
		response = append(response, CrossLinkResponse{
			CrossLink:   link,
			FromKPTitle: fromKP.Title,
			ToKPTitle:   toKP.Title,
		})
	}

	c.JSON(http.StatusOK, response)
}

// DeleteCrossLink removes a cross-disciplinary link.
// DELETE /api/v1/knowledge-points/:id/cross-links/:link_id
func (h *KnowledgeGraphHandler) DeleteCrossLink(c *gin.Context) {
	kpID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的知识点 ID"})
		return
	}

	linkID, err := strconv.ParseUint(c.Param("link_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的联结 ID"})
		return
	}

	// Verify link exists and involves this KP
	var link model.CrossLink
	if err := h.DB.Where("id = ? AND (from_kp_id = ? OR to_kp_id = ?)", linkID, kpID, kpID).
		First(&link).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "联结不存在"})
		return
	}

	// Delete from Neo4j (non-fatal)
	if h.Neo4j != nil {
		_ = h.Neo4j.DeleteCrossLink(c.Request.Context(), link.FromKPID, link.ToKPID)
	}

	// Delete from PostgreSQL
	if err := h.DB.Delete(&link).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除联结失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "跨学科联结已删除"})
}

// ── Prerequisites Management ────────────────────────────────

// CreatePrereqRequest 创建前置依赖请求。
type CreatePrereqRequest struct {
	PrereqKPID uint `json:"prereq_kp_id" binding:"required"`
}

// CreatePrerequisite creates a REQUIRES relation between two KPs.
// POST /api/v1/knowledge-points/:id/prerequisites
func (h *KnowledgeGraphHandler) CreatePrerequisite(c *gin.Context) {
	kpID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的知识点 ID"})
		return
	}

	var req CreatePrereqRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求数据格式错误: " + err.Error()})
		return
	}

	// Verify both KPs exist
	var kp model.KnowledgePoint
	if err := h.DB.First(&kp, kpID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "知识点不存在"})
		return
	}
	var prereqKP model.KnowledgePoint
	if err := h.DB.First(&prereqKP, req.PrereqKPID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "前置知识点不存在"})
		return
	}

	// Prevent self-dependency
	if uint(kpID) == req.PrereqKPID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能创建自身到自身的依赖"})
		return
	}

	// Create in Neo4j
	if h.Neo4j == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Neo4j 服务不可用"})
		return
	}

	if err := h.Neo4j.CreateRequiresRelation(c.Request.Context(), uint(kpID), req.PrereqKPID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建前置依赖失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":   "前置依赖已创建",
		"kp_id":     kpID,
		"prereq_id": req.PrereqKPID,
	})
}

// GetPrerequisites returns all prerequisite KPs for a given KP (up to 3 hops).
// GET /api/v1/knowledge-points/:id/prerequisites
func (h *KnowledgeGraphHandler) GetPrerequisites(c *gin.Context) {
	kpID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的知识点 ID"})
		return
	}

	if h.Neo4j == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Neo4j 服务不可用"})
		return
	}

	prereqs, err := h.Neo4j.GetPrerequisites(c.Request.Context(), uint(kpID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询前置依赖失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"kp_id":         kpID,
		"prerequisites": prereqs,
		"count":         len(prereqs),
	})
}

// ── Helpers ─────────────────────────────────────────────────

func isValidTrapType(t model.TrapType) bool {
	switch t {
	case model.TrapTypeConceptual, model.TrapTypeProcedural, model.TrapTypeIntuit, model.TrapTypeTransfer:
		return true
	}
	return false
}

func isValidLinkType(t string) bool {
	switch t {
	case "analogy", "shared_model", "application":
		return true
	}
	return false
}

// ── Student Knowledge Map ───────────────────────────────────

// KnowledgeMapNode 知识地图节点。
type KnowledgeMapNode struct {
	ID           uint    `json:"id"`
	Neo4jID      string  `json:"neo4j_id"`
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	ChapterID    uint    `json:"chapter_id"`
	ChapterTitle string  `json:"chapter_title"`
	Difficulty   float64 `json:"difficulty"`
	IsKeyPoint   bool    `json:"is_key_point"`
	Mastery      float64 `json:"mastery"` // 0.0 ~ 1.0, -1 = no data
	AttemptCount int     `json:"attempt_count"`
}

// KnowledgeMapEdge 知识地图边。
type KnowledgeMapEdge struct {
	Source string `json:"source"` // neo4j id, e.g. "kp_1"
	Target string `json:"target"` // neo4j id, e.g. "kp_2"
	Type   string `json:"type"`   // "REQUIRES" | "RELATES_TO"
}

// KnowledgeMapResponse 学生个人知识地图响应。
type KnowledgeMapResponse struct {
	CourseID    uint               `json:"course_id"`
	CourseTitle string             `json:"course_title"`
	Nodes       []KnowledgeMapNode `json:"nodes"`
	Edges       []KnowledgeMapEdge `json:"edges"`
	AvgMastery  float64            `json:"avg_mastery"`
	MasteredCnt int                `json:"mastered_count"` // mastery >= 0.8
	WeakCnt     int                `json:"weak_count"`     // mastery < 0.4 (with attempts)
}

// GetStudentKnowledgeMap returns the student's personal knowledge graph with mastery overlay.
// GET /api/v1/student/knowledge-map?course_id=1
func (h *KnowledgeGraphHandler) GetStudentKnowledgeMap(c *gin.Context) {
	studentID := middleware.GetUserID(c)

	courseIDStr := c.Query("course_id")
	if courseIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请指定 course_id 参数"})
		return
	}
	courseID, err := strconv.ParseUint(courseIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 course_id"})
		return
	}

	// 1. Load course with chapters and knowledge points
	var course model.Course
	if err := h.DB.Preload("Chapters", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	}).Preload("Chapters.KnowledgePoints").
		First(&course, courseID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "课程不存在"})
		return
	}

	// 2. Collect all KP IDs and build nodes
	var allKPIDs []uint
	chapterMap := make(map[uint]string) // chapterID -> title
	for _, ch := range course.Chapters {
		chapterMap[ch.ID] = ch.Title
		for _, kp := range ch.KnowledgePoints {
			allKPIDs = append(allKPIDs, kp.ID)
		}
	}

	if len(allKPIDs) == 0 {
		c.JSON(http.StatusOK, KnowledgeMapResponse{
			CourseID:    uint(courseID),
			CourseTitle: course.Title,
			Nodes:       []KnowledgeMapNode{},
			Edges:       []KnowledgeMapEdge{},
		})
		return
	}

	// 3. Load student mastery for all KPs in this course
	var masteries []model.StudentKPMastery
	h.DB.Where("student_id = ? AND kp_id IN ?", studentID, allKPIDs).Find(&masteries)
	masteryMap := make(map[uint]model.StudentKPMastery)
	for _, m := range masteries {
		masteryMap[m.KPID] = m
	}

	// 4. Build graph nodes
	nodes := make([]KnowledgeMapNode, 0, len(allKPIDs))
	var totalMastery float64
	var masteryCount int
	var masteredCnt, weakCnt int

	for _, ch := range course.Chapters {
		for _, kp := range ch.KnowledgePoints {
			node := KnowledgeMapNode{
				ID:           kp.ID,
				Neo4jID:      kp.Neo4jNodeID,
				Title:        kp.Title,
				Description:  kp.Description,
				ChapterID:    ch.ID,
				ChapterTitle: ch.Title,
				Difficulty:   kp.Difficulty,
				IsKeyPoint:   kp.IsKeyPoint,
				Mastery:      -1, // no data
			}
			if m, ok := masteryMap[kp.ID]; ok {
				node.Mastery = m.MasteryScore
				node.AttemptCount = m.AttemptCount
				totalMastery += m.MasteryScore
				masteryCount++
				if m.MasteryScore >= 0.8 {
					masteredCnt++
				}
				if m.MasteryScore < 0.4 && m.AttemptCount > 0 {
					weakCnt++
				}
			}
			// If Neo4jNodeID is empty, generate the expected ID
			if node.Neo4jID == "" {
				node.Neo4jID = fmt.Sprintf("kp_%d", kp.ID)
			}
			nodes = append(nodes, node)
		}
	}

	// 5. Get graph edges from Neo4j
	var edges []KnowledgeMapEdge
	if h.Neo4j != nil {
		graphEdges, err := h.Neo4j.GetCourseGraphEdges(c.Request.Context(), uint(courseID))
		if err == nil {
			// Build a set of valid neo4j IDs for filtering
			validIDs := make(map[string]bool)
			for _, n := range nodes {
				validIDs[n.Neo4jID] = true
			}
			for _, ge := range graphEdges {
				// Only include edges where both endpoints are in our node set
				if validIDs[ge.FromID] && validIDs[ge.ToID] {
					edges = append(edges, KnowledgeMapEdge{
						Source: ge.FromID,
						Target: ge.ToID,
						Type:   ge.Type,
					})
				}
			}
		}
	}

	// 6. Compute average mastery
	avgMastery := 0.0
	if masteryCount > 0 {
		avgMastery = totalMastery / float64(masteryCount)
	}

	if edges == nil {
		edges = []KnowledgeMapEdge{}
	}

	c.JSON(http.StatusOK, KnowledgeMapResponse{
		CourseID:    uint(courseID),
		CourseTitle: course.Title,
		Nodes:       nodes,
		Edges:       edges,
		AvgMastery:  avgMastery,
		MasteredCnt: masteredCnt,
		WeakCnt:     weakCnt,
	})
}

// parseKPNumericID extracts the numeric part from a Neo4j KP ID like "kp_123".
func parseKPNumericID(neo4jID string) (uint, bool) {
	parts := strings.SplitN(neo4jID, "_", 2)
	if len(parts) != 2 {
		return 0, false
	}
	id, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return 0, false
	}
	return uint(id), true
}

// GetCourseKnowledgeGraph returns the course knowledge graph without student mastery data.
// GET /api/v1/courses/:id/graph
func (h *KnowledgeGraphHandler) GetCourseKnowledgeGraph(c *gin.Context) {
	courseID, err := parseCourseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 course_id"})
		return
	}

	// 1. Load course with chapters and knowledge points
	var course model.Course
	if err := h.DB.Preload("Chapters", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	}).Preload("Chapters.KnowledgePoints").
		First(&course, courseID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "课程不存在"})
		return
	}

	// 2. Build nodes
	var allKPIDs []uint
	for _, ch := range course.Chapters {
		for _, kp := range ch.KnowledgePoints {
			allKPIDs = append(allKPIDs, kp.ID)
		}
	}

	if len(allKPIDs) == 0 {
		c.JSON(http.StatusOK, KnowledgeMapResponse{
			CourseID:    courseID,
			CourseTitle: course.Title,
			Nodes:       []KnowledgeMapNode{},
			Edges:       []KnowledgeMapEdge{},
		})
		return
	}

	nodes := make([]KnowledgeMapNode, 0, len(allKPIDs))
	for _, ch := range course.Chapters {
		for _, kp := range ch.KnowledgePoints {
			node := KnowledgeMapNode{
				ID:           kp.ID,
				Neo4jID:      kp.Neo4jNodeID,
				Title:        kp.Title,
				Description:  kp.Description,
				ChapterID:    ch.ID,
				ChapterTitle: ch.Title,
				Difficulty:   kp.Difficulty,
				IsKeyPoint:   kp.IsKeyPoint,
				Mastery:      -1, // no data for teacher view
				AttemptCount: 0,
			}
			if node.Neo4jID == "" {
				node.Neo4jID = fmt.Sprintf("kp_%d", kp.ID)
			}
			nodes = append(nodes, node)
		}
	}

	// 3. Get graph edges from Neo4j
	var edges []KnowledgeMapEdge
	if h.Neo4j != nil {
		graphEdges, err := h.Neo4j.GetCourseGraphEdges(c.Request.Context(), courseID)
		if err == nil {
			validIDs := make(map[string]bool)
			for _, n := range nodes {
				validIDs[n.Neo4jID] = true
			}
			for _, ge := range graphEdges {
				if validIDs[ge.FromID] && validIDs[ge.ToID] {
					edges = append(edges, KnowledgeMapEdge{
						Source: ge.FromID,
						Target: ge.ToID,
						Type:   ge.Type,
					})
				}
			}
		}
	}

	if edges == nil {
		edges = []KnowledgeMapEdge{}
	}

	c.JSON(http.StatusOK, KnowledgeMapResponse{
		CourseID:    courseID,
		CourseTitle: course.Title,
		Nodes:       nodes,
		Edges:       edges,
		AvgMastery:  -1,
	})
}
