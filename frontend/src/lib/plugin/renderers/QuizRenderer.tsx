'use client';

/**
 * Quiz Generation Skill Renderer (Refactored).
 * 
 * Uses shared hooks and components for cleaner code.
 * Phases: generating → answering → grading → reviewing
 */

import { useState, useCallback, useEffect } from 'react';
import dynamic from 'next/dynamic';
import { ProgressBar, PhaseIndicator, QuestionCard, LoadingState } from '@/components/skill-ui';
import { useMessages, useStateMachine, useAgentChannel } from '@/lib/plugin/hooks';
import { parseSkillOutput, stripSkillOutput } from '@/lib/plugin/parsers';
import type { SkillRendererProps } from '@/lib/plugin/types';
import styles from './QuizRenderer.module.css';

const MarkdownRenderer = dynamic(() => import('@/components/MarkdownRenderer'));

// -- Types -------------------------------------------------------

type QuizPhase = 'generating' | 'answering' | 'grading' | 'reviewing';

interface QuizOption {
  key: string;
  text: string;
}

interface QuizQuestion {
  id: number;
  type: 'mcq_single' | 'mcq_multiple' | 'fill_blank';
  stem: string;
  options?: QuizOption[];
  answer: string[];
  explanation: string;
}

interface QuizData {
  questions: QuizQuestion[];
}

interface GradedQuestion extends QuizQuestion {
  studentAnswer: string[];
  correct: boolean;
}

const PHASE_LABELS: Record<QuizPhase, string> = {
  generating: '出题中',
  answering: '作答中',
  grading: '批改中',
  reviewing: '查看结果',
};

const PHASE_TRANSITIONS: Record<QuizPhase, QuizPhase[]> = {
  generating: ['answering'],
  answering: ['grading'],
  grading: ['reviewing'],
  reviewing: ['generating'],
};

// -- Component ---------------------------------------------------

