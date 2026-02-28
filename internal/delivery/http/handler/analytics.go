package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/delivery/http/middleware"
	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/safety"
	"gorm.io/gorm"
)

// ============================
// Analytics Dashboard V2 Handler — Phase G
// ============================
//
// 新增端点:
//   1. GET /api/v1/sessions/:id/inquiry-tree  — 追问深度树
//   2. GET /api/v1/sessions/:id/interactions  — AI 交互日志回放
//   3. GET /api/v1/dashboard/skill-effectiveness — 技能效果评估报告

// AnalyticsHandler handles Phase G analytics dashboard APIs.
type AnalyticsHandler struct {
	DB       *gorm.DB
	Redactor *safety.PIIRedactor
}

// NewAnalyticsHandler creates a new AnalyticsHandler.
func NewAnalyticsHandler(db *gorm.DB, redactor *safety.PIIRedactor) *AnalyticsHandler {
	return &AnalyticsHandler{DB: db, Redactor: redactor}
}

// -- Inquiry Depth Tree (追问深度树) ----------------------------

// InquiryTreeNode represents a node in the inquiry depth tree.
type InquiryTreeNode struct {
	ID       uint               `json:"id"`
	Role     string             `json:"role"`    // "student" | "coach" | "system"
	Content  string             `json:"content"` // PII-masked content
	SkillID  string             `json:"skill_id,omitempty"`
	Depth    int                `json:"depth"`     // Depth level in the tree (0 = root)
	TurnType string             `json:"turn_type"` // "question" | "probe" | "correction" | "response" | "scaffold_change"
	Time     string             `json:"time"`
	Children []*InquiryTreeNode `json:"children,omitempty"`
}

// InquiryTreeResponse wraps the full inquiry tree for a session.
type InquiryTreeResponse struct {
	SessionID   uint               `json:"session_id"`
	StudentName string             `json:"student_name"`
	TotalTurns  int                `json:"total_turns"`
	MaxDepth    int                `json:"max_depth"`
	SkillUsed   string             `json:"skill_used"`
	Roots       []*InquiryTreeNode `json:"roots"`
}

// GetInquiryTree returns the inquiry depth tree for a session.
// GET /api/v1/sessions/:id/inquiry-tree
func (h *AnalyticsHandler) GetInquiryTree(c *gin.Context) {
	sessionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的会话 ID"})
		return
	}

	// Verify session exists and teacher has access
	var session model.StudentSession
	if err := h.DB.First(&session, sessionID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "会话不存在"})
		return
	}

	// Verify the teacher owns the activity
	if err := h.verifyTeacherAccess(c, session.ActivityID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权查看该会话数据"})
		return
	}

	// Load all interactions ordered by time
	var interactions []model.Interaction
	h.DB.Where("session_id = ?", sessionID).
		Order("created_at ASC").
		Find(&interactions)

	// Get student name (masked)
	var student model.User
	h.DB.Select("id, display_name").First(&student, session.StudentID)
	studentName := h.maskName(student.DisplayName)

	// Build the inquiry depth tree
	roots, maxDepth := buildInquiryTree(interactions, h.Redactor)

	c.JSON(http.StatusOK, InquiryTreeResponse{
		SessionID:   uint(sessionID),
		StudentName: studentName,
		TotalTurns:  len(interactions),
		MaxDepth:    maxDepth,
		SkillUsed:   session.ActiveSkill,
		Roots:       roots,
	})
}

