# Hanfledge 2.0 优化实施记录

## 已完成优化 (2026-03-07)

### 1. ✅ 并行化 Strategist + Designer (后端)

**文件修改:**
- `internal/agent/orchestrator.go` - 重构 `HandleTurn` 方法
- `internal/repository/neo4j/client.go` - 新增 `GetKPContext` 方法

**实现细节:**
```go
// 并行分支 A: Strategist 分析学情
go func() {
    prescription, err := o.strategist.Analyze(ctx, ...)
    strategistCh <- strategistResult{prescription, err}
}()

// 并行分支 B: Designer 预加载知识图谱上下文 (2 层邻居)
go func() {
    graphCtx, err := o.neo4j.GetKPContext(ctx, session.CurrentKP, 2)
    designerCh <- designerPreloadResult{graphCtx, err}
}()

// 汇聚层: 等待两个分支完成
stratResult := <-strategistCh
designerPreload := <-designerCh
```

**性能提升预期:**
- Strategist (BKT 查询 + 规则推理): ~150ms
- Designer 预加载 (Neo4j 图谱查询): ~100ms
- **串行总耗时**: 250ms → **并行总耗时**: max(150, 100) = 150ms
- **提升**: ~40% 延迟降低

**测试方法:**
```bash
# 启动后端
go run cmd/server/main.go

# 观察日志中的 elapsed 时间
# [DEBUG] strategist done ... elapsed=XXXms
# [DEBUG] designer preload done ... elapsed=XXXms
```

---

### 2. ✅ 前端 VAD (Voice Activity Detection)

**文件修改:**
- `frontend/src/lib/vad.ts` - 新增 VAD 工具模块
- `frontend/src/components/VoiceInput/VoiceInput.tsx` - 集成 Silero VAD
- `frontend/src/components/VoiceInput/VoiceInput.module.css` - 添加 VAD 状态样式

**依赖安装:**
```bash
cd frontend
npm install @ricky0123/vad-web
```

**实现细节:**
- 使用 Silero VAD (WebAssembly) 在浏览器端检测人声
- 只在检测到语音时才发送音频数据到后端
- 双状态指示:
  - 🔴 红色脉冲: 录音中,等待语音
  - 🟢 绿色脉冲: 检测到语音,正在发送

**配置参数:**
```typescript
{
  positiveSpeechThreshold: 0.8,  // 语音检测阈值 (越高越严格)
  minSpeechMs: 1000,              // 最小语音持续时间 1 秒
  preSpeechPadMs: 300,            // 前置缓冲 300ms
  redemptionMs: 1000,             // 后置缓冲 1 秒
}
```

**性能提升预期:**
- 传统模式: 持续发送音频流 (250ms 间隔) → 后端 ASR 持续运行
- VAD 模式: 只在检测到人声时发送 → **降低 50-70% ASR 计算开销**
- 额外好处: 减少网络带宽消耗

**测试方法:**
```bash
cd frontend
npm run dev

# 访问学生会话页面,点击麦克风按钮
# 观察:
# 1. 红色脉冲 = 等待语音
# 2. 绿色脉冲 = 检测到语音
# 3. 浏览器控制台: [VAD] 检测到语音开始/结束
```

**启用/禁用 VAD:**
```tsx
// 在 SessionInput.tsx 中
<VoiceInput
  enableVAD={true}  // 默认启用,设为 false 回退到传统模式
  onTranscript={handleVoiceTranscript}
  agentChannel={agentChannel}
/>
```

---

## 下一步优化 (优先级排序)

### 3. 🔄 双模型 Critic (SLM 哨兵)

**目标:** 使用本地 SLM (Gemma 2B / Llama 3.2 1B) 作为第一道 Critic,只在必要时调用昂贵的 GPT-4。

**实施步骤:**
1. 使用 Ollama 部署 `gemma2:2b`
   ```bash
   ollama pull gemma2:2b
   ```
2. 修改 `internal/agent/critic.go`:
   ```go
   type TwoTierCritic struct {
       fastModel  llm.Client // Gemma 2B
       deepModel  llm.Client // GPT-4
   }
   ```
