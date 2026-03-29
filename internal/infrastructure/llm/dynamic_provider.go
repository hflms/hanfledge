package llm

import (
	"context"
	"log/slog"
	"sync"

	"github.com/hflms/hanfledge/internal/domain/model"
	"gorm.io/gorm"
)

var slogDynamic = slog.With("module", "llm-dynamic")

// Compile-time check
var _ LLMProvider = (*DynamicProvider)(nil)

// DynamicProvider implements LLMProvider but dynamically routes
// to different underlying clients based on DB configuration.
type DynamicProvider struct {
	DB               *gorm.DB
	FallbackProvider LLMProvider // 初始启动时的后备配置
	mu               sync.RWMutex
	chatClients      map[string]LLMProvider
	embedClients     map[string]LLMProvider
}

// NewDynamicProvider creates a new DynamicProvider.
func NewDynamicProvider(db *gorm.DB, fallback LLMProvider) *DynamicProvider {
	return &DynamicProvider{
		DB:               db,
		FallbackProvider: fallback,
		chatClients:      make(map[string]LLMProvider),
		embedClients:     make(map[string]LLMProvider),
	}
}

// loadConfigs fetches all system configs from DB.
func (p *DynamicProvider) loadConfigs() map[string]string {
	var configs []model.SystemConfig
	if err := p.DB.Find(&configs).Error; err != nil {
		slogDynamic.Warn("failed to load system configs", "err", err)
		return nil
	}
	m := make(map[string]string)
	for _, c := range configs {
		m[c.Key] = c.Value
	}
	return m
}

// getChatProvider returns the currently selected chat provider based on config.
func (p *DynamicProvider) getChatProvider() LLMProvider {
	configs := p.loadConfigs()
	if configs == nil {
		return p.FallbackProvider
	}

	providerType := configs["LLM_PROVIDER"]
	if providerType == "" {
		return p.FallbackProvider
	}

	p.mu.RLock()
	client, exists := p.chatClients[providerType]
	p.mu.RUnlock()

	// 简单的配置检查，如果没有缓存则创建
	if !exists {
		p.mu.Lock()
		defer p.mu.Unlock()

		// 双重检查
		if client, exists = p.chatClients[providerType]; exists {
			return client
		}

		switch providerType {
		case "dashscope":
			apiKey := configs["DASHSCOPE_API_KEY"]
			chatModel := configs["DASHSCOPE_MODEL"]
			compatURL := configs["DASHSCOPE_COMPAT_BASE_URL"]
			if chatModel == "" {
				chatModel = "qwen-max"
			}
			embModel := configs["EMBEDDING_MODEL"]
			if embModel == "" {
				embModel = "text-embedding-v3"
			}
			client = NewDashScopeClient(DashScopeConfig{
				APIKey:         apiKey,
				ChatModel:      chatModel,
				EmbeddingModel: embModel,
				CompatBaseURL:  compatURL,
			})
			actualURL := compatURL
			if actualURL == "" {
				actualURL = "(default) https://dashscope.aliyuncs.com/compatible-mode/v1"
			}
			slogDynamic.Info("initialized dashscope client dynamically", "chat_model", chatModel, "compat_url", actualURL)
		case "doubao":
			apiKey := configs["DOUBAO_API_KEY"]
			chatModel := configs["DOUBAO_MODEL"]
			compatURL := configs["DOUBAO_COMPAT_BASE_URL"]
			if compatURL == "" {
				compatURL = "https://ark.cn-beijing.volces.com/api/v3"
			}
			embModel := configs["EMBEDDING_MODEL"]
			client = NewDashScopeClient(DashScopeConfig{
				APIKey:         apiKey,
				ChatModel:      chatModel,
				EmbeddingModel: embModel,
				CompatBaseURL:  compatURL,
			})
			slogDynamic.Info("initialized doubao client dynamically", "chat_model", chatModel, "compat_url", compatURL)
		case "deepseek":
			apiKey := configs["DEEPSEEK_API_KEY"]
			chatModel := configs["DEEPSEEK_MODEL"]
			compatURL := configs["DEEPSEEK_COMPAT_BASE_URL"]
			if chatModel == "" {
				chatModel = "deepseek-chat"
			}
			if compatURL == "" {
				compatURL = "https://api.deepseek.com/v1"
			}
			embModel := configs["EMBEDDING_MODEL"]
			client = NewDashScopeClient(DashScopeConfig{
				APIKey:         apiKey,
				ChatModel:      chatModel,
				EmbeddingModel: embModel,
				CompatBaseURL:  compatURL,
			})
			slogDynamic.Info("initialized deepseek client dynamically", "chat_model", chatModel, "compat_url", compatURL)
		case "openrouter":
			apiKey := configs["OPENROUTER_API_KEY"]
			chatModel := configs["OPENROUTER_MODEL"]
			compatURL := configs["OPENROUTER_COMPAT_BASE_URL"]
			if compatURL == "" {
				compatURL = "https://openrouter.ai/api/v1"
			}
			embModel := configs["EMBEDDING_MODEL"]
			client = NewDashScopeClient(DashScopeConfig{
				APIKey:         apiKey,
				ChatModel:      chatModel,
				EmbeddingModel: embModel,
				CompatBaseURL:  compatURL,
			})
			slogDynamic.Info("initialized openrouter client dynamically", "chat_model", chatModel, "compat_url", compatURL)
		case "moonshot":
			apiKey := configs["MOONSHOT_API_KEY"]
			chatModel := configs["MOONSHOT_MODEL"]
			compatURL := configs["MOONSHOT_COMPAT_BASE_URL"]
			if chatModel == "" {
				chatModel = "moonshot-v1-8k"
			}
			if compatURL == "" {
				compatURL = "https://api.moonshot.cn/v1"
			}
			embModel := configs["EMBEDDING_MODEL"]
			client = NewDashScopeClient(DashScopeConfig{
				APIKey:         apiKey,
				ChatModel:      chatModel,
				EmbeddingModel: embModel,
				CompatBaseURL:  compatURL,
			})
			slogDynamic.Info("initialized moonshot client dynamically", "chat_model", chatModel, "compat_url", compatURL)
		case "zhipu":
			apiKey := configs["ZHIPU_API_KEY"]
			chatModel := configs["ZHIPU_MODEL"]
			compatURL := configs["ZHIPU_COMPAT_BASE_URL"]
			if chatModel == "" {
				chatModel = "glm-4"
			}
			if compatURL == "" {
				compatURL = "https://open.bigmodel.cn/api/paas/v4"
			}
			embModel := configs["EMBEDDING_MODEL"]
			client = NewDashScopeClient(DashScopeConfig{
				APIKey:         apiKey,
				ChatModel:      chatModel,
				EmbeddingModel: embModel,
				CompatBaseURL:  compatURL,
			})
			slogDynamic.Info("initialized zhipu client dynamically", "chat_model", chatModel, "compat_url", compatURL)
		case "ollama":
			host := configs["OLLAMA_BASE_URL"]
			if host == "" {
				host = "http://localhost:11434"
			}
			chatModel := configs["OLLAMA_MODEL"]
			if chatModel == "" {
				chatModel = "qwen2.5:7b"
			}
			embedModel := configs["EMBEDDING_MODEL"]
			if embedModel == "" {
				embedModel = "bge-m3"
			}
			client = NewOllamaClient(host, chatModel, embedModel)
			slogDynamic.Info("initialized ollama client dynamically")
		default:
			client = p.FallbackProvider
		}
		p.chatClients[providerType] = client
	}

	return client
}

