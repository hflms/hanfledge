package model

import "time"

// ============================
// WeKnora Token 映射模型
// ============================

// WeKnoraToken 用户级 WeKnora Token 映射。
// 每个 Hanfledge 用户在 WeKnora 中有独立的身份，通过自动注册实现。
type WeKnoraToken struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"uniqueIndex;not null" json:"user_id"` // Hanfledge 用户 ID
	WKUserID     string    `gorm:"size:100" json:"wk_user_id"`          // WeKnora 用户 ID
	WKEmail      string    `gorm:"size:200;not null" json:"wk_email"`   // 自动生成的邮箱 (user_{id}@hanfledge.local)
	Token        string    `gorm:"type:text" json:"-"`                  // 当前 access token
	RefreshToken string    `gorm:"type:text" json:"-"`                  // refresh token
	ExpiresAt    time.Time `json:"expires_at"`                          // token 过期时间
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	User User `gorm:"foreignKey:UserID" json:"-"`
}
