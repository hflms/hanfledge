# Hanfledge Skill 系统优化建议

## 当前状态分析

### 现有 Skills (9个)
1. **socratic-questioning** - 苏格拉底式追问
2. **quiz-generation** - 自动出题
3. **fallacy-detective** - 谬误侦探
4. **role-play** - 角色扮演
5. **error-diagnosis** - 错误诊断
6. **cross-disciplinary** - 跨学科联结
7. **learning-survey** - 学情问卷诊断
8. **presentation-generator** - 演示文稿生成器
9. **stepped-learning** - 阶梯式学习

### 前端渲染器现状
- 每个 skill 都有独立的 Renderer 组件 (React)
- 每个 Renderer 都有独立的 CSS Module
- 大量重复代码：WebSocket 处理、消息管理、状态管理
- 缺乏统一的组件抽象层

---

## 🎯 优化方向 1: 组件架构重构

### 问题
1. **代码重复严重**
   - 每个 Renderer 都重复实现 WebSocket 订阅逻辑
   - 消息状态管理 (messages, sending, thinkingStatus) 重复
   - 滚动到底部逻辑重复
   - 输入框处理逻辑重复

2. **缺乏组件复用**
   - 没有统一的 BaseRenderer 基类
   - 没有共享的 UI 组件库（问题卡片、进度条、状态指示器）
   - 样式不统一，每个 skill 自己定义相似的样式

3. **状态管理混乱**
   - 每个 Renderer 自己管理 phase 状态
   - 缺乏统一的状态机模式
   - 难以追踪和调试状态转换

### 解决方案

#### 1.1 创建 BaseSkillRenderer 抽象层

```typescript
// frontend/src/lib/plugin/BaseSkillRenderer.tsx
export abstract class BaseSkillRenderer<TPhase extends string, TState> {
  // 统一的 WebSocket 处理
  protected useAgentChannel(agentChannel: AgentChannel) {
    // 自动处理 agent_thinking, token_delta, turn_complete
  }
  
  // 统一的消息管理
  protected useMessages() {
    // 返回 messages, addMessage, clearMessages
  }
  
  // 统一的输入处理
  protected useInput(onSend: (text: string) => void) {
    // 返回 input, setInput, handleSend, sending
  }
  
  // 抽象方法：子类必须实现
  abstract parseSkillData(content: string): TState | null;
  abstract renderSkillUI(state: TState): React.ReactNode;
  abstract getPhaseLabel(phase: TPhase): string;
}
```

#### 1.2 创建共享 UI 组件库

```typescript
// frontend/src/components/skill-ui/QuestionCard.tsx
export function QuestionCard({ question, onAnswer, disabled }) {
  // 统一的问题卡片样式
}

// frontend/src/components/skill-ui/ProgressBar.tsx
export function SkillProgressBar({ current, total, phase }) {
  // 统一的进度条
}

// frontend/src/components/skill-ui/PhaseIndicator.tsx
export function PhaseIndicator({ phase, phases }) {
  // 统一的阶段指示器
}

// frontend/src/components/skill-ui/ResultCard.tsx
export function ResultCard({ title, score, details }) {
  // 统一的结果展示卡片
}
```

#### 1.3 统一状态机模式

```typescript
// frontend/src/lib/plugin/useSkillStateMachine.ts
export function useSkillStateMachine<TPhase extends string>(
  initialPhase: TPhase,
  transitions: Record<TPhase, TPhase[]>
) {
  const [phase, setPhase] = useState(initialPhase);
  
  const transitionTo = (nextPhase: TPhase) => {
    if (transitions[phase]?.includes(nextPhase)) {
      setPhase(nextPhase);
      return true;
    }
    console.warn(`Invalid transition: ${phase} -> ${nextPhase}`);
    return false;
  };
  
  return { phase, transitionTo };
}
```

---

## 🎯 优化方向 2: 内容生成增强

### 问题
1. **结构化输出不一致**
   - 不同 skill 使用不同的 XML 标签 (`<quiz>`, `<slides>`, `<survey>`)
   - 解析逻辑分散在各个 Renderer 中
   - 缺乏统一的验证机制

2. **AI 生成内容质量不稳定**
   - 没有内容模板系统
   - 缺乏生成后的验证和修正
   - 没有内容缓存和复用机制

3. **缺乏渐进式生成**
   - 大部分 skill 一次性生成全部内容
   - 用户等待时间长
   - 无法提前展示部分结果

### 解决方案

#### 2.1 统一结构化输出协议

```typescript
// 定义统一的 Skill Output Schema
interface SkillOutput<T> {
  skill_id: string;
  phase: string;
  data: T;
  metadata?: {
    confidence?: number;
    reasoning?: string;
    alternatives?: T[];
  };
}

// 后端统一输出格式
<skill_output type="quiz">
{
  "skill_id": "general_assessment_quiz",
  "phase": "generating",
  "data": {
    "questions": [...]
  },
  "metadata": {
    "confidence": 0.92,
    "reasoning": "基于学生当前掌握度 0.65，生成中等难度题目"
  }
}
</skill_output>
```

