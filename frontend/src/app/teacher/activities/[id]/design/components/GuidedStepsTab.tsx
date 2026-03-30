'use client';

import { useState, useCallback, useRef } from 'react';

import type { SaveStepData, StepType } from '@/lib/api';
import StepEditor from './StepEditor';
import styles from '../page.module.css';

// -- Step Type Metadata -------------------------------------------

export interface StepTypeMeta {
  type: StepType;
  icon: string;
  label: string;
  color: string;
  description: string;
}

export const STEP_TYPES: StepTypeMeta[] = [
  { type: 'lecture',     icon: '\uD83D\uDCDA', label: '讲授',     color: '#6c5ce7', description: '教师讲解知识点、播放教学视频或展示课件' },
  { type: 'discussion',  icon: '\uD83D\uDCAC', label: '讨论',     color: '#0984e3', description: '学生围绕主题开展讨论、分享观点' },
  { type: 'quiz',        icon: '\u2753',        label: '测验',     color: '#e17055', description: '检测学生对知识的掌握程度' },
  { type: 'practice',    icon: '\u270D\uFE0F',  label: '练习',     color: '#00b894', description: '学生进行课堂练习和作业' },
  { type: 'reading',     icon: '\uD83D\uDCD6', label: '阅读',     color: '#fdcb6e', description: '阅读指定材料、文献或教科书' },
  { type: 'group_work',  icon: '\uD83D\uDC65', label: '小组协作', color: '#e84393', description: '分组完成协作任务或项目' },
  { type: 'reflection',  icon: '\uD83D\uDCDD', label: '反思总结', color: '#00cec9', description: '回顾学习内容、撰写学习反思' },
  { type: 'ai_tutoring', icon: '\uD83E\uDD16', label: 'AI辅导',  color: '#a29bfe', description: '使用 AI 助教进行个性化辅导' },
];

export function getStepTypeMeta(type: StepType): StepTypeMeta {
  return STEP_TYPES.find(s => s.type === type) ?? STEP_TYPES[0];
}

// -- Step Templates -----------------------------------------------

interface StepTemplate {
  label: string;
  description: string;
  steps: Array<{ title: string; step_type: StepType; description: string; duration: number }>;
}

const TEMPLATES: StepTemplate[] = [
  {
    label: '经典三段式',
    description: '讲授 → 练习 → 测验',
    steps: [
      { title: '知识讲授', step_type: 'lecture', description: '教师讲解核心概念和知识点', duration: 15 },
      { title: '课堂练习', step_type: 'practice', description: '学生运用所学知识完成练习', duration: 20 },
      { title: '随堂测验', step_type: 'quiz', description: '检测学生对本节内容的掌握程度', duration: 10 },
    ],
  },
  {
    label: '翻转课堂',
    description: '阅读 → 讨论 → 练习 → 反思',
    steps: [
      { title: '课前阅读', step_type: 'reading', description: '学生阅读预习材料', duration: 15 },
      { title: '课堂讨论', step_type: 'discussion', description: '针对预习内容进行讨论和答疑', duration: 15 },
      { title: '协作练习', step_type: 'practice', description: '在讨论基础上完成练习', duration: 15 },
      { title: '学习反思', step_type: 'reflection', description: '总结本节所学，撰写学习反思', duration: 10 },
    ],
  },
  {
    label: 'AI 辅助探究',
    description: '讲授 → AI辅导 → 练习 → 反思',
    steps: [
      { title: '知识导入', step_type: 'lecture', description: '教师引入本节主题和核心概念', duration: 10 },
      { title: 'AI 探究辅导', step_type: 'ai_tutoring', description: '学生与 AI 助教进行个性化对话探究', duration: 20 },
      { title: '巩固练习', step_type: 'practice', description: '完成针对性练习巩固知识', duration: 15 },
      { title: '总结反思', step_type: 'reflection', description: '整理学习笔记，总结收获', duration: 10 },
    ],
  },
  {
    label: '项目式学习',
    description: '阅读 → 讨论 → 小组协作 → 反思',
    steps: [
      { title: '背景阅读', step_type: 'reading', description: '阅读项目背景资料和要求', duration: 10 },
      { title: '方案讨论', step_type: 'discussion', description: '讨论项目方案和任务分工', duration: 15 },
      { title: '小组协作', step_type: 'group_work', description: '分组完成项目任务', duration: 25 },
      { title: '成果展示与反思', step_type: 'reflection', description: '展示项目成果，进行互评和反思', duration: 10 },
    ],
  },
];

