'use client';

/**
 * Plugin Registry — React Context + Provider + registration hooks.
 *
 * Provides a global registry where plugins register themselves at
 * startup. Components use `usePluginRegistry(slotName)` to discover
 * which plugins should render in a given slot.
 */

import {
  createContext,
  useContext,
  useState,
  useCallback,
  useMemo,
  type ReactNode,
} from 'react';
import type {
  SlotName,
  PluginRegistration,
  DashboardWidgetPlugin,
  SkillUIRenderer,
} from './types';

// -- Context --------------------------------------------------

interface PluginRegistryContextValue {
  /** All registered plugins. */
  plugins: PluginRegistration[];
  /** Register a new plugin. Replaces any existing plugin with the same id. */
  register: (plugin: PluginRegistration) => void;
  /** Unregister a plugin by id. */
  unregister: (pluginId: string) => void;
}

const PluginRegistryContext = createContext<PluginRegistryContextValue | null>(null);

// -- Provider -------------------------------------------------

interface PluginRegistryProviderProps {
  children: ReactNode;
}

export function PluginRegistryProvider({ children }: PluginRegistryProviderProps) {
  const [plugins, setPlugins] = useState<PluginRegistration[]>([]);

  const register = useCallback((plugin: PluginRegistration) => {
    setPlugins(prev => {
      // Replace existing registration with same id
      const filtered = prev.filter(p => p.id !== plugin.id);
      return [...filtered, plugin];
    });
  }, []);

  const unregister = useCallback((pluginId: string) => {
    setPlugins(prev => prev.filter(p => p.id !== pluginId));
  }, []);

  const value = useMemo(
    () => ({ plugins, register, unregister }),
    [plugins, register, unregister],
  );

  return (
    <PluginRegistryContext.Provider value={value}>
      {children}
    </PluginRegistryContext.Provider>
  );
}

// -- Hooks ----------------------------------------------------

/**
 * Returns the full registry context.
 * Must be called within a `<PluginRegistryProvider>`.
 */
export function usePluginRegistryContext(): PluginRegistryContextValue {
  const ctx = useContext(PluginRegistryContext);
  if (!ctx) {
    throw new Error('usePluginRegistryContext must be used within <PluginRegistryProvider>');
  }
  return ctx;
}

/**
 * Returns plugins registered for a specific slot, sorted by priority.
 */
export function usePluginRegistry(slotName: SlotName): PluginRegistration[] {
  const { plugins } = usePluginRegistryContext();
  return useMemo(
    () =>
      plugins
        .filter(p => p.slots.includes(slotName))
        .sort((a, b) => a.priority - b.priority),
    [plugins, slotName],
  );
}

// -- Convenience Registration Helpers -------------------------

/**
 * Registers a SkillUIRenderer as a plugin targeting
 * the `student.interaction.main` slot.
 */
export function buildSkillRendererRegistration(
  renderer: SkillUIRenderer,
): PluginRegistration {
  return {
    id: `skill-renderer-${renderer.skillId}`,
    name: renderer.metadata.name,
    type: 'skill_renderer',
    trustLevel: 'domain',
    slots: ['student.interaction.main'],
    priority: 10,
    Component: renderer.Component as unknown as React.FC<Record<string, unknown>>,
    version: renderer.metadata.version,
  };
}

/**
 * Builds a PluginRegistration from a DashboardWidgetPlugin
 * targeting the `teacher.dashboard.widget` slot.
 */
export function buildDashboardWidgetRegistration(
  widget: DashboardWidgetPlugin,
  priority: number = 50,
): PluginRegistration {
  return {
    id: `dashboard-widget-${widget.id}`,
    name: widget.title,
    type: 'dashboard_widget',
    trustLevel: 'core',
    slots: ['teacher.dashboard.widget'],
    priority,
    Component: widget.Component as unknown as React.FC<Record<string, unknown>>,
    version: '1.0.0',
  };
}