export default function QuizRendererRefactored({ agentChannel }: SkillRendererProps) {
  const { messages, addMessage, messagesEndRef } = useMessages();
  const { phase, transitionTo } = useStateMachine<QuizPhase>({
    initialPhase: 'generating',
    transitions: PHASE_TRANSITIONS,
  });

  const [quizData, setQuizData] = useState<QuizData | null>(null);
  const [studentAnswers, setStudentAnswers] = useState<Record<number, string[]>>({});
  const [gradedResults, setGradedResults] = useState<GradedQuestion[]>([]);
  const [score, setScore] = useState<{ correct: number; total: number } | null>(null);

  // WebSocket handling
  const { send, sending, thinkingStatus } = useAgentChannel(agentChannel, {
    onMessage: (content) => {
      // Try parse quiz data
      const parsed = parseSkillOutput<QuizData>(content, 'quiz');
      
      if (parsed && parsed.questions.length > 0) {
        setQuizData(parsed);
        setStudentAnswers({});
        transitionTo('answering');
        
        const intro = stripSkillOutput(content, 'quiz');
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
        
        if (phase === 'grading') {
          transitionTo('reviewing');
        }
      }
    },
  });

  // Submit quiz answers
  const handleSubmitQuiz = useCallback(async () => {
    if (!quizData) return;
    
    // Grade locally
    const graded: GradedQuestion[] = quizData.questions.map(q => {
      const studentAns = studentAnswers[q.id] || [];
      const correct = JSON.stringify(studentAns.sort()) === JSON.stringify(q.answer.sort());
      return { ...q, studentAnswer: studentAns, correct };
    });
    
    setGradedResults(graded);
    const correctCount = graded.filter(q => q.correct).length;
    setScore({ correct: correctCount, total: graded.length });
    
    transitionTo('grading');
    
    // Send to AI for feedback
    const summary = graded.map((q, i) => 
      `题${i + 1}: ${q.correct ? '✓' : '✗'}`
    ).join(', ');
    
    await send(`我已完成作答。${summary}。请给我详细的反馈和解析。`);
  }, [quizData, studentAnswers, send, transitionTo]);

  // Answer handlers
  const handleSingleChoice = (qid: number, key: string) => {
    setStudentAnswers(prev => ({ ...prev, [qid]: [key] }));
  };

  const handleMultipleChoice = (qid: number, key: string) => {
    setStudentAnswers(prev => {
      const current = prev[qid] || [];
      const updated = current.includes(key)
        ? current.filter(k => k !== key)
        : [...current, key];
      return { ...prev, [qid]: updated };
    });
  };

  const handleFillBlank = (qid: number, value: string) => {
    setStudentAnswers(prev => ({ ...prev, [qid]: [value] }));
  };

  // Start new quiz
  const handleNewQuiz = () => {
    setQuizData(null);
    setGradedResults([]);
    setScore(null);
    transitionTo('generating');
    send('请再出一套新题目。');
  };

  // Initial quiz generation
  useEffect(() => {
    if (phase === 'generating' && !quizData && !sending) {
      send('请根据当前知识点生成测验题目。');
    }
  }, [phase, quizData, sending, send]);

  // -- Render ------------------------------------------------------

  return (
    <div className={styles.container}>
      <PhaseIndicator
        phases={['generating', 'answering', 'grading', 'reviewing'] as const}
        currentPhase={phase}
        labels={PHASE_LABELS}
      />

      <div className={styles.messages}>
        {messages.map(msg => (
          <div key={msg.id} className={`${styles.message} ${styles[msg.role]}`}>
            <MarkdownRenderer content={msg.content} />
          </div>
        ))}

        {thinkingStatus && <LoadingState message={thinkingStatus} />}

        {phase === 'answering' && quizData && (
          <div className={styles.quizContainer}>
            <ProgressBar
              current={Object.keys(studentAnswers).length}
              total={quizData.questions.length}
              label="答题进度"
            />

            {quizData.questions.map((q, idx) => (
              <QuestionCard
                key={q.id}
                number={idx + 1}
                stem={q.stem}
                status={studentAnswers[q.id] ? 'answered' : 'unanswered'}
              >
                {q.type === 'mcq_single' && q.options && (
                  <div className={styles.options}>
                    {q.options.map(opt => (
                      <label key={opt.key} className={styles.option}>
                        <input
                          type="radio"
                          name={`q${q.id}`}
                          checked={studentAnswers[q.id]?.[0] === opt.key}
                          onChange={() => handleSingleChoice(q.id, opt.key)}
                        />
                        <span>{opt.key}. {opt.text}</span>
                      </label>
                    ))}
                  </div>
                )}

                {q.type === 'mcq_multiple' && q.options && (
                  <div className={styles.options}>
                    {q.options.map(opt => (
                      <label key={opt.key} className={styles.option}>
                        <input
                          type="checkbox"
                          checked={studentAnswers[q.id]?.includes(opt.key)}
                          onChange={() => handleMultipleChoice(q.id, opt.key)}
                        />
                        <span>{opt.key}. {opt.text}</span>
                      </label>
                    ))}
                  </div>
                )}

                {q.type === 'fill_blank' && (
                  <input
                    type="text"
                    className={styles.fillInput}
                    value={studentAnswers[q.id]?.[0] || ''}
                    onChange={(e) => handleFillBlank(q.id, e.target.value)}
                    placeholder="请输入答案"
                  />
                )}
              </QuestionCard>
            ))}

            <button
              className={styles.submitBtn}
              onClick={handleSubmitQuiz}
              disabled={Object.keys(studentAnswers).length < quizData.questions.length}
            >
              提交答案
            </button>
          </div>
        )}

        {phase === 'reviewing' && gradedResults.length > 0 && score && (
          <div className={styles.results}>
            <div className={styles.scoreCard}>
              <h3>测验结果</h3>
              <div className={styles.scoreDisplay}>
                {score.correct} / {score.total}
              </div>
              <ProgressBar
                current={score.correct}
                total={score.total}
                label="正确率"
              />
            </div>

            {gradedResults.map((q, idx) => (
              <QuestionCard
                key={q.id}
                number={idx + 1}
                stem={q.stem}
                status={q.correct ? 'correct' : 'incorrect'}
              >
                <div className={styles.answerReview}>
                  <p><strong>你的答案：</strong>{q.studentAnswer.join(', ')}</p>
                  <p><strong>正确答案：</strong>{q.answer.join(', ')}</p>
                  <div className={styles.explanation}>
                    <strong>解析：</strong>
                    <MarkdownRenderer content={q.explanation} />
                  </div>
                </div>
              </QuestionCard>
            ))}

            <button className={styles.newQuizBtn} onClick={handleNewQuiz}>
              开始新测验
            </button>
          </div>
        )}

        <div ref={messagesEndRef} />
      </div>

      {phase === 'answering' && (
        <div className={styles.inputDisabled}>
          <p>请先完成答题后再提交...</p>
        </div>
      )}
    </div>
  );
}
