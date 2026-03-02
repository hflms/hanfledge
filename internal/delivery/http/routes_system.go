package http

import (
	"github.com/gin-gonic/gin"
	"github.com/hflms/hanfledge/internal/delivery/http/handler"
	"github.com/hflms/hanfledge/internal/infrastructure/llm"
	"gorm.io/gorm"
)

// registerSystemRoutes registers system configuration routes.
func registerSystemRoutes(rg *gin.RouterGroup, db *gorm.DB, llmProvider llm.LLMProvider) {
	h := handler.NewSystemConfigHandler(db, llmProvider)

	systemGroup := rg.Group("/system")
	{
		systemGroup.GET("/config", h.GetConfigs)
		systemGroup.PUT("/config", h.UpdateConfigs)
		systemGroup.POST("/config/test-chat-model", h.TestChatModel)
		systemGroup.POST("/config/test-embedding-model", h.TestEmbeddingModel)
	}
}
