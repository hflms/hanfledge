# Hanfledge MVP V1.0 — TODO Tasks

**Last Updated:** 2026-02-27 22:27
**Tech Stack:** Go (Gin+GORM) / Next.js / PostgreSQL (pgvector) / Neo4j / Redis
**Reference:** [design.md](./design.md)

---

## 进度总览

| Phase | 状态 | 完成度 | Commit |
|---|---|---|---|
| Phase 0: 项目骨架 | ✅ 已完成 | 100% | `67ed390` |
| Phase 1: 用户与权限系统 | ✅ 已完成 | 100% | `490e294` |
| Phase 2: 课程与知识引擎 | 🔧 进行中 | 60% | `dfc176a` |
| Phase 3: 技能系统 | ⬜ 未开始 | 0% | — |
| Phase 4: AI 对话引擎 | ⬜ 未开始 | 0% | — |
| Phase 5: 学情仪表盘 | ⬜ 未开始 | 0% | — |
| Phase 6: 集成联调与部署 | ⬜ 未开始 | 0% | — |

**总进度: 28 / 73 tasks (38%)**

---

## Phase 0: 项目骨架 ✅ `67ed390`

- [x] T-0.1: Go 项目初始化 (`go mod init`, 目录结构按 design.md §11)
- [x] T-0.2: 核心依赖安装 (gin, gorm, viper, jwt-go, godotenv)
- [x] T-0.3: Config 包 — `.env` 读取，结构化配置 (Server/DB/Neo4j/Redis/JWT/LLM)
- [x] T-0.4: `cmd/server/main.go` 启动入口 + `GET /health` 端点
- [x] T-0.5: Docker Compose — PostgreSQL(pgvector) + Neo4j 5 + Redis 7 (端口: 5433/7688/6381)
- [x] T-0.6: Next.js 前端初始化 (App Router, TypeScript, Vanilla CSS)
- [x] T-0.7: `.gitignore` + `.env.example`

---

## Phase 1: 用户与权限系统 ✅ `490e294`

- [x] T-1.1: GORM Models — User, School, Class, Role, UserSchoolRole, ClassStudent
- [x] T-1.2: AutoMigrate 自动建表 (15张) + 默认角色 Seed (SYS_ADMIN/SCHOOL_ADMIN/TEACHER/STUDENT)
- [x] T-1.3: `POST /api/v1/auth/login` — JWT 登录 (bcrypt 密码验证)
- [x] T-1.4: `GET /api/v1/auth/me` — 获取当前用户 + 角色 (Preload)
- [x] T-1.5: JWT 中间件 — Bearer Token 解析，注入 user_id 到 Context
- [x] T-1.6: RBAC 中间件 — 查询用户角色，按需拦截未授权请求 (403)
- [x] T-1.7: `GET/POST /api/v1/schools` (SYS_ADMIN only)
- [x] T-1.8: `GET/POST /api/v1/classes` (SYS_ADMIN, SCHOOL_ADMIN)
- [x] T-1.9: `GET/POST /api/v1/users` + `POST /users/batch` 批量创建
- [x] T-1.10: `scripts/seed.go` — 1管理员, 1学校, 2班级, 2教师(1人双角色), 10学生

**测试账号:**
| 角色 | 手机号 | 密码 |
|---|---|---|
| 系统管理员 | 13800000001 | admin123 |
| 教师+学校管理员 | 13800000010 | teacher123 |
| 教师 | 13800000011 | teacher123 |
| 学生(高一1班) | 13800000100 | student123 |
| 学生(高一2班) | 13800000105 | student123 |

---

## Phase 2: 课程与知识引擎 🔧

### 后端 — 已完成 ✅
- [x] T-2.1: GORM Models — Course, Chapter, KnowledgePoint
- [x] T-2.2: GORM Models — Document, DocumentChunk (pgvector vector(1024))
- [x] T-2.3: Ollama LLM Client — Chat (非流式) + Embedding API
- [x] T-2.4: Neo4j Client — 连接, Schema 初始化, Course/Chapter/KP CRUD, REQUIRES 关系
- [x] T-2.5: KA-RAG Engine — Hybrid Slicing (段落切分, 短片段过滤)
- [x] T-2.6: KA-RAG Engine — LLM 知识结构提取 (JSON 输出, markdown fence 清洗)
- [x] T-2.7: KA-RAG Engine — 双写 PostgreSQL (章节/知识点) + Neo4j (图谱节点/关系)
- [x] T-2.8: `POST /api/v1/courses` — 创建课程 (TEACHER)
- [x] T-2.9: `POST /api/v1/courses/:id/materials` — PDF 上传 + 异步 KA-RAG 处理
- [x] T-2.10: `GET /api/v1/courses/:id/outline` — 大纲查询 (Preload 章节+知识点)
- [x] T-2.11: `GET /api/v1/courses/:id/documents` — 文档处理状态查询

