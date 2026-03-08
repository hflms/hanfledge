import { describe, it, expect } from 'vitest';
import { parseSkillOutput, stripSkillOutput, hasSkillOutput } from './parsers';

describe('parsers', () => {
  describe('parseSkillOutput', () => {
    it('should parse unified format', () => {
      const content = `<skill_output type="quiz">{"skill_id": "quiz", "data": {"questions": [{"id": "q1", "text": "Test?"}]}}</skill_output>`;
      const result = parseSkillOutput<{ questions: unknown[] }>(content, 'quiz');
      expect(result).toEqual({ questions: [{ id: 'q1', text: 'Test?' }] });
    });

    it('should parse legacy format', () => {
      const content = `<quiz>{"questions": [{"id": "q1"}]}</quiz>`;
      const result = parseSkillOutput<{ questions: unknown[] }>(content, 'quiz');
      expect(result).toEqual({ questions: [{ id: 'q1' }] });
    });

    it('should return null when no match', () => {
      const result = parseSkillOutput('No structured output', 'quiz');
      expect(result).toBeNull();
    });

    it('should return fallback when provided', () => {
      const fallback = { questions: [] };
      const result = parseSkillOutput('No output', 'quiz', fallback);
      expect(result).toEqual(fallback);
    });

    it('should handle malformed JSON gracefully', () => {
      const content = `<quiz>{invalid json}</quiz>`;
      const result = parseSkillOutput(content, 'quiz');
      expect(result).toBeNull();
    });
  });

  describe('stripSkillOutput', () => {
    it('should remove unified format tags', () => {
      const content = `Text before <skill_output type="quiz">{"data": "test"}</skill_output> text after`;
      const result = stripSkillOutput(content, 'quiz');
      expect(result).toBe('Text before  text after');
    });

    it('should remove legacy format tags', () => {
      const content = `Text before <quiz>{"data": "test"}</quiz> text after`;
      const result = stripSkillOutput(content, 'quiz');
      expect(result).toBe('Text before  text after');
    });

    it('should remove reasoning tags', () => {
      const content = `<reasoning>Internal thoughts</reasoning>Final answer`;
      const result = stripSkillOutput(content, 'quiz');
      expect(result).toBe('Final answer');
    });

    it('should handle multiple tags', () => {
      const content = `<thinking>...</thinking><quiz>{}</quiz><reasoning>...</reasoning>Clean text`;
      const result = stripSkillOutput(content, 'quiz');
      expect(result).toBe('Clean text');
    });
  });

  describe('hasSkillOutput', () => {
    it('should detect unified format', () => {
      const content = `<skill_output type="quiz">data</skill_output>`;
      expect(hasSkillOutput(content, 'quiz')).toBe(true);
    });

    it('should detect legacy format', () => {
      const content = `<quiz>data</quiz>`;
      expect(hasSkillOutput(content, 'quiz')).toBe(true);
    });

    it('should return false when no output', () => {
      expect(hasSkillOutput('Plain text', 'quiz')).toBe(false);
    });
  });
});
