package llm

import (
	"context"
	"sync"
	"log/slog"

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
	activeClients    map[string]LLMProvider
}

// NewDynamicProvider creates a new DynamicProvider.
func NewDynamicProvider(db *gorm.DB, fallback LLMProvider) *DynamicProvider {
	return &DynamicProvider{
		DB:               db,
		FallbackProvider: fallback,
		activeClients:    make(map[string]LLMProvider),
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

// getActiveProvider returns the currently selected LLM provider based on config.
func (p *DynamicProvider) getActiveProvider() LLMProvider {
	configs := p.loadConfigs()
	if configs == nil {
		return p.FallbackProvider
	}

	providerType := configs["LLM_PROVIDER"]
	if providerType == "" {
		return p.FallbackProvider
	}

	p.mu.RLock()
	client, exists := p.activeClients[providerType]
	p.mu.RUnlock()

	// 简单的配置检查，如果没有缓存则创建
	if !exists {
		p.mu.Lock()
		defer p.mu.Unlock()
		
		// 双重检查
		if client, exists = p.activeClients[providerType]; exists {
			return client
		}

		switch providerType {
		case "dashscope":
			apiKey := configs["DASHSCOPE_API_KEY"]
			chatModel := configs["DASHSCOPE_MODEL"]
			if chatModel == "" {
				chatModel = "qwen-max"
			}
			client = NewDashScopeClient(DashScopeConfig{
				APIKey:         apiKey,
				ChatModel:      chatModel,
				EmbeddingModel: "text-embedding-v3", // 暂定写死或加配置
			})
			slogDynamic.Info("initialized dashscope client dynamically")
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
		p.activeClients[providerType] = client
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
		
		switch opts.ProviderOverride {
		case "dashscope":
			apiKey := configs["DASHSCOPE_API_KEY"]
			chatModel := opts.ModelOverride
			if chatModel == "" {
				chatModel = configs["DASHSCOPE_MODEL"]
				if chatModel == "" {
					chatModel = "qwen-max"
				}
			}
			slogDynamic.Info("using overridden dashscope provider", "model", chatModel)
			return NewDashScopeClient(DashScopeConfig{
				APIKey:         apiKey,
				ChatModel:      chatModel,
				EmbeddingModel: "text-embedding-v3",
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
			slogDynamic.Info("using overridden ollama provider", "model", chatModel)
			return NewOllamaClient(host, chatModel, "bge-m3")
		}
	}
	return p.getActiveProvider()
}


func (p *DynamicProvider) Embed(ctx context.Context, text string) ([]float64, error) {
	provider := p.getActiveProvider()
	return provider.Embed(ctx, text)
}

func (p *DynamicProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	provider := p.getActiveProvider()
	return provider.EmbedBatch(ctx, texts)
}

// ClearCache forces the provider to re-initialize clients on the next call.
// This should be called when settings are updated.
func (p *DynamicProvider) ClearCache() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.activeClients = make(map[string]LLMProvider)
}