// getEmbeddingProvider returns the embedding provider based on config.
func (p *DynamicProvider) getEmbeddingProvider() LLMProvider {
	configs := p.loadConfigs()
	if configs == nil {
		return p.FallbackProvider
	}

	providerType := configs["EMBEDDING_PROVIDER"]
	if providerType == "" {
		providerType = configs["LLM_PROVIDER"]
	}
	if providerType == "" {
		return p.FallbackProvider
	}

	p.mu.RLock()
	client, exists := p.embedClients[providerType]
	p.mu.RUnlock()

	if !exists {
		p.mu.Lock()
		defer p.mu.Unlock()

		if client, exists = p.embedClients[providerType]; exists {
			return client
		}

		switch providerType {
		case "dashscope":
			apiKey := configs["DASHSCOPE_API_KEY"]
			compatURL := configs["DASHSCOPE_COMPAT_BASE_URL"]
			embModel := configs["EMBEDDING_MODEL"]
			if embModel == "" {
				embModel = "text-embedding-v3"
			}
			client = NewDashScopeClient(DashScopeConfig{
				APIKey:         apiKey,
				ChatModel:      configs["DASHSCOPE_MODEL"],
				EmbeddingModel: embModel,
				CompatBaseURL:  compatURL,
			})
			slogDynamic.Info("initialized dashscope embedding client dynamically", "model", embModel)
		case "doubao":
			apiKey := configs["DOUBAO_API_KEY"]
			compatURL := configs["DOUBAO_COMPAT_BASE_URL"]
			if compatURL == "" {
				compatURL = "https://ark.cn-beijing.volces.com/api/v3"
			}
			embModel := configs["EMBEDDING_MODEL"]
			client = NewDashScopeClient(DashScopeConfig{
				APIKey:         apiKey,
				ChatModel:      configs["DOUBAO_MODEL"],
				EmbeddingModel: embModel,
				CompatBaseURL:  compatURL,
			})
			slogDynamic.Info("initialized doubao embedding client dynamically", "model", embModel)
		case "deepseek":
			apiKey := configs["DEEPSEEK_API_KEY"]
			compatURL := configs["DEEPSEEK_COMPAT_BASE_URL"]
			if compatURL == "" {
				compatURL = "https://api.deepseek.com/v1"
			}
			embModel := configs["EMBEDDING_MODEL"]
			client = NewDashScopeClient(DashScopeConfig{
				APIKey:         apiKey,
				ChatModel:      configs["DEEPSEEK_MODEL"],
				EmbeddingModel: embModel,
				CompatBaseURL:  compatURL,
			})
			slogDynamic.Info("initialized deepseek embedding client dynamically", "model", embModel)
		case "openrouter":
			apiKey := configs["OPENROUTER_API_KEY"]
			compatURL := configs["OPENROUTER_COMPAT_BASE_URL"]
			if compatURL == "" {
				compatURL = "https://openrouter.ai/api/v1"
			}
			embModel := configs["EMBEDDING_MODEL"]
			client = NewDashScopeClient(DashScopeConfig{
				APIKey:         apiKey,
				ChatModel:      configs["OPENROUTER_MODEL"],
				EmbeddingModel: embModel,
				CompatBaseURL:  compatURL,
			})
			slogDynamic.Info("initialized openrouter embedding client dynamically", "model", embModel)
		case "moonshot":
			apiKey := configs["MOONSHOT_API_KEY"]
			compatURL := configs["MOONSHOT_COMPAT_BASE_URL"]
			if compatURL == "" {
				compatURL = "https://api.moonshot.cn/v1"
			}
			embModel := configs["EMBEDDING_MODEL"]
			client = NewDashScopeClient(DashScopeConfig{
				APIKey:         apiKey,
				ChatModel:      configs["MOONSHOT_MODEL"],
				EmbeddingModel: embModel,
				CompatBaseURL:  compatURL,
			})
			slogDynamic.Info("initialized moonshot embedding client dynamically", "model", embModel)
		case "zhipu":
			apiKey := configs["ZHIPU_API_KEY"]
			compatURL := configs["ZHIPU_COMPAT_BASE_URL"]
			if compatURL == "" {
				compatURL = "https://open.bigmodel.cn/api/paas/v4"
			}
			embModel := configs["EMBEDDING_MODEL"]
			client = NewDashScopeClient(DashScopeConfig{
				APIKey:         apiKey,
				ChatModel:      configs["ZHIPU_MODEL"],
				EmbeddingModel: embModel,
				CompatBaseURL:  compatURL,
			})
			slogDynamic.Info("initialized zhipu embedding client dynamically", "model", embModel)
		case "ollama":
			host := configs["OLLAMA_BASE_URL"]
			if host == "" {
				host = "http://localhost:11434"
			}
			embedModel := configs["EMBEDDING_MODEL"]
			if embedModel == "" {
				embedModel = "bge-m3"
			}
			client = NewOllamaClient(host, configs["OLLAMA_MODEL"], embedModel)
			slogDynamic.Info("initialized ollama embedding client dynamically", "model", embedModel)
		default:
			client = p.FallbackProvider
		}
		p.embedClients[providerType] = client
	}

	return client
}

