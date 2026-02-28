package model

import "time"

// ============================
// 成就系统模型 — design.md §5.2 Step 4
// ============================
//
// 三种成就类型: 连续突破徽章、深度追问勋章、谬误猎人。
// 强调纵向自我成长对比，不设学生间排行榜。

// AchievementType 成就类型枚举。
type AchievementType string

const (
	AchievementStreakBreaker AchievementType = "streak_breaker" // 连续突破徽章
	AchievementDeepInquiry   AchievementType = "deep_inquiry"   // 深度追问勋章
	AchievementFallacyHunt   AchievementType = "fallacy_hunter" // 谬误猎人
)

// AchievementTier 成就等级枚举。
type AchievementTier string

const (
	TierBronze  AchievementTier = "bronze"
	TierSilver  AchievementTier = "silver"
	TierGold    AchievementTier = "gold"
	TierDiamond AchievementTier = "diamond"
)

// AchievementDefinition 成就定义表。
// 系统预置的成就类型及其各等级解锁条件。
type AchievementDefinition struct {
	ID          uint            `gorm:"primaryKey" json:"id"`
	Type        AchievementType `gorm:"size:30;not null;uniqueIndex:idx_achievement_def" json:"type"`
	Tier        AchievementTier `gorm:"size:20;not null;uniqueIndex:idx_achievement_def" json:"tier"`
	Name        string          `gorm:"size:50;not null" json:"name"`
	Description string          `gorm:"size:200;not null" json:"description"`
	Icon        string          `gorm:"size:10;not null" json:"icon"` // Emoji icon
	Threshold   int             `gorm:"not null" json:"threshold"`    // 解锁所需数值
	SortOrder   int             `gorm:"not null;default:0" json:"sort_order"`
}

// StudentAchievement 学生成就记录表。
// 记录学生何时解锁了何种成就。
type StudentAchievement struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	StudentID     uint       `gorm:"not null;uniqueIndex:idx_student_achievement" json:"student_id"`
	AchievementID uint       `gorm:"not null;uniqueIndex:idx_student_achievement" json:"achievement_id"`
	Progress      int        `gorm:"not null;default:0" json:"progress"` // 当前进度值
	Unlocked      bool       `gorm:"not null;default:false" json:"unlocked"`
	UnlockedAt    *time.Time `json:"unlocked_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`

	// 关联
	Student     User                  `gorm:"foreignKey:StudentID" json:"-"`
	Achievement AchievementDefinition `gorm:"foreignKey:AchievementID" json:"achievement,omitempty"`
}
