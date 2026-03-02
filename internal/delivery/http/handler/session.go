package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/hflms/hanfledge/internal/agent"
	"github.com/hflms/hanfledge/internal/delivery/http/middleware"
	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/asr"
	"github.com/hflms/hanfledge/internal/infrastructure/logger"
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

var slogSession = logger.L("Session")

// SessionHandler handles WebSocket session streaming.
type SessionHandler struct {
	ActiveSessions map[uint]*wsConn
	sessionsMu     sync.RWMutex

	DB             *gorm.DB
	Orchestrator   *agent.AgentOrchestrator
	InjectionGuard *safety.InjectionGuard
	Achievement    *AchievementHandler
	ASR            asr.ASRProvider // ASR 语音识别 (nil-safe)
	upgrader       websocket.Upgrader
}

// NewSessionHandler creates a new SessionHandler.
// corsOrigins is a comma-separated list of allowed origins (e.g. "http://localhost:3000"),
// or "*" to allow all origins. ginMode controls dev vs production behavior.
func NewSessionHandler(db *gorm.DB, orchestrator *agent.AgentOrchestrator, injectionGuard *safety.InjectionGuard, asrProvider asr.ASRProvider, corsOrigins string, ginMode string) *SessionHandler {
	return &SessionHandler{
		DB:             db,
		Orchestrator:   orchestrator,
		InjectionGuard: injectionGuard,
		Achievement:    NewAchievementHandler(db),
		ASR:            asrProvider,
		upgrader:       newUpgrader(corsOrigins, ginMode),
		ActiveSessions: make(map[uint]*wsConn),
	}
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

// WebSocket upgrader factory
// newUpgrader creates a websocket.Upgrader with origin checking based on config.
// In debug mode or with corsOrigins="*", all origins are accepted.
// In release mode, only explicitly listed origins are allowed.
func newUpgrader(corsOrigins string, ginMode string) websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			// Development mode: allow all origins for convenience
			if ginMode == "debug" || ginMode == "test" {
				return true
			}
			// Wildcard: allow all (not recommended for production)
			if corsOrigins == "*" {
				return true
			}
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true // same-origin requests may not send Origin header
			}
			return isAllowedOrigin(origin, corsOrigins)
		},
	}
}

