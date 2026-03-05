/**
 * Skill Manifest Loader — Manifest-driven plugin discovery.
 *
 * Defines typed manifest data for each skill plugin and resolves
 * them to SkillUIRenderer entries, replacing the previously hardcoded
 * BUILTIN_RENDERERS array.
 *
 * Architecture: The canonical manifest.json files live at
 * plugins/skills/{name}/frontend/manifest.json (used by backend tooling).
 * This module mirrors that data as typed PluginManifest constants so the
 * frontend build stays within its project boundary (Next.js / Turbopack
 * cannot resolve imports outside the frontend/ root).
 *
 * When adding a new skill, update BOTH the manifest.json file AND the
 * corresponding entry here. See design.md 7.15-7.16.
 *
 * For core/domain plugins (trust_level: "domain"), components are bundled
 * directly. Community plugins would use iframe isolation (future).
 */

import type { PluginManifest, SkillUIRenderer } from './types';
import type { FC } from 'react';
import type { SkillRendererProps } from './types';

// -- Manifest Data (mirrors plugins/skills/{name}/frontend/manifest.json) --

const socraticManifest: PluginManifest = {
  id: 'socratic-renderer',
  name: '苏格拉底式问答渲染器',
  version: '1.0.0',
  type: 'skill_renderer',
  skillId: 'general_concept_socratic',
  trust_level: 'domain',
  author: 'hanfledge-team',
  description: '多轮对话式教学，通过提问引导学生自主发现知识',
  entry: '@/lib/plugin/renderers/SocraticRenderer',
  slots: ['student.interaction.main'],
  supported_interaction_modes: ['text'],
  permissions: ['getStudentContext', 'getKnowledgePoint', 'sendMessageToAgent', 'reportInteractionEvent'],
};

const fallacyManifest: PluginManifest = {
  id: 'fallacy-detective-renderer',
  name: '谬误侦探交互渲染器',
  version: '1.0.0',
  type: 'skill_renderer',
  skillId: 'general_assessment_fallacy',
  trust_level: 'domain',
  author: 'hanfledge-team',
  description: '挑战式学习 — 识别、标记并纠正知识性错误',
  entry: '@/lib/plugin/renderers/FallacyRenderer',
  slots: ['student.interaction.main'],
  supported_interaction_modes: ['text'],
  permissions: ['getStudentContext', 'getKnowledgePoint', 'sendMessageToAgent', 'reportInteractionEvent'],
};

const rolePlayManifest: PluginManifest = {
  id: 'role-play-renderer',
  name: '角色扮演渲染器',
  version: '1.0.0',
  type: 'skill_renderer',
  skillId: 'general_review_roleplay',
  trust_level: 'domain',
  author: 'hanfledge-team',
  description: '沉浸式学习 — 与历史人物或学科专家对话',
  entry: '@/lib/plugin/renderers/RolePlayRenderer',
  slots: ['student.interaction.main'],
  supported_interaction_modes: ['text'],
  permissions: ['getStudentContext', 'getKnowledgePoint', 'sendMessageToAgent', 'reportInteractionEvent'],
};

const quizManifest: PluginManifest = {
  id: 'quiz-renderer',
  name: '自动出题渲染器',
  version: '1.0.0',
  type: 'skill_renderer',
  skillId: 'general_assessment_quiz',
  trust_level: 'domain',
  author: 'hanfledge-team',
  description: '智能出题 — 根据知识点自动生成选择题和填空题',
  entry: '@/lib/plugin/renderers/QuizRenderer',
  slots: ['student.interaction.main'],
  supported_interaction_modes: ['text'],
  permissions: ['getStudentContext', 'getKnowledgePoint', 'sendMessageToAgent', 'reportInteractionEvent'],
};

