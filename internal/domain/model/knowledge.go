package model

import "time"

// ============================
// 课程与知识模型
// ============================

// CourseStatus 课程状态枚举。
type CourseStatus string

const (
	CourseStatusDraft     CourseStatus = "draft"
	CourseStatusPublished CourseStatus = "published"
	CourseStatusArchived  CourseStatus = "archived"
)

// Course 课程表。
type Course struct {
	ID          uint         `gorm:"primaryKey" json:"id"`
	SchoolID    uint         `gorm:"not null;index" json:"school_id"`
	TeacherID   uint         `gorm:"not null;index" json:"teacher_id"`
	Title       string       `gorm:"size:200;not null" json:"title"`
	Subject     string       `gorm:"size:50;not null" json:"subject"`
	GradeLevel  int          `gorm:"not null" json:"grade_level"`
	Description *string      `gorm:"type:text" json:"description,omitempty"`
	Status      CourseStatus `gorm:"size:20;default:draft" json:"status"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`

	School   School    `gorm:"foreignKey:SchoolID" json:"-"`
	Teacher  User      `gorm:"foreignKey:TeacherID" json:"-"`
	Chapters []Chapter `gorm:"foreignKey:CourseID" json:"chapters,omitempty"`
}

// Chapter 章节表。
type Chapter struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	CourseID  uint   `gorm:"not null;index" json:"course_id"`
	ParentID  *uint  `gorm:"index" json:"parent_id,omitempty"`
	Title     string `gorm:"size:200;not null" json:"title"`
	SortOrder int    `gorm:"default:0" json:"sort_order"`

	Course          Course           `gorm:"foreignKey:CourseID" json:"-"`
	KnowledgePoints []KnowledgePoint `gorm:"foreignKey:ChapterID" json:"knowledge_points,omitempty"`
}

// KnowledgePoint 知识点表（同步存在于 PostgreSQL 和 Neo4j）。
type KnowledgePoint struct {
	ID          uint    `gorm:"primaryKey" json:"id"`
	ChapterID   uint    `gorm:"not null;index" json:"chapter_id"`
	Neo4jNodeID string  `gorm:"size:50;index" json:"neo4j_node_id"`
	Title       string  `gorm:"size:200;not null" json:"title"`
	Description string  `gorm:"type:text" json:"description"`
	Difficulty  float64 `gorm:"default:0.5" json:"difficulty"`
	IsKeyPoint  bool    `gorm:"default:false" json:"is_key_point"`

	Chapter        Chapter         `gorm:"foreignKey:ChapterID" json:"-"`
	MountedSkills  []KPSkillMount  `gorm:"foreignKey:KPID" json:"mounted_skills,omitempty"`
	Misconceptions []Misconception `gorm:"foreignKey:KPID" json:"misconceptions,omitempty"`
}

// ── Misconception (常见误区) ────────────────────────────────

// TrapType 误区类型枚举。
type TrapType string

const (
	TrapTypeConceptual TrapType = "conceptual" // 概念性误解
	TrapTypeProcedural TrapType = "procedural" // 操作性错误
	TrapTypeIntuit     TrapType = "intuitive"  // 直觉性偏差
	TrapTypeTransfer   TrapType = "transfer"   // 迁移性混淆（跨知识点概念混淆）
)

// Misconception 常见误区/认知陷阱表（同步存在于 PostgreSQL 和 Neo4j）。
// Neo4j 关系: (KnowledgePoint)-[:HAS_TRAP]->(Misconception)
type Misconception struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	KPID        uint      `gorm:"not null;index" json:"kp_id"`
	Neo4jNodeID string    `gorm:"size:50;index" json:"neo4j_node_id"`
	Description string    `gorm:"type:text;not null" json:"description"`
	TrapType    TrapType  `gorm:"size:20;not null;default:conceptual" json:"trap_type"`
	Severity    float64   `gorm:"default:0.5" json:"severity"` // 严重度 [0,1]，越高越常见/危害越大
	CreatedAt   time.Time `json:"created_at"`

	KnowledgePoint KnowledgePoint `gorm:"foreignKey:KPID" json:"-"`
}

// ── Cross-Disciplinary Link (跨学科联结) ────────────────────

// CrossLink 跨学科联结表（记录 Neo4j 中 RELATES_TO 关系的 PG 镜像）。
type CrossLink struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	FromKPID  uint      `gorm:"not null;index" json:"from_kp_id"`
	ToKPID    uint      `gorm:"not null;index" json:"to_kp_id"`
	LinkType  string    `gorm:"size:50;not null;default:analogy" json:"link_type"` // analogy | shared_model | application
	Weight    float64   `gorm:"default:1.0" json:"weight"`                         // 关联强度 [0,1]
	CreatedAt time.Time `json:"created_at"`

	FromKP KnowledgePoint `gorm:"foreignKey:FromKPID" json:"-"`
	ToKP   KnowledgePoint `gorm:"foreignKey:ToKPID" json:"-"`
}
