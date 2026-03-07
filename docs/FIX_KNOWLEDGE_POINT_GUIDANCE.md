# 知识点引导逻辑修复

## 问题描述

学习活动指定了章节和知识点,但聊天引导的却不是指定的知识点,而是其他知识点。

## 根本原因

### 原始逻辑缺陷

```go
// 错误的逻辑
targetKPID := session.CurrentKP  // 使用会话初始化时的第一个知识点
if targetKPID == 0 && len(prescription.TargetKPSequence) > 0 {
    targetKPID = prescription.TargetKPSequence[0].KPID
}
```

**问题:**
1. 会话创建时 `CurrentKP` 被设置为活动的第一个知识点
2. Strategist 分析时会根据学生掌握度对知识点重新排序
3. 但代码优先使用 `session.CurrentKP`,导致引导的知识点与 Strategist 分析结果不一致

### 执行流程

```
1. 创建会话
   └─> CurrentKP = activity.KPIDS[0]  (例如: KP #5)

2. Strategist 分析
   └─> 根据掌握度排序
       └─> TargetKPSequence[0] = KP #3 (掌握度最低)

3. Orchestrator 使用 (错误)
   └─> targetKPID = session.CurrentKP  (KP #5)
       └─> 引导的是 KP #5,而不是 KP #3 ❌
```

## 修复方案

### 核心原则

**始终使用 Strategist 分析后的目标知识点**,因为它已经根据学生掌握度智能排序。

### 修复后的逻辑

```go
// 正确的逻辑
targetKPID := uint(0)
if len(prescription.TargetKPSequence) > 0 {
    targetKPID = prescription.TargetKPSequence[0].KPID  // 优先使用分析结果
}
// 回退到会话的 CurrentKP (仅当 Strategist 没有返回结果时)
if targetKPID == 0 {
    targetKPID = session.CurrentKP
}

// 更新会话的 CurrentKP 为实际引导的知识点
if targetKPID != session.CurrentKP && targetKPID != 0 {
    o.db.WithContext(tc.Ctx).Model(&session).Update("current_kp", targetKPID)
    session.CurrentKP = targetKPID
}
```

### 修复位置

**文件:** `internal/agent/orchestrator.go`

**位置 1: HandleTurn - 目标知识点确定** (行 ~228)
```go
// 使用 Strategist 分析后的第一个目标知识点 (已按掌握度排序)
targetKPID := uint(0)
if len(prescription.TargetKPSequence) > 0 {
    targetKPID = prescription.TargetKPSequence[0].KPID
}
// 如果 Strategist 没有返回目标,回退到会话的 CurrentKP
if targetKPID == 0 {
    targetKPID = session.CurrentKP
}
// 更新会话的 CurrentKP 为实际引导的知识点
if targetKPID != session.CurrentKP && targetKPID != 0 {
    o.db.WithContext(tc.Ctx).Model(&session).Update("current_kp", targetKPID)
    session.CurrentKP = targetKPID
}
```

**位置 2: updateMasteryAndFadeScaffold - 掌握度更新** (行 ~620)
```go
// 获取当前目标 KP (优先使用 Strategist 分析后的结果)
kpID := uint(0)
if tc.Prescription != nil && len(tc.Prescription.TargetKPSequence) > 0 {
    kpID = tc.Prescription.TargetKPSequence[0].KPID
}
// 回退到会话的 CurrentKP
if kpID == 0 {
    kpID = session.CurrentKP
}
```

## 修复效果

### 修复前

```
学习活动: [KP #5, KP #3, KP #7]
学生掌握度: KP #5 (0.6), KP #3 (0.2), KP #7 (0.8)

Strategist 分析:
  └─> 排序后: [KP #3 (0.2), KP #5 (0.6), KP #7 (0.8)]

实际引导: KP #5 ❌ (使用了 session.CurrentKP)
```

### 修复后

