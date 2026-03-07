package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/delivery/http/middleware"
	"github.com/hflms/hanfledge/internal/domain/model"
	"gorm.io/gorm"
)

// TelemetryHandler handles incoming skill telemetry events.
type TelemetryHandler struct {
	DB *gorm.DB
}

// NewTelemetryHandler creates a new TelemetryHandler.
func NewTelemetryHandler(db *gorm.DB) *TelemetryHandler {
	return &TelemetryHandler{DB: db}
}

// TelemetryRequest represents the payload for a telemetry event.
type TelemetryRequest struct {
	SessionID     uint                   `json:"session_id" binding:"required"`
	CourseID      uint                   `json:"course_id"`
	EventType     model.PathEventType    `json:"event_type" binding:"required"` // e.g. "custom"
	FromState     string                 `json:"from_state"`
	ToState       string                 `json:"to_state"`
	TriggerReason string                 `json:"trigger_reason"`
	KPID          uint                   `json:"kp_id"`
	SkillID       string                 `json:"skill_id" binding:"required"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// RecordTelemetry receives telemetry events from the frontend and logs them.
// POST /api/v1/student/telemetry
func (h *TelemetryHandler) RecordTelemetry(c *gin.Context) {
	var req TelemetryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求格式"})
		return
	}

	studentID := middleware.GetUserID(c)

	// Verify session ownership
	var session model.StudentSession
	if err := h.DB.First(&session, req.SessionID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "会话不存在"})
		return
	}
	if session.StudentID != studentID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问此会话"})
		return
	}

	// Prepare metadata as JSON string
	metadataStr := "{}"
	if req.Metadata != nil {
		bytes, err := json.Marshal(req.Metadata)
		if err == nil {
			metadataStr = string(bytes)
		}
	}

	// Create log entry
	log := model.LearningPathLog{
		StudentID:     studentID,
		SessionID:     req.SessionID,
		CourseID:      req.CourseID,
		EventType:     req.EventType,
		FromState:     req.FromState,
		ToState:       req.ToState,
		TriggerReason: req.TriggerReason,
		KPID:          req.KPID,
		SkillID:       req.SkillID,
		Metadata:      metadataStr,
		OccurredAt:    time.Now(),
	}

	if err := h.DB.Create(&log).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存遥测数据失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success"})
}
