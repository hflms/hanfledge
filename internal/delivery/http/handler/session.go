package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/hflms/hanfledge/internal/agent"
	"github.com/hflms/hanfledge/internal/delivery/http/middleware"
	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/safety"
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
	DB             *gorm.DB
	Orchestrator   *agent.AgentOrchestrator
	InjectionGuard *safety.InjectionGuard
}

// NewSessionHandler creates a new SessionHandler.
func NewSessionHandler(db *gorm.DB, orchestrator *agent.AgentOrchestrator, injectionGuard *safety.InjectionGuard) *SessionHandler {
	return &SessionHandler{DB: db, Orchestrator: orchestrator, InjectionGuard: injectionGuard}
}

// -- WebSocket Constants ----------------------------------------

const (
	// wsPongWait is the maximum time to wait for a pong from the client.
	wsPongWait = 60 * time.Second

	// wsPingInterval is the interval between heartbeat pings (must be < wsPongWait).
	wsPingInterval = 30 * time.Second

	// wsWriteWait is the maximum time to wait for a write to complete.
	wsWriteWait = 10 * time.Second
)

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins for development; tighten in production
	CheckOrigin: func(r *http.Request) bool { return true },
}

// wsConn wraps a WebSocket connection with a write mutex to prevent
// concurrent writes from the read-loop goroutine and the ping ticker.
type wsConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

// writeJSON marshals and writes a message with a write deadline.
func (w *wsConn) writeJSON(v interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
	return w.conn.WriteJSON(v)
}

// writePing sends a WebSocket ping control frame with a write deadline.
func (w *wsConn) writePing() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
	return w.conn.WriteMessage(websocket.PingMessage, nil)
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
	rawWS, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("⚠️  [WebSocket] Upgrade failed: %v", err)
		return
	}
	defer rawWS.Close()

	ws := &wsConn{conn: rawWS}

	log.Printf("🔌 [WebSocket] Connected: session=%d student=%d", sessionID, studentID)

	// ── Heartbeat: pong detection ────────────────────────────
	rawWS.SetReadDeadline(time.Now().Add(wsPongWait))
	rawWS.SetPongHandler(func(string) error {
		rawWS.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})

	// ── Heartbeat: ping ticker ───────────────────────────────
	ticker := time.NewTicker(wsPingInterval)
	defer ticker.Stop()

	// Per-session turn lock — prevents concurrent HandleTurn calls
	var turnMu sync.Mutex

	// Ping goroutine: sends periodic pings until connection closes
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case <-ticker.C:
				if err := ws.writePing(); err != nil {
					log.Printf("⚠️  [WebSocket] Ping failed: session=%d err=%v", sessionID, err)
					return
				}
			case <-done:
				return
			}
		}
	}()

	// Main read loop
	for {
		_, msgBytes, err := rawWS.ReadMessage()
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
			h.sendWSError(ws, "消息格式错误")
			continue
		}

		switch event.Event {
		case agent.EventUserMessage:
			// Acquire turn lock to prevent concurrent pipeline execution
			if !turnMu.TryLock() {
				h.sendWSError(ws, "请等待当前回答完成后再发送新消息")
				continue
			}
			h.handleUserMessage(ws, &session, studentID, event)
			turnMu.Unlock()
		default:
			h.sendWSError(ws, "未知事件类型: "+event.Event)
		}
	}

	log.Printf("🔌 [WebSocket] Session ended: session=%d", sessionID)
}

// handleUserMessage processes a user_message event through the Agent pipeline.
func (h *SessionHandler) handleUserMessage(ws *wsConn, session *model.StudentSession, studentID uint, event agent.WSEvent) {
	// Extract text from payload
	payloadBytes, _ := json.Marshal(event.Payload)
	var payload agent.UserMessagePayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil || payload.Text == "" {
		h.sendWSError(ws, "消息内容不能为空")
		return
	}

	// ── Prompt Injection 检测 ─────────────────────────────
	if h.InjectionGuard != nil {
		check := h.InjectionGuard.Check(payload.Text)
		if check.Risk == safety.RiskBlocked {
			log.Printf("🛡️  [Safety] Injection BLOCKED: session=%d reason=%s matched=%s",
				session.ID, check.Reason, check.Matched)
			h.sendWSError(ws, "您的输入包含不允许的内容，请修改后重试")
			return
		}
		if check.Risk == safety.RiskWarning {
			log.Printf("🛡️  [Safety] Injection WARNING: session=%d reason=%s",
				session.ID, check.Reason)
		}
	}

	log.Printf("💬 [WebSocket] User message: session=%d text=%s",
		session.ID, safety.RedactForLog(payload.Text, 50))

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
		h.sendWSError(ws, "AI 处理失败，请重试")
	}
}

// ── WebSocket Helpers ───────────────────────────────────────

// sendEvent sends a typed WSEvent over the WebSocket connection.
func (h *SessionHandler) sendEvent(ws *wsConn, eventType string, payload interface{}) {
	event := agent.WSEvent{
		Event:     eventType,
		Payload:   payload,
		Timestamp: time.Now().Unix(),
	}

	if err := ws.writeJSON(event); err != nil {
		log.Printf("⚠️  [WebSocket] Write failed: %v", err)
	}
}

// sendWSError sends an error message over the WebSocket connection.
func (h *SessionHandler) sendWSError(ws *wsConn, message string) {
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
