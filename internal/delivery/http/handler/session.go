package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/hflms/hanfledge/internal/agent"
	"github.com/hflms/hanfledge/internal/delivery/http/middleware"
	"github.com/hflms/hanfledge/internal/domain/model"
	"gorm.io/gorm"
)

// ============================
// Session WebSocket Handler — Phase 4
// ============================
//
// WebSocket 协议:
//   Endpoint: ws://host/api/v1/sessions/:id/stream
//
//   Client → Server: { "event": "user_message", "payload": { "text": "..." }, "timestamp": ... }
//   Server → Client: { "event": "agent_thinking" | "token_delta" | "ui_scaffold_change" | "turn_complete", ... }

// SessionHandler handles WebSocket session streaming.
type SessionHandler struct {
	DB           *gorm.DB
	Orchestrator *agent.AgentOrchestrator
}

// NewSessionHandler creates a new SessionHandler.
func NewSessionHandler(db *gorm.DB, orchestrator *agent.AgentOrchestrator) *SessionHandler {
	return &SessionHandler{DB: db, Orchestrator: orchestrator}
}

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins for development; tighten in production
	CheckOrigin: func(r *http.Request) bool { return true },
}

// StreamSession upgrades the HTTP connection to WebSocket and handles
// bidirectional AI conversation streaming.
// GET /api/v1/sessions/:id/stream (WebSocket upgrade)
func (h *SessionHandler) StreamSession(c *gin.Context) {
	sessionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的会话 ID"})
		return
	}

	studentID := middleware.GetUserID(c)

	// Verify session ownership
	var session model.StudentSession
	if err := h.DB.First(&session, sessionID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "会话不存在"})
		return
	}
	if session.StudentID != studentID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问此会话"})
		return
	}
	if session.Status != model.SessionStatusActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该会话已结束"})
		return
	}

	// Upgrade to WebSocket
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("⚠️  [WebSocket] Upgrade failed: %v", err)
		return
	}
	defer ws.Close()

	log.Printf("🔌 [WebSocket] Connected: session=%d student=%d", sessionID, studentID)

	// Main read loop
	for {
		_, msgBytes, err := ws.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Printf("🔌 [WebSocket] Client disconnected: session=%d", sessionID)
			} else {
				log.Printf("⚠️  [WebSocket] Read error: %v", err)
			}
			break
		}

		// Parse incoming event
		var event agent.WSEvent
		if err := json.Unmarshal(msgBytes, &event); err != nil {
			h.sendError(ws, "消息格式错误")
			continue
		}

		switch event.Event {
		case agent.EventUserMessage:
			h.handleUserMessage(ws, &session, studentID, event)
		default:
			h.sendError(ws, "未知事件类型: "+event.Event)
		}
	}

	log.Printf("🔌 [WebSocket] Session ended: session=%d", sessionID)
}

// handleUserMessage processes a user_message event through the Agent pipeline.
func (h *SessionHandler) handleUserMessage(ws *websocket.Conn, session *model.StudentSession, studentID uint, event agent.WSEvent) {
	// Extract text from payload
	payloadBytes, _ := json.Marshal(event.Payload)
	var payload agent.UserMessagePayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil || payload.Text == "" {
		h.sendError(ws, "消息内容不能为空")
		return
	}

	log.Printf("💬 [WebSocket] User message: session=%d text=%q",
		session.ID, truncateStr(payload.Text, 50))

	// Build TurnContext with WebSocket callbacks
	tc := &agent.TurnContext{
		Ctx:        context.Background(),
		SessionID:  session.ID,
		StudentID:  studentID,
		ActivityID: session.ActivityID,
		UserInput:  payload.Text,
		Scaffold:   session.Scaffold,

		OnThinking: func(status string) {
			h.sendEvent(ws, agent.EventAgentThinking, agent.ThinkingPayload{
				Status: status,
			})
		},

		OnTokenDelta: func(text string) {
			h.sendEvent(ws, agent.EventTokenDelta, agent.TokenDeltaPayload{
				Text: text,
			})
		},

		OnScaffold: func(action string, data interface{}) {
			h.sendEvent(ws, agent.EventUIScaffoldChange, agent.ScaffoldChangePayload{
				Action: action,
				Data:   data,
			})
		},

		OnTurnComplete: func(totalTokens int) {
			h.sendEvent(ws, agent.EventTurnComplete, agent.TurnCompletePayload{
				TotalTokens: totalTokens,
			})
		},
	}

	// Execute the Agent pipeline
	if err := h.Orchestrator.HandleTurn(tc); err != nil {
		log.Printf("⚠️  [WebSocket] Agent pipeline error: %v", err)
		h.sendError(ws, "AI 处理失败，请重试")
	}
}

// ── WebSocket Helpers ───────────────────────────────────────

// sendEvent sends a typed WSEvent over the WebSocket connection.
func (h *SessionHandler) sendEvent(ws *websocket.Conn, eventType string, payload interface{}) {
	event := agent.WSEvent{
		Event:     eventType,
		Payload:   payload,
		Timestamp: time.Now().Unix(),
	}

	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("⚠️  [WebSocket] Marshal event failed: %v", err)
		return
	}

	if err := ws.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Printf("⚠️  [WebSocket] Write failed: %v", err)
	}
}

// sendError sends an error message over the WebSocket connection.
func (h *SessionHandler) sendError(ws *websocket.Conn, message string) {
	h.sendEvent(ws, "error", map[string]string{"message": message})
}

// truncateStr truncates a string to maxLen runes.
func truncateStr(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
