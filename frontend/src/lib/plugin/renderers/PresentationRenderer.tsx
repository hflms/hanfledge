'use client';

/**
 * Presentation Generator Skill Renderer (Refactored).
 * 
 * Uses shared hooks and progressive generation.
 * Phases: generating → viewing
 */

import { useState, useCallback, useEffect, useMemo } from 'react';
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
  const [baseProgress, setBaseProgress] = useState(0);

  // WebSocket handling with progressive updates
  const { send, sending, thinkingStatus, streamingContent } = useAgentChannel(agentChannel, {
    onMessage: (content) => {
      // Try structured output formats first:
      //   1. <skill_output type="presentation">  (unified)
      //   2. <presentation>{JSON}</presentation> (legacy JSON)
      //   3. <skill_output type="presentation_generator"> (progressive.go)
      let slides: string | null = null;

      const parsed = parseSkillOutput<SlidesData>(content, 'presentation');
      if (parsed?.slides) {
        slides = parsed.slides;
      }

      if (!slides) {
        const parsedAlt = parseSkillOutput<SlidesData>(content, 'presentation_generator');
        if (parsedAlt?.slides) {
          slides = parsedAlt.slides;
        }
      }

      // Try raw <slides>...</slides> tags (SKILL.md instructs LLM to use this format)
      if (!slides) {
        const slidesMatch = content.match(/<slides>([\s\S]*?)<\/slides>/);
        if (slidesMatch) {
          slides = slidesMatch[1].trim();
        }
      }

      // Fallback: detect bare Reveal.js markdown (no wrapping tags) using --- separators
      if (!slides) {
        const separatorCount = (content.match(/\n---\n/g) || []).length;
        if (separatorCount >= 2) {
          slides = content.trim();
        }
      }

      if (slides) {
        setSlidesMarkdown(slides);
        transitionTo('viewing');
        
        // Extract any intro text outside the slides tags
        const intro = stripSkillOutput(content, 'presentation')
          .replace(/<slides>[\s\S]*?<\/slides>/g, '')
          .replace(/<skill_output[^>]*>[\s\S]*?<\/skill_output>/g, '')
          .trim();
        if (intro) {
          addMessage({
            id: `coach-${Date.now()}`,
            role: 'coach',
            content: intro,
            timestamp: Date.now(),
          });
        }
      } else {
        // Regular message (no slides detected)
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
        setBaseProgress(20);
      } else if (status.includes('幻灯片')) {
        setBaseProgress(50);
      } else if (status.includes('完善')) {
        setBaseProgress(80);
      }
    },
  });

  // Derive streaming progress from content instead of setting state in an effect.
  const streamingProgress = useMemo(() => {
    if (phase !== 'generating' || !streamingContent) return 0;
    const partialMatch = streamingContent.match(/---/g);
    if (partialMatch) {
      return Math.min(30 + partialMatch.length * 10, 90);
    }
    return 0;
  }, [phase, streamingContent]);

  // Combined progress: max of thinking-based progress and streaming-based progress
  const effectiveProgress = Math.max(baseProgress, streamingProgress);

  // Generate new presentation
  const handleNewPresentation = useCallback(() => {
    setSlidesMarkdown(null);
    setBaseProgress(0);
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
          progress={effectiveProgress}
        >
          <div className={styles.progressHints}>
            {effectiveProgress >= 20 && <p>✓ 大纲已生成</p>}
            {effectiveProgress >= 50 && <p>✓ 幻灯片内容已生成</p>}
            {effectiveProgress >= 80 && <p>✓ 正在完善细节...</p>}
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