// buildInquiryTree converts a flat list of interactions into a tree structure.
// The tree groups interactions into "inquiry rounds":
//   - Each student message starts a new subtree at the current depth
//   - Sequential coach messages are children (probes/responses)
//   - When a student replies to a coach probe, depth increases
//   - When topic changes significantly (detected by skill switch), a new root starts
func buildInquiryTree(interactions []model.Interaction, redactor *safety.PIIRedactor) ([]*InquiryTreeNode, int) {
	if len(interactions) == 0 {
		return nil, 0
	}

	var roots []*InquiryTreeNode
	maxDepth := 0
	depth := 0

	var currentRoot *InquiryTreeNode
	var currentParent *InquiryTreeNode
	var lastRole string // last non-system role for conversation flow
	var lastSkill string

	for _, inter := range interactions {
		// Mask PII in content
		content := inter.Content
		if redactor != nil {
			content, _ = redactor.Redact(content)
		}

		turnType := classifyTurnType(inter.Role, lastRole, depth)

		node := &InquiryTreeNode{
			ID:       inter.ID,
			Role:     inter.Role,
			Content:  content,
			SkillID:  inter.SkillID,
			Depth:    depth,
			TurnType: turnType,
			Time:     inter.CreatedAt.Format(time.RFC3339),
		}

		// Detect skill switch → new root
		if inter.SkillID != "" && lastSkill != "" && inter.SkillID != lastSkill {
			currentRoot = nil
			currentParent = nil
			depth = 0
			node.Depth = 0
		}

		if currentRoot == nil {
			// Start a new root tree
			currentRoot = node
			currentParent = node
			roots = append(roots, currentRoot)
		} else if inter.Role == "student" && lastRole == "coach" {
			// Student replying to coach → deepen the inquiry
			depth++
			node.Depth = depth
			if depth > maxDepth {
				maxDepth = depth
			}
			currentParent.Children = append(currentParent.Children, node)
			currentParent = node
		} else if inter.Role == "coach" && lastRole == "student" {
			// Coach responding to student → child of current parent
			node.Depth = depth
			currentParent.Children = append(currentParent.Children, node)
		} else if inter.Role == "coach" && lastRole == "coach" {
			// Sequential coach messages → same depth, sibling
			node.Depth = depth
			currentParent.Children = append(currentParent.Children, node)
		} else if inter.Role == "student" && lastRole == "student" {
			// Student follows up before coach responds → same depth
			node.Depth = depth
			if currentParent != currentRoot {
				currentParent.Children = append(currentParent.Children, node)
			} else {
				// New root for a new question thread
				currentRoot = node
				currentParent = node
				roots = append(roots, currentRoot)
				depth = 0
				node.Depth = 0
			}
		} else if inter.Role == "system" {
			// System messages are metadata nodes at the current depth
			node.Depth = depth
			node.TurnType = "scaffold_change"
			if currentParent != nil {
				currentParent.Children = append(currentParent.Children, node)
			} else {
				roots = append(roots, node)
			}
		}

		// Update lastRole only for non-system messages to preserve conversation flow
		if inter.Role != "system" {
			lastRole = inter.Role
		}
		if inter.SkillID != "" {
			lastSkill = inter.SkillID
		}
	}

	return roots, maxDepth
}

// classifyTurnType determines the turn type based on role and context.
func classifyTurnType(role, lastRole string, depth int) string {
	switch role {
	case "student":
		if depth > 0 && lastRole == "coach" {
			return "correction" // Student is correcting after AI probe
		}
		return "question"
	case "coach":
		if lastRole == "student" && depth > 0 {
			return "probe" // AI is probing deeper
		}
		return "response"
	case "system":
		return "scaffold_change"
	default:
		return "response"
	}
}

// -- AI Interaction Log Replay (AI 交互日志回放) -----------------

// InteractionLogEntry represents a single interaction in the replay log.
type InteractionLogEntry struct {
	ID                   uint     `json:"id"`
	Role                 string   `json:"role"`
	Content              string   `json:"content"`
	SkillID              string   `json:"skill_id,omitempty"`
	TokensUsed           int      `json:"tokens_used"`
	CreatedAt            string   `json:"created_at"`
	FaithfulnessScore    *float64 `json:"faithfulness_score,omitempty"`
	ActionabilityScore   *float64 `json:"actionability_score,omitempty"`
	AnswerRestraintScore *float64 `json:"answer_restraint_score,omitempty"`
	EvalStatus           string   `json:"eval_status"`
}

// InteractionLogResponse wraps the full interaction log for replay.
type InteractionLogResponse struct {
	SessionID     uint                  `json:"session_id"`
	StudentName   string                `json:"student_name"`
	ActiveSkill   string                `json:"active_skill"`
	ScaffoldLevel string                `json:"scaffold_level"`
	Status        string                `json:"status"`
	StartedAt     string                `json:"started_at"`
	EndedAt       *string               `json:"ended_at,omitempty"`
	Interactions  []InteractionLogEntry `json:"interactions"`
}