#### 2.2 内容模板系统

```go
// internal/agent/templates/quiz_template.go
type QuizTemplate struct {
    Difficulty    string
    QuestionTypes []string
    MaxQuestions  int
}

func (t *QuizTemplate) Generate(kp *model.KnowledgePoint, mastery float64) string {
    // 根据模板生成结构化 prompt
    return fmt.Sprintf(`
请根据以下知识点生成 %d 道 %s 难度的题目：
知识点：%s
学生掌握度：%.2f

要求：
- 题型：%s
- 每题必须包含详细解析
- 干扰项必须具有迷惑性
- 输出格式：<skill_output type="quiz">...</skill_output>
`, t.MaxQuestions, t.Difficulty, kp.Title, mastery, strings.Join(t.QuestionTypes, "、"))
}
```

#### 2.3 渐进式内容生成

```go
// internal/agent/coach.go
func (a *CoachAgent) GenerateQuizProgressive(ctx context.Context, tc *TurnContext) error {
    // 1. 先生成题目大纲
    outline := a.generateQuizOutline(tc)
    a.streamUIEvent(tc, "quiz_outline", outline)
    
    // 2. 逐题生成详细内容
    for i, stem := range outline.Stems {
        question := a.generateQuestionDetail(stem, tc)
        a.streamUIEvent(tc, "quiz_question_ready", map[string]interface{}{
            "index": i,
            "question": question,
        })
    }
    
    // 3. 生成批改标准
    rubric := a.generateGradingRubric(outline)
    a.streamUIEvent(tc, "quiz_complete", rubric)
    
    return nil
}
```

---

## 🎯 优化方向 3: 交互体验提升

### 问题
1. **缺乏即时反馈**
   - 用户提交答案后需要等待 AI 完整响应
   - 没有加载动画和进度提示
   - 无法取消正在进行的生成

2. **错误处理不友好**
   - WebSocket 断线后无法恢复
   - 解析失败时没有降级方案
   - 错误信息对用户不友好

3. **缺乏个性化**
   - 所有学生看到相同的 UI
   - 没有根据学习风格调整展示方式
   - 缺乏历史记录和收藏功能

### 解决方案

#### 3.1 即时反馈系统

```typescript
// frontend/src/components/skill-ui/LoadingStates.tsx
export function QuizGeneratingLoader({ progress }: { progress: number }) {
  return (
    <div className={styles.loader}>
      <div className={styles.spinner} />
      <p>正在生成题目... {progress}%</p>
      <div className={styles.progressBar}>
        <div style={{ width: `${progress}%` }} />
      </div>
      <button onClick={onCancel}>取消生成</button>
    </div>
  );
}
```

#### 3.2 优雅降级

```typescript
// frontend/src/lib/plugin/parsers.ts
export function parseSkillOutput<T>(
  content: string,
  schema: z.ZodSchema<T>,
  fallback: T
): T {
  try {
    const match = content.match(/<skill_output[^>]*>([\s\S]*?)<\/skill_output>/);
    if (!match) return fallback;
    
    const parsed = JSON.parse(match[1]);
    const validated = schema.parse(parsed);
    return validated;
  } catch (error) {
    console.error('Parse failed, using fallback:', error);
    return fallback;
  }
}
```

#### 3.3 个性化 UI

```typescript
// frontend/src/lib/plugin/usePersonalizedUI.ts
export function usePersonalizedUI(studentId: number) {
  const { data: preferences } = useQuery(['student-preferences', studentId], 
    () => apiFetch<StudentPreferences>(`/students/${studentId}/preferences`)
  );
  
  return {
    theme: preferences?.theme || 'default',
    fontSize: preferences?.fontSize || 'medium',
    animationSpeed: preferences?.animationSpeed || 'normal',
    showHints: preferences?.showHints ?? true,
  };
}
```

---

## 🎯 优化方向 4: 性能优化

### 问题
1. **重复渲染**
   - 每次 token 到达都触发整个组件重渲染
   - 大量 useEffect 依赖导致不必要的计算
   - 没有使用 React.memo 和 useMemo

2. **内存泄漏**
   - WebSocket 订阅没有正确清理
   - 定时器和事件监听器泄漏
   - 大量历史消息占用内存

3. **加载性能差**
   - 所有 Renderer 都是同步加载
   - 没有代码分割
   - 大量 CSS 重复打包

### 解决方案

#### 4.1 虚拟化长列表

```typescript
// frontend/src/components/VirtualizedMessageList.tsx
import { useVirtualizer } from '@tanstack/react-virtual';

export function VirtualizedMessageList({ messages }: { messages: ChatMessage[] }) {
  const parentRef = useRef<HTMLDivElement>(null);
  
  const virtualizer = useVirtualizer({
    count: messages.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 100,
    overscan: 5,
  });
  
  return (
    <div ref={parentRef} style={{ height: '600px', overflow: 'auto' }}>
      <div style={{ height: `${virtualizer.getTotalSize()}px` }}>
        {virtualizer.getVirtualItems().map(virtualRow => (
          <div key={virtualRow.index} style={{ transform: `translateY(${virtualRow.start}px)` }}>
            <MessageBubble message={messages[virtualRow.index]} />
          </div>
        ))}
      </div>
    </div>
  );
}
```