### 后端 — 待完成
- [ ] T-2.12: **Embedding 向量生成 + pgvector 写入**
  - 在 KA-RAG pipeline 中调用 Ollama `/api/embed`
  - 为每个 DocumentChunk 生成 1024 维向量
  - 写入 `document_chunks.embedding` (pgvector)
- [ ] T-2.13: **语义检索 API** `POST /api/v1/courses/:id/search`
  - 接收查询文本 → 生成 query embedding
  - pgvector 余弦相似度检索 Top-K chunks
  - 返回匹配的文档切片和相似度分数
- [ ] T-2.14: **Course Model 时间字段修复**
  - `CreatedAt`/`UpdatedAt` 从 `string` 改为 `time.Time`

### 前端 — 待完成
- [ ] T-2.15: **登录页面** (`/login`)
  - 手机号 + 密码表单，调用 `/auth/login`
  - JWT 存储到 cookie/localStorage
  - 登录后按角色路由 (教师→Dashboard, 学生→活动列表)
  - 现代化 UI: 渐变背景, 毛玻璃卡片
- [ ] T-2.16: **主布局 Layout** (`/teacher/layout`, `/student/layout`)
  - 左侧导航栏 (教师/学生差异化菜单)
  - 顶部 Header (用户名, 角色标签, 登出)
  - 面包屑导航
  - 响应式: 移动端折叠侧边栏
- [ ] T-2.17: **教师课程管理** (`/teacher/courses`)
  - 课程列表 (卡片/表格视图)
  - 创建课程对话框 (表单)
  - 课程状态标签 (draft/published/archived)
- [ ] T-2.18: **教材上传** (`/teacher/courses/[id]/materials`)
  - PDF 拖拽上传区域
  - 上传进度条
  - 文档处理状态轮询展示 (uploaded → processing → completed)
- [ ] T-2.19: **大纲编辑器** (`/teacher/courses/[id]/outline`)
  - 树形组件展示章节 → 知识点层级
  - 节点编辑 (标题, 难度, 是否重点)
  - 知识点上显示已挂载的技能标签

---

## Phase 3: 技能系统 ⬜

### 后端
- [ ] T-3.1: **Plugin Registry 基础版**
  - 启动时扫描 `/plugins/skills/` 目录
  - 读取 `backend/metadata.json`，注册到内存 Map
  - 提供 `GetSkill(id)`, `ListSkills(filter)` 方法
- [ ] T-3.2: **SkillPlugin 接口定义** (design.md §7.3)
  - `Match(ctx, intent) (float64, error)`
  - `LoadConstraints(ctx) (*SkillConstraints, error)`
  - `LoadTemplates(ctx, ids) ([]Template, error)`
  - `Evaluate(ctx, interaction) (*SkillEvalResult, error)`
- [ ] T-3.3: **苏格拉底引导技能实现**
  - `plugins/skills/socratic-questioning/backend/metadata.json`
  - `plugins/skills/socratic-questioning/backend/SKILL.md`
  - Match: 检测概念困惑类意图
  - LoadConstraints: 返回引导式提问约束
- [ ] T-3.4: `GET /api/v1/skills` — 技能列表 (支持 subject/level 过滤)
- [ ] T-3.5: `POST /api/v1/chapters/:id/skills` — 挂载技能 (创建 KPSkillMount)
- [ ] T-3.6: `DELETE /api/v1/chapters/:id/skills/:mount_id` — 卸载技能

### 前端
- [ ] T-3.7: **Skill Store 页面** (`/teacher/skills`)
  - 技能卡片列表 (名称, 描述, 适用学科, 类型图标)
  - 按学科/类型筛选
- [ ] T-3.8: **大纲树上的技能挂载交互**
  - 知识点节点 "+" 按钮 → 弹出技能选择面板
  - 拖拽或点选挂载
  - 已挂载技能显示为标签
- [ ] T-3.9: **技能参数配置面板**
  - 支架强度选择 (高/中/低)
  - 渐进规则编辑 (mastery 阈值触发降级)

---

## Phase 4: AI 对话引擎 ⬜

### 后端 — Agent 编排
- [ ] T-4.1: **AgentOrchestrator 骨架** (design.md §9.1)
  - goroutine + channel 通信管道
  - Strategist → Designer → Coach → Critic 编排
- [ ] T-4.2: **StrategistAgent**
  - 查询学生 `StudentKPMastery`
  - 生成 `LearningPrescription` (目标知识点序列, 初始支架, 推荐技能)
- [ ] T-4.3: **DesignerAgent**
  - 调用 RRF 混合检索
  - 组装个性化学习上下文 (知识材料 + 学生画像)
