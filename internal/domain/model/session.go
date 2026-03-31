package model

import "time"

// ============================
// 交互与学情模型
// ============================

// SessionStatus 会话状态枚举。
type SessionStatus string

const (
	SessionStatusActive    SessionStatus = "active"
	SessionStatusCompleted SessionStatus = "completed"
	SessionStatusAbandoned SessionStatus = "abandoned"
)

// SessionMode 会话模式枚举。
type SessionMode string

const (
	ModeSocratic SessionMode = "socratic"
	ModeTesting  SessionMode = "testing"
)

// StudentSession 学生学习会话表。
type StudentSession struct {
	ID          uint          `gorm:"primaryKey" json:"id"`
	StudentID   uint          `gorm:"not null;index" json:"student_id"`
	ActivityID  uint          `gorm:"not null;index" json:"activity_id"`
	CurrentKP   uint          `gorm:"not null" json:"current_kp_id"`
	ActiveSkill string        `gorm:"size:100" json:"active_skill"`
	Scaffold    ScaffoldLevel `gorm:"size:20" json:"scaffold_level"`
	SkillState  *string       `gorm:"type:jsonb" json:"skill_state,omitempty"` // 技能级会话状态 (e.g., FallacySessionState)
	IsSandbox   bool          `gorm:"default:false" json:"is_sandbox"`         // 沙盒预览会话标记 (design.md §5.1 Step 3)
	Status      SessionStatus `gorm:"size:20;default:active" json:"status"`
	Mode        SessionMode   `gorm:"size:20;default:'socratic'" json:"mode"` // socratic | testing
	StartedAt   time.Time     `json:"started_at"`
	EndedAt     *time.Time    `json:"ended_at,omitempty"`
}

// Interaction AI 交互记录表。
type Interaction struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	SessionID  uint      `gorm:"not null;index" json:"session_id"`
	KPID       uint      `gorm:"index" json:"kp_id"`           // 交互发生时的目标知识点 (用于按步骤过滤历史)
	StepIndex  int       `gorm:"default:0" json:"step_index"`  // 在活动 KP 序列中的位置 (0-based)
	Role       string    `gorm:"size:20;not null" json:"role"` // "student" | "coach" | "system" | "teacher"
	Content    string    `gorm:"type:text;not null" json:"content"`
	SkillID    string    `gorm:"size:100" json:"skill_id"`
	TokensUsed int       `gorm:"default:0" json:"tokens_used"`
	CreatedAt  time.Time `json:"created_at"`

	// 评估分数（由 RAGAS/MRBench 异步填充）
	FaithfulnessScore    *float64 `json:"faithfulness_score,omitempty"`
	ActionabilityScore   *float64 `json:"actionability_score,omitempty"`
	AnswerRestraintScore *float64 `json:"answer_restraint_score,omitempty"`           // 答案克制度: 1.0=完全不泄露, 0.0=直接给答案
	ContextPrecision     *float64 `json:"context_precision,omitempty"`                // 检索上下文精度
	ContextRecall        *float64 `json:"context_recall,omitempty"`                   // 检索上下文召回
	EvalStatus           string   `gorm:"size:20;default:pending" json:"eval_status"` // pending | evaluated | skipped
}

// StepSummary 步骤学习摘要表。
// 每次步骤转换时由 LLM 生成，概括前一步骤的学习成果，
// 供后续步骤的 Designer 注入系统提示词，实现跨步骤知识衔接。
type StepSummary struct {
	ID           uint    `gorm:"primaryKey" json:"id"`
	SessionID    uint    `gorm:"not null;index" json:"session_id"`
	KPID         uint    `gorm:"not null" json:"kp_id"`             // 该步骤对应的知识点
	StepIndex    int     `gorm:"default:0" json:"step_index"`       // 步骤序号
	Summary      string  `gorm:"type:text;not null" json:"summary"` // LLM 生成的学习摘要
	MasteryStart float64 `gorm:"default:0" json:"mastery_start"`    // 步骤开始时的掌握度
	MasteryEnd   float64 `gorm:"default:0" json:"mastery_end"`      // 步骤结束时的掌握度
	TurnCount    int     `gorm:"default:0" json:"turn_count"`       // 该步骤交互轮次
	SkillID      string  `gorm:"size:100" json:"skill_id"`          // 该步骤使用的技能
	CreatedAt    string  `json:"created_at"`
}

// StudentKPMastery 学生-知识点掌握度表。
type StudentKPMastery struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	StudentID     uint       `gorm:"not null;uniqueIndex:idx_student_kp" json:"student_id"`
	KPID          uint       `gorm:"not null;uniqueIndex:idx_student_kp" json:"kp_id"`
	MasteryScore  float64    `gorm:"default:0.1" json:"mastery_score"`
	AttemptCount  int        `gorm:"default:0" json:"attempt_count"`
	CorrectCount  int        `gorm:"default:0" json:"correct_count"`
	PassedTest    bool       `gorm:"default:false" json:"passed_test"`
	LastAttemptAt *time.Time `json:"last_attempt_at,omitempty"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// ── Error Notebook (错题本自动归档, design.md §5.2 Step 3) ──

// ErrorNotebookEntry 错题本记录表。
// 交互中暴露的错误和 AI 引导过程被自动归档为结构化错题记录，
// 关联到知识图谱节点，支持后续复习时的定向 RAG 检索。
type ErrorNotebookEntry struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	StudentID      uint       `gorm:"not null;index:idx_error_student" json:"student_id"`
	KPID           uint       `gorm:"not null;index:idx_error_kp" json:"kp_id"`
	SessionID      uint       `gorm:"not null;index" json:"session_id"`
	StudentInput   string     `gorm:"type:text;not null" json:"student_input"`   // 学生的错误回答
	CoachGuidance  string     `gorm:"type:text;not null" json:"coach_guidance"`  // AI 的引导回复
	ErrorType      string     `gorm:"size:30;default:unknown" json:"error_type"` // conceptual | procedural | intuitive | unknown
	MasteryAtError float64    `gorm:"default:0" json:"mastery_at_error"`         // 出错时的掌握度
	Resolved       bool       `gorm:"default:false" json:"resolved"`             // 后续掌握度 >= 0.8 时自动标记为已解决
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	ArchivedAt     time.Time  `json:"archived_at"`
}