#### 4.2 动态导入 Renderer

```typescript
// frontend/src/lib/plugin/SkillRendererLoader.tsx
const rendererMap: Record<string, () => Promise<{ default: React.ComponentType<SkillRendererProps> }>> = {
  'socratic_questioning': () => import('./renderers/SocraticRenderer'),
  'general_assessment_quiz': () => import('./renderers/QuizRenderer'),
  'presentation_generator': () => import('./renderers/PresentationRenderer'),
  // ...
};

export function SkillRendererLoader({ skillId, ...props }: SkillRendererLoaderProps) {
  const Renderer = lazy(rendererMap[skillId]);
  
  return (
    <Suspense fallback={<SkillLoadingFallback />}>
      <Renderer {...props} />
    </Suspense>
  );
}
```

#### 4.3 消息分页和清理

```typescript
// frontend/src/lib/plugin/useMessages.ts
export function useMessages(maxMessages = 100) {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  
  const addMessage = useCallback((msg: ChatMessage) => {
    setMessages(prev => {
      const updated = [...prev, msg];
      // 保留最近 maxMessages 条消息
      if (updated.length > maxMessages) {
        return updated.slice(-maxMessages);
      }
      return updated;
    });
  }, [maxMessages]);
  
  return { messages, addMessage };
}
```

---

## 🎯 优化方向 5: 新 Skill 建议

### 5.1 协作式学习 (Collaborative Learning)
**场景**: 模拟小组讨论，AI 扮演多个学生角色
- 前端：多人对话气泡，观点对比视图
- 后端：多 Agent 协作，生成不同观点

### 5.2 概念地图生成器 (Concept Mapper)
**场景**: 自动生成知识点关系图
- 前端：交互式图谱（D3.js / Cytoscape.js）
- 后端：从 Neo4j 提取关系，生成 Mermaid/GraphViz

### 5.3 实验模拟器 (Lab Simulator)
**场景**: 虚拟物理/化学实验
- 前端：Canvas 动画，参数调节面板
- 后端：物理引擎计算，实验结果生成

### 5.4 辩论训练 (Debate Coach)
**场景**: 训练批判性思维和论证能力
- 前端：正反方观点对比，论据强度评分
- 后端：论证结构分析，逻辑漏洞检测

### 5.5 代码调试助手 (Code Debugger)
**场景**: 编程题目的交互式调试
- 前端：代码编辑器 + 执行结果展示
- 后端：代码执行沙箱，错误诊断

---

## 📊 优先级建议

### P0 (立即执行)
1. ✅ 创建 BaseSkillRenderer 抽象层
2. ✅ 提取共享 UI 组件库
3. ✅ 统一结构化输出协议

### P1 (本周完成)
4. ✅ 实现渐进式内容生成
5. ✅ 添加即时反馈和加载状态
6. ✅ 优化 WebSocket 订阅管理

### P2 (下周完成)
7. ⏳ 实现虚拟化长列表
8. ⏳ 动态导入 Renderer
9. ⏳ 添加错误边界和降级方案

### P3 (未来迭代)
10. 🔮 开发新 Skill (概念地图、协作学习)
11. 🔮 个性化 UI 系统
12. 🔮 性能监控和分析

---

## 🛠️ 实施计划

### 第一阶段：组件重构 (3-5天)
- [ ] 创建 `frontend/src/lib/plugin/base/` 目录
- [ ] 实现 BaseSkillRenderer
- [ ] 创建 `frontend/src/components/skill-ui/` 组件库
- [ ] 重构 QuizRenderer 作为示例

### 第二阶段：内容生成优化 (3-5天)
- [ ] 设计统一的 SkillOutput Schema
- [ ] 创建 `internal/agent/templates/` 模板系统
- [ ] 实现渐进式生成（Quiz 和 Presentation）
- [ ] 添加内容验证和修正逻辑

### 第三阶段：性能优化 (2-3天)
- [ ] 实现虚拟化消息列表
- [ ] 添加 React.memo 和 useMemo
- [ ] 动态导入 Renderer
- [ ] 性能测试和优化

### 第四阶段：新功能开发 (按需)
- [ ] 概念地图生成器 (5-7天)
- [ ] 协作式学习 (7-10天)
- [ ] 实验模拟器 (10-14天)

---

## 📈 预期收益

### 代码质量
- 减少 40% 重复代码
- 提升 60% 可维护性
- 降低 50% 新 Skill 开发时间

### 用户体验
- 减少 30% 等待时间（渐进式生成）
- 提升 50% 交互流畅度（虚拟化）
- 降低 80% 错误率（统一验证）

### 性能指标
- 减少 40% 内存占用
- 提升 50% 首屏加载速度
- 降低 60% 重渲染次数
