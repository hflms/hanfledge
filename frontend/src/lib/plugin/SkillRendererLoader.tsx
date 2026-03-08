import { lazy, Suspense, ComponentType } from 'react';
import { LoadingState } from '@/components/skill-ui';
import type { SkillRendererProps } from '../types';

/**
 * Dynamic skill renderer loader with code splitting.
 * Lazy loads renderers only when needed.
 */

type RendererLoader = () => Promise<{ default: ComponentType<SkillRendererProps> }>;

const rendererMap: Record<string, RendererLoader> = {
  'socratic_questioning': () => import('../renderers/SocraticRenderer'),
  'general_assessment_quiz': () => import('../renderers/QuizRendererRefactored'),
  'presentation_generator': () => import('../renderers/PresentationRendererRefactored'),
  'learning_survey': () => import('../renderers/LearningSurveyRendererRefactored'),
  'role_play': () => import('../renderers/RolePlayRenderer'),
  'fallacy_detective': () => import('../renderers/FallacyRenderer'),
  'error_diagnosis': () => import('../renderers/ErrorDiagnosisRenderer'),
  'cross_disciplinary': () => import('../renderers/CrossDisciplinaryRenderer'),
  'stepped_learning': () => import('../renderers/SteppedLearningRenderer'),
};

interface SkillRendererLoaderProps extends SkillRendererProps {
  skillId: string;
}

export default function SkillRendererLoader({
  skillId,
  ...props
}: SkillRendererLoaderProps) {
  const loader = rendererMap[skillId];

  if (!loader) {
    return (
      <div style={{ padding: '24px', textAlign: 'center' }}>
        <p>未找到技能渲染器: {skillId}</p>
      </div>
    );
  }

  const Renderer = lazy(loader);

  return (
    <Suspense fallback={<LoadingState message="加载技能组件..." />}>
      <Renderer {...props} />
    </Suspense>
  );
}