```
学习活动: [KP #5, KP #3, KP #7]
学生掌握度: KP #5 (0.6), KP #3 (0.2), KP #7 (0.8)

Strategist 分析:
  └─> 排序后: [KP #3 (0.2), KP #5 (0.6), KP #7 (0.8)]

实际引导: KP #3 ✅ (使用 Strategist 分析结果)
会话更新: session.CurrentKP = 3
```

## 技术细节

### Strategist 排序逻辑

```go
// internal/agent/strategist.go
func sortTargetsByMastery(targets []KnowledgePointTarget) {
    sort.Slice(targets, func(i, j int) bool {
        return targets[i].CurrentMastery < targets[j].CurrentMastery
    })
}
```

**排序规则:** 掌握度低的优先 (从易到难,循序渐进)

### 会话 CurrentKP 同步

修复后,`session.CurrentKP` 会在每次 turn 时自动更新为 Strategist 分析的目标知识点,确保:
1. 前端显示的当前知识点正确
2. 后续 turn 的回退逻辑正确
3. 数据分析和报表准确

## 测试验证

### 测试场景

1. **创建学习活动**
   - 指定知识点: [KP #5, KP #3, KP #7]

2. **学生加入会话**
   - 初始掌握度: KP #5 (0.6), KP #3 (0.2), KP #7 (0.8)

3. **发送第一条消息**
   - 观察 AI 引导的知识点

### 预期结果

- ✅ AI 引导 KP #3 (掌握度最低)
- ✅ 会话 `current_kp` 更新为 3
- ✅ 后续交互围绕 KP #3 展开

### 验证方法

```bash
# 启动后端
go run cmd/server/main.go

# 观察日志
# [Strategist] prescription generated targets=3 ...
# [Orchestrator] handling turn ... current_kp=3
```

## 相关代码

### Strategist 分析

```go
// internal/agent/strategist.go
func (a *StrategistAgent) Analyze(...) (LearningPrescription, error) {
    // 1. 加载活动的知识点
    kpIDs, _ := parseKPIDs(activity.KPIDS)
    
    // 2. 查询学生掌握度
    masteryMap := ...
    
    // 3. 构建目标序列
    for _, kpID := range kpIDs {
        targets = append(targets, KnowledgePointTarget{
            KPID:           kpID,
            CurrentMastery: masteryMap[kpID],
            ...
        })
    }
    
    // 4. 按掌握度排序 (低到高)
    sortTargetsByMastery(targets)
    
    return LearningPrescription{
        TargetKPSequence: targets,  // 已排序
        ...
    }
}
```

### Designer 使用

```go
// internal/agent/designer.go
func (a *DesignerAgent) Assemble(...) (PersonalizedMaterial, error) {
    // 使用 prescription.TargetKPSequence 进行图谱检索
    graphChunks, _ := a.graphContentSearch(ctx, courseID, prescription.TargetKPSequence)
    graphNodes := a.graphSearch(ctx, prescription.TargetKPSequence)
    ...
}
```

## 影响范围

### 直接影响

- ✅ 知识点引导逻辑正确
- ✅ 掌握度更新准确
- ✅ 支架衰减基于正确的知识点

### 间接影响

- ✅ 学习路径更合理 (从易到难)
- ✅ 学生体验更好 (不会突然跳到难点)
- ✅ 数据分析更准确

## 注意事项

### 向后兼容

修复后的逻辑保持向后兼容:
- 如果 Strategist 没有返回结果,仍然回退到 `session.CurrentKP`
- 不影响现有会话的正常运行

### 性能影响

- 每次 turn 增加一次数据库更新 (更新 `current_kp`)
- 影响可忽略 (单条 UPDATE 语句)

## 总结

**问题:** 会话初始化的 `CurrentKP` 与 Strategist 分析结果不一致  
**原因:** 代码优先使用 `session.CurrentKP` 而非 Strategist 分析结果  
**修复:** 始终优先使用 `prescription.TargetKPSequence[0].KPID`  
**效果:** 知识点引导逻辑正确,学习路径更合理

---

**修复日期:** 2026-03-07  
**影响文件:** `internal/agent/orchestrator.go`  
**测试状态:** 编译通过,待功能测试