// GetInteractionLog returns the PII-anonymized interaction log for a session.
// GET /api/v1/sessions/:id/interactions
func (h *AnalyticsHandler) GetInteractionLog(c *gin.Context) {
	sessionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的会话 ID"})
		return
	}

	// Verify session exists
	var session model.StudentSession
	if err := h.DB.First(&session, sessionID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "会话不存在"})
		return
	}

	// Verify teacher access
	if err := h.verifyTeacherAccess(c, session.ActivityID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权查看该会话数据"})
		return
	}

	// Load all interactions
	var interactions []model.Interaction
	h.DB.Where("session_id = ?", sessionID).
		Order("created_at ASC").
		Find(&interactions)

	// Get student name (masked)
	var student model.User
	h.DB.Select("id, display_name").First(&student, session.StudentID)
	studentName := h.maskName(student.DisplayName)

	// Build log entries with PII masking
	entries := make([]InteractionLogEntry, len(interactions))
	for i, inter := range interactions {
		content := inter.Content
		if h.Redactor != nil {
			content, _ = h.Redactor.Redact(content)
		}

		entries[i] = InteractionLogEntry{
			ID:                   inter.ID,
			Role:                 inter.Role,
			Content:              content,
			SkillID:              inter.SkillID,
			TokensUsed:           inter.TokensUsed,
			CreatedAt:            inter.CreatedAt.Format(time.RFC3339),
			FaithfulnessScore:    inter.FaithfulnessScore,
			ActionabilityScore:   inter.ActionabilityScore,
			AnswerRestraintScore: inter.AnswerRestraintScore,
			EvalStatus:           inter.EvalStatus,
		}
	}

	resp := InteractionLogResponse{
		SessionID:     uint(sessionID),
		StudentName:   studentName,
		ActiveSkill:   session.ActiveSkill,
		ScaffoldLevel: string(session.Scaffold),
		Status:        string(session.Status),
		StartedAt:     session.StartedAt.Format(time.RFC3339),
		Interactions:  entries,
	}
	if session.EndedAt != nil {
		t := session.EndedAt.Format(time.RFC3339)
		resp.EndedAt = &t
	}

	c.JSON(http.StatusOK, resp)
}

// -- Skill Effectiveness Report (技能效果评估报告) ----------------

// SkillEffectivenessItem represents metrics for a single skill.
type SkillEffectivenessItem struct {
	SkillID             string  `json:"skill_id"`
	SessionCount        int     `json:"session_count"`
	InteractionCount    int     `json:"interaction_count"`
	EvaluatedCount      int     `json:"evaluated_count"`
	AvgFaithfulness     float64 `json:"avg_faithfulness"`
	AvgActionability    float64 `json:"avg_actionability"`
	AvgAnswerRestraint  float64 `json:"avg_answer_restraint"`
	AvgContextPrecision float64 `json:"avg_context_precision"`
	AvgContextRecall    float64 `json:"avg_context_recall"`
	AvgMasteryDelta     float64 `json:"avg_mastery_delta"` // Average mastery improvement
}

// SkillEffectivenessResponse wraps skill effectiveness metrics.
type SkillEffectivenessResponse struct {
	CourseID    uint                     `json:"course_id"`
	CourseTitle string                   `json:"course_title"`
	Items       []SkillEffectivenessItem `json:"items"`
}

