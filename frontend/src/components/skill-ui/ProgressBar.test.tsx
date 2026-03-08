import { render } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import ProgressBar from './ProgressBar';

describe('ProgressBar', () => {
  it('should render without crashing', () => {
    const { container } = render(<ProgressBar current={3} total={10} />);
    expect(container.firstChild).toBeTruthy();
  });

  it('should handle 100% progress', () => {
    const { container } = render(<ProgressBar current={10} total={10} />);
    expect(container.firstChild).toBeTruthy();
  });

  it('should handle 0% progress', () => {
    const { container } = render(<ProgressBar current={0} total={10} />);
    expect(container.firstChild).toBeTruthy();
  });
});
