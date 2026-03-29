'use client';

import { useState, useCallback, useRef } from 'react';

import {
  uploadActivityAsset,
  suggestStepContent,
  type SaveStepData,
  type ContentBlock,
  type ContentBlockType,
  type StepType,
  type SuggestStepResult,
} from '@/lib/api';
import { useToast } from '@/components/Toast';
import { STEP_TYPES, getStepTypeMeta } from './GuidedStepsTab';
import styles from '../page.module.css';

// -- Props --------------------------------------------------------

interface StepEditorProps {
  activityId: number;
  activityTitle: string;
  step: SaveStepData;
  index: number;
  totalSteps: number;
  disabled: boolean;
  onUpdate: (updated: SaveStepData) => void;
  onRemove: () => void;
  onMoveUp: () => void;
  onMoveDown: () => void;
  onDuplicate: () => void;
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
  activityTitle,
  step,
  index,
  totalSteps,
  disabled,
  onUpdate,
  onRemove,
  onMoveUp,
  onMoveDown,
  onDuplicate,
}: StepEditorProps) {
  const [uploading, setUploading] = useState(false);
  const [showTypeSelector, setShowTypeSelector] = useState(false);
  const [aiLoading, setAiLoading] = useState(false);
  const [aiSuggestion, setAiSuggestion] = useState<SuggestStepResult | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const { toast } = useToast();

  const blocks = parseBlocks(step.content_blocks ?? '[]');
  const meta = getStepTypeMeta(step.step_type ?? 'lecture');

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

  const moveBlock = useCallback((fromIndex: number, toIndex: number) => {
    if (toIndex < 0 || toIndex >= blocks.length) return;
    const next = [...blocks];
    const [moved] = next.splice(fromIndex, 1);
    next.splice(toIndex, 0, moved);
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
    e.target.value = '';
  }, [handleFileUpload]);

  // -- AI Suggest ---------------------------------------------------

  const handleAiSuggest = useCallback(async () => {
    setAiLoading(true);
    setAiSuggestion(null);
    try {
      const result = await suggestStepContent(activityId, {
        step_type: step.step_type ?? 'lecture',
        step_title: step.title,
        step_description: step.description ?? '',
        activity_title: activityTitle,
      });
      setAiSuggestion(result.suggestion);
    } catch (err) {
      console.error('AI 建议生成失败', err);
      toast('AI 建议生成失败，请稍后重试', 'error');
    } finally {
      setAiLoading(false);
    }
  }, [activityId, activityTitle, step.step_type, step.title, step.description, toast]);

  const handleAcceptSuggestion = useCallback(() => {
    if (!aiSuggestion) return;
    const newBlocks: ContentBlock[] = aiSuggestion.content_blocks.map(b => ({
      type: b.type as ContentBlockType,
      content: b.content,
    }));
    onUpdate({
      ...step,
      title: aiSuggestion.title || step.title,
      description: aiSuggestion.description || step.description,
      content_blocks: serializeBlocks(newBlocks),
      duration: aiSuggestion.duration || step.duration,
    });
    setAiSuggestion(null);
    toast('已应用 AI 建议', 'success');
  }, [aiSuggestion, step, onUpdate, toast]);

  const handleDismissSuggestion = useCallback(() => {
    setAiSuggestion(null);
  }, []);

  // -- Render -------------------------------------------------------

  return (
    <div className={styles.editorContainer}>
      {/* Editor Header */}
      <div className={styles.editorHeader}>
        <div className={styles.editorHeaderLeft}>
          <button
            type="button"
            className={styles.editorTypeBadge}
            style={{ color: meta.color, background: `${meta.color}15`, borderColor: `${meta.color}30` }}
            onClick={() => !disabled && setShowTypeSelector(!showTypeSelector)}
            aria-expanded={showTypeSelector}
            aria-label={`环节类型：${meta.label}，点击更改`}
            disabled={disabled}
          >
            <span aria-hidden="true">{meta.icon}</span>
            <span>{meta.label}</span>
            {!disabled && <span className={styles.editorTypeChevron} aria-hidden="true">{'\u25BE'}</span>}
          </button>
          <span className={styles.editorStepNum}>环节 {index + 1} / {totalSteps}</span>
        </div>
        <div className={styles.editorHeaderRight}>
          {!disabled && (
            <button
              className={styles.btnAiSuggest}
              onClick={handleAiSuggest}
              disabled={aiLoading}
              aria-label="AI 建议环节内容"
            >
              {aiLoading ? (
                <span className={styles.aiSpinner} aria-hidden="true" />
              ) : (
                <span aria-hidden="true">{'\u2728'}</span>
              )}
              <span>{aiLoading ? 'AI 生成中\u2026' : 'AI 建议'}</span>
            </button>
          )}
          <button
            className={styles.btnIcon}
            onClick={onMoveUp}
            disabled={disabled || index === 0}
            aria-label="上移环节"
          >
            {'\u25B2'}
          </button>
          <button
            className={styles.btnIcon}
            onClick={onMoveDown}
            disabled={disabled || index === totalSteps - 1}
            aria-label="下移环节"
          >
            {'\u25BC'}
          </button>
          <button
            className={styles.btnIcon}
            onClick={onDuplicate}
            disabled={disabled}
            aria-label="复制环节"
          >
            {'\u2398'}
          </button>
          <button
            className={styles.btnIcon}
            onClick={onRemove}
            disabled={disabled}
            aria-label="删除环节"
            style={{ color: 'var(--danger)' }}
          >
            {'\u2715'}
          </button>
        </div>
      </div>

      {/* Type Selector Dropdown */}
      {showTypeSelector && !disabled && (
        <div className={styles.typeSelectorDropdown}>
          {STEP_TYPES.map(t => (
            <button
              key={t.type}
              className={`${styles.typeSelectorItem} ${t.type === step.step_type ? styles.typeSelectorItemActive : ''}`}
              onClick={() => { updateField('step_type', t.type); setShowTypeSelector(false); }}
            >
              <span className={styles.typeSelectorIcon}>{t.icon}</span>
              <div className={styles.typeSelectorText}>
                <span className={styles.typeSelectorLabel}>{t.label}</span>
                <span className={styles.typeSelectorDesc}>{t.description}</span>
              </div>
            </button>
          ))}
        </div>
      )}

      {/* AI Suggestion Preview */}
      {aiSuggestion && (
        <div className={styles.aiSuggestionPanel} role="region" aria-label="AI 建议预览">
          <div className={styles.aiSuggestionHeader}>
            <span className={styles.aiSuggestionBadge}>{'\u2728'} AI 建议</span>
            <span className={styles.aiSuggestionHint}>预览建议内容，确认后将替换当前环节内容</span>
          </div>
          <div className={styles.aiSuggestionBody}>
            <div className={styles.aiSuggestionField}>
              <span className={styles.aiSuggestionLabel}>标题</span>
              <span className={styles.aiSuggestionValue}>{aiSuggestion.title}</span>
            </div>
            <div className={styles.aiSuggestionField}>
              <span className={styles.aiSuggestionLabel}>描述</span>
              <span className={styles.aiSuggestionValue}>{aiSuggestion.description}</span>
            </div>
            <div className={styles.aiSuggestionField}>
              <span className={styles.aiSuggestionLabel}>时长</span>
              <span className={styles.aiSuggestionValue}>{aiSuggestion.duration} 分钟</span>
            </div>
            {aiSuggestion.content_blocks.map((block, i) => (
              <div key={`ai-block-${i}`} className={styles.aiSuggestionContent}>
                <span className={styles.aiSuggestionLabel}>内容块 {i + 1}</span>
                <pre className={styles.aiSuggestionPre}>{block.content}</pre>
              </div>
            ))}
          </div>
          <div className={styles.aiSuggestionActions}>
            <button
              className={styles.btnPrimary}
              onClick={handleAcceptSuggestion}
              aria-label="应用 AI 建议"
            >
              采纳建议
            </button>
            <button
              className={styles.btnSecondary}
              onClick={handleDismissSuggestion}
              aria-label="忽略 AI 建议"
            >
              忽略
            </button>
            <button
              className={styles.btnSecondary}
              onClick={handleAiSuggest}
              disabled={aiLoading}
              aria-label="重新生成 AI 建议"
            >
              重新生成
            </button>
          </div>
        </div>
      )}

      {/* Editor Body */}
      <div className={styles.editorBody}>
        {/* Title */}
        <div className={styles.formGroup}>
          <label className={styles.formLabel}>环节标题</label>
          <input
            className={styles.formInput}
            value={step.title}
            onChange={(e) => updateField('title', e.target.value)}
            placeholder={`${meta.label}环节标题…`}
            disabled={disabled}
            name="step-title"
            autoComplete="off"
          />
        </div>

        {/* Description + Duration row */}
        <div className={styles.editorMetaRow}>
          <div className={styles.formGroup} style={{ flex: 1 }}>
            <label className={styles.formLabel}>环节描述</label>
            <textarea
              className={styles.formTextarea}
              value={step.description ?? ''}
              onChange={(e) => updateField('description', e.target.value)}
              placeholder="描述这个环节的学习目标和任务要求…"
              disabled={disabled}
              name="step-description"
              rows={3}
            />
          </div>
          <div className={styles.formGroup} style={{ width: '140px', flexShrink: 0 }}>
            <label className={styles.formLabel}>建议时长</label>
            <div className={styles.durationInputWrap}>
              <input
                type="number"
                className={styles.durationInput}
                value={step.duration ?? 0}
                min={0}
                max={180}
                onChange={(e) => updateField('duration', Number(e.target.value))}
                disabled={disabled}
                name="step-duration"
                aria-label="建议时长（分钟）"
              />
              <span className={styles.durationUnit}>分钟</span>
            </div>
          </div>
        </div>

        {/* Content Blocks Section */}
        <div className={styles.contentBlocksSection}>
          <div className={styles.contentBlocksHeader}>
            <span className={styles.contentBlocksLabel}>教学内容</span>
            <span className={styles.contentBlocksCount}>{blocks.length} 个内容块</span>
          </div>

          {blocks.length > 0 && (
            <div className={styles.contentBlockList}>
              {blocks.map((block, bi) => (
                <div key={`block-${bi}`} className={styles.contentBlock}>
                  <div className={styles.contentBlockHead}>
                    <span className={styles.contentBlockType}>
                      {block.type === 'markdown' ? 'Markdown' :
                       block.type === 'image' ? '图片' :
                       block.type === 'video' ? '视频' : '文件'}
                    </span>
                    <div className={styles.contentBlockActions}>
                      <button
                        className={styles.btnIconSm}
                        onClick={() => moveBlock(bi, bi - 1)}
                        disabled={disabled || bi === 0}
                        aria-label="上移内容块"
                      >
                        {'\u25B2'}
                      </button>
                      <button
                        className={styles.btnIconSm}
                        onClick={() => moveBlock(bi, bi + 1)}
                        disabled={disabled || bi === blocks.length - 1}
                        aria-label="下移内容块"
                      >
                        {'\u25BC'}
                      </button>
                      {!disabled && (
                        <button
                          className={styles.btnIconSm}
                          onClick={() => removeBlock(bi)}
                          aria-label="移除内容块"
                          style={{ color: 'var(--danger)' }}
                        >
                          {'\u2715'}
                        </button>
                      )}
                    </div>
                  </div>
                  <div className={styles.contentBlockBody}>
                    {block.type === 'markdown' ? (
                      <textarea
                        className={styles.contentBlockTextarea}
                        value={block.content}
                        onChange={(e) => updateBlock(bi, { ...block, content: e.target.value })}
                        placeholder="输入 Markdown 内容（支持标题、列表、代码块、LaTeX 公式等）…"
                        disabled={disabled}
                        name={`block-content-${bi}`}
                        aria-label={`内容块 ${bi + 1}`}
                        rows={5}
                      />
                    ) : (
                      <div className={styles.contentBlockFile}>
                        {block.type === 'image' && block.file_url && (
                          <div className={styles.contentBlockPreview}>
                            {/* eslint-disable-next-line @next/next/no-img-element */}
                            <img
                              src={block.file_url}
                              alt={block.file_name ?? '上传的图片'}
                              className={styles.contentBlockImage}
                              width={400}
                              height={200}
                              loading="lazy"
                            />
                          </div>
                        )}
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
                </div>
              ))}
            </div>
          )}

          {/* Add content block buttons */}
          {!disabled && (
            <div className={styles.addBlockBtns}>
              <button className={styles.addBlockBtn} onClick={() => addBlock('markdown')}>
                + Markdown 文本
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
                + 附件
              </button>
            </div>
          )}

          {uploading && (
            <div className={styles.uploadingIndicator} aria-live="polite">
              <div className="spinner" /> 上传中{'\u2026'}
            </div>
          )}
        </div>
      </div>

      {/* Hidden file input */}
      <input
        ref={fileInputRef}
        type="file"
        style={{ display: 'none' }}
        onChange={handleFileInputChange}
        accept="image/*,video/*,.pdf,.docx,.pptx,.xlsx"
        aria-label="上传文件"
        tabIndex={-1}
      />
    </div>
  );
}
