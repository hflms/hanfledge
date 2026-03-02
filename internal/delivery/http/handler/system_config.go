package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/llm"
	"gorm.io/gorm"
)

// SystemConfigHandler handles system configuration APIs.
type SystemConfigHandler struct {
	DB          *gorm.DB
	LLMProvider llm.LLMProvider
}

// NewSystemConfigHandler creates a new SystemConfigHandler.
func NewSystemConfigHandler(db *gorm.DB, llmProvider llm.LLMProvider) *SystemConfigHandler {
	return &SystemConfigHandler{DB: db, LLMProvider: llmProvider}
}

// GetConfigs returns all system configurations.
func (h *SystemConfigHandler) GetConfigs(c *gin.Context) {
	var configs []model.SystemConfig
	if err := h.DB.Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取配置失败", "details": err.Error()})
		return
	}

	configMap := make(map[string]string)
	for _, cfg := range configs {
		configMap[cfg.Key] = cfg.Value
	}
	c.JSON(http.StatusOK, configMap)
}

// UpdateConfigs updates multiple system configurations.
func (h *SystemConfigHandler) UpdateConfigs(c *gin.Context) {
	var input map[string]string
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		for k, v := range input {
			if err := tx.Save(&model.SystemConfig{Key: k, Value: v}).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新配置失败", "details": err.Error()})
		return
	}

	// 如果传入的是 DynamicProvider，则清除缓存
	if dp, ok := h.LLMProvider.(*llm.DynamicProvider); ok {
		dp.ClearCache()
	}

	c.JSON(http.StatusOK, gin.H{"message": "配置更新成功"})
}
