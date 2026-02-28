'use client';

import { useState, useRef } from 'react';
import { useRouter } from 'next/navigation';
import dynamic from 'next/dynamic';
import {
    createCustomSkill,
    type CreateCustomSkillData,
    type CustomSkillTemplate,
    type SkillToolConfig,
} from '@/lib/api';
import { useToast } from '@/components/Toast';
import styles from './page.module.css';

const MarkdownRenderer = dynamic(() => import('@/components/MarkdownRenderer'));

// -- Constants ------------------------------------------------

const CATEGORY_MAP: Record<string, string> = {
    'inquiry-based': '探究式教学',
    'critical-thinking': '批判性思维',
    'collaborative': '协作学习',
    'role-play': '角色扮演',
};

const SUBJECT_MAP: Record<string, string> = {
    math: '数学',
    physics: '物理',
    chemistry: '化学',
    biology: '生物',
    chinese: '语文',
    english: '英语',
    history: '历史',
    geography: '地理',
};

const STEPS = [
    { key: 'basic', label: '基本信息', num: 1 },
    { key: 'constraints', label: '编写约束', num: 2 },
    { key: 'tools', label: '配置工具', num: 3 },
    { key: 'templates', label: '上传模板', num: 4 },
];

const AVAILABLE_TOOLS: { key: string; name: string; description: string }[] = [
    { key: 'leveler', name: '难度调节器 (Leveler)', description: '根据学生年级和掌握程度自动调整提问复杂度和语言表达。' },
    { key: 'make_it_relevant', name: '时事关联 (Make it Relevant)', description: '将抽象知识点与当前时事、生活场景关联，提高学生兴趣。' },
];

const SKILL_MD_TEMPLATE = `# 技能名称

## 核心身份

你是一位……的导师。你的使命是……

## 绝对约束（不可违反）

1. **约束一。** 描述具体的行为规则。
2. **约束二。** 描述另一个规则。
3. **每次回复必须以问句结尾。** 确保对话保持探究动力。

## 引导策略

### 高支架阶段（mastery < 0.4）
- 将复杂问题拆解为简单子问题
- 提供具体的类比或生活实例

### 中支架阶段（0.4 ≤ mastery < 0.7）
- 使用开放式问题引导推理
- 引入矛盾或反例促进深度思考

### 低支架阶段（mastery ≥ 0.7）
- 仅提出高阶思考问题
- 鼓励学生自行提出问题

## 回复格式

\`\`\`
[对学生回答的反馈]

[基于当前理解水平的追问]

[以引导性问题结尾]
\`\`\`

## 评估维度

- **维度一 (dimension_key):** 描述
- **维度二 (dimension_key):** 描述
`;

const TOKEN_MAX = 2000;

// -- Helpers --------------------------------------------------

/** Approximate token count matching backend estimateTokens algorithm */
function estimateTokens(text: string): number {
    let tokens = 0;
    for (const ch of text) {
        if (ch.charCodeAt(0) > 0x4e00) {
            tokens += 1.5;  // CJK characters
        } else {
            tokens += 0.25; // Latin characters (~4 chars/token)
        }
    }
    return Math.ceil(tokens);
}

// -- Component ------------------------------------------------

