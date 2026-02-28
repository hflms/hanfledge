'use client';

/**
 * Built-in Dashboard Widget Registrations.
 *
 * Wraps the existing RadarChart, MasteryBarChart, and
 * SkillEffectivenessChart as plugin registrations so they
 * can be loaded via PluginSlot in the dashboard page.
 *
 * Usage: Call `useBuiltinDashboardPlugins()` inside a component
 * wrapped by `<PluginRegistryProvider>` to auto-register all
 * built-in dashboard widgets.
 */

import { useEffect } from 'react';
import {
  usePluginRegistryContext,
  buildDashboardWidgetRegistration,
} from '@/lib/plugin/PluginRegistry';
import type { DashboardWidgetPlugin, DashboardWidgetContext } from '@/lib/plugin/types';
import {
  getKnowledgeRadar,
  getSkillEffectiveness,
  type KnowledgeRadarData,
  type SkillEffectivenessResponse,
} from '@/lib/api';
import RadarChart from '@/app/teacher/dashboard/RadarChart';
import MasteryBarChart from '@/app/teacher/dashboard/MasteryBarChart';
import SkillEffectivenessChart from '@/app/teacher/dashboard/SkillEffectivenessChart';

// -- Widget Definitions ---------------------------------------

const radarWidget: DashboardWidgetPlugin = {
  id: 'knowledge-radar',
  title: '全班知识掌握雷达',
  description: '以雷达图展示全班各知识点平均掌握度',
  Component: ({ data }: { context: DashboardWidgetContext; data?: unknown }) => {
    const radar = data as KnowledgeRadarData | null;
    if (!radar || radar.labels.length === 0) {
      return <div style={{ textAlign: 'center', padding: 40, color: 'var(--text-muted)', fontSize: 14 }}>暂无学习数据</div>;
    }
    return <RadarChart labels={radar.labels} values={radar.values} />;
  },
  dataLoader: async (ctx: DashboardWidgetContext) => {
    try {
      return await getKnowledgeRadar(ctx.courseId);
    } catch {
      return null;
    }
  },
};

const masteryBarWidget: DashboardWidgetPlugin = {
  id: 'mastery-bar',
  title: '各知识点平均掌握度',
  description: '柱状图展示各知识点掌握百分比',
  Component: ({ data }: { context: DashboardWidgetContext; data?: unknown }) => {
    const radar = data as KnowledgeRadarData | null;
    if (!radar || radar.labels.length === 0) {
      return <div style={{ textAlign: 'center', padding: 40, color: 'var(--text-muted)', fontSize: 14 }}>暂无学习数据</div>;
    }
    return <MasteryBarChart labels={radar.labels} values={radar.values} />;
  },
  dataLoader: async (ctx: DashboardWidgetContext) => {
    try {
      return await getKnowledgeRadar(ctx.courseId);
    } catch {
      return null;
    }
  },
};

const skillEffectivenessWidget: DashboardWidgetPlugin = {
  id: 'skill-effectiveness',
  title: '技能教学效果评估 (RAGAS)',
  description: 'RAGAS 多维度教学质量评估图表',
  Component: ({ data }: { context: DashboardWidgetContext; data?: unknown }) => {
    const resp = data as SkillEffectivenessResponse | null;
    if (!resp || resp.items.length === 0) {
      return null;
    }
    return <SkillEffectivenessChart items={resp.items} />;
  },
  dataLoader: async (ctx: DashboardWidgetContext) => {
    try {
      return await getSkillEffectiveness(ctx.courseId);
    } catch {
      return null;
    }
  },
};

// -- All Built-in Widgets -------------------------------------

const BUILTIN_WIDGETS: { widget: DashboardWidgetPlugin; priority: number }[] = [
  { widget: radarWidget, priority: 10 },
  { widget: masteryBarWidget, priority: 20 },
  { widget: skillEffectivenessWidget, priority: 30 },
];

// -- Registration Hook ----------------------------------------

/**
 * Registers all built-in dashboard widgets with the plugin registry.
 * Call this once inside a component within `<PluginRegistryProvider>`.
 */
export function useBuiltinDashboardPlugins(): void {
  const { register, unregister } = usePluginRegistryContext();

  useEffect(() => {
    const ids: string[] = [];
    for (const { widget, priority } of BUILTIN_WIDGETS) {
      const reg = buildDashboardWidgetRegistration(widget, priority);
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
