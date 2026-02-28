'use client';

/**
 * Skill Renderer Plugin Registrations — Manifest-driven.
 *
 * Discovers skill renderers from manifest.json files in each
 * plugin's frontend directory and registers them with the
 * Plugin Registry for use in PluginSlot on the session page.
 *
 * Previously hardcoded; now driven by SkillManifestLoader which
 * reads manifest.json metadata and resolves renderer components.
 *
 * Usage: Call `useBuiltinSkillRenderers()` inside a component
 * wrapped by `<PluginRegistryProvider>` to auto-register all
 * discovered skill renderers.
 */

import { useEffect } from 'react';
import {
    usePluginRegistryContext,
    buildSkillRendererRegistration,
} from '@/lib/plugin/PluginRegistry';
import { MANIFEST_RENDERERS } from '@/lib/plugin/SkillManifestLoader';

// -- Registration Hook -------------------------------------------

/**
 * Registers all manifest-discovered skill renderers with the plugin registry.
 * Call this once inside a component within `<PluginRegistryProvider>`.
 *
 * Renderers are discovered from each skill's frontend manifest.json
 * and resolved to React components by the SkillManifestLoader.
 */
export function useBuiltinSkillRenderers(): void {
    const { register, unregister } = usePluginRegistryContext();

    useEffect(() => {
        const ids: string[] = [];
        for (const renderer of MANIFEST_RENDERERS) {
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
