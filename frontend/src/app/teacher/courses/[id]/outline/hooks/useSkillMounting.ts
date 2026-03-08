/**
 * Hook for skill mounting logic
 */

import { useState, useCallback, useRef } from 'react';
import { listSkills, mountSkillToKP, unmountSkillFromKP, updateKPSkillConfig, type SkillMetadata, type MountedSkill } from '@/lib/api';

export function useSkillMounting(onSuccess: () => void, onError: (msg: string) => void) {
  const [pickerChapterId, setPickerChapterId] = useState<number | null>(null);
  const [pickerKpId, setPickerKpId] = useState<number | null>(null);
  const [availableSkills, setAvailableSkills] = useState<SkillMetadata[]>([]);
  const [skillsLoading, setSkillsLoading] = useState(false);
  const [mounting, setMounting] = useState(false);
  const [unmounting, setUnmounting] = useState<number | null>(null);
  const pickerRef = useRef<HTMLDivElement>(null);

  const openPicker = useCallback(async (chapterId: number, kpId: number) => {
    setPickerChapterId(chapterId);
    setPickerKpId(kpId);
    setSkillsLoading(true);
    try {
      const skills = await listSkills();
      setAvailableSkills(skills);
    } catch (err) {
      onError(err instanceof Error ? err.message : String(err));
    } finally {
      setSkillsLoading(false);
    }
  }, [onError]);

  const closePicker = useCallback(() => {
    setPickerChapterId(null);
    setPickerKpId(null);
  }, []);

  const mountSkill = useCallback(async (skillId: string) => {
    if (!pickerKpId) return;
    setMounting(true);
    try {
      await mountSkillToKP(pickerKpId, { skill_id: skillId });
      onSuccess();
      closePicker();
    } catch (err) {
      onError(err instanceof Error ? err.message : String(err));
    } finally {
      setMounting(false);
    }
  }, [pickerKpId, onSuccess, onError, closePicker]);

  const unmountSkill = useCallback(async (kpId: number, mountId: number) => {
    setUnmounting(mountId);
    try {
      await unmountSkillFromKP(kpId, mountId);
      onSuccess();
    } catch (err) {
      onError(err instanceof Error ? err.message : String(err));
    } finally {
      setUnmounting(null);
    }
  }, [onSuccess, onError]);

  return {
    pickerChapterId,
    pickerKpId,
    availableSkills,
    skillsLoading,
    mounting,
    unmounting,
    pickerRef,
    openPicker,
    closePicker,
    mountSkill,
    unmountSkill,
  };
}

export function useSkillConfig(onSuccess: () => void, onError: (msg: string) => void) {
  const [configMount, setConfigMount] = useState<{ mount: MountedSkill; chapterId: number; kpId: number } | null>(null);
  const [configLevel, setConfigLevel] = useState<string>('high');
  const [configThreshold, setConfigThreshold] = useState<string>('');
  const [configDegradeTo, setConfigDegradeTo] = useState<string>('');
  const [configSaving, setConfigSaving] = useState(false);

  const openConfig = useCallback((mount: MountedSkill, chapterId: number, kpId: number) => {
    setConfigMount({ mount, chapterId, kpId });
    const rule = mount.progressive_rule || {};
    setConfigLevel(mount.scaffold_level || 'high');
    setConfigThreshold(rule.mastery_threshold?.toString() || '');
    setConfigDegradeTo(rule.degrade_to || '');
  }, []);

  const closeConfig = useCallback(() => setConfigMount(null), []);

  const saveConfig = useCallback(async () => {
    if (!configMount) return;
    setConfigSaving(true);
    try {
      const config = {
        scaffold_level: configLevel,
        ...(configThreshold && { mastery_threshold: parseFloat(configThreshold) }),
        ...(configDegradeTo && { degrade_to: configDegradeTo }),
      };
      await updateKPSkillConfig(configMount.kpId, configMount.mount.id, config);
      onSuccess();
      closeConfig();
    } catch (err) {
      onError(err instanceof Error ? err.message : String(err));
    } finally {
      setConfigSaving(false);
    }
  }, [configMount, configLevel, configThreshold, configDegradeTo, onSuccess, onError, closeConfig]);

  return {
    configMount,
    configLevel,
    configThreshold,
    configDegradeTo,
    configSaving,
    setConfigLevel,
    setConfigThreshold,
    setConfigDegradeTo,
    openConfig,
    closeConfig,
    saveConfig,
  };
}
