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

// providerMeta maps provider name → DB key names and default values.
var providerMeta = map[string]struct {
	apiKeyField  string
	modelField   string
	baseURLField string
	defaultModel string
	defaultBase  string
}{
	"dashscope":  {"DASHSCOPE_API_KEY", "DASHSCOPE_MODEL", "DASHSCOPE_COMPAT_BASE_URL", "qwen-max", "https://dashscope.aliyuncs.com/compatible-mode/v1"},
	"doubao":     {"DOUBAO_API_KEY", "DOUBAO_MODEL", "DOUBAO_COMPAT_BASE_URL", "ep-xxx", "https://ark.cn-beijing.volces.com/api/v3"},
	"deepseek":   {"DEEPSEEK_API_KEY", "DEEPSEEK_MODEL", "DEEPSEEK_COMPAT_BASE_URL", "deepseek-chat", "https://api.deepseek.com/v1"},
	"openrouter": {"OPENROUTER_API_KEY", "OPENROUTER_MODEL", "OPENROUTER_COMPAT_BASE_URL", "openai/gpt-4o-mini", "https://openrouter.ai/api/v1"},
	"moonshot":   {"MOONSHOT_API_KEY", "MOONSHOT_MODEL", "MOONSHOT_COMPAT_BASE_URL", "moonshot-v1-8k", "https://api.moonshot.cn/v1"},
	"zhipu":      {"ZHIPU_API_KEY", "ZHIPU_MODEL", "ZHIPU_COMPAT_BASE_URL", "glm-4", "https://open.bigmodel.cn/api/paas/v4"},
}

// buildProviderForTest creates a temporary LLM client for connectivity testing.
// apiKeyOverride and baseURLOverride, when non-empty, take precedence over DB-stored values.
func (h *SystemConfigHandler) buildProviderForTest(providerType, modelOverride, mode, apiKeyOverride, baseURLOverride string) llm.LLMProvider {
	configs := h.loadConfigMap()
	if configs == nil {
		configs = make(map[string]string)
	}

	// Ollama uses a different client implementation.
	if providerType == "ollama" {
		host := baseURLOverride
		if host == "" {
			host = configs["OLLAMA_BASE_URL"]
		}
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
	}

	// All other providers use the OpenAI-compatible DashScopeClient.
	meta, ok := providerMeta[providerType]
	if !ok {
		slog.Warn("buildProviderForTest: unknown provider, falling back", "provider", providerType)
		return h.LLMProvider
	}

	apiKey := apiKeyOverride
	if apiKey == "" {
		apiKey = configs[meta.apiKeyField]
	}
	compatURL := baseURLOverride
	if compatURL == "" {
		compatURL = configs[meta.baseURLField]
	}
	if compatURL == "" {
		compatURL = meta.defaultBase
	}

	chatModel := configs[meta.modelField]
	if chatModel == "" {
		chatModel = meta.defaultModel
	}
	embModel := configs["EMBEDDING_MODEL"]

	if mode == "chat" && modelOverride != "" {
		chatModel = modelOverride
	}
	if mode == "embed" && modelOverride != "" {
		embModel = modelOverride
	}

	slog.Info("buildProviderForTest", "provider", providerType, "chat_model", chatModel, "compat_url", compatURL)
	return llm.NewDashScopeClient(llm.DashScopeConfig{
		APIKey:         apiKey,
		ChatModel:      chatModel,
		EmbeddingModel: embModel,
		CompatBaseURL:  compatURL,
	})
}

// TestChatModel verifies whether a chat model can respond via selected provider.
func (h *SystemConfigHandler) TestChatModel(c *gin.Context) {
	var input struct {
		Provider string `json:"provider"`
		Model    string `json:"model"`
		APIKey   string `json:"apiKey"`
		BaseURL  string `json:"baseUrl"`
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
	client := h.buildProviderForTest(input.Provider, input.Model, "chat", input.APIKey, input.BaseURL)
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
		APIKey   string `json:"apiKey"`
		BaseURL  string `json:"baseUrl"`
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
	client := h.buildProviderForTest(input.Provider, input.Model, "embed", input.APIKey, input.BaseURL)
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
