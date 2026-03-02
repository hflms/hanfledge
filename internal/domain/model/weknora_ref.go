package model

import "time"

// ============================
// WeKnora 知识库引用模型
// ============================

// WeKnoraKBRef 课程引用的 WeKnora 知识库记录。
// 记录哪些 WeKnora 知识库被教师绑定到了哪门课程。
type WeKnoraKBRef struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CourseID  uint      `gorm:"not null;index" json:"course_id"`
	KBID      string    `gorm:"size:100;not null" json:"kb_id"`   // WeKnora 知识库 ID
	KBName    string    `gorm:"size:200;not null" json:"kb_name"` // 知识库名称（冗余缓存）
	AddedByID uint      `gorm:"not null" json:"added_by_id"`      // 添加人（教师 ID）
	CreatedAt time.Time `json:"created_at"`

	Course  Course `gorm:"foreignKey:CourseID" json:"-"`
	AddedBy User   `gorm:"foreignKey:AddedByID" json:"-"`
}
