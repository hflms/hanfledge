'use client';

import React, { useState, useEffect, useCallback } from 'react';
import { apiFetch } from '@/lib/api';
import { useToast } from '@/components/Toast';
import styles from './page.module.css';

export default function SystemSettingsPage() {
  const { toast } = useToast();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [testingChat, setTestingChat] = useState(false);
  const [testingEmbedding, setTestingEmbedding] = useState(false);
  const [models, setModels] = useState<string[]>([]);
  const [modelInput, setModelInput] = useState('');
  const [testModel, setTestModel] = useState('');
  const [testChatResult, setTestChatResult] = useState<string | null>(null);
  const [testEmbeddingResult, setTestEmbeddingResult] = useState<string | null>(null);
  const [configs, setConfigs] = useState({
    LLM_PROVIDER: 'ollama',
    OLLAMA_BASE_URL: 'http://localhost:11434',
    OLLAMA_MODEL: 'qwen2.5:7b',
    DASHSCOPE_API_KEY: '',
    DASHSCOPE_MODEL: 'qwen-max',
    DASHSCOPE_COMPAT_BASE_URL: '',
    EMBEDDING_PROVIDER: 'ollama',
    EMBEDDING_MODEL: 'bge-m3'
  });

  const loadConfigs = useCallback(async () => {
    try {
      const data = await apiFetch<Record<string, string>>('/system/config');
      if (data && Object.keys(data).length > 0) {
        setConfigs(prev => ({ ...prev, ...data }));
        if (data.LLM_MODELS) {
          const list = data.LLM_MODELS.split(',').map(v => v.trim()).filter(Boolean);
          setModels(list);
          if (!testModel && list.length > 0) {
            setTestModel(list[0]);
          }
        }
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
        LLM_MODELS: models.join(',')
      };
      await apiFetch('/system/config', {
        method: 'PUT',
        body: JSON.stringify(payload)
      });
      toast('系统配置保存成功', 'success');
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

  const handleAddModel = () => {
    const value = modelInput.trim();
    if (!value) return;
    if (models.includes(value)) {
      toast('模型已存在', 'error');
      return;
    }
    const next = [...models, value];
    setModels(next);
    if (!testModel) {
      setTestModel(value);
    }
    setModelInput('');
  };

  const handleRemoveModel = (name: string) => {
    const next = models.filter(m => m !== name);
    setModels(next);
    if (testModel === name) {
      setTestModel(next[0] || '');
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
    setTestingEmbedding(true);
    setTestEmbeddingResult(null);
    try {
      const res = await apiFetch<{ message: string; latency?: number; dimension?: number }>('/system/config/test-embedding-model', {
        method: 'POST',
        body: JSON.stringify({
          provider: configs.EMBEDDING_PROVIDER || configs.LLM_PROVIDER,
          model: configs.EMBEDDING_MODEL
        })
      });
      const dimInfo = res.dimension !== undefined ? `，维度 ${res.dimension}` : '';
      setTestEmbeddingResult(`✅ ${res.message}${res.latency !== undefined ? `，耗时 ${res.latency}ms` : ''}${dimInfo}`);
    } catch (error) {
      const msg = error instanceof Error ? error.message : '测试失败';
      setTestEmbeddingResult(`❌ ${msg}`);
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
            <option value="ollama">Ollama (本地私有化)</option>
            <option value="dashscope">DashScope (阿里云通义千问)</option>
          </select>
          <span className={styles.helpText}>系统所有 AI 功能将默认使用此提供商（如需针对不同场景自定义，可在具体功能页面中指定）。</span>
        </div>

        <div className={styles.formGroup}>
          <label>Embedding 提供商</label>
          <select
            name="EMBEDDING_PROVIDER"
            value={configs.EMBEDDING_PROVIDER}
            onChange={handleChange}
            className={styles.select}
          >
            <option value="ollama">Ollama (本地私有化)</option>
            <option value="dashscope">DashScope (阿里云通义千问)</option>
          </select>
          <span className={styles.helpText}>向量化模型可单独选择提供商，默认与对话提供商一致。</span>
        </div>

        <div className={styles.formGroup}>
          <label>Embedding 向量模型</label>
          <input
            type="text"
            name="EMBEDDING_MODEL"
            value={configs.EMBEDDING_MODEL}
            onChange={handleChange}
            className={styles.input}
            placeholder="例如：bge-m3 或 text-embedding-v3"
          />
          <span className={styles.helpText}>用于知识库文档处理的向量化模型。请确保提供商支持此模型。</span>
        </div>

        <div className={styles.formGroup}>
          <label>可用对话模型</label>
          <div className={styles.inlineRow}>
            <input
              type="text"
              value={modelInput}
              onChange={e => setModelInput(e.target.value)}
              className={styles.input}
              placeholder="例如：qwen2.5:7b"
            />
            <button className={styles.button} type="button" onClick={handleAddModel}>添加</button>
          </div>
          {models.length > 0 ? (
            <div className={styles.tagList}>
              {models.map(m => (
                <span key={m} className={styles.tag}>
                  {m}
                  <button type="button" className={styles.tagRemove} onClick={() => handleRemoveModel(m)} aria-label={`移除 ${m}`}>×</button>
                </span>
              ))}
            </div>
          ) : (
            <span className={styles.helpText}>当前还没有添加模型。</span>
          )}
          <span className={styles.helpText}>会保存为系统配置，用于模型选择与测试。</span>
        </div>

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
              value={configs.EMBEDDING_MODEL}
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

      <div className={styles.actions}>
        <button
          className={`${styles.button} ${styles.primaryButton}`}
          onClick={handleSave}
          disabled={saving}
        >
          {saving ? '保存中...' : '保存设置'}
        </button>
      </div>
    </div>
  );
}
