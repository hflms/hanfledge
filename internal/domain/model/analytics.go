package model

import "time"

// ============================
// 跨会话学习分析模型 — 学生画像、学习路径、通用维度
// ============================
//
// 设计原则：
// 1. 核心字段强类型 + JSONB 字段用于 Skill 级灵活扩展
// 2. LearningPathLog 采用事件溯源思想，支持轨迹回放
// 3. StudentDimensionRecord 允许任意 Skill 注入跨会话聚合指标

// ── StudentProfile 学生全局画像 ─────────────────────────────

// StudentProfile 跨会话聚合的学生全局画像表。
// 汇总学生在所有会话中的学习表现，提供全局视角的学情分析。
// Dimensions 字段采用 JSONB 格式，允许各 Skill 写入自定义聚合指标。
type StudentProfile struct {
	ID        uint `gorm:"primaryKey" json:"id"`
	StudentID uint `gorm:"not null;uniqueIndex" json:"student_id"`

	// ── 核心聚合指标 ──
	TotalStudyDuration int64   `gorm:"default:0" json:"total_study_duration"` // 总学习时长(秒)
	TotalSessions      int     `gorm:"default:0" json:"total_sessions"`       // 总会话数
	TotalInteractions  int     `gorm:"default:0" json:"total_interactions"`   // 总交互轮次
	OverallMastery     float64 `gorm:"default:0" json:"overall_mastery"`      // 全局平均掌握度
	MasteredKPCount    int     `gorm:"default:0" json:"mastered_kp_count"`    // 已掌握知识点数(mastery >= 0.8)
	WeakKPCount        int     `gorm:"default:0" json:"weak_kp_count"`        // 薄弱知识点数(mastery < 0.4)
	TotalErrorCount    int     `gorm:"default:0" json:"total_error_count"`    // 累计错误次数
	ResolvedErrorCount int     `gorm:"default:0" json:"resolved_error_count"` // 已解决错误次数
	CurrentStreak      int     `gorm:"default:0" json:"current_streak"`       // 当前连续掌握知识点数
	BestStreak         int     `gorm:"default:0" json:"best_streak"`          // 历史最佳连续掌握数

	// ── 学习风格与偏好 (JSONB) ──
	// 格式: ["general_concept_socratic", "general_assessment_quiz"]
	PreferredSkills string `gorm:"type:jsonb;default:'[]'" json:"preferred_skills"`
	// 格式: [{"kp_id": 1, "title": "光合作用", "mastery": 0.95}]
	StrengthAreas string `gorm:"type:jsonb;default:'[]'" json:"strength_areas"`
	// 格式: [{"kp_id": 5, "title": "有丝分裂", "mastery": 0.25}]
	WeaknessAreas string `gorm:"type:jsonb;default:'[]'" json:"weakness_areas"`

	// ── Skill 可扩展维度 (JSONB) ──
	// 各 Skill 可自由写入自定义聚合指标，以 skill_id 为命名空间。
	// 示例:
	// {
	//   "general_assessment_quiz": {"avg_score": 0.82, "total_quizzes": 15},
	//   "general_assessment_fallacy": {"total_identified": 12, "accuracy": 0.85},
	//   "presentation_generator": {"total_generated": 3}
	// }
	Dimensions string `gorm:"type:jsonb;default:'{}'" json:"dimensions"`

	// ── 时间戳 ──
	LastActiveAt time.Time `json:"last_active_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// 关联
	Student User `gorm:"foreignKey:StudentID" json:"-"`
}

// ── LearningPathLog 学习路径事件日志 ────────────────────────

// PathEventType 学习路径事件类型枚举。
type PathEventType string

const (
	// PathEventSkillSwitch 技能切换事件。
	// FromState=旧技能ID, ToState=新技能ID, TriggerReason=切换原因
	PathEventSkillSwitch PathEventType = "skill_switch"

	// PathEventScaffoldChange 支架等级变化事件。
	// FromState=旧等级, ToState=新等级, MasteryAtEvent=触发时的掌握度
	PathEventScaffoldChange PathEventType = "scaffold_change"

	// PathEventKPTransition 知识点跳转事件。
	// FromState=旧KP标题, ToState=新KP标题, KPID=新KP的ID
	PathEventKPTransition PathEventType = "kp_transition"

	// PathEventSessionStart 会话开始事件。
	PathEventSessionStart PathEventType = "session_start"

	// PathEventSessionEnd 会话结束事件。
	PathEventSessionEnd PathEventType = "session_end"

	// PathEventMilestone 里程碑事件（如掌握度突破阈值、解锁成就等）。
	PathEventMilestone PathEventType = "milestone"

	// PathEventCustom Skill 自定义事件。
	// 允许各 Skill 记录特有的路径事件（如出题完成、角色切换等）。
	PathEventCustom PathEventType = "custom"
)

// LearningPathLog 学习路径事件日志表。
// 采用事件溯源模式，记录学生学习过程中的每一次关键状态转换。
// 支持轨迹回放、学习路径可视化和教学效果分析。
type LearningPathLog struct {
	ID        uint          `gorm:"primaryKey" json:"id"`
	StudentID uint          `gorm:"not null;index:idx_path_student_time" json:"student_id"`
	SessionID uint          `gorm:"index" json:"session_id"`
	CourseID  uint          `gorm:"index" json:"course_id"`
	EventType PathEventType `gorm:"size:30;not null;index:idx_path_event_type" json:"event_type"`

	// ── 状态转换描述 ──
	FromState     string `gorm:"size:200" json:"from_state"`     // 来源状态（如旧技能ID、旧支架等级）
	ToState       string `gorm:"size:200" json:"to_state"`       // 目标状态（如新技能ID、新支架等级）
	TriggerReason string `gorm:"size:500" json:"trigger_reason"` // 触发原因的人类可读描述

	// ── 上下文快照 ──
	KPID           uint    `json:"kp_id,omitempty"`            // 关联知识点
	SkillID        string  `gorm:"size:100" json:"skill_id"`   // 当前活跃技能
	MasteryAtEvent float64 `json:"mastery_at_event,omitempty"` // 事件发生时的掌握度

	// ── 可扩展元数据 (JSONB) ──
	// 允许携带任意附加信息，由事件生产者自定义。
	// 示例:
	// skill_switch:    {"old_scaffold": "high", "new_scaffold": "medium"}
	// milestone:       {"achievement_id": 3, "achievement_name": "势如破竹"}
	// custom:          {"quiz_batch": 2, "correct_count": 4, "total": 5}
	Metadata string `gorm:"type:jsonb;default:'{}'" json:"metadata"`

	OccurredAt time.Time `gorm:"not null;index:idx_path_student_time" json:"occurred_at"`
}

// ── StudentDimensionRecord 通用维度记录 ─────────────────────

// StudentDimensionRecord 通用跨会话维度快照表。
// 允许任意 Skill 以 Key-Value 方式写入自定义的跨会话聚合指标。
// 每个 (StudentID, DimensionKey, SkillID, CourseID) 组合唯一。
//
// 使用场景示例:
//   - quiz skill 写入 dimension_key="quiz_avg_score", numeric_value=0.82
//   - survey skill 写入 dimension_key="learning_style", json_value={"visual": 0.7, "auditory": 0.3}
//   - presentation skill 写入 dimension_key="slides_generated", numeric_value=5
//
// 设计优势：无需修改数据模型即可支持新 Skill 的跨会话数据需求。
type StudentDimensionRecord struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	StudentID    uint   `gorm:"not null;uniqueIndex:idx_dim_unique" json:"student_id"`
	DimensionKey string `gorm:"size:100;not null;uniqueIndex:idx_dim_unique" json:"dimension_key"` // 指标键名
	SkillID      string `gorm:"size:100;uniqueIndex:idx_dim_unique" json:"skill_id"`               // 数据来源 Skill
	CourseID     uint   `gorm:"uniqueIndex:idx_dim_unique" json:"course_id"`                       // 关联课程（0=全局）

	// ── 多类型数据存储 ──
	// 使用三种值类型覆盖常见场景，查询时根据 DimensionKey 约定选择对应字段。
	NumericValue *float64 `json:"numeric_value,omitempty"` // 数值型指标（如分数、次数、百分比）
	TextValue    *string  `json:"text_value,omitempty"`    // 文本型指标（如学习风格标签）
	// 结构化指标（如趋势数组、嵌套对象）
	// 示例: {"trend": [0.3, 0.5, 0.7, 0.82], "last_10_scores": [4,5,3,5,4,5,5,5,4,5]}
	JSONValue string `gorm:"type:jsonb;default:'{}'" json:"json_value"`

	// ── 版本控制 ──
	// 每次更新递增，支持乐观并发控制和变更追踪。
	Version int `gorm:"default:1" json:"version"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
