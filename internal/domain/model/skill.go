package model

// ============================
// 技能挂载与学习活动模型
// ============================

// ScaffoldLevel 支架强度枚举。
type ScaffoldLevel string

const (
	ScaffoldHigh   ScaffoldLevel = "high"
	ScaffoldMedium ScaffoldLevel = "medium"
	ScaffoldLow    ScaffoldLevel = "low"
)

// KPSkillMount 知识点-技能挂载关联表。
type KPSkillMount struct {
	ID              uint          `gorm:"primaryKey" json:"id"`
	KPID            uint          `gorm:"not null;index" json:"kp_id"`
	SkillID         string        `gorm:"size:100;not null" json:"skill_id"`
	ScaffoldLevel   ScaffoldLevel `gorm:"size:20;default:high" json:"scaffold_level"`
	ConstraintsJSON string        `gorm:"type:jsonb" json:"constraints_json"`
	Priority        int           `gorm:"default:0" json:"priority"`
	ProgressiveRule *string       `gorm:"type:jsonb" json:"progressive_rule,omitempty"`

	KnowledgePoint KnowledgePoint `gorm:"foreignKey:KPID" json:"-"`
}

// ActivityStatus 学习活动状态枚举。
type ActivityStatus string

const (
	ActivityStatusDraft     ActivityStatus = "draft"
	ActivityStatusPublished ActivityStatus = "published"
	ActivityStatusClosed    ActivityStatus = "closed"
)

// LearningActivity 学习活动表。
type LearningActivity struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	CourseID    uint           `gorm:"not null;index" json:"course_id"`
	TeacherID   uint           `gorm:"not null;index" json:"teacher_id"`
	Title       string         `gorm:"size:200;not null" json:"title"`
	KPIDS       string         `gorm:"type:jsonb" json:"kp_ids"`
	SkillConfig string         `gorm:"type:jsonb" json:"skill_config"`
	Deadline    *string        `json:"deadline,omitempty"`
	AllowRetry  bool           `gorm:"default:true" json:"allow_retry"`
	MaxAttempts int            `gorm:"default:3" json:"max_attempts"`
	Status      ActivityStatus `gorm:"size:20;default:draft" json:"status"`
	CreatedAt   string         `json:"created_at"`
	PublishedAt *string        `json:"published_at,omitempty"`

	Course          Course                    `gorm:"foreignKey:CourseID" json:"-"`
	Teacher         User                      `gorm:"foreignKey:TeacherID" json:"-"`
	AssignedClasses []ActivityClassAssignment `gorm:"foreignKey:ActivityID" json:"assigned_classes,omitempty"`
}

// ActivityClassAssignment 学习活动-班级发布关联。
type ActivityClassAssignment struct {
	ID         uint `gorm:"primaryKey" json:"id"`
	ActivityID uint `gorm:"not null;index" json:"activity_id"`
	ClassID    uint `gorm:"not null;index" json:"class_id"`
}
