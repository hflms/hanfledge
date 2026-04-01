'use client';

import React, { useState, useEffect, useCallback } from 'react';
import { apiFetch } from '@/lib/api';
import { useToast } from '@/components/Toast';
import { useModalA11y } from '@/lib/a11y';
import styles from './page.module.css';

const providerOptions = [
  { value: 'ollama', label: 'Ollama (本地私有化)' },
  { value: 'dashscope', label: 'DashScope (阿里云通义千问)' },
  { value: 'doubao', label: 'Volcengine (火山引擎/豆包)' },
  { value: 'deepseek', label: 'DeepSeek' },
  { value: 'openrouter', label: 'OpenRouter' },
  { value: 'moonshot', label: 'Moonshot (月之暗面/Kimi)' },
  { value: 'zhipu', label: 'Zhipu (智谱清言/GLM)' }
] as const;

const providerVisualMap: Record<string, { shortLabel: string; accent: string }> = {
  ollama: { shortLabel: 'OL', accent: '#111827' },
  dashscope: { shortLabel: 'DS', accent: '#7c3aed' },
  doubao: { shortLabel: 'DB', accent: '#2563eb' },
  deepseek: { shortLabel: 'DK', accent: '#0f766e' },
  openrouter: { shortLabel: 'OR', accent: '#ea580c' },
  moonshot: { shortLabel: 'MS', accent: '#db2777' },
  zhipu: { shortLabel: 'ZP', accent: '#16a34a' }
};

type ModelModalState =
  | {
      type: 'chat';
      mode: 'add' | 'edit';
      value: string;
      originalName?: string;
    }
  | {
      type: 'embedding';
      mode: 'add' | 'edit';
      id?: string;
      provider: string;
      model: string;
    };

interface EmbeddingModelCard {
  id: string;
  provider: string;
  model: string;
}

function getProviderLabel(provider: string) {
  return providerOptions.find(option => option.value === provider)?.label || provider;
}

function getProviderVisual(provider: string) {
  return providerVisualMap[provider] || { shortLabel: provider.slice(0, 2).toUpperCase(), accent: '#4f46e5' };
}

