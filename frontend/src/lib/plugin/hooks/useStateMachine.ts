import { useState, useCallback } from 'react';

/**
 * Generic state machine hook for skill phase management.
 * Validates transitions and provides type-safe phase management.
 */

interface UseStateMachineOptions<TPhase extends string> {
  initialPhase: TPhase;
  transitions: Record<TPhase, TPhase[]>;
  onTransition?: (from: TPhase, to: TPhase) => void;
}

export function useStateMachine<TPhase extends string>({
  initialPhase,
  transitions,
  onTransition,
}: UseStateMachineOptions<TPhase>) {
  const [phase, setPhase] = useState<TPhase>(initialPhase);

  const transitionTo = useCallback((nextPhase: TPhase) => {
    const allowedTransitions = transitions[phase];
    
    if (!allowedTransitions?.includes(nextPhase)) {
      console.warn(`[StateMachine] Invalid transition: ${phase} -> ${nextPhase}`);
      return false;
    }

    onTransition?.(phase, nextPhase);
    setPhase(nextPhase);
    return true;
  }, [phase, transitions, onTransition]);

  const canTransitionTo = useCallback((nextPhase: TPhase) => {
    return transitions[phase]?.includes(nextPhase) ?? false;
  }, [phase, transitions]);

  return {
    phase,
    transitionTo,
    canTransitionTo,
  };
}
