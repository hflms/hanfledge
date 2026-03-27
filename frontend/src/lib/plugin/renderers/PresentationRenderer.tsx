'use client';

/**
 * Presentation Generator Skill Renderer (Refactored).
 * 
 * Uses shared hooks and progressive generation.
 * Phases: generating → viewing
 */

import { useState, useCallback, useEffect } from 'react';
import dynamic from 'next/dynamic';
import { LoadingState, PhaseIndicator } from '@/components/skill-ui';
import { useMessages, useStateMachine, useAgentChannel } from '@/lib/plugin/hooks';
import { parseSkillOutput, stripSkillOutput } from '@/lib/plugin/parsers';
import type { SkillRendererProps } from '@/lib/plugin/types';
import styles from './PresentationRenderer.module.css';

const RevealDeck = dynamic(() => import('@/components/RevealDeck'), { ssr: false });
const MarkdownRenderer = dynamic(() => import('@/components/MarkdownRenderer'));

// -- Types -------------------------------------------------------

type PresentationPhase = 'generating' | 'viewing';

interface SlidesData {
  slides: string;
  metadata?: {
    slide_count?: number;
    estimated_duration?: number;
  };
}

const PHASE_LABELS: Record<PresentationPhase, string> = {
  generating: '生成中',
  viewing: '查看',
};

const PHASE_TRANSITIONS: Record<PresentationPhase, PresentationPhase[]> = {
  generating: ['viewing'],
  viewing: ['generating'],
};

// -- Component ---------------------------------------------------

export default function PresentationRendererRefactored({
  agentChannel,
  knowledgePoint,
  initialMessages,
}: SkillRendererProps) {
  const { messages, addMessage, messagesEndRef } = useMessages({
    initialMessages: initialMessages as Array<{ role: 'student' | 'coach' | 'system' | 'teacher'; content: string; id: string; timestamp: number }>,
  });
  const { phase, transitionTo } = useStateMachine<PresentationPhase>({
    initialPhase: 'generating',
    transitions: PHASE_TRANSITIONS,
  });

  const [slidesMarkdown, setSlidesMarkdown] = useState<string | null>(null);
  const [generationProgress, setGenerationProgress] = useState(0);

  // WebSocket handling with progressive updates
  const { send, sending, thinkingStatus, streamingContent } = useAgentChannel(agentChannel, {
    onMessage: (content) => {
      // Try parse slides data
      const parsed = parseSkillOutput<SlidesData>(content, 'presentation');
      
      if (parsed && parsed.slides) {
        setSlidesMarkdown(parsed.slides);
        transitionTo('viewing');
        
        const intro = stripSkillOutput(content, 'presentation');
        if (intro) {
          addMessage({
            id: `coach-${Date.now()}`,
            role: 'coach',
            content: intro,
            timestamp: Date.now(),
          });
        }
      } else {
        // Regular message
        addMessage({
          id: `coach-${Date.now()}`,
          role: 'coach',
          content,
          timestamp: Date.now(),
        });
      }
    },
    onThinking: (status) => {
      // Update progress based on thinking status
      if (status.includes('大纲')) {
        setGenerationProgress(20);
      } else if (status.includes('幻灯片')) {
        setGenerationProgress(50);
      } else if (status.includes('完善')) {
        setGenerationProgress(80);
      }
    },
  });

  // Progressive generation simulation
  useEffect(() => {
    let timeout: NodeJS.Timeout;
    if (phase === 'generating' && streamingContent) {
      timeout = setTimeout(() => {
        // Check for partial slides in streaming content
        const partialMatch = streamingContent.match(/---/g);
        if (partialMatch) {
          const slideCount = partialMatch.length;
          setGenerationProgress(Math.min(30 + slideCount * 10, 90));
        }
      }, 0);
    }
    return () => clearTimeout(timeout);
  }, [phase, streamingContent]);

  // Generate new presentation
  const handleNewPresentation = useCallback(() => {
    setSlidesMarkdown(null);
    setGenerationProgress(0);
    transitionTo('generating');
    send(`请为知识点"${knowledgePoint.title}"生成一份新的演示文稿。`);
  }, [knowledgePoint.title, send, transitionTo]);

  // Restore from history or Initial generation
  useEffect(() => {
    let timeout: NodeJS.Timeout;
    if (phase === 'generating' && !slidesMarkdown && !sending) {
      timeout = setTimeout(() => {
        // Look for a past presentation in history
        const pastPresentation = [...messages].reverse().find(m => {
          if (m.role === 'coach') {
            const parsed = parseSkillOutput<SlidesData>(m.content, 'presentation');
            return parsed && parsed.slides;
          }
          return false;
        });

        if (pastPresentation) {
          const parsed = parseSkillOutput<SlidesData>(pastPresentation.content, 'presentation');
          if (parsed && parsed.slides) {
            setSlidesMarkdown(parsed.slides);
            transitionTo('viewing');
            return;
          }
        }

        send(`请为知识点"${knowledgePoint.title}"生成演示文稿。要求：
- 5-8 张幻灯片
- 包含标题、要点、示例、总结
- 使用 Reveal.js Markdown 格式
- 每张幻灯片用 --- 分隔`);
      }, 0);
    }
    return () => clearTimeout(timeout);
  }, [phase, slidesMarkdown, sending, send, knowledgePoint.title, messages, transitionTo]);

  // -- Render ------------------------------------------------------

  return (
    <div className={styles.container}>
      <PhaseIndicator
        phases={['generating', 'viewing'] as const}
        currentPhase={phase}
        labels={PHASE_LABELS}
      />

      {phase === 'generating' && (
        <LoadingState
          message={thinkingStatus || '正在生成演示文稿...'}
          progress={generationProgress}
        >
          <div className={styles.progressHints}>
            {generationProgress >= 20 && <p>✓ 大纲已生成</p>}
            {generationProgress >= 50 && <p>✓ 幻灯片内容已生成</p>}
            {generationProgress >= 80 && <p>✓ 正在完善细节...</p>}
          </div>
        </LoadingState>
      )}

      {phase === 'viewing' && slidesMarkdown && (
        <div className={styles.viewerContainer}>
          <div className={styles.controls}>
            <button className={styles.regenerateBtn} onClick={handleNewPresentation}>
              重新生成
            </button>
          </div>

          <div className={styles.deckWrapper}>
            <RevealDeck markdown={slidesMarkdown} />
          </div>

          <div className={styles.tips}>
            <p>💡 使用方向键或点击切换幻灯片</p>
            <p>按 F 键全屏查看</p>
          </div>
        </div>
      )}

      <div className={styles.messages}>
        {messages.map(msg => (
          <div key={msg.id} className={`${styles.message} ${styles[msg.role]}`}>
            <MarkdownRenderer content={msg.content} />
          </div>
        ))}
        <div ref={messagesEndRef} />
      </div>
    </div>
  );
}
