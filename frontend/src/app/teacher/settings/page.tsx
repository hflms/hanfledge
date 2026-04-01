'use client';

import React, { useState, useEffect, useCallback } from 'react';
import { apiFetch } from '@/lib/api';
import { useToast } from '@/components/Toast';
import { useModalA11y } from '@/lib/a11y';
import styles from './page.module.css';

// -- Constants ----------------------------------------

const providerOptions = [
  { value: 'ollama',     label: 'Ollama (本地私有化)' },
  { value: 'dashscope', label: 'DashScope (阿里云通义千问)' },
  { value: 'doubao',    label: 'Volcengine (火山引擎/豆包)' },
  { value: 'deepseek',  label: 'DeepSeek' },
  { value: 'openrouter',label: 'OpenRouter' },
  { value: 'moonshot',  label: 'Moonshot (月之暗面/Kimi)' },
  { value: 'zhipu',     label: 'Zhipu (智谱清言/GLM)' },
] as const;

const providerVisualMap: Record<string, { shortLabel: string; accent: string }> = {
  ollama:     { shortLabel: 'OL', accent: '#111827' },
  dashscope:  { shortLabel: 'DS', accent: '#7c3aed' },
  doubao:     { shortLabel: 'DB', accent: '#2563eb' },
  deepseek:   { shortLabel: 'DK', accent: '#0f766e' },
  openrouter: { shortLabel: 'OR', accent: '#ea580c' },
  moonshot:   { shortLabel: 'MS', accent: '#db2777' },
  zhipu:      { shortLabel: 'ZP', accent: '#16a34a' },
};

/** Maps provider → classic backend config key names. */
const providerKeyMap: Record<string, {
  apiKeyField?: string;
  modelField:   string;
  baseUrlField: string;
}> = {
  ollama:     { modelField: 'OLLAMA_MODEL',      baseUrlField: 'OLLAMA_BASE_URL' },
  dashscope:  { apiKeyField: 'DASHSCOPE_API_KEY',  modelField: 'DASHSCOPE_MODEL',  baseUrlField: 'DASHSCOPE_COMPAT_BASE_URL' },
  doubao:     { apiKeyField: 'DOUBAO_API_KEY',     modelField: 'DOUBAO_MODEL',     baseUrlField: 'DOUBAO_COMPAT_BASE_URL' },
  deepseek:   { apiKeyField: 'DEEPSEEK_API_KEY',   modelField: 'DEEPSEEK_MODEL',   baseUrlField: 'DEEPSEEK_COMPAT_BASE_URL' },
  openrouter: { apiKeyField: 'OPENROUTER_API_KEY', modelField: 'OPENROUTER_MODEL', baseUrlField: 'OPENROUTER_COMPAT_BASE_URL' },
  moonshot:   { apiKeyField: 'MOONSHOT_API_KEY',   modelField: 'MOONSHOT_MODEL',   baseUrlField: 'MOONSHOT_COMPAT_BASE_URL' },
  zhipu:      { apiKeyField: 'ZHIPU_API_KEY',      modelField: 'ZHIPU_MODEL',      baseUrlField: 'ZHIPU_COMPAT_BASE_URL' },
};

const defaultBaseUrlMap: Record<string, string> = {
  ollama:     'http://localhost:11434',
  dashscope:  'https://dashscope.aliyuncs.com/compatible-mode/v1',
  doubao:     'https://ark.cn-beijing.volces.com/api/v3',
  deepseek:   'https://api.deepseek.com/v1',
  openrouter: 'https://openrouter.ai/api/v1',
  moonshot:   'https://api.moonshot.cn/v1',
  zhipu:      'https://open.bigmodel.cn/api/paas/v4',
};

const modelPlaceholderMap: Record<string, string> = {
  ollama:     'qwen2.5:7b',
  dashscope:  'qwen-max',
  doubao:     'ep-xxx',
  deepseek:   'deepseek-chat',
  openrouter: 'anthropic/claude-3-haiku',
  moonshot:   'moonshot-v1-8k',
  zhipu:      'glm-4',
};

// -- Interfaces ----------------------------------------

interface ChatModelCard {
  id:        string;
  provider:  string;
  model:     string;
  apiKey:    string;
  baseUrl:   string;
  isDefault: boolean;
}

