package model

// SystemConfig 系统级配置表，用于存储全局配置如 AI 服务商密钥等
type SystemConfig struct {
	Key   string `gorm:"primaryKey;size:100" json:"key"`
	Value string `gorm:"type:text" json:"value"`
}