const practiceQuizManifest: PluginManifest = {
  id: 'practice-quiz-renderer',
  name: '自动出题渲染器',
  version: '1.0.0',
  type: 'skill_renderer',
  skillId: 'general_practice_quiz',
  trust_level: 'domain',
  author: 'hanfledge-team',
  description: '智能出题 — 根据知识点自动生成选择题和填空题',
  entry: '@/lib/plugin/renderers/QuizRenderer',
  slots: ['student.interaction.main'],
  supported_interaction_modes: ['text'],
  permissions: ['getStudentContext', 'getKnowledgePoint', 'sendMessageToAgent', 'reportInteractionEvent'],
};

const presentationManifest: PluginManifest = {
  id: 'presentation-renderer',
  name: '演示文稿生成渲染器',
  version: '1.0.0',
  type: 'skill_renderer',
  skillId: 'general_creation_presentation',
  trust_level: 'domain',
  author: 'hanfledge-team',
  description: '演示文稿生成 — 根据章节知识点自动生成 Markdown 格式的课堂演示文稿',
  entry: '@/lib/plugin/renderers/PresentationRenderer',
  slots: ['student.interaction.main'],
  supported_interaction_modes: ['text'],
  permissions: ['getStudentContext', 'getKnowledgePoint', 'sendMessageToAgent', 'reportInteractionEvent'],
};

const steppedLearningManifest: PluginManifest = {
  id: 'stepped-learning-renderer',
  name: '闯关/步进式学习渲染器',
  version: '1.0.0',
  type: 'skill_renderer',
  skillId: 'general_interaction_stepped',
  trust_level: 'domain',
  author: 'hanfledge-team',
  description: '页面切换 — 以类PPT分布引导的方式带学生一步步闯关学习',
  entry: '@/lib/plugin/renderers/SteppedLearningRenderer',
  slots: ['student.interaction.main'],
  supported_interaction_modes: ['page'],
  permissions: ['getStudentContext', 'getKnowledgePoint', 'sendMessageToAgent', 'reportInteractionEvent'],
};

// -- Component Registry (maps skillId -> React component) --------
//
// Each manifest above has an "entry" field pointing to the renderer
// module path. Since Next.js/webpack cannot resolve dynamic paths
// at build time, we maintain a static registry here.
// Adding a new skill requires:
//   1. Creating plugins/skills/{name}/frontend/manifest.json
//   2. Adding a typed manifest constant above (mirroring the JSON)
//   3. Adding the renderer component import below
//   4. Adding the skillId -> Component entry to COMPONENT_REGISTRY

import SocraticRenderer from './renderers/SocraticRenderer';
import FallacyRenderer from './renderers/FallacyRenderer';
import RolePlayRenderer from './renderers/RolePlayRenderer';
import QuizRenderer from './renderers/QuizRenderer';
import ErrorDiagnosisRenderer from './renderers/ErrorDiagnosisRenderer';
import CrossDisciplinaryRenderer from './renderers/CrossDisciplinaryRenderer';
import LearningSurveyRenderer from './renderers/LearningSurveyRenderer';
import PresentationRenderer from './renderers/PresentationRenderer';
import SteppedLearningRenderer from './renderers/SteppedLearningRenderer';

const COMPONENT_REGISTRY: Record<string, FC<SkillRendererProps>> = {
  general_concept_socratic: SocraticRenderer,
  general_assessment_fallacy: FallacyRenderer,
  general_review_roleplay: RolePlayRenderer,
  general_assessment_quiz: QuizRenderer,
  general_practice_quiz: QuizRenderer,
  general_diagnosis_error: ErrorDiagnosisRenderer as unknown as FC<SkillRendererProps>,
  general_synthesis_crosslink: CrossDisciplinaryRenderer as unknown as FC<SkillRendererProps>,
  general_diagnosis_survey: LearningSurveyRenderer,
  general_creation_presentation: PresentationRenderer,
  general_interaction_stepped: SteppedLearningRenderer as unknown as FC<SkillRendererProps>,
};

const MISSING_RENDERER_SKILLS: string[] = [];

