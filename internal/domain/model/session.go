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

// StudentSession 学生学习会话表。
type StudentSession struct {
	ID          uint          `gorm:"primaryKey" json:"id"`
	StudentID   uint          `gorm:"not null;index" json:"student_id"`
	ActivityID  uint          `gorm:"not null;index" json:"activity_id"`
	CurrentKP   uint          `gorm:"not null" json:"current_kp_id"`
	ActiveSkill string        `gorm:"size:100" json:"active_skill"`
	Scaffold    ScaffoldLevel `gorm:"size:20" json:"scaffold_level"`
	Status      SessionStatus `gorm:"size:20;default:active" json:"status"`
	StartedAt   time.Time     `json:"started_at"`
	EndedAt     *time.Time    `json:"ended_at,omitempty"`
}

// Interaction AI 交互记录表。
type Interaction struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	SessionID  uint      `gorm:"not null;index" json:"session_id"`
	Role       string    `gorm:"size:20;not null" json:"role"` // "student" | "coach" | "system"
	Content    string    `gorm:"type:text;not null" json:"content"`
	SkillID    string    `gorm:"size:100" json:"skill_id"`
	TokensUsed int       `gorm:"default:0" json:"tokens_used"`
	CreatedAt  time.Time `json:"created_at"`

	// 评估分数（由 RAGAS/MRBench 异步填充）
	FaithfulnessScore  *float64 `json:"faithfulness_score,omitempty"`
	ActionabilityScore *float64 `json:"actionability_score,omitempty"`
}

// StudentKPMastery 学生-知识点掌握度表。
type StudentKPMastery struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	StudentID     uint       `gorm:"not null;uniqueIndex:idx_student_kp" json:"student_id"`
	KPID          uint       `gorm:"not null;uniqueIndex:idx_student_kp" json:"kp_id"`
	MasteryScore  float64    `gorm:"default:0.1" json:"mastery_score"`
	AttemptCount  int        `gorm:"default:0" json:"attempt_count"`
	CorrectCount  int        `gorm:"default:0" json:"correct_count"`
	LastAttemptAt *time.Time `json:"last_attempt_at,omitempty"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
