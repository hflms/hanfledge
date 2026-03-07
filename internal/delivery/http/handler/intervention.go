package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/agent"
	"github.com/hflms/hanfledge/internal/domain/model"
)

// InterventionRequest represents the payload from the teacher.
type InterventionRequest struct {
	Type    string `json:"type" binding:"required,oneof=takeover whisper"`
	Content string `json:"content" binding:"required"`
}

// HandleIntervention processes a teacher's intervention in an active session.
// POST /api/v1/sessions/:id/intervention
func (h *SessionHandler) HandleIntervention(c *gin.Context) {
	sessionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的会话 ID"})
		return
	}

	var req InterventionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	// Verify session exists and is active
	var session model.StudentSession
	if err := h.DB.First(&session, sessionID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "会话不存在"})
		return
	}
	if session.Status != model.SessionStatusActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该会话已结束"})
		return
	}

	// Get active WebSocket connection
	h.sessionsMu.RLock()
	ws, ok := h.ActiveSessions[uint(sessionID)]
	h.sessionsMu.RUnlock()

	if !ok {
		c.JSON(http.StatusConflict, gin.H{"error": "学生当前不在线，无法实时干预"})
		return
	}

	// Record the intervention
	interaction := model.Interaction{
		SessionID: uint(sessionID),
		Role:      "teacher", // New role type!
		Content:   req.Content,
		CreatedAt: time.Now(),
	}

	// If it's a whisper, we mark it internally so the frontend doesn't show it directly
	// Or we can add a new type field. For now, let's prefix content or add a meta field.
	// We'll update the Interaction model to include TurnType or Meta if needed.
	// Wait, the orchestrator handles this better.

	// Save to DB
	if err := h.DB.Create(&interaction).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法保存干预记录"})
		return
	}

	if req.Type == "takeover" {
		// Send immediately to student
		h.sendEvent(ws, "teacher_takeover", map[string]interface{}{
			"id":         interaction.ID,
			"content":    req.Content,
			"created_at": interaction.CreatedAt,
		})

		// Optional: We might need to interrupt any ongoing LLM generation here.
		// For now, it just sends the message.
	} else if req.Type == "whisper" {
		// Whisper: Send an event to the orchestrator to trigger a new AI response based on the instruction
		// We'll call a new Orchestrator method for this

		// For now just ack
		h.sendEvent(ws, "system_message", map[string]string{
			"content": "老师向AI发送了一条指令...",
		})

		go h.Orchestrator.HandleWhisper(c.Request.Context(), &session, req.Content, func(evt agent.WSEvent) {
			ws.writeJSON(evt)
		})
	}

	c.JSON(http.StatusOK, gin.H{"message": "干预已发送"})
}