// -- LLMProvider Interface --

func (p *DynamicProvider) Name() string {
	return "dynamic"
}

func (p *DynamicProvider) Chat(ctx context.Context, messages []ChatMessage, opts *ChatOptions) (string, error) {
	provider := p.getProviderForRequest(opts)
	return provider.Chat(ctx, messages, opts)
}

func (p *DynamicProvider) StreamChat(ctx context.Context, messages []ChatMessage, opts *ChatOptions, onToken func(token string)) (string, error) {
	provider := p.getProviderForRequest(opts)
	return provider.StreamChat(ctx, messages, opts, onToken)
}

// getProviderForRequest checks if there's an override in opts, otherwise returns global active provider.
func (p *DynamicProvider) getProviderForRequest(opts *ChatOptions) LLMProvider {
	if opts != nil && opts.ProviderOverride != "" {
		// Temporary instance for this request
		configs := p.loadConfigs()
		if configs == nil {
			configs = make(map[string]string)
		}
		embModel := configs["EMBEDDING_MODEL"]

		switch opts.ProviderOverride {
		case "dashscope":
			apiKey := configs["DASHSCOPE_API_KEY"]
			chatModel := opts.ModelOverride
			compatURL := configs["DASHSCOPE_COMPAT_BASE_URL"]
			if chatModel == "" {
				chatModel = configs["DASHSCOPE_MODEL"]
				if chatModel == "" {
					chatModel = "qwen-max"
				}
			}
			if embModel == "" {
				embModel = "text-embedding-v3"
			}
			slogDynamic.Info("using overridden dashscope provider", "model", chatModel)
			return NewDashScopeClient(DashScopeConfig{
				APIKey:         apiKey,
				ChatModel:      chatModel,
				EmbeddingModel: embModel,
				CompatBaseURL:  compatURL,
			})
		case "doubao":
			apiKey := configs["DOUBAO_API_KEY"]
			chatModel := opts.ModelOverride
			compatURL := configs["DOUBAO_COMPAT_BASE_URL"]
			if chatModel == "" {
				chatModel = configs["DOUBAO_MODEL"]
			}
			if compatURL == "" {
				compatURL = "https://ark.cn-beijing.volces.com/api/v3"
			}
			slogDynamic.Info("using overridden doubao provider", "model", chatModel)
			return NewDashScopeClient(DashScopeConfig{
				APIKey:         apiKey,
				ChatModel:      chatModel,
				EmbeddingModel: embModel,
				CompatBaseURL:  compatURL,
			})
		case "deepseek":
			apiKey := configs["DEEPSEEK_API_KEY"]
			chatModel := opts.ModelOverride
			compatURL := configs["DEEPSEEK_COMPAT_BASE_URL"]
			if chatModel == "" {
				chatModel = configs["DEEPSEEK_MODEL"]
				if chatModel == "" {
					chatModel = "deepseek-chat"
				}
			}
			if compatURL == "" {
				compatURL = "https://api.deepseek.com/v1"
			}
			slogDynamic.Info("using overridden deepseek provider", "model", chatModel)
			return NewDashScopeClient(DashScopeConfig{
				APIKey:         apiKey,
				ChatModel:      chatModel,
				EmbeddingModel: embModel,
				CompatBaseURL:  compatURL,
			})
		case "openrouter":
			apiKey := configs["OPENROUTER_API_KEY"]
			chatModel := opts.ModelOverride
			compatURL := configs["OPENROUTER_COMPAT_BASE_URL"]
			if chatModel == "" {
				chatModel = configs["OPENROUTER_MODEL"]
			}
			if compatURL == "" {
				compatURL = "https://openrouter.ai/api/v1"
			}
			slogDynamic.Info("using overridden openrouter provider", "model", chatModel)
			return NewDashScopeClient(DashScopeConfig{
				APIKey:         apiKey,
				ChatModel:      chatModel,
				EmbeddingModel: embModel,
				CompatBaseURL:  compatURL,
			})
		case "moonshot":
			apiKey := configs["MOONSHOT_API_KEY"]
			chatModel := opts.ModelOverride
			compatURL := configs["MOONSHOT_COMPAT_BASE_URL"]
			if chatModel == "" {
				chatModel = configs["MOONSHOT_MODEL"]
				if chatModel == "" {
					chatModel = "moonshot-v1-8k"
				}
			}
			if compatURL == "" {
				compatURL = "https://api.moonshot.cn/v1"
			}
			slogDynamic.Info("using overridden moonshot provider", "model", chatModel)
			return NewDashScopeClient(DashScopeConfig{
				APIKey:         apiKey,
				ChatModel:      chatModel,
				EmbeddingModel: embModel,
				CompatBaseURL:  compatURL,
			})
		case "zhipu":
			apiKey := configs["ZHIPU_API_KEY"]
			chatModel := opts.ModelOverride
			compatURL := configs["ZHIPU_COMPAT_BASE_URL"]
			if chatModel == "" {
				chatModel = configs["ZHIPU_MODEL"]
				if chatModel == "" {
					chatModel = "glm-4"
				}
			}
			if compatURL == "" {
				compatURL = "https://open.bigmodel.cn/api/paas/v4"
			}
			slogDynamic.Info("using overridden zhipu provider", "model", chatModel)
			return NewDashScopeClient(DashScopeConfig{
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
			chatModel := opts.ModelOverride
			if chatModel == "" {
				chatModel = configs["OLLAMA_MODEL"]
				if chatModel == "" {
					chatModel = "qwen2.5:7b"
				}
			}
			if embModel == "" {
				embModel = "bge-m3"
			}
			slogDynamic.Info("using overridden ollama provider", "model", chatModel)
			return NewOllamaClient(host, chatModel, embModel)
		}
	}
	return p.getChatProvider()
}

func (p *DynamicProvider) Embed(ctx context.Context, text string) ([]float64, error) {
	provider := p.getEmbeddingProvider()
	return provider.Embed(ctx, text)
}

func (p *DynamicProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	provider := p.getEmbeddingProvider()
	return provider.EmbedBatch(ctx, texts)
}

// ClearCache forces the provider to re-initialize clients on the next call.
// This should be called when settings are updated.
func (p *DynamicProvider) ClearCache() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.chatClients = make(map[string]LLMProvider)
	p.embedClients = make(map[string]LLMProvider)
}
