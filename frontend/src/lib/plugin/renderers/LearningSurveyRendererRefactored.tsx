'use client';

/**
 * Learning Survey Skill Renderer (Refactored).
 * 
 * Uses shared hooks and components.
 * Phases: welcome → surveying → analyzing → reporting → planning
 */

import { useState, useCallback } from 'react';
import dynamic from 'next/dynamic';
import { ProgressBar, PhaseIndicator, LoadingState } from '@/components/skill-ui';
import { useMessages, useStateMachine, useAgentChannel } from '@/lib/plugin/hooks';
import { parseSkillOutput, stripSkillOutput } from '@/lib/plugin/parsers';
import type { SkillRendererProps } from '@/lib/plugin/types';
import styles from './LearningSurveyRenderer.module.css';

const StructuredMessage = dynamic(() => import('@/components/StructuredMessage'));

// -- Types -------------------------------------------------------

type SurveyPhase = 'welcome' | 'surveying' | 'analyzing' | 'reporting' | 'planning';

interface SurveyQuestion {
  id: number;
  type: 'single_choice' | 'multiple_choice' | 'likert_scale' | 'open_ended';
  stem: string;
  options?: { key: string; text: string }[];
  scale_labels?: string[];
}

interface SurveyBatch {
  dimension: string;
  dimension_label: string;
  questions: SurveyQuestion[];
}

const PHASE_LABELS: Record<SurveyPhase, string> = {
  welcome: '欢迎',
  surveying: '问卷中',
  analyzing: '分析中',
  reporting: '生成报告',
  planning: '制定方案',
};

const PHASE_TRANSITIONS: Record<SurveyPhase, SurveyPhase[]> = {
  welcome: ['surveying'],
  surveying: ['analyzing'],
  analyzing: ['reporting'],
  reporting: ['planning'],
  planning: ['surveying'],
};

// -- Component ---------------------------------------------------

