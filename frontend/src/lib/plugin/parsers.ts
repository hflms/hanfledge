/**
 * Unified structured output parser for all skills.
 * Supports multiple tag formats and provides fallback handling.
 */

export interface SkillOutput<T> {
  skill_id: string;
  phase?: string;
  data: T;
  metadata?: {
    confidence?: number;
    reasoning?: string;
    alternatives?: T[];
  };
}

/**
 * Parse structured output from coach response.
 * Supports both new unified format and legacy formats.
 */
export function parseSkillOutput<T>(
  content: string,
  tagName: string,
  fallback?: T
): T | null {
  // Try new unified format: <skill_output type="...">
  const unifiedMatch = content.match(
    new RegExp(`<skill_output[^>]*type="${tagName}"[^>]*>([\\s\\S]*?)</skill_output>`)
  );
  
  if (unifiedMatch) {
    try {
      const parsed = JSON.parse(unifiedMatch[1]) as SkillOutput<T>;
      return parsed.data;
    } catch (err) {
      console.error(`[parseSkillOutput] Failed to parse unified format:`, err);
    }
  }

  // Try legacy format: <tagName>
  const legacyMatch = content.match(
    new RegExp(`<${tagName}>([\\s\\S]*?)</${tagName}>`)
  );
  
  if (legacyMatch) {
    try {
      return JSON.parse(legacyMatch[1]) as T;
    } catch (err) {
      console.error(`[parseSkillOutput] Failed to parse legacy format:`, err);
    }
  }

  return fallback ?? null;
}

/**
 * Strip structured output tags from content.
 */
export function stripSkillOutput(content: string, tagName: string): string {
  let result = content;
  
  // Remove unified format
  result = result.replace(
    new RegExp(`<skill_output[^>]*type="${tagName}"[^>]*>[\\s\\S]*?</skill_output>`, 'g'),
    ''
  );
  
  // Remove legacy format
  result = result.replace(
    new RegExp(`<${tagName}>[\\s\\S]*?</${tagName}>`, 'g'),
    ''
  );
  
  // Remove reasoning tags
  result = result.replace(/<reasoning>[\s\S]*?<\/reasoning>/g, '');
  result = result.replace(/<thinking>[\s\S]*?<\/thinking>/g, '');
  result = result.replace(/<analysis>[\s\S]*?<\/analysis>/g, '');
  
  return result.trim();
}

/**
 * Check if content contains structured output.
 */
export function hasSkillOutput(content: string, tagName: string): boolean {
  const unifiedPattern = new RegExp(`<skill_output[^>]*type="${tagName}"`);
  const legacyPattern = new RegExp(`<${tagName}>`);
  
  return unifiedPattern.test(content) || legacyPattern.test(content);
}
