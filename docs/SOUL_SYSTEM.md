# Soul System — AI 自进化规则系统

## 概述

Soul 系统是 Hanfledge 的 AI 教学核心规则引擎，定义了所有 AI Agent 的行为准则、教学理念和对话风格。

**核心特性：**
- 📝 Markdown 格式的规则文档
- 🔄 基于教学数据的自动进化
- 📊 版本控制和历史回滚
- 👨‍💼 管理员审核和手动编辑

---

## 架构

```
soul.md (规则文件)
    ↓
Agent System (加载规则)
    ↓
Teaching Data (收集反馈)
    ↓
Soul Evolution Service (分析 + 建议)
    ↓
Admin Review (审核 + 应用)
    ↓
soul.md (更新规则)
```

---

## API 端点

### 查看规则
```http
GET /api/v1/system/soul
Authorization: Bearer <admin_token>
```

**响应：**
```json
{
  "content": "# Hanfledge Soul...",
  "version": "1.0.1709913600",
  "updated_at": "2026-03-08T23:45:00Z"
}
```

### 更新规则
```http
PUT /api/v1/system/soul
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "content": "# Updated rules...",
  "reason": "优化苏格拉底式提问策略"
}
```

### 查看历史
```http
GET /api/v1/system/soul/history
Authorization: Bearer <admin_token>
```

**响应：**
```json
{
  "versions": [
    {
      "id": 1,
      "version": "1.0.1709913600",
      "content": "...",
      "updated_by": 1,
      "reason": "初始版本",
      "is_active": true,
      "created_at": "2026-03-08T23:45:00Z"
    }
  ]
}
```

### 回滚版本
```http
POST /api/v1/system/soul/rollback
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "version_id": 1
}
```

### 触发进化分析
```http
POST /api/v1/system/soul/evolve
Authorization: Bearer <admin_token>
```

---

## 前端界面

**路径：** `/admin/soul`

**功能：**
- 查看当前规则内容
- 在线编辑规则
- 查看版本历史
- 一键回滚到历史版本
- 查看进化建议

**权限：** 仅 SYS_ADMIN 可访问

---

## 自动进化机制

### 数据收集

系统每周自动收集以下指标：

| 指标 | 数据源 | 用途 |
|------|--------|------|
| 平均掌握度提升 | StudentKPMastery | 评估教学效果 |
| 教师干预率 | Interaction (role=teacher) | 识别 AI 不足 |
| 技能使用频率 | Interaction (skill_id) | 优化技能策略 |
| 学生满意度 | 反馈数据 | 调整对话风格 |

### 进化触发

1. **自动触发：** 每周日凌晨 2:00
2. **手动触发：** 管理员点击"分析数据"按钮
3. **异常触发：** 掌握度提升 < 10% 持续 2 周

### 建议生成

使用 LLM 分析数据并生成建议：

```
输入：当前规则 + 教学数据洞察
输出：3-5 条优化建议（内容 + 效果 + 风险）
```

### 审核流程

1. 系统生成建议 → 通知管理员
2. 管理员审核建议 → 编辑规则
3. 管理员保存 → 创建新版本
4. 系统重新加载 → 应用到所有 Agent

---

## 规则加载

### 启动时加载

```go
// cmd/server/main.go
soulContent, _ := os.ReadFile("soul.md")
orchestrator.LoadSoulRules(string(soulContent))
```

### 注入到 Agent

每个 Agent 的 System Prompt 包含 Soul 规则：

```go
systemPrompt := fmt.Sprintf(`%s

## 教学规则（Soul）
%s

请严格遵循以上规则进行教学。`, basePrompt, soulRules)
```

### 热重载

管理员更新规则后，系统自动重新加载：

```go
// After soul.md update
orchestrator.ReloadSoulRules()
```

---

## 版本管理

### 自动备份

每次更新自动创建备份：
```
soul.md.backup.1709913600
soul.md.backup.1709913700
...
```

保留最近 10 个备份，自动清理旧备份。

### 数据库版本

所有版本存储在 `soul_versions` 表：

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint | 主键 |
| version | string | 版本号 |
| content | text | 规则内容 |
| updated_by | uint | 更新人 |
| reason | text | 更新原因 |
| is_active | bool | 是否当前版本 |
| created_at | timestamp | 创建时间 |

---

## 使用示例

### 管理员手动更新

1. 访问 `/admin/soul`
2. 点击"编辑"按钮
3. 修改规则内容
4. 填写修改原因
5. 点击"保存"

### 查看进化建议

1. 系统每周自动分析
2. 管理员收到通知
3. 访问 `/admin/soul`
4. 查看"进化建议"标签
5. 审核并应用建议

### 回滚到历史版本

1. 访问 `/admin/soul`
2. 查看"版本历史"
3. 选择目标版本
4. 点击"回滚"按钮
5. 确认操作

---

## 最佳实践

### 规则编写

1. **清晰性：** 使用简洁明了的语言
2. **可执行：** 规则应当可被 LLM 理解和执行
3. **可测试：** 规则效果应当可通过数据验证
4. **版本化：** 重大更新应增加版本号

### 更新频率

- **小调整：** 每月 1-2 次
- **大更新：** 每季度 1 次
- **紧急修复：** 发现问题立即更新

### 测试验证

更新规则后应：
1. 在沙盒环境测试
2. 观察 1-2 天的教学效果
3. 收集教师反馈
4. 确认无问题后推广

---

## 监控指标

### 规则效果

- 掌握度提升率
- 教师干预率
- 学生满意度
- 对话轮次

### 异常检测

- 掌握度下降 > 10%
- 教师干预率 > 20%
- 学生投诉增加
- 系统错误率上升

---

## 未来增强

- [ ] LLM 驱动的自动规则生成
- [ ] A/B 测试不同规则版本
- [ ] 规则效果可视化面板
- [ ] 多语言规则支持
- [ ] 规则冲突检测