// isAllowedOrigin checks whether the given origin is in the comma-separated
// list of allowed origins.
func isAllowedOrigin(origin string, allowedCSV string) bool {
	for _, allowed := range strings.Split(allowedCSV, ",") {
		if strings.TrimSpace(allowed) == origin {
			return true
		}
	}
	return false
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
//
//	@Summary      会话 WebSocket 流
//	@Description  升级为 WebSocket 连接，实现双向 AI 对话流。客户端发送 user_message/voice_start/voice_data/voice_end 事件，服务端返回 agent_thinking/token_delta/ui_scaffold_change/turn_complete 事件
//	@Tags         Sessions
//	@Security     BearerAuth
//	@Param        id  path  int  true  "会话 ID"
//	@Success      101 "WebSocket 升级成功"
//	@Failure      400 {object}  ErrorResponse
//	@Failure      403 {object}  ErrorResponse
//	@Failure      404 {object}  ErrorResponse
//	@Router       /sessions/{id}/stream [get]
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
	rawWS, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slogSession.Warn("websocket upgrade failed", "err", err)
		return
	}
	defer rawWS.Close()

	ws := &wsConn{conn: rawWS}

	// Register active session
	h.sessionsMu.Lock()
	h.ActiveSessions[uint(sessionID)] = ws
	h.sessionsMu.Unlock()
	defer func() {
		h.sessionsMu.Lock()
		delete(h.ActiveSessions, uint(sessionID))
		h.sessionsMu.Unlock()
	}()

	slogSession.Info("websocket connected", "session", sessionID, "student", studentID)

	// ── Welcome Message (first connection only) ──────────────
	// Check if session has any prior interactions; if not, send a welcome message
	var interactionCount int64
	h.DB.Model(&model.Interaction{}).Where("session_id = ?", sessionID).Count(&interactionCount)
	if interactionCount == 0 {
		// Load activity and knowledge point context for the greeting
		var activity model.LearningActivity
		var kpTitle string
		if err := h.DB.First(&activity, session.ActivityID).Error; err == nil {
			var kp model.KnowledgePoint
			if err := h.DB.First(&kp, session.CurrentKP).Error; err == nil {
				kpTitle = kp.Title
			}
		}

		// Build welcome content
		welcome := fmt.Sprintf("👋 欢迎开始学习活动「%s」！", activity.Title)
		if kpTitle != "" {
			welcome += fmt.Sprintf("\n\n📚 当前知识点：**%s**", kpTitle)
		}
		welcome += "\n\n请在下方输入框中回答或提问，AI 导师会引导你逐步思考。准备好了就开始吧！"

		h.sendEvent(ws, "system_message", map[string]string{
			"content": welcome,
		})
	}

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
					slogSession.Warn("websocket ping failed", "session", sessionID, "err", err)
					return
				}
			case <-done:
				return
			}
		}
	}()

	// Main read loop
	var voiceBuffer []byte // Accumulates audio chunks during voice recording
	var voiceLang string   // Language from voice_start

	for {
		_, msgBytes, err := rawWS.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				slogSession.Info("client disconnected", "session", sessionID)
			} else {
				slogSession.Warn("websocket read error", "err", err)
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

		case agent.EventVoiceStart:
			// Voice recording started — reset buffer and parse config
			voiceBuffer = voiceBuffer[:0]
			payloadBytes, _ := json.Marshal(event.Payload)
			var startPayload agent.VoiceStartPayload
			if err := json.Unmarshal(payloadBytes, &startPayload); err == nil {
				voiceLang = startPayload.Language
			}
			if voiceLang == "" {
				voiceLang = "zh-CN"
			}
			slogSession.Info("voice start", "session", sessionID, "lang", voiceLang)

		case agent.EventVoiceData:
			// Voice data chunk — decode base64 and append to buffer
			payloadBytes, _ := json.Marshal(event.Payload)
			var dataPayload agent.VoiceDataPayload
			if err := json.Unmarshal(payloadBytes, &dataPayload); err != nil {
				continue
			}
			decoded, err := base64.StdEncoding.DecodeString(dataPayload.Data)
			if err != nil {
				slogSession.Warn("voice data decode failed", "err", err)
				continue
			}
			voiceBuffer = append(voiceBuffer, decoded...)

		case agent.EventVoiceEnd:
			// Voice recording ended — transcribe collected audio
			slogSession.Info("voice end", "session", sessionID, "buffer_bytes", len(voiceBuffer))
			if h.ASR == nil {
				h.sendWSError(ws, "语音识别服务未配置")
				continue
			}
			if len(voiceBuffer) == 0 {
				h.sendWSError(ws, "未收到语音数据")
				continue
			}

			// Run ASR in background to avoid blocking the read loop
			audioCopy := make([]byte, len(voiceBuffer))
			copy(audioCopy, voiceBuffer)
			voiceBuffer = voiceBuffer[:0]
			lang := voiceLang

			go func() {
				result, err := h.ASR.Transcribe(context.Background(), audioCopy, asr.TranscribeConfig{
					Language:          lang,
					SampleRate:        16000,
					Format:            "webm",
					EnablePunctuation: true,
				})
				if err != nil {
					slogSession.Warn("transcription failed", "session", sessionID, "err", err)
					h.sendWSError(ws, "语音识别失败，请重试")
					return
				}
				h.sendEvent(ws, agent.EventVoiceResult, agent.VoiceResultPayload{
					Text:       result.Text,
					Confidence: result.Confidence,
					IsFinal:    true,
				})
				slogSession.Info("transcribed", "session", sessionID, "text", truncateStr(result.Text, 50))
			}()

		case "ping":
			// Application-level heartbeat from frontend — silently ignore
			// (WebSocket-level pings are handled separately by the gorilla library)

		default:
			h.sendWSError(ws, "未知事件类型: "+event.Event)
		}
	}

	slogSession.Info("session ended", "session", sessionID)
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
			slogSession.Warn("injection blocked", "session", session.ID, "reason", check.Reason, "matched", check.Matched)
			h.sendWSError(ws, "您的输入包含不允许的内容，请修改后重试")
			return
		}
		if check.Risk == safety.RiskWarning {
			slogSession.Warn("injection warning", "session", session.ID, "reason", check.Reason)
		}
	}

	slogSession.Info("user message", "session", session.ID, "text", safety.RedactForLog(payload.Text, 50))

	// Build TurnContext with WebSocket callbacks
	agentCtx, agentCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer agentCancel()

	tc := &agent.TurnContext{
		Ctx:              agentCtx,
		SessionID:        session.ID,
		StudentID:        studentID,
		ActivityID:       session.ActivityID,
		UserInput:        payload.Text,
		Scaffold:         session.Scaffold,
		IsSandbox:        session.IsSandbox,
		ProviderOverride: payload.ProviderOverride,
		ModelOverride:    payload.ModelOverride,

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

			// ── Achievement Evaluation (design.md §5.2 Step 4) ──
			// Skip for sandbox sessions — teacher previews should not earn achievements
			if h.Achievement != nil && !session.IsSandbox {
				h.evaluateAchievementsOnScaffold(action, data, studentID)
			}
		},

		OnTurnComplete: func(totalTokens int) {
			h.sendEvent(ws, agent.EventTurnComplete, agent.TurnCompletePayload{
				TotalTokens: totalTokens,
			})

			// ── Deep Inquiry Achievement (design.md §5.2 Step 4) ──
			// Skip for sandbox sessions — teacher previews should not earn achievements
			if h.Achievement != nil && !session.IsSandbox {
				go h.Achievement.EvaluateDeepInquiry(studentID, session.ID)
			}
		},
	}

	// Execute the Agent pipeline
	if err := h.Orchestrator.HandleTurn(tc); err != nil {
		slogSession.Error("agent pipeline failed", "err", err)
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
		slogSession.Error("websocket write failed", "err", err)
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

// -- Achievement Evaluation Helpers ----------------------------------------

// evaluateAchievementsOnScaffold triggers achievement checks based on scaffold events.
// - "scaffold_change" with mastery >= 0.8 → streak breaker
// - "fallacy_identified" → fallacy hunter
func (h *SessionHandler) evaluateAchievementsOnScaffold(action string, data interface{}, studentID uint) {
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return
	}

	switch action {
	case "scaffold_change":
		// Check if mastery crossed the 0.8 threshold
		if mastery, ok := dataMap["mastery"].(float64); ok && mastery >= 0.8 {
			go h.Achievement.EvaluateStreakBreaker(studentID)
		}

	case "fallacy_identified":
		// Extract cumulative identified count
		if count, ok := dataMap["identified_count"]; ok {
			var c int
			switch v := count.(type) {
			case float64:
				c = int(v)
			case int:
				c = v
			}
			if c > 0 {
				go h.Achievement.EvaluateFallacyHunter(studentID, c)
			}
		}
	}
}