// -- Props --------------------------------------------------------

interface GuidedStepsTabProps {
  activityId: number;
  activityTitle: string;
  steps: SaveStepData[];
  disabled: boolean;
  onStepsChange: (steps: SaveStepData[]) => void;
}

// -- Component ----------------------------------------------------

export default function GuidedStepsTab({
  activityId,
  activityTitle,
  steps,
  disabled,
  onStepsChange,
}: GuidedStepsTabProps) {
  const [selectedIndex, setSelectedIndex] = useState<number>(0);
  const [showTemplates, setShowTemplates] = useState(false);
  const [showAddMenu, setShowAddMenu] = useState(false);

  // -- Drag & Drop State -------------------------------------------

  const [dragIndex, setDragIndex] = useState<number | null>(null);
  const [dropTargetIndex, setDropTargetIndex] = useState<number | null>(null);
  const dragCounterRef = useRef(0);

  // -- Add Step by type -------------------------------------------

  const handleAddStep = useCallback((stepType: StepType = 'lecture') => {
    const meta = getStepTypeMeta(stepType);
    const newStep: SaveStepData = {
      title: '',
      description: '',
      step_type: stepType,
      sort_order: steps.length,
      content_blocks: '[]',
      duration: stepType === 'quiz' ? 10 : stepType === 'lecture' ? 15 : 20,
    };
    const next = [...steps, newStep];
    onStepsChange(next);
    setSelectedIndex(next.length - 1);
    setShowAddMenu(false);
  }, [steps, onStepsChange]);

  // -- Apply Template ---------------------------------------------

  const handleApplyTemplate = useCallback((template: StepTemplate) => {
    const newSteps: SaveStepData[] = template.steps.map((t, i) => ({
      title: t.title,
      description: t.description,
      step_type: t.step_type,
      sort_order: steps.length + i,
      content_blocks: '[]',
      duration: t.duration,
    }));
    onStepsChange([...steps, ...newSteps]);
    setSelectedIndex(steps.length);
    setShowTemplates(false);
  }, [steps, onStepsChange]);

  // -- Remove Step ------------------------------------------------

  const handleRemoveStep = useCallback((index: number) => {
    const next = steps.filter((_, i) => i !== index);
    onStepsChange(next);
    if (selectedIndex >= next.length) {
      setSelectedIndex(Math.max(0, next.length - 1));
    } else if (selectedIndex > index) {
      setSelectedIndex(selectedIndex - 1);
    }
  }, [steps, onStepsChange, selectedIndex]);

  // -- Update Step ------------------------------------------------

  const handleUpdateStep = useCallback((index: number, updated: SaveStepData) => {
    const next = steps.map((s, i) => (i === index ? updated : s));
    onStepsChange(next);
  }, [steps, onStepsChange]);

  // -- Move Step --------------------------------------------------

  const handleMoveStep = useCallback((fromIndex: number, toIndex: number) => {
    if (toIndex < 0 || toIndex >= steps.length) return;
    const next = [...steps];
    const [moved] = next.splice(fromIndex, 1);
    next.splice(toIndex, 0, moved);
    onStepsChange(next);
    setSelectedIndex(toIndex);
  }, [steps, onStepsChange]);

  // -- Duplicate Step ---------------------------------------------

  const handleDuplicateStep = useCallback((index: number) => {
    const source = steps[index];
    const duplicate: SaveStepData = {
      ...source,
      id: undefined,
      title: source.title ? `${source.title}（副本）` : '',
      sort_order: steps.length,
    };
    const next = [...steps];
    next.splice(index + 1, 0, duplicate);
    onStepsChange(next);
    setSelectedIndex(index + 1);
  }, [steps, onStepsChange]);

  // -- Drag & Drop Handlers ----------------------------------------

  const handleDragStart = useCallback((e: React.DragEvent, index: number) => {
    setDragIndex(index);
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData('text/plain', String(index));
    // Disable text selection during drag
    document.body.style.userSelect = 'none';
    // Make the dragged item semi-transparent
    if (e.currentTarget instanceof HTMLElement) {
      requestAnimationFrame(() => {
        (e.currentTarget as HTMLElement).style.opacity = '0.4';
      });
    }
  }, []);

  const handleDragEnd = useCallback((e: React.DragEvent) => {
    document.body.style.userSelect = '';
    if (e.currentTarget instanceof HTMLElement) {
      e.currentTarget.style.opacity = '';
    }
    setDragIndex(null);
    setDropTargetIndex(null);
    dragCounterRef.current = 0;
  }, []);

  const handleDragEnter = useCallback((e: React.DragEvent, index: number) => {
    e.preventDefault();
    dragCounterRef.current++;
    setDropTargetIndex(index);
  }, []);

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    dragCounterRef.current--;
    if (dragCounterRef.current <= 0) {
      dragCounterRef.current = 0;
      setDropTargetIndex(null);
    }
  }, []);

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.dataTransfer.dropEffect = 'move';
  }, []);

  const handleDrop = useCallback((e: React.DragEvent, toIndex: number) => {
    e.preventDefault();
    dragCounterRef.current = 0;
    const fromIndex = dragIndex;
    setDragIndex(null);
    setDropTargetIndex(null);
    if (fromIndex === null || fromIndex === toIndex) return;
    handleMoveStep(fromIndex, toIndex);
  }, [dragIndex, handleMoveStep]);

  // -- Total duration ---------------------------------------------

  const totalDuration = steps.reduce((sum, s) => sum + (s.duration ?? 0), 0);

  return (
    <div className={styles.guidedLayout}>
      {/* Left: Timeline Sidebar */}
      <div className={styles.timelineSidebar}>
        {/* Header */}
        <div className={styles.timelineHeader}>
          <div className={styles.timelineTitle}>教学流程</div>
          <div className={styles.timelineStats}>
            <span>{steps.length} 个环节</span>
            {totalDuration > 0 && <span> · {totalDuration} 分钟</span>}
          </div>
        </div>

        {/* Timeline Items */}
        <div className={styles.timelineList}>
          {steps.map((step, index) => {
            const meta = getStepTypeMeta(step.step_type ?? 'lecture');
            const isSelected = index === selectedIndex;
            const isDragging = index === dragIndex;
            const isDropTarget = index === dropTargetIndex && dragIndex !== null && dragIndex !== index;
            return (
              <div
                key={`timeline-${index}`}
                className={`${styles.timelineItem} ${isSelected ? styles.timelineItemSelected : ''} ${isDragging ? styles.timelineItemDragging : ''} ${isDropTarget ? styles.timelineItemDropTarget : ''}`}
                role="button"
                tabIndex={0}
                aria-pressed={isSelected}
                aria-label={`${step.title || `${meta.label} ${index + 1}`}，${meta.label}，${(step.duration ?? 0) > 0 ? `${step.duration} 分钟` : ''}`}
                onClick={() => setSelectedIndex(index)}
                onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); setSelectedIndex(index); } }}
                draggable={!disabled}
                onDragStart={(e) => handleDragStart(e, index)}
                onDragEnd={handleDragEnd}
                onDragEnter={(e) => handleDragEnter(e, index)}
                onDragLeave={handleDragLeave}
                onDragOver={handleDragOver}
                onDrop={(e) => handleDrop(e, index)}
              >
                <div className={styles.timelineConnector}>
                  <div
                    className={styles.timelineDot}
                    style={{ background: meta.color }}
                  >
                    <span className={styles.timelineDotIcon}>{meta.icon}</span>
                  </div>
                  {index < steps.length - 1 && <div className={styles.timelineLine} />}
                </div>
                <div className={styles.timelineContent}>
                  <div className={styles.timelineItemTitle}>
                    {step.title || `${meta.label} ${index + 1}`}
                  </div>
                  <div className={styles.timelineItemMeta}>
                    <span className={styles.timelineTag} style={{ color: meta.color, background: `${meta.color}15` }}>
                      {meta.label}
                    </span>
                    {(step.duration ?? 0) > 0 && (
                      <span className={styles.timelineDuration}>{step.duration} 分钟</span>
                    )}
                  </div>
                </div>
                {!disabled && (
                  <div className={styles.timelineDragHandle} aria-hidden="true">&#x2807;</div>
                )}
              </div>
            );
          })}
        </div>

        {/* Add Step Actions */}
        {!disabled && (
          <div className={styles.timelineActions}>
            <div className={styles.addStepGroup}>
              <button
                className={styles.btnAddStep}
                onClick={() => setShowAddMenu(!showAddMenu)}
                aria-expanded={showAddMenu}
              >
                + 添加环节
              </button>
              <button
                className={styles.btnTemplate}
                onClick={() => setShowTemplates(!showTemplates)}
                aria-expanded={showTemplates}
              >
                模板
              </button>
            </div>

            {/* Add Step Type Menu */}
            {showAddMenu && (
              <div className={styles.addStepMenu}>
                {STEP_TYPES.map(meta => (
                  <button
                    key={meta.type}
                    className={styles.addStepMenuItem}
                    onClick={() => handleAddStep(meta.type)}
                  >
                    <span className={styles.addStepMenuIcon}>{meta.icon}</span>
                    <div className={styles.addStepMenuText}>
                      <span className={styles.addStepMenuLabel}>{meta.label}</span>
                      <span className={styles.addStepMenuDesc}>{meta.description}</span>
                    </div>
                  </button>
                ))}
              </div>
            )}

            {/* Template Picker */}
            {showTemplates && (
              <div className={styles.templatePicker}>
                <div className={styles.templatePickerTitle}>快速套用模板</div>
                {TEMPLATES.map((t, i) => (
                  <button
                    key={`tpl-${i}`}
                    className={styles.templateCard}
                    onClick={() => handleApplyTemplate(t)}
                  >
                    <div className={styles.templateCardLabel}>{t.label}</div>
                    <div className={styles.templateCardDesc}>{t.description}</div>
                    <div className={styles.templateCardSteps}>
                      {t.steps.map((s, si) => {
                        const sm = getStepTypeMeta(s.step_type);
                        return (
                          <span key={si} className={styles.templateStepBadge} style={{ color: sm.color, background: `${sm.color}15` }}>
                            {sm.icon} {sm.label}
                          </span>
                        );
                      })}
                    </div>
                  </button>
                ))}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Right: Step Editor Panel */}
      <div className={styles.editorPanel}>
        {steps.length === 0 ? (
          <div className={styles.emptyEditor}>
            <div className={styles.emptyEditorIcon}>{'\uD83C\uDFD7\uFE0F'}</div>
            <div className={styles.emptyEditorTitle}>开始设计教学流程</div>
            <div className={styles.emptyEditorText}>
              从左侧添加教学环节，或使用模板快速创建课程流程。
              支持讲授、讨论、测验、练习、阅读、小组协作、反思总结和 AI 辅导等多种环节类型。
            </div>
            {!disabled && (
              <div className={styles.emptyEditorActions}>
                <button className={styles.btnPrimary} onClick={() => handleAddStep('lecture')}>
                  + 添加第一个环节
                </button>
                <button className={styles.btnSecondary} onClick={() => setShowTemplates(true)}>
                  使用模板
                </button>
              </div>
            )}
          </div>
        ) : steps[selectedIndex] ? (
          <StepEditor
            key={`editor-${selectedIndex}`}
            activityId={activityId}
            activityTitle={activityTitle}
            step={steps[selectedIndex]}
            index={selectedIndex}
            totalSteps={steps.length}
            disabled={disabled}
            onUpdate={(updated) => handleUpdateStep(selectedIndex, updated)}
            onRemove={() => handleRemoveStep(selectedIndex)}
            onMoveUp={() => handleMoveStep(selectedIndex, selectedIndex - 1)}
            onMoveDown={() => handleMoveStep(selectedIndex, selectedIndex + 1)}
            onDuplicate={() => handleDuplicateStep(selectedIndex)}
          />
        ) : null}
      </div>
    </div>
  );
}
