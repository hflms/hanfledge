'use client';

import { useCallback } from 'react';

import type { SaveStepData } from '@/lib/api';
import StepEditor from './StepEditor';
import styles from '../page.module.css';

// -- Props --------------------------------------------------------

interface GuidedStepsTabProps {
  activityId: number;
  steps: SaveStepData[];
  disabled: boolean;
  onStepsChange: (steps: SaveStepData[]) => void;
}

// -- Component ----------------------------------------------------

export default function GuidedStepsTab({
  activityId,
  steps,
  disabled,
  onStepsChange,
}: GuidedStepsTabProps) {
  // -- Add Step ----------------------------------------------------

  const handleAddStep = useCallback(() => {
    const newStep: SaveStepData = {
      title: '',
      description: '',
      sort_order: steps.length,
      content_blocks: '[]',
      duration: 0,
    };
    onStepsChange([...steps, newStep]);
  }, [steps, onStepsChange]);

  // -- Remove Step -------------------------------------------------

  const handleRemoveStep = useCallback((index: number) => {
    const next = steps.filter((_, i) => i !== index);
    onStepsChange(next);
  }, [steps, onStepsChange]);

  // -- Update Step -------------------------------------------------

  const handleUpdateStep = useCallback((index: number, updated: SaveStepData) => {
    const next = steps.map((s, i) => (i === index ? updated : s));
    onStepsChange(next);
  }, [steps, onStepsChange]);

  // -- Move Step (drag-and-drop substitute: up/down arrows) -------

  const handleMoveStep = useCallback((fromIndex: number, toIndex: number) => {
    if (toIndex < 0 || toIndex >= steps.length) return;
    const next = [...steps];
    const [moved] = next.splice(fromIndex, 1);
    next.splice(toIndex, 0, moved);
    onStepsChange(next);
  }, [steps, onStepsChange]);

  return (
    <div className={styles.stepsPanel}>
      {/* Toolbar */}
      <div className={styles.stepsToolbar}>
        <span className={styles.stepsCount}>
          {steps.length > 0 ? `${steps.length} 个环节` : '尚未添加环节'}
        </span>
        {!disabled && (
          <button className={styles.btnSecondary} onClick={handleAddStep}>
            + 添加环节
          </button>
        )}
      </div>

      {/* Steps list */}
      {steps.length === 0 ? (
        <div className={styles.emptySteps}>
          <div className={styles.emptyStepsIcon}>&#128221;</div>
          <div className={styles.emptyStepsText}>
            点击「添加环节」开始设计学习流程
          </div>
          {!disabled && (
            <button className={styles.btnPrimary} onClick={handleAddStep}>
              添加第一个环节
            </button>
          )}
        </div>
      ) : (
        <div className={styles.stepsList}>
          {steps.map((step, index) => (
            <StepEditor
              key={`step-${index}`}
              activityId={activityId}
              step={step}
              index={index}
              totalSteps={steps.length}
              disabled={disabled}
              onUpdate={(updated) => handleUpdateStep(index, updated)}
              onRemove={() => handleRemoveStep(index)}
              onMoveUp={() => handleMoveStep(index, index - 1)}
              onMoveDown={() => handleMoveStep(index, index + 1)}
            />
          ))}
        </div>
      )}
    </div>
  );
}