- [ ] T-4.4: **CoachAgent**
  - 加载对应 Skill 的 SKILL.md 约束
  - 多轮对话状态管理 (历史消息)
  - 流式 LLM 调用 (Ollama streaming)
- [ ] T-4.5: **CriticAgent**
  - Actor-Critic 审查: 是否泄露答案? 启发深度是否足够?
  - 不合格则打回 Coach 重新生成

### 后端 — 检索与会话
- [ ] T-4.6: **RRF 混合检索** (design.md §8.1)
  - pgvector 语义检索 Top-50
  - Neo4j Cypher 图谱检索 Top-50
  - RRF 倒数排名融合 → Top-10
- [ ] T-4.7: **学习活动 CRUD**
  - `POST /api/v1/activities` — 教师发布活动 (关联课程+知识点+班级)
  - `GET /api/v1/activities` — 学生查看可参与活动
  - `POST /api/v1/activities/:id/join` — 学生加入活动，创建 Session
- [ ] T-4.8: **WebSocket Handler** (design.md §14.2)
  - `ws://<host>/api/v1/sessions/:id/stream`
  - 客户端事件: `user_message`
  - 服务端事件: `agent_thinking`, `token_delta`, `ui_scaffold_change`, `turn_complete`
- [ ] T-4.9: **BKT 算法实现** (design.md §9.2)
  - 贝叶斯后验更新 `UpdateMastery(prior, correct) → float64`
  - 每次学生回答后更新 `StudentKPMastery`
- [ ] T-4.10: **支架渐隐逻辑**
  - mastery ≥ 0.6 → medium, ≥ 0.8 → low
  - 发送 `ui_scaffold_change` 事件通知前端

### 前端
- [ ] T-4.11: **学生活动列表** (`/student/activities`)
  - 可用活动卡片 (课程名, 知识点范围, 截止日期)
  - 已完成/进行中状态标记
- [ ] T-4.12: **AI 对话主页面** (`/student/session/[id]`)
  - WebSocket 连接管理 (连接/断线重连)
  - 流式打字机效果 (逐 token 渲染)
  - 消息气泡 (学生蓝色/AI 白色)
  - 输入框 + 发送按钮
- [ ] T-4.13: **支架 UI 组件** (design.md §7.13)
  - 高支架: 分步引导面板 + 关键词高亮
  - 中支架: 底部关键词 Tag 提示
  - 低支架: 纯空白输入框
  - 根据 `ui_scaffold_change` 动态切换
- [ ] T-4.14: **Agent 思考状态展示**
  - "Strategist 正在分析学情..." 加载动画
  - "Designer 正在检索知识图谱..." 进度指示
  - "Coach 正在组织回复..." 打字指示器

---

## Phase 5: 学情仪表盘 ⬜

### 后端
- [ ] T-5.1: `GET /api/v1/dashboard/knowledge-radar`
  - 聚合全班 mastery_score (按知识点)
  - 返回雷达图格式: `{labels: [...], values: [...]}`
- [ ] T-5.2: `GET /api/v1/students/:id/mastery`
  - 个人所有知识点掌握度
  - 历史趋势 (按时间排序的 mastery 变化)
- [ ] T-5.3: `GET /api/v1/activities/:id/sessions`
  - 活动统计: 完成率, 平均时长, 平均掌握度

### 前端
- [ ] T-5.4: **教师 Dashboard** (`/teacher/dashboard`)
  - 全班知识漏洞雷达图 (ECharts)
  - 活动参与统计卡片 (完成率, 平均时长)
  - 最近活动列表
- [ ] T-5.5: **学生掌握度详情**
  - 个人 mastery 变化趋势折线图
  - 知识点掌握度热力图/进度条
- [ ] T-5.6: 安装 ECharts (`npm install echarts echarts-for-react`)

---

## Phase 6: 集成联调与部署 ⬜

- [ ] T-6.1: **端到端流程测试**
  - 教师: 上传PDF → 查看大纲 → 挂载技能 → 发布活动
  - 学生: 打开活动 → AI对话 → mastery更新
  - 教师: 查看仪表盘 → 看到学情数据
- [ ] T-6.2: `Dockerfile.backend` — Go 多阶段构建 (builder + alpine)
- [ ] T-6.3: `Dockerfile.frontend` — Next.js standalone 模式
- [ ] T-6.4: `docker-compose.prod.yml` — 全栈生产部署 (Nginx + Go + Next.js + DB)
- [ ] T-6.5: 输入过滤中间件 — 防 Prompt Injection (正则+关键词)
- [ ] T-6.6: PII 脱敏中间件 — LLM 调用前替换学生姓名/学校名
- [ ] T-6.7: 性能基准测试 — 单节点并发对话数
- [ ] T-6.8: README.md 完善 — 快速开始指南, 架构图, API 文档链接
