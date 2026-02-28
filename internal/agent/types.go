package agent

import (
	"context"

	"github.com/hflms/hanfledge/internal/domain/model"
)

// ============================
// 多 Agent 编排 — 通信类型定义
// ============================

// ── Scaffold Level (re-export from model for convenience) ───

// ScaffoldLevel 支架强度枚举。
type ScaffoldLevel = model.ScaffoldLevel

const (
	ScaffoldHigh   = model.ScaffoldHigh
	ScaffoldMedium = model.ScaffoldMedium
	ScaffoldLow    = model.ScaffoldLow
)

// ── Agent Interface ─────────────────────────────────────────

// Agent 定义所有 Agent 的通用接口。
type Agent interface {
	Name() string
}

// ── Strategist → Designer Channel Type ──────────────────────

// LearningPrescription 策略师输出的"学习处方"。
type LearningPrescription struct {
	SessionID        uint                   `json:"session_id"`
	StudentID        uint                   `json:"student_id"`
	TargetKPSequence []KnowledgePointTarget `json:"target_kp_sequence"`
	InitialScaffold  ScaffoldLevel          `json:"initial_scaffold"`
	RecommendedSkill string                 `json:"recommended_skill"`
	PrereqGaps       []string               `json:"prereq_gaps"`
}

// KnowledgePointTarget 单个知识点的学习目标。
type KnowledgePointTarget struct {
	KPID           uint          `json:"kp_id"`
	CurrentMastery float64       `json:"current_mastery"`
	TargetMastery  float64       `json:"target_mastery"`
	ScaffoldLevel  ScaffoldLevel `json:"scaffold_level"`
	SkillID        string        `json:"skill_id"`
}

// ── Designer → Coach Channel Type ───────────────────────────

// PersonalizedMaterial 设计师组装的个性化学习材料。
type PersonalizedMaterial struct {
	SessionID       uint                 `json:"session_id"`
	Prescription    LearningPrescription `json:"prescription"`
	RetrievedChunks []RetrievedChunk     `json:"retrieved_chunks"`
	GraphContext    []GraphNode          `json:"graph_context"`
	SystemPrompt    string               `json:"system_prompt"`
}

// RetrievedChunk 混合检索召回的文档片段。
type RetrievedChunk struct {
	Content    string  `json:"content"`
	Source     string  `json:"source"` // "semantic" | "graph"
	Score      float64 `json:"score"`
	ChunkIndex int     `json:"chunk_index"`
}

// GraphNode 图谱上下文节点（知识点及其关系）。
type GraphNode struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Difficulty float64 `json:"difficulty"`
	Relation   string  `json:"relation"` // "target" | "prerequisite" | "related"
	Depth      int     `json:"depth"`
}

// ── Coach → Critic Channel Type ─────────────────────────────

// DraftResponse 教练的初稿回复（待审查）。
type DraftResponse struct {
	SessionID     uint          `json:"session_id"`
	Content       string        `json:"content"`
	SkillID       string        `json:"skill_id"`
	ScaffoldLevel ScaffoldLevel `json:"scaffold_level"`
	TokensUsed    int           `json:"tokens_used"`
}

// ── Critic → Coach Channel Type ─────────────────────────────

// ReviewResult 审查者的审查结果。
type ReviewResult struct {
	SessionID    uint    `json:"session_id"`
	Approved     bool    `json:"approved"`
	Feedback     string  `json:"feedback"`
	LeakageScore float64 `json:"leakage_score"` // 答案泄露分数 [0.0, 1.0]，越高越可能泄露
	DepthScore   float64 `json:"depth_score"`   // 启发深度分数 [0.0, 1.0]，越高越有深度
	Revision     string  `json:"revision"`      // 审查者建议的修订版（如果未通过）
}

// ── Coach → Strategist Channel Type ─────────────────────────

// MasteryUpdate 掌握度更新事件（Coach 完成一轮交互后发送）。
type MasteryUpdate struct {
	SessionID    uint    `json:"session_id"`
	StudentID    uint    `json:"student_id"`
	KPID         uint    `json:"kp_id"`
	Correct      bool    `json:"correct"`
	NewMastery   float64 `json:"new_mastery"`
	AttemptCount int     `json:"attempt_count"`
}

// ── WebSocket Event Types ───────────────────────────────────

// WSEvent WebSocket 通信事件。
type WSEvent struct {
	Event     string      `json:"event"`
	Payload   interface{} `json:"payload"`
	Timestamp int64       `json:"timestamp"`
}

// WebSocket 事件类型常量。
const (
	EventUserMessage      = "user_message"
	EventAgentThinking    = "agent_thinking"
	EventTokenDelta       = "token_delta"
	EventUIScaffoldChange = "ui_scaffold_change"
	EventTurnComplete     = "turn_complete"
)

// ThinkingPayload agent_thinking 事件的载荷。
type ThinkingPayload struct {
	Status string `json:"status"`
}

// TokenDeltaPayload token_delta 事件的载荷。
type TokenDeltaPayload struct {
	Text string `json:"text"`
}

// ScaffoldChangePayload ui_scaffold_change 事件的载荷。
type ScaffoldChangePayload struct {
	Action string      `json:"action"`
	Data   interface{} `json:"data"`
}

// TurnCompletePayload turn_complete 事件的载荷。
type TurnCompletePayload struct {
	TotalTokens int `json:"total_tokens"`
}

// UserMessagePayload user_message 事件的载荷。
type UserMessagePayload struct {
	Text string `json:"text"`
}

// ── Session Turn Context ────────────────────────────────────

// TurnContext 单轮对话上下文，贯穿整个 Agent 管道。
type TurnContext struct {
	Ctx        context.Context
	SessionID  uint
	StudentID  uint
	ActivityID uint
	UserInput  string
	Scaffold   ScaffoldLevel

	// 管道中间产物
	Prescription *LearningPrescription
	Material     *PersonalizedMaterial
	Draft        *DraftResponse
	Review       *ReviewResult

	// 流式输出回调
	OnThinking     func(status string)
	OnTokenDelta   func(text string)
	OnScaffold     func(action string, data interface{})
	OnTurnComplete func(totalTokens int)
}
