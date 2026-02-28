'use client';

/**
 * Built-in Skill Renderer Registrations.
 *
 * Wraps the Socratic, Fallacy Detective, and Role-Play renderers
 * as plugin registrations so they can be loaded via PluginSlot
 * in the `student.interaction.main` slot on the session page.
 *
 * Usage: Call `useBuiltinSkillRenderers()` inside a component
 * wrapped by `<PluginRegistryProvider>` to auto-register all
 * built-in skill renderers.
 *
 * Follows the same pattern as DashboardPlugins.tsx.
 */

import { useEffect } from 'react';
import {
    usePluginRegistryContext,
    buildSkillRendererRegistration,
} from '@/lib/plugin/PluginRegistry';
import type { SkillUIRenderer } from '@/lib/plugin/types';
import SocraticRenderer from '@/lib/plugin/renderers/SocraticRenderer';
import FallacyRenderer from '@/lib/plugin/renderers/FallacyRenderer';
import RolePlayRenderer from '@/lib/plugin/renderers/RolePlayRenderer';
import QuizRenderer from '@/lib/plugin/renderers/QuizRenderer';

// -- Renderer Definitions ----------------------------------------

const socraticRenderer: SkillUIRenderer = {
    skillId: 'general_concept_socratic',
    metadata: {
        name: '苏格拉底式问答',
        version: '1.0.0',
        description: '多轮对话式教学，通过提问引导学生自主发现知识',
        supportedInteractionModes: ['text'],
    },
    Component: SocraticRenderer,
};

const fallacyRenderer: SkillUIRenderer = {
    skillId: 'general_assessment_fallacy',
    metadata: {
        name: '谬误侦探',
        version: '1.0.0',
        description: '挑战式学习 — 识别、标记并纠正知识性错误',
        supportedInteractionModes: ['text'],
    },
    Component: FallacyRenderer,
};

const rolePlayRenderer: SkillUIRenderer = {
    skillId: 'general_review_roleplay',
    metadata: {
        name: '角色扮演',
        version: '1.0.0',
        description: '沉浸式学习 — 与历史人物或学科专家对话',
        supportedInteractionModes: ['text'],
    },
    Component: RolePlayRenderer,
};

const quizRenderer: SkillUIRenderer = {
    skillId: 'general_assessment_quiz',
    metadata: {
        name: '自动出题',
        version: '1.0.0',
        description: '智能出题 — 根据知识点自动生成选择题和填空题',
        supportedInteractionModes: ['text'],
    },
    Component: QuizRenderer,
};

// -- All Built-in Renderers --------------------------------------

const BUILTIN_RENDERERS: SkillUIRenderer[] = [
    socraticRenderer,
    fallacyRenderer,
    rolePlayRenderer,
    quizRenderer,
];

// -- Registration Hook -------------------------------------------

/**
 * Registers all built-in skill renderers with the plugin registry.
 * Call this once inside a component within `<PluginRegistryProvider>`.
 */
export function useBuiltinSkillRenderers(): void {
    const { register, unregister } = usePluginRegistryContext();

    useEffect(() => {
        const ids: string[] = [];
        for (const renderer of BUILTIN_RENDERERS) {
            const reg = buildSkillRendererRegistration(renderer);
            register(reg);
            ids.push(reg.id);
        }
        return () => {
            for (const id of ids) {
                unregister(id);
            }
        };
    }, [register, unregister]);
}
