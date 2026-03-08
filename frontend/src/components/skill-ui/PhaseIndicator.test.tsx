import { render } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import PhaseIndicator from './PhaseIndicator';

describe('PhaseIndicator', () => {
  const phases = ['analyzing', 'generating', 'complete'] as const;
  const labels = {
    analyzing: '分析中',
    generating: '生成中',
    complete: '完成',
  };

  it('should render without crashing', () => {
    const { container } = render(
      <PhaseIndicator phases={phases} currentPhase="analyzing" labels={labels} />
    );
    expect(container.firstChild).toBeTruthy();
  });

  it('should render all phases', () => {
    const { container } = render(
      <PhaseIndicator phases={phases} currentPhase="generating" labels={labels} />
    );
    expect(container.firstChild).toBeTruthy();
  });
});
