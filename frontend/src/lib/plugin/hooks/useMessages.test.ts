import { renderHook, act } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { useMessages } from './useMessages';

describe('useMessages', () => {
  it('should initialize with empty messages', () => {
    const { result } = renderHook(() => useMessages());
    expect(result.current.messages).toEqual([]);
  });

  it('should add a message', () => {
    const { result } = renderHook(() => useMessages());
    act(() => {
      result.current.addMessage({
        id: 'm1',
        role: 'student',
        content: 'Hello',
        timestamp: Date.now(),
      });
    });
    expect(result.current.messages).toHaveLength(1);
    expect(result.current.messages[0]).toMatchObject({
      role: 'student',
      content: 'Hello',
    });
  });

  it('should clear messages', () => {
    const { result } = renderHook(() => useMessages());
    act(() => {
      result.current.addMessage({
        id: 'm1',
        role: 'student',
        content: 'Test',
        timestamp: Date.now(),
      });
      result.current.clearMessages();
    });
    expect(result.current.messages).toEqual([]);
  });

  it('should limit messages to maxMessages', () => {
    const { result } = renderHook(() => useMessages({ maxMessages: 2 }));
    act(() => {
      result.current.addMessage({ id: 'm1', role: 'student', content: '1', timestamp: 1 });
      result.current.addMessage({ id: 'm2', role: 'student', content: '2', timestamp: 2 });
      result.current.addMessage({ id: 'm3', role: 'student', content: '3', timestamp: 3 });
    });
    expect(result.current.messages).toHaveLength(2);
    expect(result.current.messages[0].content).toBe('2');
    expect(result.current.messages[1].content).toBe('3');
  });
});

