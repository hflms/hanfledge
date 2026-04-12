import { useMemo } from 'react';
import { getRendererBySkillId } from '@/lib/plugin/SkillManifestLoader';
import { type StudentSession } from '@/lib/api';

export function usePluginRenderer(session: StudentSession | null) {
    const activeSkill = session?.active_skill || '';

    const matchedRenderer = useMemo(() => {
        if (!activeSkill) return null;
        return getRendererBySkillId(activeSkill);
    }, [activeSkill]);

    const activePlugin = matchedRenderer?.Component ? matchedRenderer : null;

    return {
        activeSkill,
        matchedRenderer,
        activePlugin,
    };
}
