import { renderHook, act } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { useStateMachine } from './useStateMachine';

describe('useStateMachine', () => {
  const config = {
    initialPhase: 'idle' as const,
    transitions: {
      idle: ['loading'],
      loading: ['ready', 'error'],
      ready: ['complete'],
      complete: [],
      error: ['idle'],
    },
  };

  it('should initialize with initial phase', () => {
    const { result } = renderHook(() => useStateMachine(config));
    expect(result.current.phase).toBe('idle');
  });

  it('should transition to valid state', () => {
    const { result } = renderHook(() => useStateMachine(config));
    act(() => {
      result.current.transitionTo('loading');
    });
    expect(result.current.phase).toBe('loading');
  });

  it('should reject invalid transition', () => {
    const { result } = renderHook(() => useStateMachine(config));
    act(() => {
      const success = result.current.transitionTo('complete' as 'loading');
      expect(success).toBe(false);
    });
    expect(result.current.phase).toBe('idle');
  });

  it('should handle error state transition', () => {
    const { result } = renderHook(() => useStateMachine(config));
    act(() => {
      result.current.transitionTo('loading');
    });
    expect(result.current.phase).toBe('loading');
    
    act(() => {
      result.current.transitionTo('error');
    });
    expect(result.current.phase).toBe('error');
  });
});