function createModelId() {
  return `embedding-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

function parseEmbeddingModels(rawValue: string | undefined, fallbackProvider: string, fallbackModel: string) {
  if (rawValue) {
    try {
      const parsed = JSON.parse(rawValue) as Array<Partial<EmbeddingModelCard>>;
      const normalized = parsed
        .filter(item => item && typeof item.model === 'string' && item.model.trim())
        .map(item => ({
          id: typeof item.id === 'string' && item.id ? item.id : createModelId(),
          provider: typeof item.provider === 'string' && item.provider ? item.provider : fallbackProvider,
          model: item.model!.trim()
        }));

      if (normalized.length > 0) {
        return normalized;
      }
    } catch {
      // Ignore invalid historical value and fallback to single-model config.
    }
  }

  if (!fallbackModel.trim()) {
    return [] as EmbeddingModelCard[];
  }

  return [{
    id: createModelId(),
    provider: fallbackProvider,
    model: fallbackModel.trim()
  }];
}

export default function SystemSettingsPage() {
  const { toast } = useToast();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [testingChat, setTestingChat] = useState(false);
  const [testingEmbedding, setTestingEmbedding] = useState(false);
  const [models, setModels] = useState<string[]>([]);
  const [embeddingModels, setEmbeddingModels] = useState<EmbeddingModelCard[]>([]);
  const [modelInput, setModelInput] = useState('');
  const [testModel, setTestModel] = useState('');
  const [testChatResult, setTestChatResult] = useState<string | null>(null);
  const [testEmbeddingResult, setTestEmbeddingResult] = useState<string | null>(null);
  const [modalTestLoading, setModalTestLoading] = useState(false);
  const [modalTestResult, setModalTestResult] = useState<string | null>(null);
  const [modelModal, setModelModal] = useState<ModelModalState | null>(null);
  const [configs, setConfigs] = useState({
    LLM_PROVIDER: 'ollama',
    OLLAMA_BASE_URL: 'http://localhost:11434',
    OLLAMA_MODEL: 'qwen2.5:7b',
    DASHSCOPE_API_KEY: '',
    DASHSCOPE_MODEL: 'qwen-max',
    DASHSCOPE_COMPAT_BASE_URL: '',
    DOUBAO_API_KEY: '',
    DOUBAO_MODEL: 'ep-xxx',
    DOUBAO_COMPAT_BASE_URL: 'https://ark.cn-beijing.volces.com/api/v3',
    DEEPSEEK_API_KEY: '',
    DEEPSEEK_MODEL: 'deepseek-chat',
    DEEPSEEK_COMPAT_BASE_URL: 'https://api.deepseek.com/v1',
    OPENROUTER_API_KEY: '',
    OPENROUTER_MODEL: '',
    OPENROUTER_COMPAT_BASE_URL: 'https://openrouter.ai/api/v1',
    MOONSHOT_API_KEY: '',
    MOONSHOT_MODEL: 'moonshot-v1-8k',
    MOONSHOT_COMPAT_BASE_URL: 'https://api.moonshot.cn/v1',
    ZHIPU_API_KEY: '',
    ZHIPU_MODEL: 'glm-4',
    ZHIPU_COMPAT_BASE_URL: 'https://open.bigmodel.cn/api/paas/v4',
    EMBEDDING_PROVIDER: 'ollama',
    EMBEDDING_MODEL: 'bge-m3'
  });
  const closeModelModal = useCallback(() => {
    setModelModal(null);
    setModalTestLoading(false);
    setModalTestResult(null);
  }, []);
  const modelModalRef = useModalA11y(!!modelModal, closeModelModal);
  const isActiveEmbeddingModel = (item: EmbeddingModelCard) => (
    item.provider === configs.EMBEDDING_PROVIDER && item.model === configs.EMBEDDING_MODEL
  );
  const displayEmbeddingModels = [
    ...embeddingModels.filter(isActiveEmbeddingModel),
    ...embeddingModels.filter(item => !isActiveEmbeddingModel(item))
  ];

  const loadConfigs = useCallback(async () => {
    try {
      const data = await apiFetch<Record<string, string>>('/system/config');
      if (data && Object.keys(data).length > 0) {
        setConfigs(prev => ({ ...prev, ...data }));
        if (data.LLM_MODELS) {
          const list = data.LLM_MODELS.split(',').map(v => v.trim()).filter(Boolean);
          setModels(list);
          setTestModel(prev => prev || list[0] || '');
        }

        const embeddingProvider = data.EMBEDDING_PROVIDER || data.LLM_PROVIDER || 'ollama';
        const embeddingModel = data.EMBEDDING_MODEL || '';
        setEmbeddingModels(parseEmbeddingModels(data.EMBEDDING_MODELS, embeddingProvider, embeddingModel));
      }
    } catch (error) {
      console.error('Failed to load configs:', error);
      toast('加载配置失败', 'error');
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => {
    loadConfigs();
  }, [loadConfigs]);

  const handleSave = async () => {
    setSaving(true);
    try {
      const payload = {
        ...configs,
        LLM_MODELS: models.join(','),
        EMBEDDING_MODELS: JSON.stringify(embeddingModels)
      };
      await apiFetch('/system/config', {
        method: 'PUT',
        body: JSON.stringify(payload)
      });
      toast('系统配置保存成功', 'success');

      if (configs.EMBEDDING_MODEL) {
        await runEmbeddingTest(configs.EMBEDDING_PROVIDER || configs.LLM_PROVIDER, configs.EMBEDDING_MODEL, true);
      }
    } catch (error) {
      console.error('Failed to save configs:', error);
      toast(error instanceof Error ? error.message : '保存配置失败', 'error');
    } finally {
      setSaving(false);
    }
  };

  const handleChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    const { name, value } = e.target;
    setConfigs(prev => ({ ...prev, [name]: value }));
  };

  const openAddChatModelModal = () => {
    setModalTestResult(null);
    setModelModal({ type: 'chat', mode: 'add', value: modelInput.trim() });
  };

  const openEditChatModelModal = (name: string) => {
    setModalTestResult(null);
    setModelModal({ type: 'chat', mode: 'edit', value: name, originalName: name });
  };

  const openEmbeddingModelModal = () => {
    setModalTestResult(null);
    setModelModal({
      type: 'embedding',
      mode: 'add',
      provider: configs.EMBEDDING_PROVIDER || configs.LLM_PROVIDER,
      model: ''
    });
  };

  const openEditEmbeddingModelModal = (item: EmbeddingModelCard) => {
    setModalTestResult(null);
    setModelModal({
      type: 'embedding',
      mode: 'edit',
      id: item.id,
      provider: item.provider,
      model: item.model
    });
  };

  const handleDuplicateEmbeddingModel = (item: EmbeddingModelCard) => {
    setModalTestResult(null);
    setModelModal({
      type: 'embedding',
      mode: 'add',
      provider: item.provider,
      model: `${item.model}-copy`
    });
    toast('已复制配置到新增弹窗，并自动追加 -copy', 'success');
  };

  const handleRemoveModel = (name: string) => {
    const next = models.filter(m => m !== name);
    setModels(next);
    if (testModel === name) {
      setTestModel(next[0] || '');
    }
  };

  const handleRemoveEmbeddingModel = (id: string) => {
    const next = embeddingModels.filter(item => item.id !== id);
    setEmbeddingModels(next);

    const removed = embeddingModels.find(item => item.id === id);
    if (!removed) {
      return;
    }

    const isActive = removed.provider === configs.EMBEDDING_PROVIDER && removed.model === configs.EMBEDDING_MODEL;
    if (isActive) {
      const fallback = next[0];
      setConfigs(prev => ({
        ...prev,
        EMBEDDING_PROVIDER: fallback?.provider || prev.LLM_PROVIDER,
        EMBEDDING_MODEL: fallback?.model || ''
      }));
      setTestEmbeddingResult(null);
    }
  };

  const handleModelModalChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    const { name, value } = e.target;
    setModelModal(prev => {
      if (!prev) return prev;
      if (prev.type === 'chat' && name === 'value') {
        return { ...prev, value };
      }
      if (prev.type === 'embedding' && (name === 'provider' || name === 'model')) {
        return {
          ...prev,
          ...(name === 'provider' ? { provider: value } : { model: value })
        };
      }
      return prev;
    });
  };

  const handleSaveModelModal = () => {
    if (!modelModal) return;

    if (modelModal.type === 'chat') {
      const value = modelModal.value.trim();
      if (!value) {
        toast('请输入模型名称', 'error');
        return;
      }

      const duplicated = models.some(model => model === value && model !== modelModal.originalName);
      if (duplicated) {
        toast('模型已存在', 'error');
        return;
      }

      if (modelModal.mode === 'add') {
        const next = [...models, value];
        setModels(next);
        if (!testModel) {
          setTestModel(value);
        }
        setModelInput('');
      } else if (modelModal.originalName) {
        setModels(prev => prev.map(model => (model === modelModal.originalName ? value : model)));
        if (testModel === modelModal.originalName) {
          setTestModel(value);
        }
      }

      closeModelModal();
      return;
    }

    const normalizedModel = modelModal.model.trim();
    if (!normalizedModel) {
      toast('请输入 Embedding 模型名称', 'error');
      return;
    }

    const duplicated = embeddingModels.some(item => (
      item.id !== modelModal.id &&
      item.provider === modelModal.provider &&
      item.model === normalizedModel
    ));
    if (duplicated) {
      toast('该 Embedding 模型已存在', 'error');
      return;
    }

    if (modelModal.mode === 'add') {
      const created = {
        id: createModelId(),
        provider: modelModal.provider || configs.LLM_PROVIDER,
        model: normalizedModel
      };
      setEmbeddingModels(prev => [...prev, created]);
      if (embeddingModels.length === 0 || !configs.EMBEDDING_MODEL) {
        setConfigs(prev => ({
          ...prev,
          EMBEDDING_PROVIDER: created.provider,
          EMBEDDING_MODEL: created.model
        }));
      }
    } else if (modelModal.id) {
      const previous = embeddingModels.find(item => item.id === modelModal.id);
      setEmbeddingModels(prev => prev.map(item => (
        item.id === modelModal.id
          ? { ...item, provider: modelModal.provider || configs.LLM_PROVIDER, model: normalizedModel }
          : item
      )));

      if (previous && previous.provider === configs.EMBEDDING_PROVIDER && previous.model === configs.EMBEDDING_MODEL) {
        setConfigs(prev => ({
          ...prev,
          EMBEDDING_PROVIDER: modelModal.provider || prev.LLM_PROVIDER,
          EMBEDDING_MODEL: normalizedModel
        }));
      }
    }

    closeModelModal();
  };

  const handleSetActiveEmbeddingModel = (item: EmbeddingModelCard) => {
    setConfigs(prev => ({
      ...prev,
      EMBEDDING_PROVIDER: item.provider,
      EMBEDDING_MODEL: item.model
    }));
    setTestEmbeddingResult(null);
    toast(`已将 ${item.model} 设为当前生效 Embedding 模型`, 'success');
  };

  const handleSetDefaultTestModel = (model: string) => {
    setTestModel(model);
    setTestChatResult(null);
    toast(`已将 ${model} 设为默认测试模型`, 'success');
  };

  const handleModalTest = async () => {
    if (!modelModal) return;

    if (modelModal.type === 'chat') {
      const model = modelModal.value.trim();
      if (!model) {
        setModalTestResult('❌ 请先输入对话模型名称');
        return;
      }

      setModalTestLoading(true);
      setModalTestResult(null);
      try {
        const res = await apiFetch<{ message: string; latency?: number }>('/system/config/test-chat-model', {
          method: 'POST',
          body: JSON.stringify({
            provider: configs.LLM_PROVIDER,
            model
          })
        });
        setModalTestResult(`✅ ${res.message}${res.latency !== undefined ? `，耗时 ${res.latency}ms` : ''}`);
      } catch (error) {
        const msg = error instanceof Error ? error.message : '测试失败';
        setModalTestResult(`❌ ${msg}`);
      } finally {
        setModalTestLoading(false);
      }
      return;
    }

    const model = modelModal.model.trim();
    if (!model) {
      setModalTestResult('❌ 请先输入 Embedding 模型名称');
      return;
    }

    setModalTestLoading(true);
    setModalTestResult(null);
    try {
      const res = await apiFetch<{ message: string; latency?: number; dimension?: number }>('/system/config/test-embedding-model', {
        method: 'POST',
        body: JSON.stringify({
          provider: modelModal.provider || configs.LLM_PROVIDER,
          model
        })
      });
      const dimInfo = res.dimension !== undefined ? `，维度 ${res.dimension}` : '';
      setModalTestResult(`✅ ${res.message}${res.latency !== undefined ? `，耗时 ${res.latency}ms` : ''}${dimInfo}`);
    } catch (error) {
      const msg = error instanceof Error ? error.message : '测试失败';
      setModalTestResult(`❌ ${msg}`);
    } finally {
      setModalTestLoading(false);
    }
  };

  const handleTestChatModel = async () => {
    setTestingChat(true);
    setTestChatResult(null);
    try {
      const res = await apiFetch<{ message: string; reply?: string; latency?: number }>('/system/config/test-chat-model', {
        method: 'POST',
        body: JSON.stringify({
          provider: configs.LLM_PROVIDER,
          model: testModel
        })
      });
      setTestChatResult(`✅ ${res.message}${res.latency !== undefined ? `，耗时 ${res.latency}ms` : ''}`);
    } catch (error) {
      const msg = error instanceof Error ? error.message : '测试失败';
      setTestChatResult(`❌ ${msg}`);
    } finally {
      setTestingChat(false);
    }
  };

  const handleTestEmbeddingModel = async () => {
    await runEmbeddingTest(configs.EMBEDDING_PROVIDER || configs.LLM_PROVIDER, configs.EMBEDDING_MODEL);
  };

  const runEmbeddingTest = async (provider: string, model: string, silentSuccess = false) => {
    setTestingEmbedding(true);
    setTestEmbeddingResult(null);
    try {
      const res = await apiFetch<{ message: string; latency?: number; dimension?: number }>('/system/config/test-embedding-model', {
        method: 'POST',
        body: JSON.stringify({
          provider,
          model
        })
      });
      const dimInfo = res.dimension !== undefined ? `，维度 ${res.dimension}` : '';
      const result = `✅ ${res.message}${res.latency !== undefined ? `，耗时 ${res.latency}ms` : ''}${dimInfo}`;
      setTestEmbeddingResult(result);
      if (!silentSuccess) {
        toast('当前生效 Embedding 模型测试通过', 'success');
      }
    } catch (error) {
      const msg = error instanceof Error ? error.message : '测试失败';
      setTestEmbeddingResult(`❌ ${msg}`);
      toast(`当前生效 Embedding 模型校验失败：${msg}`, 'error');
    } finally {
      setTestingEmbedding(false);
    }
  };

  if (loading) {
    return <div className={styles.container}>加载中...</div>;
  }

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1>系统 AI 设置</h1>
        <p>配置系统所使用的 AI 提供商及其相关参数（全局生效）。</p>
      </div>

      <div className={styles.card}>
        <h2>基础设置</h2>
        <div className={styles.formGroup} style={{ marginTop: '20px' }}>
          <label>默认 AI 提供商</label>
          <select
            name="LLM_PROVIDER"
            value={configs.LLM_PROVIDER}
            onChange={handleChange}
            className={styles.select}
          >
            {providerOptions.map(option => (
              <option key={option.value} value={option.value}>{option.label}</option>
            ))}
          </select>
          <span className={styles.helpText}>系统所有 AI 功能将默认使用此提供商（如需针对不同场景自定义，可在具体功能页面中指定）。</span>
        </div>
      </div>

      <div className={styles.card}>
        <div className={styles.sectionIntro}>
          <div>
            <h2>模型管理</h2>
            <p>已添加的模型按用途分类展示，点击卡片即可编辑。</p>
          </div>
        </div>

        <div className={styles.modelSection}>
          <div className={styles.modelSectionHeader}>
            <div>
              <h3>对话模型</h3>
              <p>用于对话、问答和 AI 能力测试。</p>
            </div>
            <button className={`${styles.button} ${styles.secondaryButton}`} type="button" onClick={openAddChatModelModal}>添加模型</button>
          </div>

          <div className={styles.modelGrid}>
            {models.map(model => (
              <button
                key={model}
                type="button"
                className={styles.modelCard}
                onClick={() => openEditChatModelModal(model)}
                style={{ '--model-accent': getProviderVisual(configs.LLM_PROVIDER).accent } as React.CSSProperties}
              >
                <div className={styles.modelCardTop}>
                  <span className={styles.modelKind}>
                    <span
                      className={styles.providerBadge}
                      style={{ backgroundColor: getProviderVisual(configs.LLM_PROVIDER).accent }}
                    >
                      {getProviderVisual(configs.LLM_PROVIDER).shortLabel}
                    </span>
                    对话模型
                  </span>
                  <span className={styles.modelProvider}>{getProviderLabel(configs.LLM_PROVIDER)}</span>
                </div>
                <strong className={styles.modelName}>{model}</strong>
                <span className={styles.modelHint}>点击卡片编辑模型名称</span>
                {testModel === model && <span className={styles.modelMeta}>当前测试模型</span>}
                <div className={styles.modelCardActions}>
                  <button
                    type="button"
                    className={`${styles.button} ${styles.ghostButton}`}
                    disabled={testModel === model}
                    onClick={e => {
                      e.stopPropagation();
                      handleSetDefaultTestModel(model);
                    }}
                  >
                    {testModel === model ? '默认测试模型' : '设为默认测试'}
                  </button>
                </div>
              </button>
            ))}

            <button
              type="button"
              className={`${styles.modelCard} ${styles.addModelCard}`}
              onClick={openAddChatModelModal}
              style={{ '--model-accent': getProviderVisual(configs.LLM_PROVIDER).accent } as React.CSSProperties}
            >
              <span className={styles.addModelIcon}>+</span>
              <strong className={styles.modelName}>添加对话模型</strong>
              <span className={styles.modelHint}>支持录入多个可选对话模型</span>
            </button>
          </div>
        </div>

        <div className={styles.modelSection}>
          <div className={styles.modelSectionHeader}>
            <div>
              <h3>Embedding 模型</h3>
              <p>用于知识库文档向量化、召回与检索。</p>
            </div>
          </div>

          <div className={styles.modelGrid}>
            {displayEmbeddingModels.map(item => {
              const isActive = isActiveEmbeddingModel(item);
              return (
                <button
                  key={item.id}
                  type="button"
                  className={`${styles.modelCard} ${isActive ? styles.activeModelCard : ''}`}
                  onClick={() => openEditEmbeddingModelModal(item)}
                  style={{ '--model-accent': getProviderVisual(item.provider).accent } as React.CSSProperties}
                >
                  {isActive && <span className={styles.activeBadge}>Active</span>}
                  <div className={styles.modelCardTop}>
                    <span className={styles.modelKind}>
                      <span
                        className={styles.providerBadge}
                        style={{ backgroundColor: getProviderVisual(item.provider).accent }}
                      >
                        {getProviderVisual(item.provider).shortLabel}
                      </span>
                      Embedding
                    </span>
                    <span className={styles.modelProvider}>{getProviderLabel(item.provider)}</span>
                  </div>
                  <strong className={styles.modelName}>{item.model}</strong>
                  <span className={styles.modelHint}>点击卡片编辑 Embedding 提供商和模型</span>
                  {isActive && <span className={styles.modelMeta}>当前系统向量模型</span>}
                  <div className={styles.modelCardActions}>
                    <button
                      type="button"
                      className={`${styles.button} ${styles.ghostButton}`}
                      onClick={e => {
                        e.stopPropagation();
                        handleDuplicateEmbeddingModel(item);
                      }}
                    >
                      复制配置
                    </button>
                    <button
                      type="button"
                      className={`${styles.button} ${styles.ghostButton}`}
                      disabled={isActive}
                      onClick={e => {
                        e.stopPropagation();
                        handleSetActiveEmbeddingModel(item);
                      }}
                    >
                      {isActive ? '当前生效模型' : '设为当前生效'}
                    </button>
                  </div>
                </button>
              );
            })}

            <button
              type="button"
              className={`${styles.modelCard} ${styles.addModelCard}`}
              onClick={openEmbeddingModelModal}
              style={{ '--model-accent': getProviderVisual(configs.EMBEDDING_PROVIDER || configs.LLM_PROVIDER).accent } as React.CSSProperties}
            >
              <span className={styles.addModelIcon}>+</span>
              <strong className={styles.modelName}>添加 Embedding 模型</strong>
              <span className={styles.modelHint}>支持维护多个向量模型卡片，并指定当前生效项</span>
            </button>
          </div>
        </div>
      </div>

      <div className={styles.card}>
        <h2>模型测试</h2>

        <div className={styles.formGroup}>
          <label>对话模型测试</label>
          <div className={styles.inlineRow}>
            <select
              value={testModel}
              onChange={e => setTestModel(e.target.value)}
              className={styles.select}
            >
              <option value="">请选择模型</option>
              {models.map(m => (
                <option key={m} value={m}>{m}</option>
              ))}
            </select>
            <button
              className={`${styles.button} ${styles.secondaryButton}`}
              type="button"
              onClick={handleTestChatModel}
              disabled={testingChat || !testModel}
            >
              {testingChat ? '测试中...' : '测试对话模型'}
            </button>
          </div>
          {testChatResult && <div className={styles.testResult}>{testChatResult}</div>}
        </div>

        <div className={styles.formGroup}>
          <label>Embedding 模型测试</label>
          <div className={styles.inlineRow}>
            <input
              type="text"
              value={configs.EMBEDDING_MODEL ? `${getProviderLabel(configs.EMBEDDING_PROVIDER || configs.LLM_PROVIDER)} / ${configs.EMBEDDING_MODEL}` : ''}
              readOnly
              className={styles.input}
            />
            <button
              className={`${styles.button} ${styles.secondaryButton}`}
              type="button"
              onClick={handleTestEmbeddingModel}
              disabled={testingEmbedding || !configs.EMBEDDING_MODEL}
            >
              {testingEmbedding ? '测试中...' : '测试向量模型'}
            </button>
          </div>
          {testEmbeddingResult && <div className={styles.testResult}>{testEmbeddingResult}</div>}
        </div>
      </div>

      {configs.LLM_PROVIDER === 'ollama' && (
        <div className={styles.card}>
          <h2>Ollama 配置</h2>
          <div className={styles.formGroup} style={{ marginTop: '20px' }}>
            <label>Ollama 服务地址 (Base URL)</label>
            <input
              type="text"
              name="OLLAMA_BASE_URL"
              value={configs.OLLAMA_BASE_URL}
              onChange={handleChange}
              className={styles.input}
              placeholder="http://localhost:11434"
            />
          </div>
          <div className={styles.formGroup}>
            <label>Ollama 对话模型</label>
            <input
              type="text"
              name="OLLAMA_MODEL"
              value={configs.OLLAMA_MODEL}
              onChange={handleChange}
              className={styles.input}
              placeholder="例如：qwen2.5:7b"
            />
          </div>
        </div>
      )}

      {configs.LLM_PROVIDER === 'dashscope' && (
        <div className={styles.card}>
          <h2>DashScope (通义千问) 配置</h2>
          <div className={styles.formGroup} style={{ marginTop: '20px' }}>
            <label>API Key</label>
            <input
              type="password"
              name="DASHSCOPE_API_KEY"
              value={configs.DASHSCOPE_API_KEY}
              onChange={handleChange}
              className={styles.input}
              placeholder="sk-..."
            />
          </div>
          <div className={styles.formGroup}>
            <label>对话模型 (Chat Model)</label>
            <input
              type="text"
              name="DASHSCOPE_MODEL"
              value={configs.DASHSCOPE_MODEL}
              onChange={handleChange}
              className={styles.input}
              placeholder="例如：qwen-max"
            />
          </div>
          <div className={styles.formGroup}>
            <label>API 地址 (可选)</label>
            <input
              type="text"
              name="DASHSCOPE_COMPAT_BASE_URL"
              value={configs.DASHSCOPE_COMPAT_BASE_URL || ''}
              onChange={handleChange}
              className={styles.input}
              placeholder="留空使用默认: https://dashscope.aliyuncs.com/compatible-mode/v1"
            />
            <span className={styles.helpText}>自定义 OpenAI 兼容接口地址。如使用代理或自建网关请在此填写，留空则使用 DashScope 官方地址。</span>
          </div>
        </div>
      )}

      {configs.LLM_PROVIDER === 'doubao' && (
        <div className={styles.card}>
          <h2>Volcengine (火山引擎/豆包) 配置</h2>
          <div className={styles.formGroup} style={{ marginTop: '20px' }}>
            <label>API Key</label>
            <input
              type="password"
              name="DOUBAO_API_KEY"
              value={configs.DOUBAO_API_KEY}
              onChange={handleChange}
              className={styles.input}
              placeholder="sk-..."
            />
          </div>
          <div className={styles.formGroup}>
            <label>推理接入点 (Model / Endpoint)</label>
            <input
              type="text"
              name="DOUBAO_MODEL"
              value={configs.DOUBAO_MODEL}
              onChange={handleChange}
              className={styles.input}
              placeholder="例如：ep-xxx"
            />
          </div>
          <div className={styles.formGroup}>
            <label>API 地址</label>
            <input
              type="text"
              name="DOUBAO_COMPAT_BASE_URL"
              value={configs.DOUBAO_COMPAT_BASE_URL || ''}
              onChange={handleChange}
              className={styles.input}
              placeholder="默认: https://ark.cn-beijing.volces.com/api/v3"
            />
          </div>
        </div>
      )}

      {configs.LLM_PROVIDER === 'deepseek' && (
        <div className={styles.card}>
          <h2>DeepSeek 配置</h2>
          <div className={styles.formGroup} style={{ marginTop: '20px' }}>
            <label>API Key</label>
            <input
              type="password"
              name="DEEPSEEK_API_KEY"
              value={configs.DEEPSEEK_API_KEY}
              onChange={handleChange}
              className={styles.input}
              placeholder="sk-..."
            />
          </div>
          <div className={styles.formGroup}>
            <label>对话模型 (Chat Model)</label>
            <input
              type="text"
              name="DEEPSEEK_MODEL"
              value={configs.DEEPSEEK_MODEL}
              onChange={handleChange}
              className={styles.input}
              placeholder="例如：deepseek-chat 或 deepseek-reasoner"
            />
          </div>
          <div className={styles.formGroup}>
            <label>API 地址</label>
            <input
              type="text"
              name="DEEPSEEK_COMPAT_BASE_URL"
              value={configs.DEEPSEEK_COMPAT_BASE_URL || ''}
              onChange={handleChange}
              className={styles.input}
              placeholder="默认: https://api.deepseek.com/v1"
            />
          </div>
        </div>
      )}

      {configs.LLM_PROVIDER === 'openrouter' && (
        <div className={styles.card}>
          <h2>OpenRouter 配置</h2>
          <div className={styles.formGroup} style={{ marginTop: '20px' }}>
            <label>API Key</label>
            <input
              type="password"
              name="OPENROUTER_API_KEY"
              value={configs.OPENROUTER_API_KEY}
              onChange={handleChange}
              className={styles.input}
              placeholder="sk-or-v1-..."
            />
          </div>
          <div className={styles.formGroup}>
            <label>对话模型 (Chat Model)</label>
            <input
              type="text"
              name="OPENROUTER_MODEL"
              value={configs.OPENROUTER_MODEL}
              onChange={handleChange}
              className={styles.input}
              placeholder="例如：anthropic/claude-3-haiku"
            />
          </div>
          <div className={styles.formGroup}>
            <label>API 地址</label>
            <input
              type="text"
              name="OPENROUTER_COMPAT_BASE_URL"
              value={configs.OPENROUTER_COMPAT_BASE_URL || ''}
              onChange={handleChange}
              className={styles.input}
              placeholder="默认: https://openrouter.ai/api/v1"
            />
          </div>
        </div>
      )}

      {configs.LLM_PROVIDER === 'moonshot' && (
        <div className={styles.card}>
          <h2>Moonshot (月之暗面/Kimi) 配置</h2>
          <div className={styles.formGroup} style={{ marginTop: '20px' }}>
            <label>API Key</label>
            <input
              type="password"
              name="MOONSHOT_API_KEY"
              value={configs.MOONSHOT_API_KEY}
              onChange={handleChange}
              className={styles.input}
              placeholder="sk-..."
            />
          </div>
          <div className={styles.formGroup}>
            <label>对话模型 (Chat Model)</label>
            <input
              type="text"
              name="MOONSHOT_MODEL"
              value={configs.MOONSHOT_MODEL}
              onChange={handleChange}
              className={styles.input}
              placeholder="例如：moonshot-v1-8k"
            />
          </div>
          <div className={styles.formGroup}>
            <label>API 地址</label>
            <input
              type="text"
              name="MOONSHOT_COMPAT_BASE_URL"
              value={configs.MOONSHOT_COMPAT_BASE_URL || ''}
              onChange={handleChange}
              className={styles.input}
              placeholder="默认: https://api.moonshot.cn/v1"
            />
          </div>
        </div>
      )}

      {configs.LLM_PROVIDER === 'zhipu' && (
        <div className={styles.card}>
          <h2>Zhipu (智谱清言/GLM) 配置</h2>
          <div className={styles.formGroup} style={{ marginTop: '20px' }}>
            <label>API Key</label>
            <input
              type="password"
              name="ZHIPU_API_KEY"
              value={configs.ZHIPU_API_KEY}
              onChange={handleChange}
              className={styles.input}
              placeholder="sk-..."
            />
          </div>
          <div className={styles.formGroup}>
            <label>对话模型 (Chat Model)</label>
            <input
              type="text"
              name="ZHIPU_MODEL"
              value={configs.ZHIPU_MODEL}
              onChange={handleChange}
              className={styles.input}
              placeholder="例如：glm-4"
            />
          </div>
          <div className={styles.formGroup}>
            <label>API 地址</label>
            <input
              type="text"
              name="ZHIPU_COMPAT_BASE_URL"
              value={configs.ZHIPU_COMPAT_BASE_URL || ''}
              onChange={handleChange}
              className={styles.input}
              placeholder="默认: https://open.bigmodel.cn/api/paas/v4"
            />
          </div>
        </div>
      )}

      <div className={styles.actions}>
        <button
          className={`${styles.button} ${styles.primaryButton}`}
          onClick={handleSave}
          disabled={saving}
        >
          {saving ? '保存中...' : '保存设置'}
        </button>
      </div>

      {modelModal && (
        <div className={styles.modalOverlay} onClick={closeModelModal}>
          <div
            className={styles.modal}
            onClick={e => e.stopPropagation()}
            ref={modelModalRef}
            role="dialog"
            aria-modal="true"
            aria-labelledby="model-settings-modal-title"
            tabIndex={-1}
          >
            <div className={styles.modalHeader}>
              <div>
                <h2 className={styles.modalTitle} id="model-settings-modal-title">
                  {modelModal.type === 'chat'
                    ? modelModal.mode === 'add'
                      ? '添加对话模型'
                      : '编辑对话模型'
                    : '编辑 Embedding 模型'}
                </h2>
                <p className={styles.modalDescription}>
                  {modelModal.type === 'chat'
                    ? '更新模型名称后，保存设置即可全局生效。'
                    : modelModal.mode === 'add'
                      ? '新增一个 Embedding 模型卡片，并可稍后设为当前生效。'
                      : '编辑当前 Embedding 模型卡片，并可更新其 provider 与模型名。'}
                </p>
              </div>
              <button
                type="button"
                className={styles.modalClose}
                onClick={closeModelModal}
                aria-label="关闭对话框"
              >
                ×
              </button>
            </div>

            <div className={styles.modalBody}>
              {modelModal.type === 'chat' ? (
                <>
                  <div className={styles.modalProviderSummary}>
                    <span
                      className={styles.modalProviderIcon}
                      style={{ backgroundColor: getProviderVisual(configs.LLM_PROVIDER).accent }}
                    >
                      {getProviderVisual(configs.LLM_PROVIDER).shortLabel}
                    </span>
                    <div>
                      <strong>{getProviderLabel(configs.LLM_PROVIDER)}</strong>
                      <p>当前对话模型会使用默认 AI 提供商进行测试。</p>
                    </div>
                  </div>
                  <div className={styles.formGroup}>
                    <label>模型名称</label>
                    <input
                      type="text"
                      name="value"
                      value={modelModal.value}
                      onChange={handleModelModalChange}
                      className={styles.input}
                      placeholder="例如：qwen2.5:7b"
                    />
                    <span className={styles.helpText}>会加入系统的可用对话模型列表，用于选择和测试。</span>
                  </div>
                </>
              ) : (
                <>
                  <div className={styles.modalProviderSummary}>
                    <span
                      className={styles.modalProviderIcon}
                      style={{ backgroundColor: getProviderVisual(modelModal.provider).accent }}
                    >
                      {getProviderVisual(modelModal.provider).shortLabel}
                    </span>
                    <div>
                      <strong>{getProviderLabel(modelModal.provider)}</strong>
                      <p>当前 Embedding 模型可独立于默认对话提供商配置。</p>
                    </div>
                  </div>
                  <div className={styles.formGroup}>
                    <label>Embedding 提供商</label>
                    <select
                      name="provider"
                      value={modelModal.provider}
                      onChange={handleModelModalChange}
                      className={styles.select}
                    >
                      {providerOptions.map(option => (
                        <option key={option.value} value={option.value}>{option.label}</option>
                      ))}
                    </select>
                    <span className={styles.helpText}>向量模型可独立于默认对话提供商进行配置。</span>
                  </div>

                  <div className={styles.formGroup}>
                    <label>Embedding 向量模型</label>
                    <input
                      type="text"
                      name="model"
                      value={modelModal.model}
                      onChange={handleModelModalChange}
                      className={styles.input}
                      placeholder="例如：bge-m3 或 text-embedding-v3"
                    />
                    <span className={styles.helpText}>请确保当前提供商支持该 Embedding 模型。</span>
                  </div>
                </>
              )}

              <div className={styles.modalTestPanel}>
                <div className={styles.modalTestHeader}>
                  <div>
                    <strong>即时连通性测试</strong>
                    <p>保存前可先测试当前弹窗中的模型配置。</p>
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
              {modelModal.type === 'chat' && modelModal.mode === 'edit' && modelModal.originalName && (
                <button
                  type="button"
                  className={`${styles.button} ${styles.dangerButton}`}
                  onClick={() => {
                    handleRemoveModel(modelModal.originalName!);
                    closeModelModal();
                  }}
                >
                  删除模型
                </button>
              )}
              {modelModal.type === 'embedding' && modelModal.mode === 'edit' && modelModal.id && (
                <button
                  type="button"
                  className={`${styles.button} ${styles.dangerButton}`}
                  onClick={() => {
                    handleRemoveEmbeddingModel(modelModal.id!);
                    closeModelModal();
                  }}
                >
                  删除模型
                </button>
              )}
              <button type="button" className={`${styles.button} ${styles.secondaryButton}`} onClick={closeModelModal}>
                取消
              </button>
              <button type="button" className={`${styles.button} ${styles.primaryButton}`} onClick={handleSaveModelModal}>
                确认
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
