package model

import (
	"time"
)

// AnalyticsEvent represents a frontend performance or interaction event.
type AnalyticsEvent struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Type      string    `gorm:"type:varchar(50);not null;index" json:"type"` // render, interaction, error, performance
	Component string    `gorm:"type:varchar(100);not null;index" json:"component"`
	Data      string    `gorm:"type:jsonb" json:"data"`
	UserID    *uint     `gorm:"index" json:"user_id,omitempty"`
	SessionID *uint     `gorm:"index" json:"session_id,omitempty"`
	Timestamp int64     `gorm:"not null;index" json:"timestamp"`
	CreatedAt time.Time `json:"created_at"`
}

func (AnalyticsEvent) TableName() string {
	return "analytics_events"
}
