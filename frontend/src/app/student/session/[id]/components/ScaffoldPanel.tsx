import styles from '../page.module.css';

// -- Types -------------------------------------------------------

export type ScaffoldLevel = 'high' | 'medium' | 'low';

export interface ScaffoldData {
    steps?: string[];
    keywords?: string[];
}

export const SCAFFOLD_LABELS: Record<ScaffoldLevel, string> = {
    high: '高支架',
    medium: '中支架',
    low: '低支架',
};

export const SCAFFOLD_DESCRIPTIONS: Record<ScaffoldLevel, string> = {
    high: '分步引导模式 — 按照步骤思考',
    medium: '关键词提示模式 — 围绕关键概念思考',
    low: '自由思考模式 — 独立解决问题',
};

interface ScaffoldPanelProps {
    level: ScaffoldLevel;
    data: ScaffoldData;
    transition: boolean;
}

// -- Component ---------------------------------------------------

export default function ScaffoldPanel({ level, data, transition }: ScaffoldPanelProps) {
    switch (level) {
        case 'high':
            return <ScaffoldHigh data={data} transition={transition} />;
        case 'medium':
            return <ScaffoldMedium data={data} transition={transition} />;
        case 'low':
            return null; // Low scaffold = blank input only
    }
}

// -- Sub-components ----------------------------------------------

function ScaffoldHigh({ data, transition }: { data: ScaffoldData; transition: boolean }) {
    const steps = data.steps || [
        '阅读 AI 导师的引导，理解学习目标',
        '回忆相关的知识点和概念',
        '尝试用自己的语言描述思路',
        '在对话中逐步推导和验证',
    ];
    const keywords = data.keywords || [];

    return (
        <div className={`${styles.scaffoldPanel} ${transition ? styles.scaffoldTransition : ''}`}>
            <div className={styles.scaffoldPanelHeader}>
                分步引导
            </div>
            <div className={styles.scaffoldSteps}>
                {steps.map((step, i) => (
                    <div key={i} className={styles.scaffoldStep}>
                        <span className={styles.stepNumber}>{i + 1}</span>
                        <span>{step}</span>
                    </div>
                ))}
            </div>
            {keywords.length > 0 && (
                <div className={styles.scaffoldTags}>
                    {keywords.map((kw, i) => (
                        <span key={i} className={styles.keywordHighlight}>{kw}</span>
                    ))}
                </div>
            )}
        </div>
    );
}

function ScaffoldMedium({ data, transition }: { data: ScaffoldData; transition: boolean }) {
    const keywords = data.keywords || ['关键概念', '前置知识', '核心思路'];

    return (
        <div className={`${styles.scaffoldTags} ${transition ? styles.scaffoldTransition : ''}`}>
            {keywords.map((kw, i) => (
                <span key={i} className={styles.scaffoldTag}>{kw}</span>
            ))}
        </div>
    );
}