const errorDiagnosisManifest: PluginManifest = {
  id: 'general_diagnosis_error',
  name: '错误诊断技能',
  version: '1.0.0',
  type: 'skill_renderer',
  skillId: 'general_diagnosis_error',
  trust_level: 'core',
  author: 'hanfledge-team',
  description: '自动分析学生错误原因，定位知识缺口，生成补救路径',
  entry: './renderers/ErrorDiagnosisRenderer',
  slots: ['student.interaction.main'],
  permissions: ['getStudentContext', 'getKnowledgePoint'],
};

const crossDisciplinaryManifest: PluginManifest = {
  id: 'general_synthesis_crosslink',
  name: '跨学科关联技能',
  version: '1.0.0',
  type: 'skill_renderer',
  skillId: 'general_synthesis_crosslink',
  trust_level: 'core',
  author: 'hanfledge-team',
  description: '发现并展示跨学科概念关联，促进知识迁移',
  entry: './renderers/CrossDisciplinaryRenderer',
  slots: ['student.interaction.main'],
  permissions: ['getStudentContext', 'getKnowledgePoint'],
};

const learningSurveyManifest: PluginManifest = {
  id: 'learning-survey-renderer',
  name: '学情问卷诊断渲染器',
  version: '1.0.0',
  type: 'skill_renderer',
  skillId: 'general_diagnosis_survey',
  trust_level: 'domain',
  author: 'hanfledge-team',
  description: '学情诊断 — 通过结构化问卷生成学生学习画像',
  entry: '@/lib/plugin/renderers/LearningSurveyRenderer',
  slots: ['student.interaction.main'],
  supported_interaction_modes: ['text'],
  permissions: ['getStudentContext', 'getKnowledgePoint', 'sendMessageToAgent', 'reportInteractionEvent'],
};

// -- All Discovered Manifests ----------------------------------------

/**
 * All skill manifest.json files discovered from the plugins directory.
 * Typed as PluginManifest for validation.
 */
export const SKILL_MANIFESTS: PluginManifest[] = [
  socraticManifest,
  fallacyManifest,
  rolePlayManifest,
  quizManifest,
  practiceQuizManifest,
  presentationManifest,
  steppedLearningManifest,
  errorDiagnosisManifest,
  crossDisciplinaryManifest,
  learningSurveyManifest,
];

// -- Manifest -> SkillUIRenderer Conversion --------------------------

/**
 * Resolves a manifest into a SkillUIRenderer by looking up the component
 * in the registry. Returns null if no matching component is found.
 */
function resolveManifest(manifest: PluginManifest): SkillUIRenderer | null {
  const skillId = manifest.skillId;
  if (!skillId) {
    console.warn(`[PluginLoader] Manifest ${manifest.id} has no skillId, skipping`);
    return null;
  }

  const Component = COMPONENT_REGISTRY[skillId];
  if (!Component) {
    MISSING_RENDERER_SKILLS.push(skillId);
    console.warn(`[PluginLoader] No component registered for skillId: ${skillId}`);
    return null;
  }

  return {
    skillId,
    metadata: {
      name: manifest.name,
      version: manifest.version,
      description: manifest.description,
      supportedInteractionModes: manifest.supported_interaction_modes || ['text'],
    },
    Component,
  };
}

/**
 * All skill UI renderers resolved from manifest files.
 * This replaces the previously hardcoded BUILTIN_RENDERERS array.
 */
export const MANIFEST_RENDERERS: SkillUIRenderer[] = SKILL_MANIFESTS
  .map(resolveManifest)
  .filter((r): r is SkillUIRenderer => r !== null);

export function getMissingRendererSkillIds(): string[] {
  return Array.from(new Set(MISSING_RENDERER_SKILLS));
}

/**
 * Look up a renderer by its skillId.
 */
export function getRendererBySkillId(skillId: string): SkillUIRenderer | undefined {
  return MANIFEST_RENDERERS.find(r => r.skillId === skillId);
}

/**
 * Get all registered skill IDs.
 */
export function getRegisteredSkillIds(): string[] {
  return MANIFEST_RENDERERS.map(r => r.skillId);
}
