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
	Difficulty  float64 `gorm:"default:0.5" json:"difficulty"`
	IsKeyPoint  bool    `gorm:"default:false" json:"is_key_point"`

	Chapter       Chapter        `gorm:"foreignKey:ChapterID" json:"-"`
	MountedSkills []KPSkillMount `gorm:"foreignKey:KPID" json:"mounted_skills,omitempty"`
}
