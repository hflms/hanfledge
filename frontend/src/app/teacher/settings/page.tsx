'use client';

import React, { useState, useEffect, useCallback } from 'react';
import { apiFetch } from '@/lib/api';
import { useToast } from '@/components/Toast';
import styles from './page.module.css';

export default function SystemSettingsPage() {
  const { toast } = useToast();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [configs, setConfigs] = useState({
    LLM_PROVIDER: 'ollama',
    OLLAMA_BASE_URL: 'http://localhost:11434',
    OLLAMA_MODEL: 'qwen2.5:7b',
    DASHSCOPE_API_KEY: '',
    DASHSCOPE_MODEL: 'qwen-max',
    EMBEDDING_MODEL: 'bge-m3'
  });

  const loadConfigs = useCallback(async () => {
    try {
      const data = await apiFetch<Record<string, string>>('/system/config');
      if (data && Object.keys(data).length > 0) {
        setConfigs(prev => ({ ...prev, ...data }));
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
      await apiFetch('/system/config', {
        method: 'PUT',
        body: JSON.stringify(configs)
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