3. 第一道检查: 长度 + 是否泄露答案 (本地 SLM, <100ms)
4. 第二道检查: 深度语义评估 (仅在必要时, GPT-4)

**预期收益:**
- 80% 的回复通过快速检查 → 节省 GPT-4 调用成本
- Critic 延迟: 2000ms → 100ms (快速通过时)

---

### 4. 🔄 GraphRAG 路径索引

**目标:** 使用 Neo4j `shortestPath` 算法找到学生"已知点"到"目标点"的最短学习路径。

**实施步骤:**
1. 在 `internal/repository/neo4j/client.go` 添加:
   ```go
   func (c *Client) FindLearningPath(ctx context.Context, fromKPID, toKPID uint) ([]KnowledgePoint, error) {
       query := `
       MATCH path = shortestPath(
           (start:KnowledgePoint {id: $from})-[:PREREQUISITE_OF*]-(end:KnowledgePoint {id: $to})
       )
       RETURN [node in nodes(path) | node.name] as path_names
       `
       // ...
   }
   ```
2. 在 Designer 中使用路径作为 Prompt context

**预期收益:**
- AI 引导更精准,减少"跳跃式"教学
- 学生学习路径更连贯

---

### 5. 🔄 FSRS 算法集成

**目标:** 从简单的 BKT 阈值转向科学的遗忘曲线模型。

**实施步骤:**
1. 在 `internal/domain/model/` 添加 `fsrs.go`:
   ```go
   type FSRSCard struct {
       Stability  float64
       Difficulty float64
       LastReview time.Time
   }
   
   func (c *FSRSCard) CalculateRetention(now time.Time) float64 {
       t := now.Sub(c.LastReview).Hours() / 24
       return math.Exp(math.Log(0.9) * t / c.Stability)
   }
   ```
2. 混合支架衰减:
   ```go
   score := 0.6*bktMastery + 0.4*fsrsStability
   ```

**预期收益:**
- 防止"碰巧做对"导致支架撤走过快
- 更科学的复习时机推荐

---

### 6. 🔄 NATS JetStream 事件总线 (长期架构)

**目标:** 替换内存锁,支持横向扩展。

**实施步骤:**
1. 部署 NATS:
   ```bash
   docker run -d -p 4222:4222 nats:latest -js
   ```
2. 创建 `internal/infrastructure/eventbus/nats.go`
3. WebSocket handler 改为发布事件,而非直接调用 Orchestrator
4. Orchestrator 订阅事件并处理

**预期收益:**
- 支持多实例部署
- 消息顺序性由 NATS Sequence Number 保证
- 无状态后端,易于扩展

---

## 性能基准 (优化前)

| 指标                | 当前值      | 目标值      |
|---------------------|-------------|-------------|
| Strategist 耗时     | ~150ms      | ~150ms      |
| Designer 耗时       | ~200ms      | ~100ms      |
| Coach LLM 耗时      | ~2000ms     | ~2000ms     |
| Critic 耗时         | ~2000ms     | ~100ms      |
| **总 TTFT**         | **~4500ms** | **~2500ms** |
| ASR 无效计算率      | ~60%        | ~10%        |

---

## 测试清单

- [x] 后端编译通过
- [x] 前端编译通过
- [ ] 并行化功能测试 (观察日志 elapsed 时间)
- [ ] VAD 功能测试 (浏览器麦克风权限 + 语音检测)
- [ ] 端到端会话测试 (学生发送消息 → AI 回复)
- [ ] 性能基准测试 (对比优化前后 TTFT)

---

## 回滚方案

### 并行化回滚:
```go
// 恢复串行执行
prescription, err := o.strategist.Analyze(ctx, ...)
if err != nil {
    return fmt.Errorf("strategist failed: %w", err)
}
```

### VAD 回滚:
```tsx
// 禁用 VAD
<VoiceInput enableVAD={false} ... />
```

---

## 参考资料

- [Silero VAD](https://github.com/snakers4/silero-vad)
- [@ricky0123/vad-web](https://github.com/ricky0123/vad)
- [FSRS Algorithm](https://github.com/open-spaced-repetition/fsrs4anki/wiki/The-Algorithm)
- [NATS JetStream](https://docs.nats.io/nats-concepts/jetstream)
