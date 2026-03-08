import { render } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import QuestionCard from './QuestionCard';

describe('QuestionCard', () => {
  it('should render without crashing', () => {
    const { container } = render(
      <QuestionCard number={1} stem="What is 2+2?">
        <div>Options here</div>
      </QuestionCard>
    );
    expect(container.firstChild).toBeTruthy();
  });

  it('should render with correct status', () => {
    const { container } = render(
      <QuestionCard number={1} stem="Test?" status="correct">
        <div>Answer</div>
      </QuestionCard>
    );
    expect(container.firstChild).toBeTruthy();
  });

  it('should render with incorrect status', () => {
    const { container } = render(
      <QuestionCard number={1} stem="Test?" status="incorrect">
        <div>Answer</div>
      </QuestionCard>
    );
    expect(container.firstChild).toBeTruthy();
  });
});
