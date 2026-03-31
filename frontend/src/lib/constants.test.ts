import { describe, it, expect } from 'vitest';
import {
  DOCUMENT_STATUS_LABEL,
  DOCUMENT_STATUS_ICON,
  COURSE_STATUS_MAP,
  ACTIVITY_STATUS_MAP,
  SESSION_STATUS_MAP,
  CUSTOM_SKILL_STATUS_LABELS,
  CATEGORY_MAP,
  CATEGORY_ICONS,
  SUBJECT_MAP,
  SCAFFOLD_MAP,
} from './constants';

// -- Document Status ------------------------------------------

describe('DOCUMENT_STATUS_LABEL', () => {
  it('has labels for all document statuses', () => {
    expect(DOCUMENT_STATUS_LABEL).toHaveProperty('uploaded');
    expect(DOCUMENT_STATUS_LABEL).toHaveProperty('processing');
    expect(DOCUMENT_STATUS_LABEL).toHaveProperty('completed');
    expect(DOCUMENT_STATUS_LABEL).toHaveProperty('failed');
  });

  it('all values are non-empty Chinese strings', () => {
    for (const [key, value] of Object.entries(DOCUMENT_STATUS_LABEL)) {
      expect(value, `${key} should be non-empty`).toBeTruthy();
      expect(typeof value).toBe('string');
    }
  });
});

describe('DOCUMENT_STATUS_ICON', () => {
  it('has icons for the same keys as labels', () => {
    const labelKeys = Object.keys(DOCUMENT_STATUS_LABEL);
    const iconKeys = Object.keys(DOCUMENT_STATUS_ICON);
    expect(iconKeys).toEqual(labelKeys);
  });
});

// -- Course Status --------------------------------------------

describe('COURSE_STATUS_MAP', () => {
  it('has labels for draft, published, archived', () => {
    expect(COURSE_STATUS_MAP.draft).toBe('草稿');
    expect(COURSE_STATUS_MAP.published).toBe('已发布');
    expect(COURSE_STATUS_MAP.archived).toBe('已归档');
  });

  it('has exactly 3 entries', () => {
    expect(Object.keys(COURSE_STATUS_MAP)).toHaveLength(3);
  });
});

// -- Activity Status ------------------------------------------

describe('ACTIVITY_STATUS_MAP', () => {
  it('is a superset of course statuses (draft, published)', () => {
    expect(ACTIVITY_STATUS_MAP).toHaveProperty('draft');
    expect(ACTIVITY_STATUS_MAP).toHaveProperty('published');
  });

  it('includes dashboard-specific statuses', () => {
    expect(ACTIVITY_STATUS_MAP).toHaveProperty('active');
    expect(ACTIVITY_STATUS_MAP).toHaveProperty('completed');
    expect(ACTIVITY_STATUS_MAP).toHaveProperty('abandoned');
    expect(ACTIVITY_STATUS_MAP).toHaveProperty('closed');
  });
});

// -- Session Status -------------------------------------------

describe('SESSION_STATUS_MAP', () => {
  it('has active, completed, abandoned', () => {
    expect(SESSION_STATUS_MAP.active).toBe('学习中');
    expect(SESSION_STATUS_MAP.completed).toBe('已完成');
    expect(SESSION_STATUS_MAP.abandoned).toBe('已放弃');
  });
});

// -- Custom Skill Status --------------------------------------

describe('CUSTOM_SKILL_STATUS_LABELS', () => {
  it('covers all custom skill statuses', () => {
    expect(CUSTOM_SKILL_STATUS_LABELS.draft).toBe('草稿');
    expect(CUSTOM_SKILL_STATUS_LABELS.published).toBe('已发布');
    expect(CUSTOM_SKILL_STATUS_LABELS.shared).toBe('已共享');
    expect(CUSTOM_SKILL_STATUS_LABELS.archived).toBe('已归档');
  });
});

// -- Categories -----------------------------------------------

describe('CATEGORY_MAP', () => {
  it('has labels for known categories', () => {
    expect(CATEGORY_MAP['inquiry-based']).toBe('探究式教学');
    expect(CATEGORY_MAP['critical-thinking']).toBe('批判性思维');
    expect(CATEGORY_MAP['collaborative']).toBe('协作学习');
    expect(CATEGORY_MAP['role-play']).toBe('角色扮演');
    expect(CATEGORY_MAP['content-creation']).toBe('内容创作');
  });
});

describe('CATEGORY_ICONS', () => {
  it('has icons matching CATEGORY_MAP keys', () => {
    for (const key of Object.keys(CATEGORY_MAP)) {
      expect(CATEGORY_ICONS, `missing icon for ${key}`).toHaveProperty(key);
    }
  });
});

// -- Subjects -------------------------------------------------

describe('SUBJECT_MAP', () => {
  it('has labels for common subjects', () => {
    expect(SUBJECT_MAP.math).toBe('数学');
    expect(SUBJECT_MAP.physics).toBe('物理');
    expect(SUBJECT_MAP.chemistry).toBe('化学');
    expect(SUBJECT_MAP.english).toBe('英语');
  });

  it('has at least 8 subjects', () => {
    expect(Object.keys(SUBJECT_MAP).length).toBeGreaterThanOrEqual(8);
  });
});

// -- Scaffold Levels ------------------------------------------

describe('SCAFFOLD_MAP', () => {
  it('maps all three scaffold levels', () => {
    expect(SCAFFOLD_MAP.high).toBe('高');
    expect(SCAFFOLD_MAP.medium).toBe('中');
    expect(SCAFFOLD_MAP.low).toBe('低');
  });

  it('has exactly 3 entries', () => {
    expect(Object.keys(SCAFFOLD_MAP)).toHaveLength(3);
  });
});
