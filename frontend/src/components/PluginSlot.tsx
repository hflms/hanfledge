'use client';

/**
 * PluginSlot — Extension-point component.
 *
 * Place `<PluginSlot name="..." />` in the UI to mark a location
 * where registered plugins will be rendered. If no plugins are
 * registered for that slot, the optional `fallback` is shown.
 *
 * Per design.md §7.12.
 */

import type { ReactNode } from 'react';
import type { SlotName } from '@/lib/plugin/types';
import { usePluginRegistry } from '@/lib/plugin/PluginRegistry';
import PluginSandbox from './PluginSandbox';

interface PluginSlotProps {
  /** Slot identifier — must match a SlotName. */
  name: SlotName;
  /** Context data passed through to each plugin. */
  context?: Record<string, unknown>;
  /** Fallback UI shown when no plugins are registered for this slot. */
  fallback?: ReactNode;
}

export default function PluginSlot({ name, context, fallback }: PluginSlotProps) {
  const plugins = usePluginRegistry(name);

  if (plugins.length === 0) {
    return <>{fallback}</>;
  }

  return (
    <>
      {plugins.map((plugin) => (
        <PluginSandbox
          key={plugin.id}
          plugin={plugin}
          context={context}
          isolation={plugin.trustLevel === 'community' ? 'iframe' : 'shadow-dom'}
        />
      ))}
    </>
  );
}