export default function LearningSurveyRendererRefactored({
  agentChannel,
}: SkillRendererProps) {
  const { messages, addMessage, messagesEndRef } = useMessages();
  const { phase, transitionTo } = useStateMachine<SurveyPhase>({
    initialPhase: 'welcome',
    transitions: PHASE_TRANSITIONS,
  });

  const [currentSurvey, setCurrentSurvey] = useState<SurveyBatch | null>(null);
  const [answers, setAnswers] = useState<Record<number, string | string[]>>({});
  const [progress, setProgress] = useState({ completed: 0, total: 6 });

  // WebSocket handling
  const { send, sending, thinkingStatus, streamingContent } = useAgentChannel(agentChannel, {
    onMessage: (content) => {
      const parsed = parseSkillOutput<SurveyBatch>(content, 'survey');
      
      if (parsed && parsed.questions) {
        setCurrentSurvey(parsed);
        setAnswers({});
        transitionTo('surveying');
        
        const intro = stripSkillOutput(content, 'survey');
        if (intro) {
          addMessage({
            id: `coach-${Date.now()}`,
            role: 'coach',
            content: intro,
            timestamp: Date.now(),
          });
        }
      } else {
        addMessage({
          id: `coach-${Date.now()}`,
          role: 'coach',
          content,
          timestamp: Date.now(),
        });
      }
    },
    onScaffoldChange: (data: any) => {
      if (data?.action === 'survey_questions') {
        setProgress({
          completed: data.data?.completed_dims || 0,
          total: data.data?.total_dims || 6,
        });
      } else if (data?.action === 'survey_analysis') {
        transitionTo('analyzing');
      } else if (data?.action === 'learning_profile') {
        transitionTo('reporting');
      } else if (data?.action === 'learning_plan') {
        transitionTo('planning');
      }
    },
  });

  // Submit survey answers
  const handleSubmitSurvey = useCallback(async () => {
    if (!currentSurvey) return;
    
    const answerText = Object.entries(answers)
      .map(([qid, ans]) => `Q${qid}: ${Array.isArray(ans) ? ans.join(', ') : ans}`)
      .join('\n');
    
    await send(`我的回答：\n${answerText}`);
    setCurrentSurvey(null);
    setProgress(prev => ({ ...prev, completed: prev.completed + 1 }));
  }, [currentSurvey, answers, send]);

  // Answer handlers
  const handleSingleChoice = (qid: number, value: string) => {
    setAnswers(prev => ({ ...prev, [qid]: value }));
  };

  const handleMultipleChoice = (qid: number, value: string) => {
    setAnswers(prev => {
      const current = (prev[qid] as string[]) || [];
      const updated = current.includes(value)
        ? current.filter(v => v !== value)
        : [...current, value];
      return { ...prev, [qid]: updated };
    });
  };

  const handleOpenEnded = (qid: number, value: string) => {
    setAnswers(prev => ({ ...prev, [qid]: value }));
  };

  // -- Render ------------------------------------------------------

  return (
    <div className={styles.container}>
      <PhaseIndicator
        phases={['welcome', 'surveying', 'analyzing', 'reporting', 'planning'] as const}
        currentPhase={phase}
        labels={PHASE_LABELS}
      />

      {phase !== 'welcome' && (
        <ProgressBar
          current={progress.completed}
          total={progress.total}
          label="诊断进度"
        />
      )}

      <div className={styles.content}>
        {thinkingStatus && <LoadingState message={thinkingStatus} />}

        {phase === 'surveying' && currentSurvey && (
          <div className={styles.surveyContainer}>
            <h3 className={styles.dimensionTitle}>
              {currentSurvey.dimension_label}
            </h3>

            {currentSurvey.questions.map((q, idx) => (
              <div key={q.id} className={styles.questionBlock}>
                <p className={styles.questionStem}>
                  {idx + 1}. {q.stem}
                </p>

                {q.type === 'single_choice' && q.options && (
                  <div className={styles.options}>
                    {q.options.map(opt => (
                      <label key={opt.key} className={styles.option}>
                        <input
                          type="radio"
                          name={`q${q.id}`}
                          checked={answers[q.id] === opt.key}
                          onChange={() => handleSingleChoice(q.id, opt.key)}
                        />
                        <span>{opt.text}</span>
                      </label>
                    ))}
                  </div>
                )}

                {q.type === 'multiple_choice' && q.options && (
                  <div className={styles.options}>
                    {q.options.map(opt => (
                      <label key={opt.key} className={styles.option}>
                        <input
                          type="checkbox"
                          checked={(answers[q.id] as string[])?.includes(opt.key)}
                          onChange={() => handleMultipleChoice(q.id, opt.key)}
                        />
                        <span>{opt.text}</span>
                      </label>
                    ))}
                  </div>
                )}

                {q.type === 'likert_scale' && q.scale_labels && (
                  <div className={styles.likertScale}>
                    {q.scale_labels.map((label, i) => (
                      <label key={i} className={styles.likertOption}>
                        <input
                          type="radio"
                          name={`q${q.id}`}
                          checked={answers[q.id] === String(i + 1)}
                          onChange={() => handleSingleChoice(q.id, String(i + 1))}
                        />
                        <span>{label}</span>
                      </label>
                    ))}
                  </div>
                )}

                {q.type === 'open_ended' && (
                  <textarea
                    className={styles.openEndedInput}
                    value={(answers[q.id] as string) || ''}
                    onChange={(e) => handleOpenEnded(q.id, e.target.value)}
                    placeholder="请输入你的想法..."
                    rows={3}
                  />
                )}
              </div>
            ))}

            <button
              className={styles.submitBtn}
              onClick={handleSubmitSurvey}
              disabled={Object.keys(answers).length < currentSurvey.questions.length}
            >
              提交答案
            </button>
          </div>
        )}

        <div className={styles.messages}>
          {messages.map(msg => (
            <div key={msg.id} className={`${styles.message} ${styles[msg.role]}`}>
              <StructuredMessage content={msg.content} />
            </div>
          ))}
          <div ref={messagesEndRef} />
        </div>
      </div>
    </div>
  );
}
