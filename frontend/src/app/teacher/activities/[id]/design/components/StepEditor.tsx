'use client';

import { useState, useCallback, useRef } from 'react';

import {
  uploadActivityAsset,
  type SaveStepData,
  type ContentBlock,
  type ContentBlockType,
} from '@/lib/api';
import { useToast } from '@/components/Toast';
import styles from '../page.module.css';

// -- Props --------------------------------------------------------

interface StepEditorProps {
  activityId: number;
  step: SaveStepData;
  index: number;
  totalSteps: number;
  disabled: boolean;
  onUpdate: (updated: SaveStepData) => void;
  onRemove: () => void;
  onMoveUp: () => void;
  onMoveDown: () => void;
}

// -- Helpers -------------------------------------------------------

function parseBlocks(json: string): ContentBlock[] {
  try {
    const parsed: unknown = JSON.parse(json);
    if (Array.isArray(parsed)) return parsed as ContentBlock[];
  } catch {
    // ignore parse errors
  }
  return [];
}

function serializeBlocks(blocks: ContentBlock[]): string {
  return JSON.stringify(blocks);
}

// -- Component ----------------------------------------------------

export default function StepEditor({
  activityId,
  step,
  index,
  totalSteps,
  disabled,
  onUpdate,
  onRemove,
  onMoveUp,
  onMoveDown,
}: StepEditorProps) {
  const [expanded, setExpanded] = useState(true);
  const [uploading, setUploading] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const { toast } = useToast();

  const blocks = parseBlocks(step.content_blocks ?? '[]');

  // -- Field updaters -----------------------------------------------

  const updateField = useCallback(<K extends keyof SaveStepData>(field: K, value: SaveStepData[K]) => {
    onUpdate({ ...step, [field]: value });
  }, [step, onUpdate]);

  const updateBlock = useCallback((blockIndex: number, updated: ContentBlock) => {
    const next = blocks.map((b, i) => (i === blockIndex ? updated : b));
    onUpdate({ ...step, content_blocks: serializeBlocks(next) });
  }, [blocks, step, onUpdate]);

  const removeBlock = useCallback((blockIndex: number) => {
    const next = blocks.filter((_, i) => i !== blockIndex);
    onUpdate({ ...step, content_blocks: serializeBlocks(next) });
  }, [blocks, step, onUpdate]);

  const addBlock = useCallback((type: ContentBlockType) => {
    const newBlock: ContentBlock = { type, content: '' };
    const next = [...blocks, newBlock];
    onUpdate({ ...step, content_blocks: serializeBlocks(next) });
  }, [blocks, step, onUpdate]);

  // -- File Upload ---------------------------------------------------

  const pendingBlockTypeRef = useRef<ContentBlockType>('file');

  const handleFileUpload = useCallback(async (file: File, blockType: ContentBlockType) => {
    setUploading(true);
    try {
      const result = await uploadActivityAsset(activityId, file);
      const newBlock: ContentBlock = {
        type: blockType,
        content: result.file_url,
        file_name: result.file_name,
        file_url: result.file_url,
        mime_type: result.mime_type,
      };
      const next = [...blocks, newBlock];
      onUpdate({ ...step, content_blocks: serializeBlocks(next) });
      toast('文件上传成功', 'success');
    } catch (err) {
      console.error('文件上传失败', err);
      toast('文件上传失败', 'error');
    } finally {
      setUploading(false);
    }
  }, [activityId, blocks, step, onUpdate, toast]);

  const triggerFileInput = useCallback((blockType: ContentBlockType) => {
    pendingBlockTypeRef.current = blockType;
    fileInputRef.current?.click();
  }, []);

  const handleFileInputChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      handleFileUpload(file, pendingBlockTypeRef.current);
    }
    // Reset input
    e.target.value = '';
  }, [handleFileUpload]);

  // -- Render -------------------------------------------------------

  return (
    <div className={`${styles.stepCard} ${expanded ? styles.stepCardExpanded : ''}`}>
      {/* Header */}
      <div className={styles.stepHeader} onClick={() => setExpanded(!expanded)}>
        <span className={styles.stepDragHandle} title="拖拽排序">&#9776;</span>
        <span className={styles.stepNumber}>{index + 1}</span>
        <input
          className={styles.stepTitleInput}
          value={step.title}
          onChange={(e) => updateField('title', e.target.value)}
          onClick={(e) => e.stopPropagation()}
          placeholder={`环节 ${index + 1} 标题`}
          disabled={disabled}
        />
        <div className={styles.stepActions}>
          <button
            className={styles.btnIcon}
            onClick={(e) => { e.stopPropagation(); onMoveUp(); }}
            disabled={disabled || index === 0}
            title="上移"
          >
            &#9650;
          </button>
          <button
            className={styles.btnIcon}
            onClick={(e) => { e.stopPropagation(); onMoveDown(); }}
            disabled={disabled || index === totalSteps - 1}
            title="下移"
          >
            &#9660;
          </button>
          <button
            className={styles.btnIcon}
            onClick={(e) => { e.stopPropagation(); onRemove(); }}
            disabled={disabled}
            title="删除环节"
            style={{ color: 'var(--danger)' }}
          >
            &#10005;
          </button>
        </div>
        <span className={`${styles.stepExpandIcon} ${expanded ? styles.stepExpandIconOpen : ''}`}>
          &#9660;
        </span>
      </div>

      {/* Body (expanded) */}
      {expanded && (
        <div className={styles.stepBody}>
          {/* Description + Duration */}
          <div className={styles.stepMetaRow}>
            <div className={styles.formGroup}>
              <label className={styles.formLabel}>环节描述</label>
              <textarea
                className={styles.formTextarea}
                value={step.description ?? ''}
                onChange={(e) => updateField('description', e.target.value)}
                placeholder="描述这个环节的学习目标..."
                disabled={disabled}
                rows={2}
              />
            </div>
            <div className={styles.formGroup}>
              <label className={styles.formLabel}>时长（分钟）</label>
              <input
                type="number"
                className={styles.durationInput}
                value={step.duration ?? 0}
                min={0}
                onChange={(e) => updateField('duration', Number(e.target.value))}
                disabled={disabled}
              />
            </div>
          </div>

          {/* Content Blocks */}
          <div className={styles.contentBlocksSection}>
            <span className={styles.contentBlocksLabel}>内容块</span>

            {blocks.length > 0 && (
              <div className={styles.contentBlockList}>
                {blocks.map((block, bi) => (
                  <div key={`block-${bi}`} className={styles.contentBlock}>
                    <div className={styles.contentBlockBody}>
                      <span className={styles.contentBlockType}>{block.type}</span>
                      {block.type === 'markdown' ? (
                        <textarea
                          className={styles.contentBlockTextarea}
                          value={block.content}
                          onChange={(e) => updateBlock(bi, { ...block, content: e.target.value })}
                          placeholder="输入 Markdown 内容..."
                          disabled={disabled}
                          rows={3}
                        />
                      ) : (
                        <div className={styles.contentBlockFile}>
                          {block.file_name && (
                            <a
                              className={styles.contentBlockFileLink}
                              href={block.file_url}
                              target="_blank"
                              rel="noopener noreferrer"
                            >
                              {block.file_name}
                            </a>
                          )}
                          {!block.file_name && block.content && (
                            <a
                              className={styles.contentBlockFileLink}
                              href={block.content}
                              target="_blank"
                              rel="noopener noreferrer"
                            >
                              查看文件
                            </a>
                          )}
                        </div>
                      )}
                    </div>
                    {!disabled && (
                      <button
                        className={styles.btnIcon}
                        onClick={() => removeBlock(bi)}
                        title="移除内容块"
                        style={{ color: 'var(--danger)', flexShrink: 0 }}
                      >
                        &#10005;
                      </button>
                    )}
                  </div>
                ))}
              </div>
            )}

            {/* Add content block buttons */}
            {!disabled && (
              <div className={styles.addBlockBtns}>
                <button className={styles.addBlockBtn} onClick={() => addBlock('markdown')}>
                  + Markdown
                </button>
                <button
                  className={styles.addBlockBtn}
                  onClick={() => triggerFileInput('image')}
                  disabled={uploading}
                >
                  + 图片
                </button>
                <button
                  className={styles.addBlockBtn}
                  onClick={() => triggerFileInput('video')}
                  disabled={uploading}
                >
                  + 视频
                </button>
                <button
                  className={styles.addBlockBtn}
                  onClick={() => triggerFileInput('file')}
                  disabled={uploading}
                >
                  + 文件
                </button>
              </div>
            )}

            {uploading && (
              <div style={{ fontSize: '13px', color: 'var(--text-muted)' }}>
                上传中...
              </div>
            )}
          </div>

          {/* Hidden file input */}
          <input
            ref={fileInputRef}
            type="file"
            style={{ display: 'none' }}
            onChange={handleFileInputChange}
            accept="image/*,video/*,.pdf,.docx,.pptx,.xlsx"
          />
        </div>
      )}
    </div>
  );
}
