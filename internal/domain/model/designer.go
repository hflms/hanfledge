package model

import "time"

// ============================
// 教学设计者模型
// ============================

// InterventionStyle 教学干预风格枚举。
type InterventionStyle string

const (
	StyleQuestioning  InterventionStyle = "questioning"  // 苏格拉底式追问
	StyleCoaching     InterventionStyle = "coaching"     // 教练式引导
	StyleDiagnostic   InterventionStyle = "diagnostic"   // 诊断式教学
	StyleFacilitation InterventionStyle = "facilitation" // 促进式引导
)

// InstructionalDesigner 教学设计者表。
type InstructionalDesigner struct {
	ID                string            `gorm:"primaryKey;size:100" json:"id"`
	Name              string            `gorm:"size:100;not null" json:"name"`
	Description       string            `gorm:"type:text" json:"description"`
	InterventionStyle InterventionStyle `gorm:"size:20;not null" json:"intervention_style"`
	IsBuiltIn         bool              `gorm:"default:false" json:"is_built_in"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}
