# 禁用前置知识点自动插入功能

## 问题描述

对话引导的知识点并不是创建活动时指定的知识点,而是其他知识点。

## 根本原因

### 前置知识点自动插入机制

Strategist 在分析时会自动检查每个知识点的前置知识,如果发现前置知识掌握度不足 (< 0.6),会**自动插入**这些前置知识点到学习序列中。

```go
// 原始逻辑
if a.neo4j != nil {
    gapTargets, gapDescs := a.checkPrereqGapsEnriched(ctx, kpID, studentID, masteryMap, prereqInserted)
    targets = append(targets, gapTargets...)  // 自动插入前置 KP ❌
    prereqGaps = append(prereqGaps, gapDescs...)
}
```

### 问题场景

```
活动指定知识点: [KP #5, KP #7]

Neo4j 图谱关系:
  KP #5 → 前置: KP #3 (掌握度 0.3)
  KP #7 → 前置: KP #6 (掌握度 0.4)

Strategist 分析:
  1. 检查 KP #5 → 发现前置 KP #3 掌握不足
     → 自动插入 KP #3 到序列
  2. 检查 KP #7 → 发现前置 KP #6 掌握不足
     → 自动插入 KP #6 到序列

最终序列: [KP #3, KP #6, KP #5, KP #7]
           ^^^^^^^^^^^^^^^^
           这两个不在活动指定列表中! ❌

实际引导: KP #3 (不是活动指定的知识点)
```

## 修复方案

### 核心原则

**严格遵守活动指定的知识点列表,不自动插入前置知识点。**

### 修复方法

禁用前置知识点自动插入功能,只记录前置知识差距用于提示:

```go
// 修复后的逻辑
if a.neo4j != nil {
    // 只检查差距,不自动插入
    _, gapDescs := a.checkPrereqGapsEnriched(ctx, kpID, studentID, masteryMap, nil)
    prereqGaps = append(prereqGaps, gapDescs...)  // 仅记录差距描述
}

// 只添加活动指定的知识点
targets = append(targets, KnowledgePointTarget{
    KPID:           kpID,  // 活动指定的 KP
    CurrentMastery: currentMastery,
    TargetMastery:  0.8,
    ScaffoldLevel:  scaffold,
    SkillID:        skillID,
})
```

## 修复效果

### 修复前

```
活动指定: [KP #5, KP #7]
Strategist 分析: [KP #3, KP #6, KP #5, KP #7]
                  ^^^^^^^^^^^^^^^^
                  自动插入的前置 KP

实际引导: KP #3 ❌ (不在活动列表中)
```

### 修复后

```
活动指定: [KP #5, KP #7]
Strategist 分析: [KP #5, KP #7]
                  ^^^^^^^^^^^^
                  严格遵守活动列表

实际引导: KP #5 ✅ (活动指定的第一个 KP)
前置差距提示: "KP #3 (mastery=0.3)" (仅记录,不插入)
```

## 技术细节

### 修改位置

**文件:** `internal/agent/strategist.go`  
**函数:** `Analyze`  
**行数:** ~107

### 修改内容

```go
// 修改前
if a.neo4j != nil {
    gapTargets, gapDescs := a.checkPrereqGapsEnriched(ctx, kpID, studentID, masteryMap, prereqInserted)
    targets = append(targets, gapTargets...)  // 自动插入
    prereqGaps = append(prereqGaps, gapDescs...)
}

// 修改后
if a.neo4j != nil {
    _, gapDescs := a.checkPrereqGapsEnriched(ctx, kpID, studentID, masteryMap, nil)
    prereqGaps = append(prereqGaps, gapDescs...)  // 只记录差距
}
```

### 保留的功能

- ✅ 前置知识差距检测 (记录在 `prereqGaps`)
- ✅ 可用于 AI 提示或前端显示
- ✅ 不影响活动知识点的学习顺序

### 移除的功能

- ❌ 自动插入前置知识点到学习序列
- ❌ 前置知识点去重逻辑 (`prereqInserted`)

## 影响分析

### 正面影响

1. **知识点引导准确**
   - 严格遵守教师指定的学习路径
   - 学生学习的就是活动要求的内容

2. **学习路径可控**
   - 教师完全控制学习顺序
   - 不会出现"意外"的知识点

3. **用户体验一致**
   - 活动描述与实际学习内容一致
   - 学生不会困惑"为什么学这个"

### 潜在影响

1. **前置知识不足**
   - 学生可能缺少前置知识
   - **解决方案:** 通过 `prereqGaps` 提示学生或教师

2. **学习效果**
   - 可能需要更多支架支持
   - **解决方案:** BKT 会自动调整支架等级

## 替代方案 (未来优化)

### 方案 1: 可配置的前置插入

```go
// 活动配置中添加选项
type LearningActivity struct {
    ...
    AutoInsertPrereqs bool `json:"auto_insert_prereqs"` // 是否自动插入前置
}

// Strategist 中检查配置
if activity.AutoInsertPrereqs && a.neo4j != nil {
    gapTargets, gapDescs := a.checkPrereqGapsEnriched(...)
    targets = append(targets, gapTargets...)
}
```

### 方案 2: 前置知识提示

```go
// 在 AI 回复中提示前置知识差距
if len(prereqGaps) > 0 {
    systemPrompt += "\n\n注意: 学生可能缺少以下前置知识:\n"
    for _, gap := range prereqGaps {
        systemPrompt += "- " + gap + "\n"
    }
    systemPrompt += "请在引导时适当补充这些基础概念。"
}
```

### 方案 3: 前置知识推荐

```go
// 在前端显示前置知识推荐
if len(prereqGaps) > 0 {
    toast("建议先复习: " + strings.Join(prereqGaps, ", "), "info")
}
```

## 测试验证

### 测试场景

1. **创建活动**
   - 指定知识点: [KP #5, KP #7]
   - KP #5 有前置 KP #3 (学生掌握度 0.3)

2. **学生加入**
   - 观察第一条 AI 引导消息

3. **验证结果**
   - ✅ AI 引导 KP #5 (活动指定的)
   - ✅ 不引导 KP #3 (前置知识点)
   - ✅ 日志中记录前置差距

### 日志输出

```
[Strategist] analyzing student student_id=1 activity_id=1
[Strategist] prescription generated targets=2 scaffold=high gaps=1
[Orchestrator] handling turn session_id=1 current_kp=5
```

**关键指标:**
- `targets=2` - 只有活动指定的 2 个知识点
- `current_kp=5` - 引导的是活动的第一个知识点

## 总结

**问题:** 自动插入前置知识点导致引导错误的知识点  
**原因:** `checkPrereqGapsEnriched` 自动插入前置 KP 到学习序列  
**修复:** 禁用自动插入,只记录前置差距用于提示  
**效果:** 严格遵守活动指定的知识点列表

---

**修复日期:** 2026-03-07  
**影响文件:** `internal/agent/strategist.go`  
**测试状态:** 编译通过,待功能测试
