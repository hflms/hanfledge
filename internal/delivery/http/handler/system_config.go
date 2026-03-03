package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

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

func (h *SystemConfigHandler) loadConfigMap() map[string]string {
	var configs []model.SystemConfig
	if err := h.DB.Find(&configs).Error; err != nil {
		return nil
	}
	configMap := make(map[string]string)
	for _, cfg := range configs {
		configMap[cfg.Key] = cfg.Value
	}
	return configMap
}

func (h *SystemConfigHandler) buildProviderForTest(providerType string, modelOverride string, mode string) llm.LLMProvider {
	configs := h.loadConfigMap()
	if configs == nil {
		configs = make(map[string]string)
	}

	switch providerType {
	case "dashscope":
		apiKey := configs["DASHSCOPE_API_KEY"]
		chatModel := configs["DASHSCOPE_MODEL"]
		embModel := configs["EMBEDDING_MODEL"]
		compatURL := configs["DASHSCOPE_COMPAT_BASE_URL"]
		if chatModel == "" {
			chatModel = "qwen-max"
		}
		if embModel == "" {
			embModel = "text-embedding-v3"
		}
		if mode == "chat" && modelOverride != "" {
			chatModel = modelOverride
		}
		if mode == "embed" && modelOverride != "" {
			embModel = modelOverride
		}
		actualURL := compatURL
		if actualURL == "" {
			actualURL = "(default) https://dashscope.aliyuncs.com/compatible-mode/v1"
		}
		slog.Info("[DEBUG] buildProviderForTest dashscope", "chat_model", chatModel, "compat_url", actualURL)
		return llm.NewDashScopeClient(llm.DashScopeConfig{
			APIKey:         apiKey,
			ChatModel:      chatModel,
			EmbeddingModel: embModel,
			CompatBaseURL:  compatURL,
		})
	case "ollama":
		host := configs["OLLAMA_BASE_URL"]
		if host == "" {
			host = "http://localhost:11434"
		}
		chatModel := configs["OLLAMA_MODEL"]
		if chatModel == "" {
			chatModel = "qwen2.5:7b"
		}
		embModel := configs["EMBEDDING_MODEL"]
		if embModel == "" {
			embModel = "bge-m3"
		}
		if mode == "chat" && modelOverride != "" {
			chatModel = modelOverride
		}
		if mode == "embed" && modelOverride != "" {
			embModel = modelOverride
		}
		return llm.NewOllamaClient(host, chatModel, embModel)
	default:
		return h.LLMProvider
	}
}

// TestChatModel verifies whether a chat model can respond via selected provider.
func (h *SystemConfigHandler) TestChatModel(c *gin.Context) {
	var input struct {
		Provider string `json:"provider"`
		Model    string `json:"model"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}
	if input.Provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider 不能为空"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()

	start := time.Now()
	client := h.buildProviderForTest(input.Provider, input.Model, "chat")
	resp, err := client.Chat(ctx, []llm.ChatMessage{
		{Role: "system", Content: "你是一个简洁的 AI 助手。"},
		{Role: "user", Content: "请回复: ok"},
	}, &llm.ChatOptions{})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "模型可用",
		"provider": input.Provider,
		"model":    input.Model,
		"latency":  time.Since(start).Milliseconds(),
		"reply":    resp,
	})
}

// TestEmbeddingModel verifies whether an embedding model can respond via selected provider.
func (h *SystemConfigHandler) TestEmbeddingModel(c *gin.Context) {
	var input struct {
		Provider string `json:"provider"`
		Model    string `json:"model"`
		Text     string `json:"text"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}
	if input.Provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider 不能为空"})
		return
	}
	if input.Text == "" {
		input.Text = "test"
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()

	start := time.Now()
	client := h.buildProviderForTest(input.Provider, input.Model, "embed")
	vec, err := client.Embed(ctx, input.Text)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "模型可用",
		"provider":  input.Provider,
		"model":     input.Model,
		"latency":   time.Since(start).Milliseconds(),
		"dimension": len(vec),
	})
}
