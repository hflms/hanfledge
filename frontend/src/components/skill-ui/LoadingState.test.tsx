import { render } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import LoadingState from './LoadingState';

describe('LoadingState', () => {
  it('should render without crashing', () => {
    const { container } = render(<LoadingState />);
    expect(container.firstChild).toBeTruthy();
  });

  it('should render with custom message', () => {
    const { container } = render(<LoadingState message="Processing..." />);
    expect(container.firstChild).toBeTruthy();
  });

  it('should render with progress', () => {
    const { container } = render(<LoadingState progress={50} />);
    expect(container.firstChild).toBeTruthy();
  });
});