interface EmbeddingModelCard {
  id:       string;
  provider: string;
  model:    string;
  apiKey:   string;
  baseUrl:  string;
}

type ModelModalState =
  | {
      type:      'chat';
      mode:      'add' | 'edit';
      id?:       string;
      provider:  string;
      model:     string;
      apiKey:    string;
      baseUrl:   string;
      isDefault: boolean;
    }
  | {
      type:     'embedding';
      mode:     'add' | 'edit';
      id?:      string;
      provider: string;
      model:    string;
      apiKey:   string;
      baseUrl:  string;
    };

// -- Helpers ----------------------------------------

function getProviderLabel(provider: string) {
  return providerOptions.find(o => o.value === provider)?.label ?? provider;
}

function getProviderVisual(provider: string) {
  return providerVisualMap[provider] ?? { shortLabel: provider.slice(0, 2).toUpperCase(), accent: '#4f46e5' };
}

function createModelId() {
  return `model-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

function parseChatModels(
  rawValue: string | undefined,
  allConfigs: Record<string, string>,
): ChatModelCard[] {
  if (rawValue) {
    try {
      const parsed = JSON.parse(rawValue) as Array<Partial<ChatModelCard>>;
      const normalized = parsed
        .filter(item => item && typeof item.model === 'string' && item.model.trim())
        .map(item => ({
          id:        typeof item.id === 'string' && item.id ? item.id : createModelId(),
          provider:  typeof item.provider === 'string' && item.provider ? item.provider : 'ollama',
          model:     item.model!.trim(),
          apiKey:    typeof item.apiKey === 'string' ? item.apiKey : '',
          baseUrl:   typeof item.baseUrl === 'string' ? item.baseUrl : '',
          isDefault: item.isDefault === true,
        }));
      if (normalized.length > 0) {
        if (!normalized.some(m => m.isDefault)) normalized[0].isDefault = true;
        return normalized;
      }
    } catch { /* fall through */ }
  }

  // Backward compat: build from old LLM_MODELS list + per-provider keys
  const provider = allConfigs.LLM_PROVIDER || 'ollama';
  const keys = providerKeyMap[provider];
  const apiKey = keys?.apiKeyField ? (allConfigs[keys.apiKeyField] ?? '') : '';
  const baseUrl = keys?.baseUrlField ? (allConfigs[keys.baseUrlField] ?? '') : '';
  const names = (allConfigs.LLM_MODELS ?? '').split(',').map(v => v.trim()).filter(Boolean);
  if (names.length === 0) return [];
  return names.map((model, idx) => ({
    id: createModelId(), provider, model, apiKey, baseUrl, isDefault: idx === 0,
  }));
}

function parseEmbeddingModels(
  rawValue: string | undefined,
  allConfigs: Record<string, string>,
): { models: EmbeddingModelCard[]; activeId: string } {
  const fallbackProvider = allConfigs.EMBEDDING_PROVIDER || allConfigs.LLM_PROVIDER || 'ollama';
  const fallbackModel    = allConfigs.EMBEDDING_MODEL ?? '';
  const fKeys            = providerKeyMap[fallbackProvider];
  const fallbackApiKey   = fKeys?.apiKeyField  ? (allConfigs[fKeys.apiKeyField]  ?? '') : '';
  const fallbackBaseUrl  = fKeys?.baseUrlField ? (allConfigs[fKeys.baseUrlField] ?? '') : '';

  let models: EmbeddingModelCard[] = [];

  if (rawValue) {
    try {
      const parsed = JSON.parse(rawValue) as Array<Partial<EmbeddingModelCard>>;
      const normalized = parsed
        .filter(item => item && typeof item.model === 'string' && item.model.trim())
        .map(item => ({
          id:       typeof item.id === 'string' && item.id ? item.id : createModelId(),
          provider: typeof item.provider === 'string' && item.provider ? item.provider : fallbackProvider,
          model:    item.model!.trim(),
          apiKey:   typeof item.apiKey === 'string' ? item.apiKey : '',
          baseUrl:  typeof item.baseUrl === 'string' ? item.baseUrl : '',
        }));
      if (normalized.length > 0) models = normalized;
    } catch { /* fall through */ }
  }

  if (models.length === 0 && fallbackModel.trim()) {
    models = [{ id: createModelId(), provider: fallbackProvider, model: fallbackModel.trim(), apiKey: fallbackApiKey, baseUrl: fallbackBaseUrl }];
  }

  const active = models.find(m => m.provider === fallbackProvider && m.model === fallbackModel);
  return { models, activeId: active?.id ?? models[0]?.id ?? '' };
}

// -- Component ----------------------------------------

export default function SystemSettingsPage() {
  const { toast } = useToast();

  const [loading,          setLoading]          = useState(true);
  const [saving,           setSaving]           = useState(false);
  const [chatModels,       setChatModels]       = useState<ChatModelCard[]>([]);
  const [embeddingModels,  setEmbeddingModels]  = useState<EmbeddingModelCard[]>([]);
  const [activeEmbeddingId,setActiveEmbeddingId]= useState<string>('');
  const [modalTestLoading, setModalTestLoading] = useState(false);
  const [modalTestResult,  setModalTestResult]  = useState<string | null>(null);
  const [modelModal,       setModelModal]       = useState<ModelModalState | null>(null);

  const closeModelModal = useCallback(() => {
    setModelModal(null);
    setModalTestLoading(false);
    setModalTestResult(null);
  }, []);

  const modelModalRef = useModalA11y(!!modelModal, closeModelModal);

  const isActiveEmbedding = useCallback(
    (item: EmbeddingModelCard) => item.id === activeEmbeddingId,
    [activeEmbeddingId],
  );

  const displayEmbeddingModels = [
    ...embeddingModels.filter(m => m.id === activeEmbeddingId),
    ...embeddingModels.filter(m => m.id !== activeEmbeddingId),
  ];

  // -- Load ----------------------------------------

  const loadConfigs = useCallback(async () => {
    try {
      const data = await apiFetch<Record<string, string>>('/system/config');
      if (data && Object.keys(data).length > 0) {
        setChatModels(parseChatModels(data.CHAT_MODELS, data));
        const { models, activeId } = parseEmbeddingModels(data.EMBEDDING_MODELS, data);
        setEmbeddingModels(models);
        setActiveEmbeddingId(activeId);
      }
    } catch (error) {
      console.error('Failed to load configs:', error);
      toast('加载配置失败', 'error');
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => { loadConfigs(); }, [loadConfigs]);

  // -- Save ----------------------------------------

  const handleSave = async () => {
    setSaving(true);
    try {
      const defaultChat      = chatModels.find(m => m.isDefault) ?? chatModels[0];
      const activeEmbedding  = embeddingModels.find(m => m.id === activeEmbeddingId) ?? embeddingModels[0];

      const payload: Record<string, string> = {
        CHAT_MODELS:      JSON.stringify(chatModels),
        LLM_MODELS:       chatModels.map(m => m.model).join(','),
        EMBEDDING_MODELS: JSON.stringify(embeddingModels),
      };

      // Write default chat model → classic backend keys
      if (defaultChat) {
        const k = providerKeyMap[defaultChat.provider];
        payload.LLM_PROVIDER = defaultChat.provider;
        if (k?.modelField)                         payload[k.modelField]   = defaultChat.model;
        if (k?.apiKeyField && defaultChat.apiKey)  payload[k.apiKeyField]  = defaultChat.apiKey;
        payload[k.baseUrlField] = defaultChat.baseUrl || defaultBaseUrlMap[defaultChat.provider] || '';
      }

      // Write other chat models (non-overwriting, so default model wins)
      for (const cm of chatModels) {
        if (cm.id === defaultChat?.id) continue;
        const k = providerKeyMap[cm.provider];
        if (k?.apiKeyField && cm.apiKey  && !payload[k.apiKeyField])  payload[k.apiKeyField]  = cm.apiKey;
        if (k?.baseUrlField && cm.baseUrl && !payload[k.baseUrlField]) payload[k.baseUrlField] = cm.baseUrl;
      }

      // Write active embedding → classic backend keys
      if (activeEmbedding) {
        payload.EMBEDDING_PROVIDER = activeEmbedding.provider;
        payload.EMBEDDING_MODEL    = activeEmbedding.model;
        const k = providerKeyMap[activeEmbedding.provider];
        if (k?.apiKeyField  && activeEmbedding.apiKey  && !payload[k.apiKeyField])  payload[k.apiKeyField]  = activeEmbedding.apiKey;
        if (k?.baseUrlField && activeEmbedding.baseUrl && !payload[k.baseUrlField]) payload[k.baseUrlField] = activeEmbedding.baseUrl;
      }

      await apiFetch('/system/config', { method: 'PUT', body: JSON.stringify(payload) });
      toast('系统配置保存成功', 'success');

      // Validate active embedding silently
      if (activeEmbedding) {
        await runEmbeddingTest(activeEmbedding.provider, activeEmbedding.model, true);
      }
    } catch (error) {
      console.error('Failed to save configs:', error);
      toast(error instanceof Error ? error.message : '保存配置失败', 'error');
    } finally {
      setSaving(false);
    }
  };

  // -- Modal open handlers ----------------------------------------

  const openAddChatModelModal = () => {
    setModalTestResult(null);
    setModelModal({
      type: 'chat', mode: 'add',
      provider: 'ollama', model: '', apiKey: '', baseUrl: '',
      isDefault: chatModels.length === 0,
    });
  };

  const openEditChatModelModal = (item: ChatModelCard) => {
    setModalTestResult(null);
    setModelModal({
      type: 'chat', mode: 'edit',
      id: item.id, provider: item.provider, model: item.model,
      apiKey: item.apiKey, baseUrl: item.baseUrl, isDefault: item.isDefault,
    });
  };

  const openAddEmbeddingModal = () => {
    setModalTestResult(null);
    setModelModal({
      type: 'embedding', mode: 'add',
      provider: embeddingModels[0]?.provider ?? 'ollama',
      model: '', apiKey: '', baseUrl: '',
    });
  };

  const openEditEmbeddingModal = (item: EmbeddingModelCard) => {
    setModalTestResult(null);
    setModelModal({
      type: 'embedding', mode: 'edit',
      id: item.id, provider: item.provider, model: item.model,
      apiKey: item.apiKey, baseUrl: item.baseUrl,
    });
  };

  const handleDuplicateChatModel = (item: ChatModelCard) => {
    setModalTestResult(null);
    setModelModal({
      type: 'chat', mode: 'add',
      provider: item.provider, model: `${item.model}-copy`,
      apiKey: item.apiKey, baseUrl: item.baseUrl, isDefault: false,
    });
    toast('已复制配置，请修改后保存', 'success');
  };

  const handleDuplicateEmbeddingModel = (item: EmbeddingModelCard) => {
    setModalTestResult(null);
    setModelModal({
      type: 'embedding', mode: 'add',
      provider: item.provider, model: `${item.model}-copy`,
      apiKey: item.apiKey, baseUrl: item.baseUrl,
    });
    toast('已复制配置，请修改后保存', 'success');
  };

  // -- Model mutations ----------------------------------------

  const handleRemoveChatModel = (id: string) => {
    setChatModels(prev => {
      const wasDefault = prev.find(m => m.id === id)?.isDefault ?? false;
      const next = prev.filter(m => m.id !== id);
      if (wasDefault && next.length > 0) next[0] = { ...next[0], isDefault: true };
      return next;
    });
  };

  const handleRemoveEmbeddingModel = (id: string) => {
    setEmbeddingModels(prev => prev.filter(m => m.id !== id));
    if (activeEmbeddingId === id) {
      const remaining = embeddingModels.filter(m => m.id !== id);
      setActiveEmbeddingId(remaining[0]?.id ?? '');
    }
  };

  const handleSetDefaultChatModel = (id: string) => {
    setChatModels(prev => prev.map(m => ({ ...m, isDefault: m.id === id })));
    toast('已更新默认对话模型', 'success');
  };

  const handleSetActiveEmbeddingModel = (item: EmbeddingModelCard) => {
    setActiveEmbeddingId(item.id);
    toast(`已将 ${item.model} 设为当前生效 Embedding 模型`, 'success');
  };

  // -- Modal field change ----------------------------------------

  const handleModalChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    const { name, value, type } = e.target;
    setModelModal(prev => {
      if (!prev) return prev;
      if (name === 'provider') {
        // Reset baseUrl when switching provider (defaults differ)
        return { ...prev, provider: value, baseUrl: '' };
      }
      if (name === 'isDefault' && type === 'checkbox') {
        const checked = (e.target as HTMLInputElement).checked;
        if (prev.type === 'chat') return { ...prev, isDefault: checked };
        return prev;
      }
      return { ...prev, [name]: value };
    });
  };

  // -- Modal save ----------------------------------------

  const handleSaveModelModal = () => {
    if (!modelModal) return;

    if (modelModal.type === 'chat') {
      const model = modelModal.model.trim();
      if (!model) { toast('请输入模型名称', 'error'); return; }

      if (modelModal.mode === 'add') {
        const created: ChatModelCard = {
          id: createModelId(),
          provider: modelModal.provider, model,
          apiKey: modelModal.apiKey, baseUrl: modelModal.baseUrl,
          isDefault: modelModal.isDefault,
        };
        setChatModels(prev => {
          const base = modelModal.isDefault ? prev.map(m => ({ ...m, isDefault: false })) : [...prev];
          return [...base, created];
        });
      } else if (modelModal.id) {
        setChatModels(prev => {
          let next = prev.map(m => (
            m.id === modelModal.id
              ? { ...m, provider: modelModal.provider, model, apiKey: modelModal.apiKey, baseUrl: modelModal.baseUrl, isDefault: modelModal.isDefault }
              : modelModal.isDefault ? { ...m, isDefault: false } : m
          ));
          // Ensure at least one default
          if (!next.some(m => m.isDefault) && next.length > 0) {
            next = next.map((m, i) => ({ ...m, isDefault: i === 0 }));
          }
          return next;
        });
      }
      closeModelModal();
      return;
    }

    // Embedding
    const model = modelModal.model.trim();
    if (!model) { toast('请输入 Embedding 模型名称', 'error'); return; }

    if (modelModal.mode === 'add') {
      const created: EmbeddingModelCard = {
        id: createModelId(),
        provider: modelModal.provider, model,
        apiKey: modelModal.apiKey, baseUrl: modelModal.baseUrl,
      };
      setEmbeddingModels(prev => [...prev, created]);
      if (embeddingModels.length === 0) setActiveEmbeddingId(created.id);
    } else if (modelModal.id) {
      setEmbeddingModels(prev => prev.map(m => (
        m.id === modelModal.id
          ? { ...m, provider: modelModal.provider, model, apiKey: modelModal.apiKey, baseUrl: modelModal.baseUrl }
          : m
      )));
    }
    closeModelModal();
  };

  // -- Modal test ----------------------------------------

  const handleModalTest = async () => {
    if (!modelModal) return;
    const model = modelModal.model.trim();
    const label = modelModal.type === 'chat' ? '对话模型名称' : 'Embedding 模型名称';
    if (!model) { setModalTestResult(`❌ 请先输入${label}`); return; }

    setModalTestLoading(true);
    setModalTestResult(null);
    try {
      if (modelModal.type === 'chat') {
        const res = await apiFetch<{ message: string; latency?: number }>('/system/config/test-chat-model', {
          method: 'POST',
          body: JSON.stringify({
            provider: modelModal.provider,
            model,
            apiKey:  modelModal.apiKey,
            baseUrl: modelModal.baseUrl,
          }),
        });
        setModalTestResult(`✅ ${res.message}${res.latency !== undefined ? `，耗时 ${res.latency}ms` : ''}`);
      } else {
        const res = await apiFetch<{ message: string; latency?: number; dimension?: number }>('/system/config/test-embedding-model', {
          method: 'POST',
          body: JSON.stringify({
            provider: modelModal.provider,
            model,
            apiKey:  modelModal.apiKey,
            baseUrl: modelModal.baseUrl,
          }),
        });
        const dimInfo = res.dimension !== undefined ? `，维度 ${res.dimension}` : '';
        setModalTestResult(`✅ ${res.message}${res.latency !== undefined ? `，耗时 ${res.latency}ms` : ''}${dimInfo}`);
      }
    } catch (error) {
      setModalTestResult(`❌ ${error instanceof Error ? error.message : '测试失败'}`);
    } finally {
      setModalTestLoading(false);
    }
  };

  const runEmbeddingTest = async (provider: string, model: string, silentSuccess = false) => {
    try {
      const res = await apiFetch<{ message: string; latency?: number; dimension?: number }>('/system/config/test-embedding-model', {
        method: 'POST',
        body: JSON.stringify({ provider, model }),
      });
      const dimInfo = res.dimension !== undefined ? `，维度 ${res.dimension}` : '';
      if (!silentSuccess) toast(`✅ ${res.message}${res.latency !== undefined ? `，耗时 ${res.latency}ms` : ''}${dimInfo}`, 'success');
    } catch (error) {
      if (!silentSuccess) toast(`Embedding 模型校验失败：${error instanceof Error ? error.message : '测试失败'}`, 'error');
    }
  };

  // -- Render ----------------------------------------

  if (loading) return <div className={styles.container}>加载中...</div>;

  const modalVisual = modelModal ? getProviderVisual(modelModal.provider) : null;

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1>系统 AI 设置</h1>
        <p>配置系统所使用的 AI 提供商及模型，保存后全局生效。</p>
      </div>

      <div className={styles.card}>
        <div className={styles.sectionIntro}>
          <div>
            <h2>模型管理</h2>
            <p>按用途分类展示已添加的模型，点击卡片编辑配置。</p>
          </div>
        </div>

        {/* -- 对话模型 -- */}
        <div className={styles.modelSection}>
          <div className={styles.modelSectionHeader}>
            <div>
              <h3>对话模型</h3>
              <p>用于对话、问答和 AI 能力调用。</p>
            </div>
            <button className={`${styles.button} ${styles.secondaryButton}`} type="button" onClick={openAddChatModelModal}>
              添加模型
            </button>
          </div>

          <div className={styles.modelGrid}>
            {chatModels.map(item => {
              const visual = getProviderVisual(item.provider);
              return (
                <button
                  key={item.id}
                  type="button"
                  className={`${styles.modelCard} ${item.isDefault ? styles.activeModelCard : ''}`}
                  onClick={() => openEditChatModelModal(item)}
                  style={{ '--model-accent': visual.accent } as React.CSSProperties}
                >
                  {item.isDefault && <span className={styles.activeBadge}>默认</span>}
                  <div className={styles.modelCardTop}>
                    <span className={styles.modelKind}>
                      <span className={styles.providerBadge} style={{ backgroundColor: visual.accent }}>
                        {visual.shortLabel}
                      </span>
                      对话模型
                    </span>
                    <span className={styles.modelProvider}>{getProviderLabel(item.provider)}</span>
                  </div>
                  <strong className={styles.modelName}>{item.model}</strong>
                  <span className={styles.modelHint}>{item.apiKey ? '已配置 API Key' : item.provider === 'ollama' ? '本地部署' : '未配置 API Key'}</span>
                  <div className={styles.modelCardActions}>
                    <button
                      type="button"
                      className={`${styles.button} ${styles.ghostButton}`}
                      disabled={item.isDefault}
                      onClick={e => { e.stopPropagation(); handleSetDefaultChatModel(item.id); }}
                    >
                      {item.isDefault ? '当前默认' : '设为默认'}
                    </button>
                    <button
                      type="button"
                      className={`${styles.button} ${styles.ghostButton}`}
                      onClick={e => { e.stopPropagation(); handleDuplicateChatModel(item); }}
                    >
                      复制
                    </button>
                  </div>
                </button>
              );
            })}

            <button
              type="button"
              className={`${styles.modelCard} ${styles.addModelCard}`}
              onClick={openAddChatModelModal}
              style={{ '--model-accent': '#4f46e5' } as React.CSSProperties}
            >
              <span className={styles.addModelIcon}>+</span>
              <strong className={styles.modelName}>添加对话模型</strong>
              <span className={styles.modelHint}>支持多个提供商和模型</span>
            </button>
          </div>
        </div>

        {/* -- Embedding 模型 -- */}
        <div className={styles.modelSection}>
          <div className={styles.modelSectionHeader}>
            <div>
              <h3>Embedding 模型</h3>
              <p>用于知识库文档向量化、召回与检索。</p>
            </div>
          </div>

          <div className={styles.modelGrid}>
            {displayEmbeddingModels.map(item => {
              const isActive = isActiveEmbedding(item);
              const visual   = getProviderVisual(item.provider);
              return (
                <button
                  key={item.id}
                  type="button"
                  className={`${styles.modelCard} ${isActive ? styles.activeModelCard : ''}`}
                  onClick={() => openEditEmbeddingModal(item)}
                  style={{ '--model-accent': visual.accent } as React.CSSProperties}
                >
                  {isActive && <span className={styles.activeBadge}>Active</span>}
                  <div className={styles.modelCardTop}>
                    <span className={styles.modelKind}>
                      <span className={styles.providerBadge} style={{ backgroundColor: visual.accent }}>
                        {visual.shortLabel}
                      </span>
                      Embedding
                    </span>
                    <span className={styles.modelProvider}>{getProviderLabel(item.provider)}</span>
                  </div>
                  <strong className={styles.modelName}>{item.model}</strong>
                  <span className={styles.modelHint}>{item.apiKey ? '已配置 API Key' : item.provider === 'ollama' ? '本地部署' : '未配置 API Key'}</span>
                  <div className={styles.modelCardActions}>
                    <button
                      type="button"
                      className={`${styles.button} ${styles.ghostButton}`}
                      disabled={isActive}
                      onClick={e => { e.stopPropagation(); handleSetActiveEmbeddingModel(item); }}
                    >
                      {isActive ? '当前生效' : '设为生效'}
                    </button>
                    <button
                      type="button"
                      className={`${styles.button} ${styles.ghostButton}`}
                      onClick={e => { e.stopPropagation(); handleDuplicateEmbeddingModel(item); }}
                    >
                      复制
                    </button>
                  </div>
                </button>
              );
            })}

            <button
              type="button"
              className={`${styles.modelCard} ${styles.addModelCard}`}
              onClick={openAddEmbeddingModal}
              style={{ '--model-accent': '#4f46e5' } as React.CSSProperties}
            >
              <span className={styles.addModelIcon}>+</span>
              <strong className={styles.modelName}>添加 Embedding 模型</strong>
              <span className={styles.modelHint}>支持维护多个向量模型</span>
            </button>
          </div>
        </div>
      </div>

      <div className={styles.actions}>
        <button
          className={`${styles.button} ${styles.primaryButton}`}
          onClick={handleSave}
          disabled={saving}
        >
          {saving ? '保存中...' : '保存设置'}
        </button>
      </div>

      {/* -- Modal -- */}
      {modelModal && modalVisual && (
        <div className={styles.modalOverlay} onClick={closeModelModal}>
          <div
            className={styles.modal}
            onClick={e => e.stopPropagation()}
            ref={modelModalRef}
            role="dialog"
            aria-modal="true"
            aria-labelledby="model-modal-title"
            tabIndex={-1}
          >
            <div className={styles.modalHeader}>
              <div>
                <h2 className={styles.modalTitle} id="model-modal-title">
                  {modelModal.type === 'chat'
                    ? modelModal.mode === 'add' ? '添加对话模型' : '编辑对话模型'
                    : modelModal.mode === 'add' ? '添加 Embedding 模型' : '编辑 Embedding 模型'}
                </h2>
                <p className={styles.modalDescription}>
                  {modelModal.type === 'chat'
                    ? '配置提供商、模型名称及 API 凭据，确认后在卡片列表中生效。'
                    : '配置向量化提供商及模型，设为生效后用于知识库检索。'}
                </p>
              </div>
              <button type="button" className={styles.modalClose} onClick={closeModelModal} aria-label="关闭对话框">×</button>
            </div>

            <div className={styles.modalBody}>
              {/* Provider summary */}
              <div className={styles.modalProviderSummary}>
                <span
                  className={styles.modalProviderIcon}
                  style={{ backgroundColor: modalVisual.accent }}
                >
                  {modalVisual.shortLabel}
                </span>
                <div>
                  <strong>{getProviderLabel(modelModal.provider)}</strong>
                  <p>{modelModal.provider === 'ollama' ? '本地部署，无需 API Key' : '云服务，需配置 API Key'}</p>
                </div>
              </div>

              {/* Provider selector */}
              <div className={styles.formGroup}>
                <label>AI 提供商</label>
                <select name="provider" value={modelModal.provider} onChange={handleModalChange} className={styles.select}>
                  {providerOptions.map(opt => (
                    <option key={opt.value} value={opt.value}>{opt.label}</option>
                  ))}
                </select>
              </div>

              {/* Model name */}
              <div className={styles.formGroup}>
                <label>模型名称</label>
                <input
                  type="text"
                  name="model"
                  value={modelModal.model}
                  onChange={handleModalChange}
                  className={styles.input}
                  placeholder={`例如：${modelPlaceholderMap[modelModal.provider] ?? 'model-name'}`}
                />
                <span className={styles.helpText}>请填写提供商文档中对应的模型 ID。</span>
              </div>

              {/* Ollama: Base URL */}
              {modelModal.provider === 'ollama' && (
                <div className={styles.formGroup}>
                  <label>Ollama 服务地址</label>
                  <input
                    type="text"
                    name="baseUrl"
                    value={modelModal.baseUrl}
                    onChange={handleModalChange}
                    className={styles.input}
                    placeholder="http://localhost:11434"
                  />
                  <span className={styles.helpText}>留空将使用默认 http://localhost:11434。</span>
                </div>
              )}

              {/* Non-Ollama: API Key */}
              {modelModal.provider !== 'ollama' && (
                <div className={styles.formGroup}>
                  <label>API Key</label>
                  <input
                    type="password"
                    name="apiKey"
                    value={modelModal.apiKey}
                    onChange={handleModalChange}
                    className={styles.input}
                    placeholder="sk-..."
                    autoComplete="new-password"
                  />
                  <span className={styles.helpText}>凭据加密存储，保存后即可使用。</span>
                </div>
              )}

              {/* Non-Ollama: Custom Base URL (optional) */}
              {modelModal.provider !== 'ollama' && (
                <div className={styles.formGroup}>
                  <label>API 地址（可选）</label>
                  <input
                    type="text"
                    name="baseUrl"
                    value={modelModal.baseUrl}
                    onChange={handleModalChange}
                    className={styles.input}
                    placeholder={`留空使用默认：${defaultBaseUrlMap[modelModal.provider] ?? '官方地址'}`}
                  />
                  <span className={styles.helpText}>使用代理或自建网关时填写，否则留空。</span>
                </div>
              )}

              {/* Set as default (chat only) */}
              {modelModal.type === 'chat' && (
                <div className={styles.formGroup}>
                  <label className={styles.checkboxLabel}>
                    <input
                      type="checkbox"
                      name="isDefault"
                      checked={modelModal.isDefault}
                      onChange={handleModalChange}
                      className={styles.checkbox}
                    />
                    设为默认对话模型
                  </label>
                  <span className={styles.helpText}>默认模型用于系统 AI 功能的首选配置，同时更新全局提供商。</span>
                </div>
              )}

              {/* Test panel */}
              <div className={styles.modalTestPanel}>
                <div className={styles.modalTestHeader}>
                  <div>
                    <strong>连通性测试</strong>
                    <p>使用已保存的凭据测试连接。新增模型请先保存后再测试。</p>
                  </div>
                  <button
                    type="button"
                    className={`${styles.button} ${styles.secondaryButton}`}
                    onClick={handleModalTest}
                    disabled={modalTestLoading}
                  >
                    {modalTestLoading ? '测试中...' : '立即测试'}
                  </button>
                </div>
                {modalTestResult && <div className={styles.testResult}>{modalTestResult}</div>}
              </div>
            </div>

            <div className={styles.modalActions}>
              {modelModal.mode === 'edit' && modelModal.id && (
                <button
                  type="button"
                  className={`${styles.button} ${styles.dangerButton}`}
                  onClick={() => {
                    if (modelModal.type === 'chat')      handleRemoveChatModel(modelModal.id!);
                    else                                 handleRemoveEmbeddingModel(modelModal.id!);
                    closeModelModal();
                  }}
                >
                  删除模型
                </button>
              )}
              <button type="button" className={`${styles.button} ${styles.secondaryButton}`} onClick={closeModelModal}>取消</button>
              <button type="button" className={`${styles.button} ${styles.primaryButton}`} onClick={handleSaveModelModal}>确认</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