export default function CreateCustomSkillPage() {
    const router = useRouter();
    const { toast } = useToast();

    // Step tracking
    const [currentStep, setCurrentStep] = useState(0);

    // Step 1: Basic info
    const [name, setName] = useState('');
    const [description, setDescription] = useState('');
    const [category, setCategory] = useState('');
    const [subjects, setSubjects] = useState<string[]>([]);
    const [tags, setTags] = useState<string[]>([]);
    const [tagInput, setTagInput] = useState('');

    // Skill ID segments
    const [idSubject, setIdSubject] = useState('');
    const [idScenario, setIdScenario] = useState('');
    const [idMethod, setIdMethod] = useState('');

    // Step 2: Constraints (SKILL.md)
    const [skillMd, setSkillMd] = useState('');

    // Step 3: Tools
    const [toolsConfig, setToolsConfig] = useState<Record<string, SkillToolConfig>>({});

    // Step 4: Templates
    const [templates, setTemplates] = useState<CustomSkillTemplate[]>([]);

    // Submit state
    const [submitting, setSubmitting] = useState(false);

    // Refs
    const tagInputRef = useRef<HTMLInputElement>(null);

    // -- Computed -----------------------------------------------

    const skillId = [idSubject, idScenario, idMethod].filter(Boolean).join('_');
    const tokenCount = estimateTokens(skillMd);
    const tokenStatus: 'ok' | 'warn' | 'over' =
        tokenCount > TOKEN_MAX ? 'over' : tokenCount > TOKEN_MAX * 0.8 ? 'warn' : 'ok';

    // -- Validation ---------------------------------------------

    function isStep1Valid(): boolean {
        return name.trim().length > 0 && idSubject.length > 0 && idScenario.length > 0 && idMethod.length > 0;
    }

    function isStep2Valid(): boolean {
        return skillMd.trim().length > 0 && tokenCount <= TOKEN_MAX;
    }

    function canSubmit(): boolean {
        return isStep1Valid() && isStep2Valid();
    }

    // -- Tag handling -------------------------------------------

    function handleTagKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
        if ((e.key === 'Enter' || e.key === ',') && tagInput.trim()) {
            e.preventDefault();
            const newTag = tagInput.trim().replace(/,$/, '');
            if (newTag && !tags.includes(newTag)) {
                setTags(prev => [...prev, newTag]);
            }
            setTagInput('');
        } else if (e.key === 'Backspace' && !tagInput && tags.length > 0) {
            setTags(prev => prev.slice(0, -1));
        }
    }

    function removeTag(tag: string) {
        setTags(prev => prev.filter(t => t !== tag));
    }

    // -- Tool toggle --------------------------------------------

    function toggleTool(toolKey: string) {
        setToolsConfig(prev => {
            const existing = prev[toolKey];
            if (existing?.enabled) {
                // Disable: remove from config
                const next = { ...prev };
                delete next[toolKey];
                return next;
            }
            // Enable
            const toolDef = AVAILABLE_TOOLS.find(t => t.key === toolKey);
            return {
                ...prev,
                [toolKey]: {
                    enabled: true,
                    description: toolDef?.description || '',
                },
            };
        });
    }

    // -- Template management ------------------------------------

    function addTemplate() {
        setTemplates(prev => [
            ...prev,
            {
                id: `tpl_${Date.now()}`,
                file_name: '',
                content: '',
            },
        ]);
    }

    function updateTemplate(index: number, field: 'file_name' | 'content', value: string) {
        setTemplates(prev => prev.map((t, i) => i === index ? { ...t, [field]: value } : t));
    }

    function removeTemplate(index: number) {
        setTemplates(prev => prev.filter((_, i) => i !== index));
    }

    // -- Submit -------------------------------------------------

    async function handleSubmit() {
        if (!canSubmit()) return;
        setSubmitting(true);
        try {
            const data: CreateCustomSkillData = {
                skill_id: skillId,
                name: name.trim(),
                description: description.trim() || undefined,
                category: category || undefined,
                subjects: subjects.length > 0 ? subjects : undefined,
                tags: tags.length > 0 ? tags : undefined,
                skill_md: skillMd,
                tools_config: Object.keys(toolsConfig).length > 0 ? toolsConfig : undefined,
                templates: templates.filter(t => t.file_name.trim() && t.content.trim()).length > 0
                    ? templates.filter(t => t.file_name.trim() && t.content.trim())
                    : undefined,
            };
            await createCustomSkill(data);
            toast('技能创建成功！', 'success');
            router.push('/teacher/skills');
        } catch (err) {
            const msg = err instanceof Error ? err.message : '创建失败，请稍后重试';
            toast(msg, 'error');
        } finally {
            setSubmitting(false);
        }
    }

    // -- Insert template ----------------------------------------

    function insertTemplate() {
        setSkillMd(SKILL_MD_TEMPLATE.replace('# 技能名称', `# ${name || '技能名称'}`));
    }

    // -- Navigation ---------------------------------------------

    function goNext() {
        if (currentStep < STEPS.length - 1) setCurrentStep(currentStep + 1);
    }

    function goPrev() {
        if (currentStep > 0) setCurrentStep(currentStep - 1);
    }

    // -- Render -------------------------------------------------

    return (
        <div className={`fade-in ${styles.page}`}>
            {/* Header */}
            <div className={styles.pageHeader}>
                <button className={styles.backBtn} onClick={() => router.push('/teacher/skills')}>
                    &larr; 返回
                </button>
                <div>
                    <h1 className={styles.pageTitle}>新建自定义技能</h1>
                    <div className={styles.pageSubtitle}>通过可视化表单创建教学技能，无需编写代码</div>
                </div>
            </div>

            {/* Steps Bar */}
            <div className={styles.stepsBar}>
                {STEPS.map((step, i) => (
                    <button
                        key={step.key}
                        className={`${styles.step} ${i === currentStep ? styles.stepActive : ''} ${i < currentStep ? styles.stepDone : ''}`}
                        onClick={() => setCurrentStep(i)}
                    >
                        <span className={styles.stepNum}>
                            {i < currentStep ? '\u2713' : step.num}
                        </span>
                        <span className={styles.stepLabel}>{step.label}</span>
                    </button>
                ))}
            </div>

            {/* Step 1: Basic Info */}
            {currentStep === 0 && (
                <div className={styles.formPanel}>
                    <div className={styles.formRow}>
                        <label className={`${styles.formLabel} ${styles.formRequired}`}>技能名称</label>
                        <input
                            className={styles.formInput}
                            type="text"
                            placeholder="例如：实验类比引导"
                            value={name}
                            onChange={e => setName(e.target.value)}
                            maxLength={100}
                        />
                    </div>

                    <div className={styles.formRow}>
                        <label className={`${styles.formLabel} ${styles.formRequired}`}>
                            技能 ID（三段式命名空间）
                        </label>
                        <div className={styles.skillIdBuilder}>
                            <select value={idSubject} onChange={e => setIdSubject(e.target.value)}>
                                <option value="">学科</option>
                                {Object.entries(SUBJECT_MAP).map(([key, label]) => (
                                    <option key={key} value={key}>{label}</option>
                                ))}
                                <option value="general">通用</option>
                            </select>
                            <span className={styles.skillIdSep}>_</span>
                            <input
                                type="text"
                                placeholder="场景 (如 experiment)"
                                value={idScenario}
                                onChange={e => setIdScenario(e.target.value.replace(/[^a-z0-9-]/g, ''))}
                                maxLength={30}
                            />
                            <span className={styles.skillIdSep}>_</span>
                            <input
                                type="text"
                                placeholder="方法 (如 analogy)"
                                value={idMethod}
                                onChange={e => setIdMethod(e.target.value.replace(/[^a-z0-9-]/g, ''))}
                                maxLength={30}
                            />
                        </div>
                        {skillId && (
                            <div className={styles.skillIdPreview}>
                                预览: <code>{skillId}</code>
                            </div>
                        )}
                        <div className={styles.formHint}>
                            格式：学科_场景_方法，仅小写字母、数字和连字符
                        </div>
                    </div>

                    <div className={styles.formRow}>
                        <label className={styles.formLabel}>描述</label>
                        <textarea
                            className={styles.formTextarea}
                            placeholder="描述该技能的教学目标和使用场景..."
                            value={description}
                            onChange={e => setDescription(e.target.value)}
                            rows={3}
                            maxLength={500}
                        />
                    </div>

                    <div className={styles.formRow2Col}>
                        <div className={styles.formRow}>
                            <label className={styles.formLabel}>教学类型</label>
                            <select
                                className={styles.formSelect}
                                value={category}
                                onChange={e => setCategory(e.target.value)}
                            >
                                <option value="">选择类型</option>
                                {Object.entries(CATEGORY_MAP).map(([key, label]) => (
                                    <option key={key} value={key}>{label}</option>
                                ))}
                            </select>
                        </div>

                        <div className={styles.formRow}>
                            <label className={styles.formLabel}>适用学科</label>
                            <div className={styles.chipGroup}>
                                {Object.entries(SUBJECT_MAP).map(([key, label]) => (
                                    <button
                                        key={key}
                                        className={`${styles.chip} ${subjects.includes(key) ? styles.chipSelected : ''}`}
                                        onClick={() =>
                                            setSubjects(prev =>
                                                prev.includes(key)
                                                    ? prev.filter(s => s !== key)
                                                    : [...prev, key]
                                            )
                                        }
                                        type="button"
                                    >
                                        {label}
                                    </button>
                                ))}
                            </div>
                        </div>
                    </div>

                    <div className={styles.formRow}>
                        <label className={styles.formLabel}>标签</label>
                        <div
                            className={styles.tagInputWrapper}
                            onClick={() => tagInputRef.current?.focus()}
                        >
                            {tags.map(tag => (
                                <span key={tag} className={styles.tagChip}>
                                    {tag}
                                    <button
                                        type="button"
                                        className={styles.tagChipRemove}
                                        onClick={e => { e.stopPropagation(); removeTag(tag); }}
                                    >
                                        &times;
                                    </button>
                                </span>
                            ))}
                            <input
                                ref={tagInputRef}
                                className={styles.tagInput}
                                type="text"
                                placeholder={tags.length === 0 ? '输入标签后按 Enter 添加...' : ''}
                                value={tagInput}
                                onChange={e => setTagInput(e.target.value)}
                                onKeyDown={handleTagKeyDown}
                            />
                        </div>
                        <div className={styles.formHint}>按 Enter 或逗号添加标签</div>
                    </div>
                </div>
            )}

            {/* Step 2: Constraint Editor */}
            {currentStep === 1 && (
                <div className={styles.formPanel}>
                    <div className={styles.constraintEditor}>
                        <div className={styles.constraintEditorPane}>
                            <div className={styles.paneHeader}>
                                <span className={styles.paneTitle}>SKILL.md 编辑器</span>
                                <span
                                    className={`${styles.tokenCounter} ${tokenStatus === 'warn' ? styles.tokenWarn : ''} ${tokenStatus === 'over' ? styles.tokenOver : ''}`}
                                >
                                    {tokenCount} / {TOKEN_MAX} tokens
                                </span>
                            </div>
                            <textarea
                                className={styles.skillMdTextarea}
                                placeholder="在此编写技能的约束规则（Markdown 格式）..."
                                value={skillMd}
                                onChange={e => setSkillMd(e.target.value)}
                            />
                            {tokenStatus === 'over' && (
                                <div className={styles.formError}>
                                    超出 token 上限，请精简内容（当前 {tokenCount}，上限 {TOKEN_MAX}）
                                </div>
                            )}
                        </div>

                        <div className={styles.constraintEditorPane}>
                            <div className={styles.paneHeader}>
                                <span className={styles.paneTitle}>实时预览</span>
                                <button
                                    type="button"
                                    className={styles.templateBtn}
                                    onClick={insertTemplate}
                                >
                                    插入模板
                                </button>
                            </div>
                            {skillMd.trim() ? (
                                <div className={styles.previewPane}>
                                    <MarkdownRenderer content={skillMd} />
                                </div>
                            ) : (
                                <div className={`${styles.previewPane} ${styles.previewEmpty}`}>
                                    <span>在左侧编写约束规则</span>
                                    <span>预览将实时显示</span>
                                    <button
                                        type="button"
                                        className={styles.templateBtn}
                                        onClick={insertTemplate}
                                    >
                                        使用模板快速开始
                                    </button>
                                </div>
                            )}
                        </div>
                    </div>
                </div>
            )}

            {/* Step 3: Tools Configuration */}
            {currentStep === 2 && (
                <div className={styles.formPanel}>
                    <div className={styles.formRow}>
                        <label className={styles.formLabel}>选择需要启用的辅助工具</label>
                        <div className={styles.formHint}>
                            这些工具会在 AI 教练使用该技能时自动加载
                        </div>
                    </div>
                    <div className={styles.toolConfigList}>
                        {AVAILABLE_TOOLS.map(tool => (
                            <label key={tool.key} className={styles.toolConfigItem}>
                                <input
                                    type="checkbox"
                                    className={styles.toolToggle}
                                    checked={!!toolsConfig[tool.key]?.enabled}
                                    onChange={() => toggleTool(tool.key)}
                                />
                                <div className={styles.toolConfigInfo}>
                                    <div className={styles.toolConfigName}>{tool.name}</div>
                                    <div className={styles.toolConfigDesc}>{tool.description}</div>
                                </div>
                            </label>
                        ))}
                    </div>
                </div>
            )}

            {/* Step 4: Templates */}
            {currentStep === 3 && (
                <div className={styles.formPanel}>
                    <div className={styles.formRow}>
                        <label className={styles.formLabel}>模板文件（可选）</label>
                        <div className={styles.formHint}>
                            可上传评分量规或 Prompt 模板。每个模板需指定文件名和内容。
                        </div>
                    </div>

                    <div className={styles.templateList}>
                        {templates.map((tpl, i) => (
                            <div key={tpl.id} className={styles.templateCard}>
                                <div className={styles.templateCardHeader}>
                                    <input
                                        type="text"
                                        placeholder="文件名（如 rubric.md）"
                                        value={tpl.file_name}
                                        onChange={e => updateTemplate(i, 'file_name', e.target.value)}
                                    />
                                    <button
                                        type="button"
                                        className={styles.removeBtn}
                                        onClick={() => removeTemplate(i)}
                                    >
                                        删除
                                    </button>
                                </div>
                                <textarea
                                    className={styles.templateTextarea}
                                    placeholder="模板内容..."
                                    value={tpl.content}
                                    onChange={e => updateTemplate(i, 'content', e.target.value)}
                                />
                            </div>
                        ))}

                        <button type="button" className={styles.addBtn} onClick={addTemplate}>
                            + 添加模板文件
                        </button>
                    </div>
                </div>
            )}

            {/* Bottom Bar */}
            <div className={styles.bottomBar}>
                {currentStep > 0 ? (
                    <button type="button" className={styles.btnSecondary} onClick={goPrev}>
                        上一步
                    </button>
                ) : (
                    <div />
                )}

                <div className={styles.btnGroup}>
                    {currentStep < STEPS.length - 1 ? (
                        <button
                            type="button"
                            className={styles.btnPrimary}
                            onClick={goNext}
                            disabled={currentStep === 0 && !isStep1Valid()}
                        >
                            下一步
                        </button>
                    ) : (
                        <button
                            type="button"
                            className={styles.btnPrimary}
                            onClick={handleSubmit}
                            disabled={!canSubmit() || submitting}
                        >
                            {submitting ? '创建中...' : '创建技能（草稿）'}
                        </button>
                    )}
                </div>
            </div>
        </div>
    );
}
