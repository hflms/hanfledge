'use client';

import { useState, useEffect } from 'react';
import { apiFetch } from '@/lib/api';
import styles from './page.module.css';

interface SoulVersion {
  id: number;
  version: string;
  content: string;
  updated_by: number;
  reason: string;
  is_active: boolean;
  created_at: string;
}

export default function SoulPage() {
  const [content, setContent] = useState('');
  const [version, setVersion] = useState('');
  const [editing, setEditing] = useState(false);
  const [reason, setReason] = useState('');
  const [history, setHistory] = useState<SoulVersion[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadSoul();
    loadHistory();
  }, []);

  const loadSoul = async () => {
    try {
      const data = await apiFetch<{ content: string; version: string }>('/system/soul');
      setContent(data.content);
      setVersion(data.version);
    } catch (err) {
      alert('加载失败');
    } finally {
      setLoading(false);
    }
  };

  const loadHistory = async () => {
    try {
      const data = await apiFetch<{ versions: SoulVersion[] }>('/system/soul/history');
      setHistory(data.versions);
    } catch (err) {
      console.error('加载历史失败', err);
    }
  };

  const handleSave = async () => {
    if (!reason.trim()) {
      alert('请填写修改原因');
      return;
    }

    try {
      await apiFetch('/system/soul', {
        method: 'PUT',
        body: JSON.stringify({ content, reason }),
      });
      alert('保存成功');
      setEditing(false);
      setReason('');
      loadSoul();
      loadHistory();
    } catch (err) {
      alert('保存失败');
    }
  };

  const handleRollback = async (versionId: number) => {
    if (!confirm('确定要回滚到此版本吗？')) return;

    try {
      await apiFetch('/system/soul/rollback', {
        method: 'POST',
        body: JSON.stringify({ version_id: versionId }),
      });
      alert('回滚成功');
      loadSoul();
      loadHistory();
    } catch (err) {
      alert('回滚失败');
    }
  };

  if (loading) return <div className={styles.loading}>加载中...</div>;

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1>🧠 Soul — AI 教学核心规则</h1>
        <div className={styles.meta}>
          <span>当前版本: {version}</span>
          {!editing && <button onClick={() => setEditing(true)}>编辑</button>}
        </div>
      </div>

      <div className={styles.content}>
        {editing ? (
          <div className={styles.editor}>
            <textarea
              value={content}
              onChange={(e) => setContent(e.target.value)}
              className={styles.textarea}
            />
            <div className={styles.editorActions}>
              <input
                type="text"
                placeholder="修改原因（必填）"
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                className={styles.reasonInput}
              />
              <button onClick={handleSave} className={styles.saveBtn}>保存</button>
              <button onClick={() => setEditing(false)} className={styles.cancelBtn}>取消</button>
            </div>
          </div>
        ) : (
          <div className={styles.preview}>
            <pre>{content}</pre>
          </div>
        )}
      </div>

      <div className={styles.history}>
        <h2>版本历史</h2>
        <div className={styles.versions}>
          {history.map((v) => (
            <div key={v.id} className={styles.versionCard}>
              <div className={styles.versionHeader}>
                <span className={styles.versionNum}>{v.version}</span>
                {v.is_active && <span className={styles.activeBadge}>当前</span>}
                <span className={styles.versionDate}>
                  {new Date(v.created_at).toLocaleString('zh-CN')}
                </span>
              </div>
              <div className={styles.versionReason}>{v.reason || '无说明'}</div>
              {!v.is_active && (
                <button
                  onClick={() => handleRollback(v.id)}
                  className={styles.rollbackBtn}
                >
                  回滚
                </button>
              )}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
