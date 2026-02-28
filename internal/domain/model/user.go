package model

import (
	"time"

	"gorm.io/gorm"
)

// ============================
// 枚举类型定义
// ============================

// UserStatus 用户状态枚举。
type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusInactive UserStatus = "inactive"
	UserStatusBanned   UserStatus = "banned"
)

// SchoolStatus 学校状态枚举。
type SchoolStatus string

const (
	SchoolStatusActive   SchoolStatus = "active"
	SchoolStatusInactive SchoolStatus = "inactive"
)

// RoleName 角色名称常量。
type RoleName string

const (
	RoleSysAdmin    RoleName = "SYS_ADMIN"
	RoleSchoolAdmin RoleName = "SCHOOL_ADMIN"
	RoleTeacher     RoleName = "TEACHER"
	RoleStudent     RoleName = "STUDENT"
)

// ============================
// 用户与权限模型
// ============================

// User 全局用户表。身份在整个平台唯一。
type User struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Phone        string         `gorm:"uniqueIndex;size:20" json:"phone"`
	Email        *string        `gorm:"uniqueIndex;size:100" json:"email,omitempty"`
	PasswordHash string         `gorm:"size:255;not null" json:"-"`
	DisplayName  string         `gorm:"size:50;not null" json:"display_name"`
	AvatarURL    *string        `gorm:"size:500" json:"avatar_url,omitempty"`
	Status       UserStatus     `gorm:"size:20;default:active" json:"status"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联
	SchoolRoles []UserSchoolRole `gorm:"foreignKey:UserID" json:"school_roles,omitempty"`
}

// School 学校/租户表。
type School struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Name      string         `gorm:"size:100;not null" json:"name"`
	Code      string         `gorm:"uniqueIndex;size:20" json:"code"`
	Region    string         `gorm:"size:100" json:"region"`
	Status    SchoolStatus   `gorm:"size:20;default:active" json:"status"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联
	Classes []Class `gorm:"foreignKey:SchoolID" json:"classes,omitempty"`
}

// Class 班级表。隶属于学校。
type Class struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	SchoolID     uint           `gorm:"not null;index" json:"school_id"`
	Name         string         `gorm:"size:50;not null" json:"name"`
	GradeLevel   int            `gorm:"not null" json:"grade_level"`
	AcademicYear string         `gorm:"size:10" json:"academic_year"`
	CreatedAt    time.Time      `json:"created_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联
	School   School         `gorm:"foreignKey:SchoolID" json:"-"`
	Students []ClassStudent `gorm:"foreignKey:ClassID" json:"students,omitempty"`
}

// Role 角色字典表。
type Role struct {
	ID   uint     `gorm:"primaryKey" json:"id"`
	Name RoleName `gorm:"uniqueIndex;size:20;not null" json:"name"`
}

// UserSchoolRole 用户-学校-角色关联表（RBAC 核心）。
// 一个用户可在同一学校拥有多个角色。
type UserSchoolRole struct {
	ID       uint  `gorm:"primaryKey" json:"id"`
	UserID   uint  `gorm:"not null;index" json:"user_id"`
	SchoolID *uint `gorm:"index" json:"school_id"` // SYS_ADMIN 时为 nil
	RoleID   uint  `gorm:"not null" json:"role_id"`

	User   User    `gorm:"foreignKey:UserID" json:"-"`
	School *School `gorm:"foreignKey:SchoolID" json:"school,omitempty"`
	Role   Role    `gorm:"foreignKey:RoleID" json:"role,omitempty"`
}

// ClassStudent 班级-学生关联表。
type ClassStudent struct {
	ID        uint `gorm:"primaryKey" json:"id"`
	ClassID   uint `gorm:"not null;index" json:"class_id"`
	StudentID uint `gorm:"not null;index" json:"student_id"`

	Class   Class `gorm:"foreignKey:ClassID" json:"-"`
	Student User  `gorm:"foreignKey:StudentID" json:"-"`
}
