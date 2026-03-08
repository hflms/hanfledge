package model

import "time"

// SoulVersion represents a version of the soul.md AI rules.
type SoulVersion struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Version   string    `gorm:"type:varchar(20);not null" json:"version"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	UpdatedBy uint      `gorm:"not null" json:"updated_by"`
	Reason    string    `gorm:"type:text" json:"reason"`
	IsActive  bool      `gorm:"default:false;index" json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

func (SoulVersion) TableName() string {
	return "soul_versions"
}