// GetSkillEffectiveness returns aggregated skill effectiveness metrics.
// GET /api/v1/dashboard/skill-effectiveness?course_id=1
func (h *AnalyticsHandler) GetSkillEffectiveness(c *gin.Context) {
	courseIDStr := c.Query("course_id")
	if courseIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "course_id 参数必填"})
		return
	}
	courseID, err := strconv.ParseUint(courseIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 course_id"})
		return
	}

	// Verify teacher owns this course
	teacherID := middleware.GetUserID(c)
	var course model.Course
	if err := h.DB.First(&course, courseID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "课程不存在"})
		return
	}
	if course.TeacherID != teacherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权查看该课程数据"})
		return
	}

	// Get all activity IDs for this course
	var activityIDs []uint
	h.DB.Model(&model.LearningActivity{}).
		Where("course_id = ?", courseID).
		Pluck("id", &activityIDs)

	if len(activityIDs) == 0 {
		c.JSON(http.StatusOK, SkillEffectivenessResponse{
			CourseID:    uint(courseID),
			CourseTitle: course.Title,
			Items:       []SkillEffectivenessItem{},
		})
		return
	}

	// Get all session IDs for these activities (exclude sandbox sessions)
	var sessionIDs []uint
	h.DB.Model(&model.StudentSession{}).
		Where("activity_id IN ? AND is_sandbox = ?", activityIDs, false).
		Pluck("id", &sessionIDs)

	if len(sessionIDs) == 0 {
		c.JSON(http.StatusOK, SkillEffectivenessResponse{
			CourseID:    uint(courseID),
			CourseTitle: course.Title,
			Items:       []SkillEffectivenessItem{},
		})
		return
	}

	// Aggregate interactions by skill_id
	type skillAgg struct {
		SkillID             string
		InteractionCount    int64
		EvaluatedCount      int64
		AvgFaithfulness     float64
		AvgActionability    float64
		AvgAnswerRestraint  float64
		AvgContextPrecision float64
		AvgContextRecall    float64
	}
	var aggs []skillAgg

	h.DB.Model(&model.Interaction{}).
		Select(`skill_id,
			COUNT(*) as interaction_count,
			COUNT(CASE WHEN eval_status = 'evaluated' THEN 1 END) as evaluated_count,
			COALESCE(AVG(faithfulness_score), 0) as avg_faithfulness,
			COALESCE(AVG(actionability_score), 0) as avg_actionability,
			COALESCE(AVG(answer_restraint_score), 0) as avg_answer_restraint,
			COALESCE(AVG(context_precision), 0) as avg_context_precision,
			COALESCE(AVG(context_recall), 0) as avg_context_recall`).
		Where("session_id IN ? AND skill_id != '' AND role = 'coach'", sessionIDs).
		Group("skill_id").
		Scan(&aggs)

	// Count sessions per skill
	type sessionSkillCount struct {
		ActiveSkill  string
		SessionCount int64
	}
	var sessionCounts []sessionSkillCount
	h.DB.Model(&model.StudentSession{}).
		Select("active_skill, COUNT(*) as session_count").
		Where("activity_id IN ? AND active_skill != '' AND is_sandbox = ?", activityIDs, false).
		Group("active_skill").
		Scan(&sessionCounts)

	// Build session count map
	sessionCountMap := make(map[string]int)
	for _, sc := range sessionCounts {
		sessionCountMap[sc.ActiveSkill] = int(sc.SessionCount)
	}

	// Build response items
	items := make([]SkillEffectivenessItem, 0, len(aggs))
	for _, agg := range aggs {
		items = append(items, SkillEffectivenessItem{
			SkillID:             agg.SkillID,
			SessionCount:        sessionCountMap[agg.SkillID],
			InteractionCount:    int(agg.InteractionCount),
			EvaluatedCount:      int(agg.EvaluatedCount),
			AvgFaithfulness:     agg.AvgFaithfulness,
			AvgActionability:    agg.AvgActionability,
			AvgAnswerRestraint:  agg.AvgAnswerRestraint,
			AvgContextPrecision: agg.AvgContextPrecision,
			AvgContextRecall:    agg.AvgContextRecall,
		})
	}

	c.JSON(http.StatusOK, SkillEffectivenessResponse{
		CourseID:    uint(courseID),
		CourseTitle: course.Title,
		Items:       items,
	})
}

// -- Helpers --------------------------------------------------

// verifyTeacherAccess checks that the current user is the teacher who owns the activity.
func (h *AnalyticsHandler) verifyTeacherAccess(c *gin.Context, activityID uint) error {
	teacherID := middleware.GetUserID(c)
	var activity model.LearningActivity
	if err := h.DB.First(&activity, activityID).Error; err != nil {
		return err
	}
	if activity.TeacherID != teacherID {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// maskName returns a partially masked name for privacy.
// e.g., "张三" → "张*", "李明明" → "李**", "John" → "J***"
func (h *AnalyticsHandler) maskName(name string) string {
	runes := []rune(name)
	if len(runes) <= 1 {
		return name
	}
	masked := string(runes[0]) + strings.Repeat("*", len(runes)-1)
	return masked
}
