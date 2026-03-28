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

// ActivityType 学习活动类型枚举。
type ActivityType string

const (
	ActivityTypeAutonomous ActivityType = "autonomous" // 全自主学习 (skills组织)
	ActivityTypeGuided     ActivityType = "guided"     // 教师规定环节，定制化
)

// LearningActivity 学习活动表。
type LearningActivity struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	CourseID       uint           `gorm:"not null;index" json:"course_id"`
	TeacherID      uint           `gorm:"not null;index" json:"teacher_id"`
	Title          string         `gorm:"size:200;not null" json:"title"`
	Description    string         `gorm:"size:2000" json:"description,omitempty"`
	Type           ActivityType   `gorm:"size:50;default:autonomous" json:"type"` // 活动类型
	DesignerID     string         `gorm:"size:100" json:"designer_id,omitempty"`
	DesignerConfig string         `gorm:"type:jsonb" json:"designer_config,omitempty"`
	StepsConfig    string         `gorm:"type:jsonb" json:"steps_config,omitempty"` // 旧版简易环节配置(向后兼容)
	KPIDS          string         `gorm:"type:jsonb" json:"kp_ids"`
	SkillConfig    string         `gorm:"type:jsonb" json:"skill_config"`
	Deadline       *string        `json:"deadline,omitempty"`
	AllowRetry     bool           `gorm:"default:true" json:"allow_retry"`
	MaxAttempts    int            `gorm:"default:3" json:"max_attempts"`
	Status         ActivityStatus `gorm:"size:20;default:draft" json:"status"`
	CreatedAt      string         `json:"created_at"`
	UpdatedAt      string         `json:"updated_at,omitempty"`
	PublishedAt    *string        `json:"published_at,omitempty"`

	Course          Course                    `gorm:"foreignKey:CourseID" json:"-"`
	Teacher         User                      `gorm:"foreignKey:TeacherID" json:"-"`
	AssignedClasses []ActivityClassAssignment `gorm:"foreignKey:ActivityID" json:"assigned_classes,omitempty"`
	Steps           []ActivityStep            `gorm:"foreignKey:ActivityID;constraint:OnDelete:CASCADE" json:"steps,omitempty"`
}

// ActivityClassAssignment 学习活动-班级发布关联。
type ActivityClassAssignment struct {
	ID         uint `gorm:"primaryKey" json:"id"`
	ActivityID uint `gorm:"not null;index" json:"activity_id"`
	ClassID    uint `gorm:"not null;index" json:"class_id"`
}

// ContentBlockType 内容块类型枚举。
type ContentBlockType string

const (
	ContentBlockMarkdown ContentBlockType = "markdown"
	ContentBlockFile     ContentBlockType = "file"
	ContentBlockVideo    ContentBlockType = "video"
	ContentBlockImage    ContentBlockType = "image"
)

// ActivityStep 学习活动环节表。
// 每个环节包含标题、描述和一组有序的内容块。
type ActivityStep struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	ActivityID  uint   `gorm:"not null;index" json:"activity_id"`
	Title       string `gorm:"size:200;not null" json:"title"`
	Description string `gorm:"size:2000" json:"description,omitempty"`
	SortOrder   int    `gorm:"not null;default:0" json:"sort_order"`
	// ContentBlocks 存储环节的展示内容，为 JSON 数组。
	// 每个元素: { "type": "markdown|file|video|image", "content": "...", "file_name": "...", "file_url": "...", "file_size": 0 }
	ContentBlocks string `gorm:"type:jsonb;default:'[]'" json:"content_blocks"`
	Duration      int    `gorm:"default:0" json:"duration,omitempty"` // 建议时长(分钟)
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at,omitempty"`
}

// ============================
// 教师自定义技能 (design.md §6.4)
// ============================

// CustomSkillStatus 自定义技能状态枚举。
type CustomSkillStatus string

const (
	CustomSkillStatusDraft     CustomSkillStatus = "draft"     // 草稿
	CustomSkillStatusPublished CustomSkillStatus = "published" // 已发布（仅创建者可用）
	CustomSkillStatusShared    CustomSkillStatus = "shared"    // 已分享（校级或平台级）
	CustomSkillStatusArchived  CustomSkillStatus = "archived"  // 已归档
)

// CustomSkillVisibility 自定义技能可见范围。
type CustomSkillVisibility string

const (
	VisibilityPrivate  CustomSkillVisibility = "private"  // 仅创建者
	VisibilitySchool   CustomSkillVisibility = "school"   // 校内共享
	VisibilityPlatform CustomSkillVisibility = "platform" // 全平台共享
)

// CustomSkill 教师自定义技能表。
// 教师通过可视化表单创建，存储在数据库中（区别于内置技能存储在文件系统）。
// SkillID 字段遵循三段式命名: {subject}_{scenario}_{method}
type CustomSkill struct {
	ID          uint                  `gorm:"primaryKey" json:"id"`
	SkillID     string                `gorm:"size:100;uniqueIndex;not null" json:"skill_id"`
	TeacherID   uint                  `gorm:"not null;index" json:"teacher_id"`
	SchoolID    uint                  `gorm:"index" json:"school_id"`
	Name        string                `gorm:"size:200;not null" json:"name"`
	Description string                `gorm:"size:1000" json:"description"`
	Category    string                `gorm:"size:50" json:"category"`
	Subjects    string                `gorm:"type:jsonb;default:'[]'" json:"subjects"`
	Tags        string                `gorm:"type:jsonb;default:'[]'" json:"tags"`
	SkillMD     string                `gorm:"type:text;not null" json:"skill_md"`
	ToolsConfig string                `gorm:"type:jsonb;default:'{}'" json:"tools_config"`
	Templates   string                `gorm:"type:jsonb;default:'[]'" json:"templates"`
	Status      CustomSkillStatus     `gorm:"size:20;default:draft;not null" json:"status"`
	Visibility  CustomSkillVisibility `gorm:"size:20;default:private;not null" json:"visibility"`
	Version     int                   `gorm:"default:1;not null" json:"version"`
	UsageCount  int                   `gorm:"default:0" json:"usage_count"`
	CreatedAt   string                `json:"created_at"`
	UpdatedAt   string                `json:"updated_at"`

	Teacher User `gorm:"foreignKey:TeacherID" json:"-"`
}

// CustomSkillVersion 自定义技能版本历史表。
// 每次更新已发布的技能时，旧版本存入此表用于回滚。
type CustomSkillVersion struct {
	ID            uint   `gorm:"primaryKey" json:"id"`
	CustomSkillID uint   `gorm:"not null;index" json:"custom_skill_id"`
	Version       int    `gorm:"not null" json:"version"`
	SkillMD       string `gorm:"type:text;not null" json:"skill_md"`
	ToolsConfig   string `gorm:"type:jsonb" json:"tools_config"`
	Templates     string `gorm:"type:jsonb" json:"templates"`
	ChangeLog     string `gorm:"size:500" json:"change_log"`
	CreatedAt     string `json:"created_at"`

	CustomSkill CustomSkill `gorm:"foreignKey:CustomSkillID" json:"-"`
}
