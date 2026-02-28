package model

import "time"

// -- Plugin Marketplace Models ----------------------------------

// MarketplacePlugin represents a plugin listing in the community marketplace.
type MarketplacePlugin struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	PluginID    string     `gorm:"uniqueIndex;size:100;not null" json:"plugin_id"`
	Name        string     `gorm:"size:200;not null" json:"name"`
	Description string     `gorm:"type:text" json:"description"`
	Version     string     `gorm:"size:50;not null" json:"version"`
	Author      string     `gorm:"size:100" json:"author"`
	AuthorID    uint       `gorm:"index" json:"author_id"`
	Type        string     `gorm:"size:50;not null" json:"type"`                 // skill, editor, theme, etc.
	TrustLevel  string     `gorm:"size:50;default:community" json:"trust_level"` // core, domain, community
	Category    string     `gorm:"size:100" json:"category"`
	Tags        string     `gorm:"type:text" json:"tags"` // JSON array stored as text
	IconURL     string     `gorm:"size:500" json:"icon_url,omitempty"`
	RepoURL     string     `gorm:"size:500" json:"repo_url,omitempty"`
	PackageURL  string     `gorm:"size:500" json:"package_url,omitempty"`
	Downloads   int        `gorm:"default:0" json:"downloads"`
	Rating      float64    `gorm:"default:0" json:"rating"`
	RatingCount int        `gorm:"default:0" json:"rating_count"`
	Status      string     `gorm:"size:50;default:pending" json:"status"` // pending, approved, rejected, deprecated
	ReviewedBy  *uint      `json:"reviewed_by,omitempty"`
	ReviewedAt  *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// MarketplaceReview is a user review/rating for a marketplace plugin.
type MarketplaceReview struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	PluginID  string    `gorm:"index;size:100;not null" json:"plugin_id"`
	UserID    uint      `gorm:"index" json:"user_id"`
	Rating    int       `gorm:"not null" json:"rating"` // 1-5
	Comment   string    `gorm:"type:text" json:"comment,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// InstalledPlugin tracks which plugins are installed per school.
type InstalledPlugin struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	SchoolID  uint      `gorm:"index;not null" json:"school_id"`
	PluginID  string    `gorm:"index;size:100;not null" json:"plugin_id"`
	Version   string    `gorm:"size:50" json:"version"`
	Config    string    `gorm:"type:text" json:"config,omitempty"` // JSON config overrides
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
