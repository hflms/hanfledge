# 知识点引导问题修复总结

## 问题描述

学习活动中 AI 教学的内容与指定的知识点不相关，无论添加什么知识点，AI 都在教学相同或无关的内容。

## 根本原因分析

### 1. 系统 Prompt 缺少目标知识点 (Commit: 2bf13fd)

**问题：** `Designer.buildSystemPrompt` 没有明确告诉 AI 当前要教学的知识点名称。

**影响：** AI 只能根据检索到的参考材料猜测教学内容，导致内容不稳定。

**修复：**
```go
// 在系统 Prompt 开头添加【当前教学目标】
if len(prescription.TargetKPSequence) > 0 {
    currentKP := prescription.TargetKPSequence[0]
    var kp model.KnowledgePoint
    if err := a.db.First(&kp, currentKP.KPID).Error; err == nil {
        sb.WriteString(fmt.Sprintf("**你必须围绕这个知识点进行教学：%s**\n", kp.Title))
    }
}
```

### 2. RAG 检索使用错误的查询 (Commit: 186461c)

**问题：** `Designer.Assemble` 使用学生输入（如"你好"）作为 RAG 检索查询，而不是知识点标题。

**影响：** 检索到的材料与目标知识点无关。

**修复：**
```go
// 构建增强查询：知识点标题 + 学生输入
enhancedQuery := userInput
if len(prescription.TargetKPSequence) > 0 {
    var kp model.KnowledgePoint
    if err := a.db.First(&kp, prescription.TargetKPSequence[0].KPID).Error; err == nil {
        enhancedQuery = kp.Title + " " + userInput
    }
}
```

### 3. 前置知识点自动插入 (Commit: 347f758)

**问题：** Strategist 自动检查前置知识，如果掌握度不足会自动插入前置知识点到教学序列。

**影响：** 实际教学的知识点不是活动指定的知识点。

**修复：** 禁用 `checkPrereqGapsEnriched` 的自动插入功能，只记录前置知识差距。

### 4. nil map 赋值错误 (Commit: b831dda)

**问题：** 禁用前置知识点自动插入后，传入 `nil` map 导致 panic。

**修复：**
```go
prereqInserted := make(map[uint]bool)
_, gapDescs := a.checkPrereqGapsEnriched(ctx, kpID, studentID, masteryMap, prereqInserted)
```

## 完整修复链路

```
1. 活动创建
   ↓ 指定知识点 IDs: [45, 46, 47]
   
2. 会话创建 (JoinActivity)
   ↓ session.CurrentKP = 45 (第一个知识点)
   
3. Strategist.Analyze
   ↓ 分析活动的所有知识点
   ↓ 按掌握度排序
   ↓ 返回 TargetKPSequence[0].KPID = 45
   ✅ 日志: "strategist analysis complete target_kp_id=45 target_kp_title='循环结构'"
   
4. Designer.Assemble
   ↓ 查询知识点标题: "循环结构"
   ↓ 构建增强查询: "循环结构 你好"
   ✅ 日志: "enhanced query with KP title kp_title='循环结构'"
   ↓ RAG 检索: 使用增强查询
   ↓ 检索到与"循环结构"相关的材料
   
5. Designer.buildSystemPrompt
   ↓ 添加【当前教学目标】
   ↓ "你必须围绕这个知识点进行教学：循环结构"
   ✅ 日志: "system prompt includes target KP kp_title='循环结构'"
   
6. Coach 生成回复
   ↓ 基于系统 Prompt 和检索材料
   ↓ 围绕"循环结构"进行教学
   ✅ 教学内容与指定知识点一致
```

## 验证方法

### 1. 查看日志

启动后端后，观察以下日志：

```bash
# Strategist 分析结果
grep "strategist analysis complete" <log>

# Designer 查询增强
grep "enhanced query with KP title" <log>

# System Prompt 构建
grep "system prompt includes target KP" <log>
```

### 2. 数据库验证

```sql
-- 检查活动的知识点配置
SELECT id, title, kp_ids FROM learning_activities WHERE id = <activity_id>;

-- 检查会话的 CurrentKP
SELECT id, current_kp FROM student_sessions WHERE id = <session_id>;

-- 检查知识点标题
SELECT id, title FROM knowledge_points WHERE id = <kp_id>;
```

### 3. 前端验证

1. 创建学习活动，指定知识点"循环结构"
2. 学生加入活动
3. 观察 AI 的第一条回复
4. 验证内容是否围绕"循环结构"展开

## 相关 Commits

| Commit | 说明 |
|--------|------|
| 347f758 | 禁用前置知识点自动插入 |
| 0b6db45 | 修复知识点引导逻辑 |
| 2bf13fd | 在系统 Prompt 中明确目标知识点 |
| b831dda | 修复 nil map 赋值错误 |
| 186461c | 将知识点标题加入 RAG 检索查询 |
| 44bbfcb | 添加知识点传递链路日志 |
| d8a0615 | 添加测试脚本 |

## 测试脚本

运行测试脚本查看完整的测试指南：

```bash
bash scripts/test-kp-flow.sh
```

## 常见问题

### Q1: AI 仍然教学错误的知识点

**排查步骤：**
1. 检查日志中的 `target_kp_id` 是否正确
2. 检查 `enhanced_query` 是否包含知识点标题
3. 检查 `system prompt includes target KP` 日志

### Q2: 日志中没有知识点信息

**可能原因：**
- `prescription.TargetKPSequence` 为空
- 知识点 ID 不存在
- 数据库查询失败

**解决方法：**
- 检查 Strategist.Analyze 返回值
- 验证 knowledge_points 表数据
- 检查数据库连接

### Q3: 知识点标题查询失败

**可能原因：**
- 知识点 ID 不存在
- 数据库连接问题

**解决方法：**
```sql
SELECT * FROM knowledge_points WHERE id = <kp_id>;
```

## 总结

通过以上修复，确保了：

1. ✅ Strategist 正确分析目标知识点
2. ✅ Designer 使用知识点标题进行 RAG 检索
3. ✅ System Prompt 明确告知 AI 教学目标
4. ✅ 教学内容与指定知识点强相关
5. ✅ 完整的日志追踪链路

**核心原则：** 知识点标题必须贯穿整个 Agent 流程，从 Strategist → Designer → Coach，确保每个环节都以目标知识点为中心。
