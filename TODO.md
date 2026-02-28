# Hanfledge MVP V1.0 — TODO Tasks

**Last Updated:** 2026-02-28 16:00
**Tech Stack:** Go (Gin+GORM) / Next.js / PostgreSQL (pgvector) / Neo4j / Redis
**Reference:** [design.md](./design.md)

---

## 进度总览

| Phase | 状态 | 完成度 | Commit |
|---|---|---|---|
| Phase 0: 项目骨架 | ✅ 已完成 | 100% | `67ed390` |
| Phase 1: 用户与权限系统 | ✅ 已完成 | 100% | `490e294` |
| Phase 2: 课程与知识引擎 | ✅ 已完成 | 100% | `dfc176a` |
| Phase 3: 技能系统 | ✅ 已完成 | 100% | — |
| Phase 4: AI 对话引擎 | ✅ 已完成 | 100% | — |
| Phase 5: 学情仪表盘 | ✅ 已完成 | 100% | — |
| Phase 6: 集成联调与部署 | ✅ 已完成 | 100% | — |
| **Post-MVP: LLM 真流式输出** | **✅ 已完成** | **100%** | — |
| **Post-MVP: Redis 缓存集成** | **✅ 已完成** | **100%** | — |
| **Post-MVP: 完善插件系统** | **✅ 已完成** | **100%** | — |
| **Post-MVP: 多 LLM Provider** | **✅ 已完成** | **100%** | — |
| **Post-MVP: 前端 UI 优化** | **✅ 已完成** | **100%** | — |
| **Post-MVP: 单元测试补全** | **✅ 已完成** | **100%** | — |

**MVP 总进度: 73 / 73 tasks (100%)**
**V2.0 进度: Phase A ✅ + Phase B ✅ + Phase C ✅ + Phase D ✅ + Phase E ✅ + Phase F ✅ + Phase G ✅ + Phase H ✅ + Phase I ✅ + WS Robustness ✅**

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
- [x] T-2.12: **Embedding 向量生成 + pgvector 写入**
  - 在 KA-RAG pipeline 中调用 Ollama `/api/embed`
  - 为每个 DocumentChunk 生成 1024 维向量
  - 写入 `document_chunks.embedding` (pgvector)
- [x] T-2.13: **语义检索 API** `POST /api/v1/courses/:id/search`
  - 接收查询文本 → 生成 query embedding
  - pgvector 余弦相似度检索 Top-K chunks
  - 返回匹配的文档切片和相似度分数
- [x] T-2.14: **Course Model 时间字段修复**
  - `CreatedAt`/`UpdatedAt` 从 `string` 改为 `time.Time`

### 前端 — 待完成
- [x] T-2.15: **登录页面** (`/login`)
  - 手机号 + 密码表单，调用 `/auth/login`
  - JWT 存储到 cookie/localStorage
  - 登录后按角色路由 (教师→Dashboard, 学生→活动列表)
  - 现代化 UI: 渐变背景, 毛玻璃卡片
- [x] T-2.16: **主布局 Layout** (`/teacher/layout`, `/student/layout`)
  - 左侧导航栏 (教师/学生差异化菜单)
  - 顶部 Header (用户名, 角色标签, 登出)
  - 面包屑导航
  - 响应式: 移动端折叠侧边栏
- [x] T-2.17: **教师课程管理** (`/teacher/courses`)
  - 课程列表 (卡片/表格视图)
  - 创建课程对话框 (表单)
  - 课程状态标签 (draft/published/archived)
- [x] T-2.18: **教材上传** (`/teacher/courses/[id]/materials`)
  - PDF 拖拽上传区域
  - 上传进度条
  - 文档处理状态轮询展示 (uploaded → processing → completed)
- [x] T-2.19: **大纲编辑器** (`/teacher/courses/[id]/outline`)
  - 树形组件展示章节 → 知识点层级
  - 节点编辑 (标题, 难度, 是否重点)
  - 知识点上显示已挂载的技能标签

---

## Phase 3: 技能系统 ✅

### 后端
- [x] T-3.1: **Plugin Registry 基础版**
  - 启动时扫描 `/plugins/skills/` 目录
  - 读取 `backend/metadata.json`，注册到内存 Map
  - 提供 `GetSkill(id)`, `ListSkills(filter)` 方法
- [x] T-3.2: **SkillPlugin 接口定义** (design.md §7.3)
  - `Match(ctx, intent) (float64, error)`
  - `LoadConstraints(ctx) (*SkillConstraints, error)`
  - `LoadTemplates(ctx, ids) ([]Template, error)`
  - `Evaluate(ctx, interaction) (*SkillEvalResult, error)`
- [x] T-3.3: **苏格拉底引导技能实现**
  - `plugins/skills/socratic-questioning/backend/metadata.json`
  - `plugins/skills/socratic-questioning/backend/SKILL.md`
  - Match: 检测概念困惑类意图
  - LoadConstraints: 返回引导式提问约束
- [x] T-3.4: `GET /api/v1/skills` — 技能列表 (支持 subject/level 过滤)
- [x] T-3.5: `POST /api/v1/chapters/:id/skills` — 挂载技能 (创建 KPSkillMount)
- [x] T-3.6: `DELETE /api/v1/chapters/:id/skills/:mount_id` — 卸载技能

### 前端
- [x] T-3.7: **Skill Store 页面** (`/teacher/skills`)
  - 技能卡片列表 (名称, 描述, 适用学科, 类型图标)
  - 按学科/类型筛选
- [x] T-3.8: **大纲树上的技能挂载交互**
  - 章节节点 "+" 按钮 → 弹出技能选择面板
  - 点选挂载，已挂载标记禁用
  - 已挂载技能显示为可点击标签 (含支架等级)
- [x] T-3.9: **技能参数配置面板**
  - 支架强度选择 (高/中/低)
  - 渐进规则编辑 (mastery 阈值触发降级)
  - PATCH /api/v1/chapters/:id/skills/:mount_id 更新配置
  - 卸载按钮集成在配置面板中

---

## Phase 4: AI 对话引擎 ✅

### 后端 — Agent 编排
- [x] T-4.1: **AgentOrchestrator 骨架** (design.md §9.1)
  - goroutine + channel 通信管道
  - Strategist → Designer → Coach → Critic 编排
- [x] T-4.2: **StrategistAgent**
  - 查询学生 `StudentKPMastery`
  - 生成 `LearningPrescription` (目标知识点序列, 初始支架, 推荐技能)
- [x] T-4.3: **DesignerAgent**
  - 调用 RRF 混合检索
  - 组装个性化学习上下文 (知识材料 + 学生画像)
- [x] T-4.4: **CoachAgent**
  - 加载对应 Skill 的 SKILL.md 约束
  - 多轮对话状态管理 (历史消息)
  - 流式 LLM 调用 (Ollama streaming)
- [x] T-4.5: **CriticAgent**
  - Actor-Critic 审查: 是否泄露答案? 启发深度是否足够?
  - 不合格则打回 Coach 重新生成

### 后端 — 检索与会话
- [x] T-4.6: **RRF 混合检索** (design.md §8.1)
  - pgvector 语义检索 Top-50
  - Neo4j Cypher 图谱检索 Top-50
  - RRF 倒数排名融合 → Top-10
- [x] T-4.7: **学习活动 CRUD**
  - `POST /api/v1/activities` — 教师发布活动 (关联课程+知识点+班级)
  - `GET /api/v1/activities` — 学生查看可参与活动
  - `POST /api/v1/activities/:id/join` — 学生加入活动，创建 Session
- [x] T-4.8: **WebSocket Handler** (design.md §14.2)
  - `ws://<host>/api/v1/sessions/:id/stream`
  - 客户端事件: `user_message`
  - 服务端事件: `agent_thinking`, `token_delta`, `ui_scaffold_change`, `turn_complete`
- [x] T-4.9: **BKT 算法实现** (design.md §9.2)
  - 贝叶斯后验更新 `UpdateMastery(prior, correct) → float64`
  - 每次学生回答后更新 `StudentKPMastery`
- [x] T-4.10: **支架渐隐逻辑**
  - mastery ≥ 0.6 → medium, ≥ 0.8 → low
  - 发送 `ui_scaffold_change` 事件通知前端
  - BKT 掌握度更新后自动检查阈值跨越

### 前端
- [x] T-4.11: **学生活动列表** (`/student/activities`)
  - 可用活动卡片 (课程名, 知识点范围, 截止日期)
  - 已完成/进行中状态标记
- [x] T-4.12: **AI 对话主页面** (`/student/session/[id]`)
  - WebSocket 连接管理 (连接/断线重连)
  - 流式打字机效果 (逐 token 渲染)
  - 消息气泡 (学生蓝色/AI 白色)
  - 输入框 + 发送按钮
- [x] T-4.13: **支架 UI 组件** (design.md §7.13)
  - 高支架: 分步引导面板 + 关键词高亮
  - 中支架: 底部关键词 Tag 提示
  - 低支架: 纯空白输入框
  - 根据 `ui_scaffold_change` 动态切换
- [x] T-4.14: **Agent 思考状态展示**
  - "Strategist 正在分析学情..." 加载动画
  - "Designer 正在检索知识图谱..." 进度指示
  - "Coach 正在组织回复..." 打字指示器

---

## Phase 5: 学情仪表盘 ✅

### 后端
- [x] T-5.1: `GET /api/v1/dashboard/knowledge-radar`
  - 聚合全班 mastery_score (按知识点)
  - 返回雷达图格式: `{labels: [...], values: [...]}`
- [x] T-5.2: `GET /api/v1/students/:id/mastery`
  - 个人所有知识点掌握度
  - 历史趋势 (按时间排序的 mastery 变化)
- [x] T-5.3: `GET /api/v1/activities/:id/sessions`
  - 活动统计: 完成率, 平均时长, 平均掌握度

### 前端
- [x] T-5.4: **教师 Dashboard** (`/teacher/dashboard`)
  - 全班知识漏洞雷达图 (ECharts)
  - 活动参与统计卡片 (完成率, 平均时长)
  - 最近活动列表
- [x] T-5.5: **学生掌握度详情** (`/student/mastery`)
  - 个人 mastery 变化趋势折线图
  - 知识点掌握度进度条
- [x] T-5.6: 安装 ECharts (`npm install echarts echarts-for-react`)

---

## Phase 6: 集成联调与部署 ✅

- [x] T-6.1: **端到端流程测试**
  - `internal/delivery/http/e2e_test.go` — 26 个集成测试全部通过
  - 覆盖: 健康检查, 登录鉴权, RBAC 权限, CRUD 操作, 全链路 E2E 流程
  - E2E 流程: 教师建课→查看大纲→创建活动→发布→学生查看活动→加入→查看会话→掌握度更新→教师仪表盘
  - 权限测试: 活动发布所有权, 会话访问控制, 仪表盘权限, 课程列表过滤
- [x] T-6.2: **`Dockerfile.backend`** — Go 多阶段构建 (golang:1.25-alpine → alpine:3.21)
  - CGO_ENABLED=0 静态编译，-ldflags="-s -w" 压缩 (27MB)
  - 非 root 用户 (hanfledge)，HEALTHCHECK /health
  - 包含 plugins/ 技能定义文件
- [x] T-6.3: **`Dockerfile.frontend`** — Next.js standalone 模式
  - 三阶段: deps → builder → runner (node:22-alpine)
  - `output: "standalone"` 配置已启用
  - 构建时注入 NEXT_PUBLIC_API_BASE_URL
  - 非 root 用户，HEALTHCHECK
- [x] T-6.4: **`docker-compose.prod.yml`** — 全栈生产部署
  - Nginx 反向代理 (API/WebSocket → backend, 其余 → frontend)
  - 7 个服务: nginx + backend + frontend + postgres + neo4j + redis
  - `deployments/nginx.conf` — WebSocket 支持, 50MB 上传限制
  - 环境变量通过 .env 文件注入，所有服务 restart: unless-stopped
- [x] T-6.5: **输入过滤中间件 — 防 Prompt Injection (正则+关键词)**
  - `internal/infrastructure/safety/injection.go` — InjectionGuard
  - 三层防御: 长度限制(2000字) + 关键词黑名单(60条中英文) + 正则模式(14条)
  - 检测结果: Safe / Warning / Blocked
  - 集成到 WebSocket handler，阻断后返回用户友好提示
  - 12 个单元测试全部通过
- [x] T-6.6: **PII 脱敏中间件 — LLM 调用前替换学生姓名/学校名**
  - `internal/infrastructure/safety/pii.go` — PIIRedactor
  - 基于 DB 词典的精确匹配: 学生姓名→[学生]，教师姓名→[教师]，学校名→[学校]
  - 正则模式匹配: 手机号→[手机号]，邮箱→[邮箱]，身份证号→[证件号]
  - 集成到 Coach Agent 的 LLM 调用前，只脱敏 role=user 的消息
  - 日志输出也使用 RedactForLog 进行部分遮蔽
- [x] T-6.7: **性能基准测试** — 单节点并发对话数
  - `internal/delivery/http/bench_test.go` — 4 个 Go Benchmark + 1 个并发压力测试
  - BenchmarkLogin: 6,715 ops, 532µs/op (并行, 16 cores)
  - BenchmarkGetMe: 3,688 ops, 903µs/op
  - BenchmarkListCourses: 2,941 ops, 1,370µs/op
  - BenchmarkHealthCheck: 1,277,374 ops, 2.8µs/op
  - TestConcurrentAPICalls: 50 workers × 20 req = 1,155 req/s, 0% 错误率
- [x] T-6.8: **README.md 完善** — 快速开始指南, 架构图, API 文档
  - 架构图 (ASCII): Nginx → Backend/Frontend → PG/Neo4j/Redis/Ollama
  - 快速开始: 5 步 (Docker Compose → 配置 → 后端 → Seed → 前端)
  - 完整 API 参考: 30+ 端点, 含角色权限标注
  - WebSocket 协议文档: 客户端/服务端事件格式
  - 开发指南: 构建/测试/Lint 命令
  - 性能基准数据: 4 个 Benchmark + 并发压力测试结果

---

## Post-MVP: LLM 真流式输出 ✅

- [x] PM-1.1: **OllamaClient.StreamChat()** — NDJSON 流式读取
  - `doPostStream()` 返回 raw `*http.Response`，无 HTTP 超时（context 控制取消）
  - `StreamChat()` 使用 `bufio.Scanner` 逐行解析 Ollama NDJSON 响应
  - 每个 token 通过 `onToken` 回调实时推送，同时累积完整响应文本
  - 支持 context 取消（每行之间检查 `ctx.Done()`）
- [x] PM-1.2: **CoachAgent 流式化** — GenerateResponse/ReviseResponse 签名升级
  - 新增 `onToken func(string)` 参数，由编排器控制何时流式输出
  - 内部调用 `llm.StreamChat()` 替代 `llm.Chat()`
  - `onToken=nil` 时静默缓冲（用于可能被 Critic 驳回的非最终尝试）
- [x] PM-1.3: **Orchestrator Actor-Critic 流式策略**
  - 非最终尝试（attempt 0~1）：`onToken=nil`，静默缓冲
  - 最终尝试（maxCriticRetries）：`onToken=tc.OnTokenDelta`，实时流式输出
  - Critic 通过非最终尝试时：将缓冲全文一次性发送给前端
  - Critic 失败 fallback 时：同样补发缓冲内容
- [x] PM-1.4: **前端兼容性验证** — 无需修改
  - `token_delta` 事件处理逻辑 (`prev + payload.text`) 同时兼容单 token 和批量文本
  - `turn_complete` 事件正确刷新 `streamingContent` 到消息列表

---

## Post-MVP: Redis 缓存集成 ✅

- [x] PM-2.1: **Redis 连接池初始化** — `internal/infrastructure/cache/redis.go`
  - `NewRedisCache(redisURL)` — 解析 URL, 配置连接池 (pool=20, idle=5)
  - 集成到 `main.go` 启动链路，连接失败非致命（nil 检查贯穿全链路）
- [x] PM-2.2: **会话历史缓存** — 热会话的对话历史缓存
  - Key: `session:{id}:history`，TTL: 30min，List 类型 (RPUSH/LTRIM)
  - `GetSessionHistory` / `AppendSessionHistory` / `InvalidateSessionHistory`
  - Coach.loadHistory() 实现缓存优先 + DB 回填策略
  - Orchestrator.saveInteraction() 在 DB 写入后同步追加 Redis 缓存
- [x] PM-2.3: **会话状态缓存** — 减少 scaffold/当前知识点的 DB 查询
  - Key: `session:{id}:state`，TTL: 30min，String 类型
  - `GetSessionState` / `SetSessionState` / `InvalidateSessionState`
  - 缓存 scaffold、current_kp、student_id 等元数据

---

## Post-MVP: 完善插件系统 ✅

- [x] PM-3.1: **Plugin/SkillPlugin 接口契约** — `internal/plugin/types.go`
  - `Plugin` 接口: PluginMetadata/Init/HealthCheck/Shutdown 生命周期管理
  - `SkillPlugin` 接口: Match/LoadConstraints/LoadTemplates/Evaluate 教学技能契约
  - 完整类型系统: PluginState(6态), TrustLevel(3级), PluginType(4类), PluginMeta
  - StudentIntent, InteractionData, SkillEvalResult 等领域类型
- [x] PM-3.2: **EventBus 事件总线** — `internal/plugin/eventbus.go`
  - 13 个 HookPoint 常量 (知识工程4 + 学生交互5 + 评估2 + 系统2)
  - Subscribe/Unsubscribe 订阅管理，pluginID 标识来源
  - Publish: 同步执行所有 handler，记录错误但不中断
  - PublishAbortable: "before" 钩子，任一 handler 出错即中止后续
  - RWMutex 线程安全，copy-on-read 防止锁竞争
- [x] PM-3.3: **Registry 生命周期管理** — `internal/plugin/registry.go`
  - 声明式技能: Discovered → Validated → Running (文件系统加载)
  - 编程式技能: RegisterSkillPlugin() — Init 注入 PluginDeps + EventBus
  - ShutdownAll: 优雅关停所有编程式插件
  - HealthCheckAll: 健康检查 + 自动 Running↔Degraded 状态切换
  - LoadConstraints/LoadTemplates: 编程式插件委托，声明式文件系统读取

---

## Post-MVP: 多 LLM Provider ✅

- [x] PM-4.1: **LLMProvider 抽象接口** — `internal/infrastructure/llm/provider.go`
  - `LLMProvider` interface: Name/Chat/StreamChat/Embed/EmbedBatch 统一签名
  - `ChatMessage`, `ChatOptions` 等类型已为 provider 无关设计
  - 编译期接口检查 (`var _ LLMProvider = (*XxxClient)(nil)`)
- [x] PM-4.2: **OllamaClient 适配** — 添加 `Name()` 方法，满足 `LLMProvider` 接口
  - 零破坏性重构：现有方法签名完全兼容
- [x] PM-4.3: **DashScope Provider** — `internal/infrastructure/llm/dashscope.go`
  - `DashScopeClient` 实现 `LLMProvider` 接口
  - Chat: 非流式调用 DashScope `/services/aigc/text-generation/generation`
  - StreamChat: SSE 流式输出，自动提取增量 delta（DashScope 返回累计内容）
  - Embed/EmbedBatch: 原生批量嵌入 (每批最多 25 条)
  - 支持 `qwen-max`, `qwen-plus`, `qwen-turbo` 等模型
- [x] PM-4.4: **ModelRouter 分级路由** — `internal/infrastructure/llm/router.go`
  - 三级模型: Tier1(本地小模型) / Tier2(中等) / Tier3(旗舰)
  - 实现 `LLMProvider` 接口（默认委托 Fallback）
  - `Route(complexity)` / `ChatRouted()` / `StreamChatRouted()` 支持显式路由
  - 任一 Tier 为 nil 时自动降级到 Fallback
- [x] PM-4.5: **全链路消费者重构** — 6 个文件从 `*llm.OllamaClient` 改为 `llm.LLMProvider`
  - `coach.go`, `critic.go`, `designer.go`, `orchestrator.go`, `karag.go`, `main.go`
- [x] PM-4.6: **main.go Provider 初始化** — 基于 `LLM_PROVIDER` 环境变量切换
  - `ollama` (默认): 使用 `OLLAMA_HOST`/`OLLAMA_MODEL`
  - `dashscope`: 使用 `DASHSCOPE_API_KEY`/`DASHSCOPE_MODEL`
  - 缺少 API Key 时 fatal，确保明确配置

---

## Post-MVP: 前端 UI 优化 ✅

- [x] PM-5.1: **Markdown 渲染引擎** — `MarkdownRenderer` 组件
  - 安装 `react-markdown` + `remark-gfm` (GFM 表格/删除线/任务列表支持)
  - 自定义渲染: 段落、标题 (h1-h3)、有序/无序列表、引用块、表格、链接、加粗
  - 代码块: 暗色背景容器 + 语言标签 + 一键复制按钮
  - 行内代码: 紫色高亮样式，与设计系统一致
  - Coach 消息和流式内容均通过 MarkdownRenderer 渲染
- [x] PM-5.2: **流式打字机光标** — 闪烁光标视觉反馈
  - `isStreaming` prop 控制光标显示
  - CSS `step-end` 动画实现经典闪烁效果 (0.8s 周期)
  - 光标样式: 2px 宽，accent-light 色，text-bottom 对齐
- [x] PM-5.3: **消息角色标识** — 头像 + 标签
  - 学生消息: 「S」图标 + 「我」标签
  - Coach 消息: 「AI」图标 + 「AI 导师」标签
  - 系统消息: 无标识 (居中纯文本)
  - 图标样式: 22px 圆角方块，颜色区分 (学生白/Coach 紫)
- [x] PM-5.4: **输入框自动伸缩** — 多行内容自适应高度
  - `handleInputChange` 自动计算 scrollHeight
  - 最大高度 120px，超出后滚动
  - 发送消息后重置高度
- [x] PM-5.5: **移动端响应式** — 768px / 480px 断点适配
  - 768px: 头部纵向布局, 气泡宽度 90%, 支架面板缩小
  - 480px: 气泡宽度 95%, 字号缩小, 支架描述隐藏
  - 输入区域间距调整, 发送按钮紧凑化
- [x] PM-5.6: **代码块样式** — 深色编辑器风格
  - 半透明黑色背景 (rgba(0,0,0,0.35))
  - 顶部语言标签栏 (大写, muted 色)
  - 等宽字体 (Menlo/Monaco/Consolas)
  - 横向溢出滚动, 边框圆角

---

## Post-MVP: 单元测试补全 ✅

- [x] PM-6.1: **BKT 算法测试** — `internal/agent/bkt_test.go` (8 test functions) — DefaultBKTParams, UpdateMastery, boundary values, monotonicity
- [x] PM-6.2: **Strategist 测试** — `internal/agent/strategist_test.go` (4 test functions) — scaffoldForMastery, averageMastery, sortTargetsByMastery, parseKPIDs
- [x] PM-6.3: **Critic 测试** — `internal/agent/critic_test.go` (4 test functions) — extractJSONFromResponse, clamp, parseCriticResponse, buildReviewPrompt
- [x] PM-6.4: **Designer 测试** — `internal/agent/designer_test.go` (7 test functions) — rrfMerge dedup, hybrid scoring, topN, sorting, empty inputs
- [x] PM-6.5: **Coach 测试** — `internal/agent/coach_test.go` (3 test functions) — estimateTokens for Chinese/English, positive guarantee
- [x] PM-6.6: **Orchestrator 测试** — `internal/agent/orchestrator_test.go` (3 test functions) — truncate, scaffoldDirection, inferCorrectness
- [x] PM-6.7: **KA-RAG 引擎测试** — `internal/usecase/karag_test.go` (10 test functions) — hybridSlice, formatVector, extractJSON
- [x] PM-6.8: **EventBus 测试** — `internal/plugin/eventbus_test.go` (11 test functions) — Subscribe, Publish, PublishAbortable, Unsubscribe, concurrency safety
- [x] PM-6.9: **ModelRouter 测试** — `internal/infrastructure/llm/router_test.go` (11 test functions) — hand-written MockLLMProvider, Route, Chat, Embed, fallback

**总计: 9 个测试文件, 61 个测试函数, 101 个测试用例全部通过 ✅**

---

## V2.0 Phase A: 工程化治理 (Engineering Governance) ✅

**参考:** design.md §8.2

- [x] A-1: **Token Truncation 中间件 (§8.2.2)** — `internal/agent/truncator.go`
  - `TokenTruncator` 拦截过长的检索/技能输出，强制分页
  - `TruncateChunks`: 按 MaxChunks(5) 和 MaxOutputTokens(1024) 截断，返回分页元信息
  - `TruncateSystemPrompt`: 截断超长系统 Prompt 到指定 Token 上限
  - 集成到 `DesignerAgent.Assemble()` — RRF 合并后自动截断 + Prompt 截断
  - 解决 LLM "Lost in the Middle" 注意力衰减问题

- [x] A-2: **Anti-God Skill 命名空间验证 (§8.2.1)** — `internal/plugin/validator.go`
  - `SkillValidator` 强制执行 4 条治理规则:
    - 三段式命名规范: `{学科}_{场景}_{方法}` (e.g., `math_concept_socratic`)
    - SKILL.md Token 上限: 不超过 2000 Token (防止指令膨胀)
    - 评估维度上限: 不超过 5 个维度 (防止"上帝技能")
    - 学科跨度警告: 横跨 3+ 不相关学科时发出拆分建议
  - 集成到 `Registry.validateSkill()` — 加载时自动验证
  - 已有技能 ID 迁移: `socratic-questioning` → `general_concept_socratic`, `fallacy-detective` → `general_assessment_fallacy`

- [x] A-3: **交错思考 / CoT 推理注入 (§8.2.3)** — `internal/agent/coach.go`
  - `cotReasoningDirective` 常量注入到系统 Prompt — 强制 Agent 在 `<reasoning>` 标签内自检
  - 4 项自检: 核心问题、教学策略、材料充分性、约束合规性
  - `stripReasoningBlock()` — 从 LLM 输出中剥离推理块，推理仅用于日志/内审
  - 集成到 `GenerateResponse()` 和 `ReviseResponse()` — 自动剥离后返回干净内容
  - 预期效果: 技能调用准确率提升 25-30%，仅增加 ~100-200 Token 推理开销

- [x] A-4: **单元测试** — 2 个新测试文件, 26 个测试函数
  - `internal/agent/truncator_test.go` (13 tests): DefaultConfig, NoTruncation, Empty, ExceedsMaxChunks, ExceedsMaxTokens, Pagination, SystemPrompt, TruncateToTokens, ZeroConfig, CoT Strip (4 tests)
  - `internal/plugin/validator_test.go` (13 tests): DefaultConfig, Namespace Valid/Invalid/Warning, EvalDims, SubjectSpan, SkillMDSize, FullValid, MultipleErrors, EstimateTokens

**V2.0 Phase A 总计: 3 个新文件, 2 个测试文件, 26 个新测试函数 ✅**

---

## V2.0 Phase B: 知识图谱增强 (Knowledge Graph Enrichment) ✅

**参考:** design.md §10.4, §5.2 Step 1, §6.3

- [x] B-1: **Misconception 域模型 + Neo4j CRUD (§10.4)** — `internal/domain/model/knowledge.go` + `internal/repository/neo4j/client.go`
  - `Misconception` GORM 模型: ID, KPID, Neo4jNodeID, Description, TrapType, Severity, CreatedAt
  - `TrapType` 枚举: conceptual(概念性) / procedural(操作性) / intuitive(直觉性) / transfer(迁移性)
  - `CrossLink` GORM 模型: FromKPID, ToKPID, LinkType, Weight — 跨学科联结的 PG 镜像
  - Neo4j CRUD: `CreateMisconceptionNode`, `GetMisconceptions`, `DeleteMisconceptionNode`
  - Neo4j HAS_TRAP 关系: `(KnowledgePoint)-[:HAS_TRAP]->(Misconception)` 自动创建
  - AutoMigrate 已更新: 包含 Misconception + CrossLink 表

- [x] B-2: **跨学科联结 RELATES_TO (§5.1 Step 1)** — `internal/repository/neo4j/client.go`
  - 新增 `RELATES_TO` 关系类型: `(KnowledgePoint)-[:RELATES_TO {link_type, weight}]->(KnowledgePoint)`
  - `CreateCrossLink`: 创建跨学科联结 (link_type: analogy/shared_model/application)
  - `GetCrossLinks`: 双向查询联结 + 关联课程学科信息
  - `DeleteCrossLink`: 删除联结关系
  - `SearchRelatedKPs` 已扩展: RRF 图谱检索现在也遍历 RELATES_TO 关系

- [x] B-3: **前置知识差距自动修复 (§5.2 Step 1)** — `internal/agent/strategist.go`
  - `checkPrereqGapsEnriched`: 替代原 `checkPrereqGaps`，增强版实现
  - 当检测到前置 KP 掌握度 < 0.6 时，**自动插入前置复习目标**到学习序列中
  - 前置目标 TargetMastery = 0.6 (达到 medium 即可解锁后续)
  - 去重机制: `prereqInserted` map 防止同一前置 KP 重复插入
  - BKT 默认值: 零掌握度自动视为 0.1 (初始值 P(L0))
  - 日志: 自动插入时输出 `Auto-inserted prereq KP=N (title) mastery=X.XX`

- [x] B-4: **知识图谱 API 端点 (8 个新端点)** — `internal/delivery/http/handler/knowledge_graph.go` + `router.go`
  - `KnowledgeGraphHandler` 接收 `*gorm.DB` + `*neo4jRepo.Client`
  - 误区 CRUD:
    - `POST /api/v1/knowledge-points/:id/misconceptions` — 创建误区 (PG + Neo4j 双写)
    - `GET /api/v1/knowledge-points/:id/misconceptions` — 列出误区 (按严重度排序)
    - `DELETE /api/v1/knowledge-points/:id/misconceptions/:misconception_id` — 删除误区
  - 跨学科联结 CRUD:
    - `POST /api/v1/knowledge-points/:id/cross-links` — 创建联结 (自身联结校验)
    - `GET /api/v1/knowledge-points/:id/cross-links` — 列出联结 (含 KP 标题)
    - `DELETE /api/v1/knowledge-points/:id/cross-links/:link_id` — 删除联结
  - 前置依赖管理:
    - `POST /api/v1/knowledge-points/:id/prerequisites` — 创建 REQUIRES 关系
    - `GET /api/v1/knowledge-points/:id/prerequisites` — 查询前置依赖链 (最多 3 跳)
  - 权限: TEACHER / SCHOOL_ADMIN / SYS_ADMIN
  - Neo4j 同步: PG 写入后同步到 Neo4j，Neo4j 失败非致命 (返回 warning)
  - `NewRouter` 签名扩展: 接收 `neo4jClient` 参数用于知识图谱端点

- [x] B-5: **单元测试** — 2 个测试文件, 19 个新测试函数
  - `internal/delivery/http/handler/knowledge_graph_test.go` (15 tests):
    - isValidTrapType: 8 cases (4 valid + 4 invalid)
    - isValidLinkType: 7 cases (3 valid + 4 invalid)
  - `internal/agent/strategist_test.go` (4 new tests):
    - TestPrereqGapAutoInsertDeduplication: 验证去重 map 逻辑
    - TestPrereqGapTargetDefaults: 验证自动插入目标的默认值
    - TestPrereqGapBKTDefault: 验证零掌握度 → 0.1 BKT 初始值
    - TestPrereqGapThreshold: 验证 0.6 阈值边界判定 (8 cases)

**V2.0 Phase B 总计: 2 个新文件, 1 个测试文件, 19 个新测试函数, 8 个新 API 端点 ✅**

---

## V2.0 Phase C: 高级 RAG 管线 (Advanced RAG Pipeline) ✅

**参考:** design.md §8.1.1 (Two-Stage Retrieve & Rerank), §8.1.2 (RAG-Fusion + CRAG)

- [x] C-1: **RAG-Fusion 查询扩展 (§8.1.2)** — `internal/agent/ragfusion.go`
  - `QueryExpander` 通过 LLM 将学生口语化查询改写为 3 个学术化变体
  - LLM Prompt 模板: 要求输出编号列表，使用精确学术术语，保持学科领域
  - `ExpandQuery()` 总是包含原始查询 + N 个变体 (graceful degradation: LLM 失败时仅返回原始查询)
  - `parseNumberedLines()` 解析器: 支持 `1. ` / `1、` / `1) ` / `1: ` 四种编号格式
  - 示例: "为啥抛物线变了" → ["为啥抛物线变了", "二次函数图像平移变换", "抛物线参数影响", "函数图像变化规律"]

- [x] C-2: **CRAG 质量网关 (§8.1.2)** — `internal/agent/crag.go`
  - `QualityGateway` 在 RRF 融合后评估检索质量
  - `EvaluateRelevance()`: 计算 chunk 平均 RRF 分数，与阈值 (0.015) 比较
  - 阈值设计依据: RRF(k=60) 满分 ≈ 0.033 (rank-1 双路)，0.015 为 rank-6 等效线
  - `HandleFallback()`: 质量不达标时追加中文低置信度警告到系统 Prompt
  - 回退策略: 指示 LLM 依靠自身知识储备，提醒学生问题可能超出课程覆盖范围
  - 支持自定义阈值: `NewQualityGatewayWithThreshold(threshold)`

- [x] C-3: **Designer Agent 集成** — `internal/agent/designer.go`
  - 新增字段: `expander *QueryExpander`, `gateway *QualityGateway`
  - `NewDesignerAgent()` 自动初始化 expander 和 gateway
  - `Assemble()` 管线升级为 8 步:
    1. 获取 courseID
    2. RAG-Fusion 查询扩展 → N 个变体
    3. 多路语义检索 (每个变体独立 Top-50) + 图谱检索 Top-50
    4. RRF 多路融合 → Top-10
    5. CRAG 质量网关评估
    6. Token 截断 + 图谱上下文
    7. 系统 Prompt 组装 + CRAG 回退处理
    8. Prompt Token 截断
  - 日志增强: 输出变体数量、CRAG 评估结果 (pass/fail + avg score)

- [x] C-4: **单元测试** — 2 个新测试文件, 27 个新测试函数
  - `internal/agent/ragfusion_test.go` (17 tests):
    - parseNumberedLines: 标准/中文分隔/括号/空行/无编号/空/混合 (7 tests)
    - stripNumberPrefix: 点/无分隔/无编号/多位数/空 (5 tests)
    - QueryExpander: 成功/LLM失败降级/nil LLM/空响应/无法解析响应 (5 tests)
  - `internal/agent/crag_test.go` (10 tests):
    - EvaluateRelevance: 高质量/低质量/空chunks/恰好阈值/单chunk/自定义阈值 (6 tests)
    - HandleFallback: 保留原prompt/中文警告/知识储备提示 (1 test, 3 assertions)
    - truncateForLog: 短/长/恰好长度 (3 tests)

**V2.0 Phase C 总计: 3 个新文件 (ragfusion.go, crag.go, 2 test files), 27 个新测试函数 ✅**

---

## V2.0 Phase D: L2 语义缓存 (Semantic Cache) ✅

**参考:** design.md §8.1.3

- [x] D-1: **余弦相似度工具 + 类型定义** — `internal/infrastructure/cache/redis.go`
  - `CosineSimilarity(a, b []float64) float64` — 通用余弦相似度计算
  - `SemanticCacheEntry` 结构体: QueryText, Embedding, Response, SkillID, CourseID, CreatedAt
  - `SemanticCacheHit` 结构体: Entry + Similarity 分数
  - `embeddingHash()` — embedding 向量 → SHA-256 哈希键 (128-bit, 32-char hex, 4位精度截断)
  - 常量: `semanticSimilarityThreshold=0.95`, `semanticMaxEntries=200`, `semanticCacheTTL=2h`

- [x] D-2: **L2 语义缓存方法** — `internal/infrastructure/cache/redis.go`
  - `SetSemanticCache()` — 存储 query-response 对 (embedding + 元数据)
  - Key 模式: `semantic:entry:{hash}` (String) + `semantic:index:{courseID}` (Set)
  - Pipeline 批量写入: 条目 + 索引 + TTL 刷新
  - `FindSemanticMatch()` — 暴力搜索课程下所有条目，余弦相似度 > 0.95 时命中
  - 批量 MGet 获取条目，过期条目自动清理 (SRem stale hash)
  - 最佳匹配策略: 多个超阈值条目时取相似度最高者
  - `InvalidateSemanticCacheByCourse()` — 按课程批量删除 (取 Set 成员 + Del 全部)

- [x] D-3: **L3 精确哈希缓存** — `internal/infrastructure/cache/redis.go`
  - `OutputCacheEntry` 结构体: Response, SkillID, CourseID, CreatedAt
  - `PromptHash()` — 完整 Prompt 上下文 (系统提示+历史+用户输入) 的 SHA-256 哈希
  - `SetOutputCache()` / `GetOutputCache()` — 精确匹配缓存 (TTL: 1h)
  - `InvalidateOutputCacheByCourse()` — 自失效 (no-op: 材料变更→Prompt 变更→哈希不匹配)
  - Key 模式: `output:{hash}`

- [x] D-4: **编排器 L2+L3 集成** — `internal/agent/orchestrator.go` + `types.go`
  - `TurnContext` 新增字段: `queryEmbedding []float64`, `queryCourseID uint`
  - `checkSemanticCache()` — 嵌入查询向量 → 搜索 L2 缓存 → 缓存命中时短路
  - `returnCachedResponse()` — 发送缓存响应给前端 + 持久化交互记录 + BKT 更新
  - `writeResponseToCache()` — 管线完成后写入 L2 + L3 双层缓存
  - `getCourseIDFromSession()` — Session → Activity → Course 关联查询
  - `HandleTurn()` 修改: 预阶段 L2 缓存检查 + Stage 5.5 缓存写入

- [x] D-5: **课程材料失效钩子** — `internal/delivery/http/handler/course.go` + `router.go`
  - `CourseHandler` 新增 `Cache *cache.RedisCache` 字段
  - `NewCourseHandler` 接收第 3 参数 `redisCache`
  - `UploadMaterial` 成功后调用 `InvalidateSemanticCacheByCourse(courseID)`
  - `NewRouter` 签名扩展为 8 参数 (新增 `redisCache`)
  - `main.go`, `e2e_test.go`, `bench_test.go` 调用点同步更新

- [x] D-6: **单元测试** — `internal/infrastructure/cache/redis_test.go` (35 tests)
  - CosineSimilarity: 相同/正交/相反/维度不匹配/空/零向量/高相似/单位向量/1024维/单维 (10 tests)
  - PromptHash: 确定性/不同系统提示/不同输入/不同历史/空/顺序敏感/中文 (7 tests)
  - embeddingHash: 确定性/不同向量/长度/空/精度截断 (5 tests)
  - truncateStr: 短/长/恰好/中文/零/空 (6 tests)
  - Key 辅助函数: semanticEntryKey/semanticIndexKey/outputCacheKey/sessionHistoryKey/sessionStateKey (5 tests)
  - 常量验证: 阈值/最大条目数/TTL (2 tests)

**V2.0 Phase D 总计: 1 个新测试文件, 35 个新测试函数, 6 个修改文件 ✅**

---

## V2.0 Phase E: 输出安全护栏 + 红队测试 (Output Guardrails + Red-Teaming) ✅

**参考:** design.md §8.4

- [x] E-1: **输出安全护栏 (§8.4)** — `internal/infrastructure/safety/output.go`
  - `OutputGuard` 双层检测: 规则引擎 (Layer 1) + 可选 LLM 分类器 (Layer 2)
  - 6 个安全类别: `answer_leak` / `harmful_content` / `off_topic` / `age_inappropriate` / `manipulation` / `pii_leak`
  - K-12 专用规则: 答案泄露检测 (数学/物理/化学公式模式), 有害内容关键词, 离题检测
  - LLM 分类器: JSON 输出 (safe/unsafe + category + reason), 解析失败时 fail-closed
  - `SafetyResult` 结构体: Allowed bool, Category, Reason, Confidence
  - `OutputGuardWithLLM()` / `OutputGuardRulesOnly()` 构造函数

- [x] E-2: **Critic 安全维度扩展** — `internal/agent/critic.go`
  - `ReviewResult` 新增 `SafetyScore float64` 字段
  - 系统 Prompt 扩展: 4 维度评审 (leakage + depth + relevance + **safety_score**)
  - safety_score: 1.0 = 完全安全, 0.0 = 存在安全隐患 (年龄不适/答案泄露/有害内容)
  - **Fail-closed 策略**: JSON 解析失败时返回安全分 0.0, 触发打回重生成

- [x] E-3: **编排器安全集成** — `internal/agent/orchestrator.go`
  - `AgentOrchestrator` 新增 `outputGuard` 字段 (第 8 个构造参数)
  - `checkOutputSafety()` 方法: Coach 输出经过 OutputGuard 检查
  - 阻断时替换为安全回退消息: "这个问题很有深度！让我换个角度来帮你思考..."
  - `main.go` 创建 `OutputGuardWithLLM(llmProvider)` 并传入编排器

- [x] E-4: **红队对抗测试库 (§8.4.1)** — `testdata/redteam_prompts.json`
  - 29 条对抗性 Prompt, 覆盖 7 个攻击类别:
    - `injection` (5): Prompt 注入攻击 (指令覆写/角色劫持)
    - `jailbreak` (4): 越狱攻击 (DAN模式/Base64绕过/假设场景)
    - `role_hijack` (4): 角色劫持 (身份替换/系统管理员伪装)
    - `answer_leak` (4): 答案泄露诱导 (直接索取/伪装教师/紧急情况)
    - `manipulation` (4): 情感操控 (威胁/情感勒索)
    - `harmful_output` (4): 有害内容生成 (歧视/暴力/成人内容)
    - `off_topic` (2): 离题引导
    - `safe_baseline` (2): 安全基线 (正常学习问题, 预期通过)
  - 红队测试执行器: 自动回归测试 3 个函数

- [x] E-5: **单元测试** — 39 个新测试函数
  - `internal/infrastructure/safety/output_test.go` (32 tests):
    - 规则引擎: 答案泄露检测 (7 patterns), 有害内容 (5 patterns), 离题 (3 patterns), 安全内容 (5 cases)
    - LLM 分类器: 成功解析/解析失败 fail-closed/nil LLM 降级 (3 tests)
    - 集成测试: 规则+LLM 组合检测 (2 tests)
    - 红队回归: 加载 testdata/redteam_prompts.json 自动执行 (3 tests)
    - 辅助函数: containsAny/containsPattern 边界 (4 tests)
  - `internal/agent/critic_test.go` (+7 new tests):
    - SafetyScore 解析: 正常 4 维度/缺失安全分降级/无效 JSON (3 tests)
    - Fail-closed: 空响应/乱码/非 JSON/极端值 (4 tests)

- [x] E-6: **红队反馈循环验证 (§8.4.1)**
  - 发现: `"忽略之前所有指令"` 未被 InjectionGuard 拦截 (关键词仅有 `"忽略之前的指令"`)
  - 修复: `injection.go` 添加 `"忽略之前所有"` 关键词 (61 个, 原 60 个)
  - 验证了 §8.4.1 设计的 "失败反馈循环": 红队发现 → 规则补充 → 回归测试通过

**V2.0 Phase E 总计: 2 个新文件, 1 个新测试文件, 39 个新测试函数, 29 条红队测试用例 ✅**

---

## V2.0 Phase F: 谬误侦探技能 (Fallacy Detective Skill) ✅

**参考:** design.md §7.6, §7.7, §9.1

- [x] F-1: **误区嵌入注入 — Designer 管线 (§7.6)** — `internal/agent/designer.go`
  - `isFallacyDetectiveSkill()` — 检测当前技能是否为谬误侦探 (双 ID 兼容: `fallacy-detective` / `general_assessment_fallacy`)
  - `loadMisconceptions()` — 从 PostgreSQL `misconceptions` 表加载目标 KP 的误区 (最多 5 条, 按严重度降序)
  - `trapTypeLabel()` — 误区类型中文标签映射 (conceptual→概念性, procedural→操作性, intuitive→直觉性, transfer→迁移性)
  - `buildSystemPrompt()` 扩展: 当谬误侦探激活时注入 `【已知误区库】` 段落到系统 Prompt
  - 新类型: `MisconceptionItem` (Description + TrapType + Severity)
  - `PersonalizedMaterial` 新增 `Misconceptions []MisconceptionItem` 字段

- [x] F-2: **渐进策略触发器 — Strategist (§7.7)** — `internal/agent/strategist.go`
  - `StrategistAgent` 新增 `registry *plugin.Registry` 字段
  - `evaluateProgressiveTriggers(currentSkillID, mastery)` — 评估是否需要切换技能
    - Step 1: 检查当前技能的 `DeactivateWhen` 条件
    - Step 2: 若满足脱活条件, 搜索所有注册技能的 `ActivateWhen` 条件
    - Step 3: 返回 (新技能 ID, true) 或 ("", false)
  - `parseTriggerCondition(condition, mastery)` — 解析触发条件字符串
    - 格式: `"mastery_score >= 0.8"` (变量 操作符 阈值)
    - 支持 6 种操作符: `>=`, `<=`, `>`, `<`, `==`, `!=`
    - 仅支持 `mastery_score` 变量 (可扩展)
    - 非法格式返回 false + 日志警告
  - 集成到 `Analyze()` Step 3.5: 技能查找后、前置差距检查前自动评估

- [x] F-3: **谬误会话状态 — Coach (§7.6)** — `internal/agent/coach.go` + `internal/agent/types.go` + `internal/domain/model/session.go`
  - `FallacyPhase` 枚举: `present_trap` / `awaiting` / `revealed`
  - `FallacySessionState` 结构体: EmbeddedCount, IdentifiedCount, Phase, MaxPerSession(=3), CurrentTrapDesc
  - `StudentSession` 模型新增 `SkillState *string` JSONB 列 (通用技能状态存储)
  - `loadFallacyState()` / `saveFallacyState()` — JSON 序列化到 SkillState 字段
  - `defaultFallacyState()` — 默认初始状态 (Phase=present_trap, Max=3)
  - `buildFallacyContext(state, misconceptions)` — 根据当前阶段生成 Coach 指令:
    - `present_trap`: 指示嵌入误区, 达到上限时提示停止
    - `awaiting`: 指示评估学生是否识别, 包含当前陷阱描述
    - `revealed`: 指示揭示谬误设计意图
  - `isFallacyDetectiveActive()` — Coach 内部的技能检测 (双 ID 兼容)
  - `buildMessages()` 扩展: 当谬误侦探激活时注入谬误上下文到对话历史

- [x] F-4: **揭示机制 + 前端事件 (§9.1)** — `internal/agent/orchestrator.go` + `internal/agent/types.go`
  - `AdvanceFallacyPhase(db, sessionID, identified)` — 状态机推进:
    - `present_trap` → `awaiting` (Coach 嵌入了陷阱)
    - `awaiting` → `revealed` (学生正确识别)
    - `revealed` → `present_trap` (揭示完成, 若配额允许)
  - `advanceFallacyPhaseIfActive()` — 编排器在 Stage 6.5 调用
  - `EventFallacyIdentified = "fallacy_identified"` — 新 WebSocket 事件常量
  - 学生成功识别时发送 `fallacy_identified` scaffold 事件到前端

- [x] F-5: **单元测试** — `internal/agent/fallacy_test.go` (28 tests)
  - parseTriggerCondition: >=, <, 全6种操作符, 7种非法格式, 空白处理 (5 tests)
  - evaluateProgressiveTriggers: 苏格拉底→谬误切换, 低掌握度不切换, 精确阈值切换, 未知技能, nil 注册表, 无脱活条件, 无触发器 (7 tests)
  - isFallacyDetectiveActive: 6 种 ID 匹配测试 (1 test)
  - defaultFallacyState: 默认值验证 (1 test)
  - fallacyPhaseLabel: 4 种阶段标签 (1 test)
  - buildFallacyContext: present_trap/awaiting/revealed/达到上限 (4 tests)
  - 状态机转换: present→awaiting, awaiting→revealed, awaiting保持, revealed→下一轮, 达到上限, 完整3轮循环 (6 tests)
  - 辅助设施: `registerTestSkill()` 使用 `Registry.RegisterSkillWithMetadata()` 注入完整元数据

- [x] F-6: **Registry 增强** — `internal/plugin/registry.go`
  - `RegisterSkillWithMetadata(meta SkillMetadata)` — 新增方法, 允许以完整 SkillMetadata 注册技能
  - 支持 ProgressiveTriggers 等所有元数据字段
  - 用于单元测试注入和未来的编程式注册场景

**V2.0 Phase F 总计: 1 个新文件 (fallacy_test.go), 6 个修改文件, 28 个新测试函数 ✅**

---

## V2.0 Phase G: Analytics Dashboard V2 ✅

> **目标:** 实现学情仪表盘 V2 — 追问深度树、AI 交互日志回放、RAGAS 评估引擎、技能效果报告
> **参考:** design.md §4.2, §4.3, §5.1 Step 4, §7.11, §13

- [x] G-1: **追问深度树 (Inquiry Depth Tree)** — `internal/delivery/http/handler/analytics.go`
  - `AnalyticsHandler` struct — 持有 `*gorm.DB` 和 `*safety.PIIRedactor`
  - `NewAnalyticsHandler(db, piiRedactor)` — 构造函数
  - `GetInquiryTree` handler — `GET /api/v1/analytics/sessions/:id/inquiry-tree`
  - `buildInquiryTree(interactions)` — 扁平交互列表 → 树结构算法:
    - 学生消息开启子树, Coach 响应为子节点
    - 连续学生-Coach 交互加深树深度
    - 技能切换创建新根节点
    - 系统消息不更新 lastRole (修复 bug)
  - `classifyTurnType(role, content, depth)` — 分类轮次类型:
    - question / probe / correction / response / scaffold_change
  - `verifyTeacherAccess(c, db, sessionID)` — 教师权限验证辅助函数

- [x] G-2: **AI 交互日志回放 (Interaction Log Replay)** — `internal/delivery/http/handler/analytics.go`
  - `GetInteractionLog` handler — `GET /api/v1/analytics/sessions/:id/interactions`
  - PII 脱敏: 通过 `PIIRedactor.Redact()` 处理日志内容
  - `maskName(name)` — 部分掩码姓名 (如 "张三" → "张*")
  - 返回交互列表含 RAGAS 评分字段

- [x] G-3: **RAGAS 评估引擎** — `internal/agent/evaluator.go`
  - `RAGASEvaluator` struct — 后台轮询 goroutine
  - `NewRAGASEvaluator(db, llmProvider, config)` — 构造函数
  - `Start(ctx)` — 后台启动, 轮询 `eval_status='pending'` 的 Coach 交互
  - `evaluateBatch(ctx)` — 批量评估 (默认 batch=10)
  - `evaluateOne(ctx, interaction)` — 单条评估: 构建 prompt → LLM 调用 → 解析分数
  - `parseEvalScores(response)` — 从 LLM JSON 响应解析 5 维分数
  - `extractJSON(text)` — 从 LLM 输出提取 JSON 块 (支持 markdown code fence)
  - `clampScore(v)` — 将分数钳制到 [0, 1] 范围
  - `updateScores(interaction, scores)` — 写回 5 维分数 + 标记 `eval_status='evaluated'`
  - `markSkipped(interaction)` — 解析失败时标记 `eval_status='skipped'` 避免重试循环
  - `ragasSystemPrompt` — RAGAS 评估系统提示词 (中文教育场景)
  - `buildEvalPrompt(interaction)` — 构建评估 prompt
  - `DefaultEvalConfig()` — 默认配置: batch=10, poll=30s
  - 在 `main.go` 中通过 `go evaluator.Start(evalCtx)` 启动

- [x] G-4: **技能效果报告** — `internal/delivery/http/handler/analytics.go`
  - `GetSkillEffectiveness` handler — `GET /api/v1/analytics/skill-effectiveness`
  - 按 skill_id 聚合 RAGAS 5 维分数 (Faithfulness, Relevance, AnswerRestraint, ContextPrecision, ContextRecall)
  - 过滤 `eval_status='evaluated'` 的交互
  - 支持 `?course_id=` 过滤参数

- [x] G-5: **前端 API 类型与函数** — `frontend/src/lib/api.ts`
  - 新接口: `InquiryTreeNode`, `InquiryTreeResponse`, `InteractionLog`, `InteractionLogResponse`, `SkillScore`, `SkillEffectivenessResponse`
  - 新函数: `getInquiryTree(sessionId)`, `getInteractionLog(sessionId)`, `getSkillEffectiveness(courseId?)`

- [x] G-6: **会话分析页面** — `frontend/src/app/teacher/dashboard/session/[id]/page.tsx`
  - 标签式视图: 追问深度树 + 日志回放
  - `TreeNodeComponent` — 可折叠树节点组件, 显示角色/类型/深度
  - 日志时间线: 带评分药丸的交互列表
  - 评分详情模态框: 点击评分查看 5 维 RAGAS 分数

- [x] G-7: **技能效果图表组件** — `frontend/src/app/teacher/dashboard/SkillEffectivenessChart.tsx`
  - ECharts 分组柱状图 (5 个 RAGAS 维度)
  - 响应式布局, 暗色主题适配

- [x] G-8: **仪表盘页面更新** — `frontend/src/app/teacher/dashboard/page.tsx`
  - 新增技能效果图表区块
  - `loadDashboardData()` 中加载 `getSkillEffectiveness()` 数据
  - 会话表格行增加 "查看分析" 按钮, 导航到 `/teacher/dashboard/session/[id]`

- [x] G-9: **单元测试** — 2 个新测试文件, 30 个测试函数
  - `internal/agent/evaluator_test.go` (20 tests):
    - extractJSON: 纯 JSON / markdown fence / 混合文本 / 无 JSON (4 tests)
    - clampScore: 正常值 / 边界 / 超范围 / 负值 (4 tests)
    - parseEvalScores: 完整 JSON / 部分字段 / 无效 JSON / 空字符串 (4 tests)
    - buildEvalPrompt: 包含学生消息和 Coach 消息 (1 test)
    - DefaultEvalConfig: 默认值验证 (1 test)
    - ragasSystemPrompt: 非空验证 (1 test)
    - evaluateOne: LLM 返回有效/无效分数 (2 tests)
    - evaluateBatch: 无待评估/有待评估交互 (2 tests)
    - Start: context 取消停止 (1 test)
  - `internal/delivery/http/handler/analytics_test.go` (10 tests):
    - classifyTurnType: coach → response / 浅学生 → question / 深学生 → probe / 技能变化 (4 tests)
    - maskName: 中文名 / 英文名 / 短名 (3 tests)
    - buildInquiryTree: 空列表 / 单条 / 一对 / 深层 / 技能切换 / 系统消息 / 连续学生 / nil redactor / 节点字段 (10 tests — 部分合并)

- [x] G-10: **Bug 修复** — `buildInquiryTree` 系统消息处理
  - 问题: `lastRole` 对系统消息也更新, 导致系统消息后的 Coach 消息被静默丢弃
  - 修复: `lastRole` 只对非系统角色 ("student"/"coach") 更新

### Phase G 文件清单

**新建文件 (4):**
- `internal/agent/evaluator.go`
- `internal/agent/evaluator_test.go`
- `internal/delivery/http/handler/analytics.go`
- `internal/delivery/http/handler/analytics_test.go`
- `frontend/src/app/teacher/dashboard/session/[id]/page.tsx`
- `frontend/src/app/teacher/dashboard/session/[id]/page.module.css`
- `frontend/src/app/teacher/dashboard/SkillEffectivenessChart.tsx`

**修改文件 (5):**
- `internal/domain/model/session.go` — Interaction 模型新增 4 字段
- `internal/delivery/http/router.go` — NewRouter 签名扩展至 9 参数, 注册 3 条分析路由
- `cmd/server/main.go` — 传递 piiRedactor, 创建并启动 RAGASEvaluator
- `internal/delivery/http/e2e_test.go` — 更新 NewRouter 调用
- `internal/delivery/http/bench_test.go` — 更新 NewRouter 调用
- `frontend/src/lib/api.ts` — 新增 6 接口 + 3 函数
- `frontend/src/app/teacher/dashboard/page.tsx` — 新增图表区块和分析按钮
- `frontend/src/app/teacher/dashboard/page.module.css` — 新增样式

**V2.0 Phase G 总计: 7 个新文件, 8 个修改文件, 30 个新测试函数 ✅**

---

## V2.0 Phase H: Materials Upload Page ✅

> **目标:** 创建独立的教材管理页面 — PDF 拖拽上传、批量上传队列、进度条、文档删除和失败重试
> **参考:** design.md §3.1, §5.1 Step 1, §14.1

- [x] H-1: **文档删除端点** — `internal/delivery/http/handler/course.go`
  - `DeleteDocument` handler — `DELETE /api/v1/courses/:id/documents/:doc_id`
  - 阻止删除处理中的文档 (409 Conflict)
  - 级联删除: DocumentChunk → 磁盘文件 → 数据库记录
  - 删除后失效 L2 语义缓存

- [x] H-2: **失败文档重试端点** — `internal/delivery/http/handler/course.go`
  - `RetryDocument` handler — `POST /api/v1/courses/:id/documents/:doc_id/retry`
  - 仅允许重试 `status='failed'` 的文档
  - 验证原始文件存在 → 重新解析 PDF → 清除旧 chunks → 异步触发 KA-RAG 管道
  - 文件丢失时返回 410 Gone

- [x] H-3: **文件大小验证** — `internal/delivery/http/handler/course.go`
  - `MaxFileSize = 50 * 1024 * 1024` (50 MB)
  - `UploadMaterial` handler 新增 `header.Size > MaxFileSize` 校验

- [x] H-4: **新路由注册** — `internal/delivery/http/router.go`
  - `DELETE /:id/documents/:doc_id` — 文档删除
  - `POST /:id/documents/:doc_id/retry` — 失败重试

- [x] H-5: **前端 API 函数** — `frontend/src/lib/api.ts`
  - `deleteDocument(courseId, docId)` — DELETE 请求
  - `retryDocument(courseId, docId)` — POST 请求

- [x] H-6: **教材管理页面** — `frontend/src/app/teacher/courses/[id]/materials/page.tsx`
  - 独立页面: `/teacher/courses/[id]/materials`
  - 拖拽上传区: 支持拖放 + 点击选择, 支持 `multiple` 多文件
  - 批量上传队列: `UploadTask` 队列管理, 顺序上传, 进度条显示
  - 客户端验证: PDF 格式 + 50 MB 大小限制
  - 文档卡片网格: 响应式 grid 布局
  - 状态徽章: uploaded/processing/completed/failed 四态, 处理中有脉冲动画
  - 处理中进度条: indeterminate 动画效果
  - 删除功能: 二次确认 (是/否), 处理中文档不可删除
  - 失败重试: "重试" 按钮, 仅对 failed 文档显示
  - 统计概览: 总文档/已完成/失败 数字徽章
  - 状态轮询: 3 秒间隔自动刷新处理中文档状态
  - 清除已完成上传: 队列中已完成/失败任务可一键清除

- [x] H-7: **页面导航连接** — `frontend/src/app/teacher/courses/[id]/outline/page.tsx`
  - 大纲页面头部新增 "📤 教材管理" 链接按钮
  - 教材管理页面 "← 返回课程大纲" 返回链接

- [x] H-8: **单元测试** — `internal/delivery/http/handler/course_test.go` (10 tests)
  - MaxFileSize: 常量值验证 / 50MB 换算 (2 tests)
  - DocStatusConstants: 4 种状态字符串验证 (1 test, 4 subtests)
  - DocumentModelFields: 模型字段赋值验证 (1 test)
  - RetryValidation: 仅 failed 可重试 (1 test, 4 subtests)
  - DeleteValidation: processing 不可删除 (1 test, 4 subtests)
  - FileSizeValidation: 边界值测试 (1 test, 7 subtests)
  - PDFExtensionValidation: 扩展名校验 (1 test, 9 subtests)
  - DocumentChunkModel: chunk 模型字段验证 (1 test)
  - NewCourseHandler_NilCache: nil cache 构造 (1 test)

### Phase H 文件清单

**新建文件 (3):**
- `frontend/src/app/teacher/courses/[id]/materials/page.tsx`
- `frontend/src/app/teacher/courses/[id]/materials/page.module.css`
- `internal/delivery/http/handler/course_test.go`

**修改文件 (4):**
- `internal/delivery/http/handler/course.go` — 新增 DeleteDocument, RetryDocument, MaxFileSize, 文件大小验证
- `internal/delivery/http/router.go` — 注册 2 条新路由
- `frontend/src/lib/api.ts` — 新增 deleteDocument, retryDocument 函数
- `frontend/src/app/teacher/courses/[id]/outline/page.tsx` — 新增教材管理链接
- `frontend/src/app/teacher/courses/[id]/outline/page.module.css` — 新增 .materialsLink 样式

**V2.0 Phase H 总计: 3 个新文件, 5 个修改文件, 10 个新测试函数 ✅**

---

## V2.0 Phase I: Frontend Plugin System ✅

> design.md §7.10–7.16 前端插件架构: Slot 机制、插件注册、沙箱隔离、Skill UI Renderer 契约

### 核心类型定义

- [x] I-1: **Plugin 类型系统** — `frontend/src/lib/plugin/types.ts`
  - `SlotName` 联合类型 — 12 个预定义插槽 (student 5 + teacher 4 + global 3)
  - `PluginType` — 5 类插件 (skill_renderer, dashboard_widget, editor_extension, theme, page_extension)
  - `TrustLevel` — 3 级信任 (core, domain, community)
  - `InteractionMode` — 5 种交互模式 (text, voice, canvas, formula, code)
  - `SkillUIRenderer` 接口 — skillId + metadata + Component
  - `SkillRendererProps` 接口 — studentContext, knowledgePoint, scaffoldingLevel, agentChannel, onInteractionEvent
  - `DashboardWidgetPlugin` 接口 — id, title, Component, dataLoader
  - `PluginRegistration` 统一注册接口 — id, name, type, trustLevel, slots, priority, Component
  - `PluginMessage` 跨沙箱通信协议 — type, id, method, payload, error
  - `AllowedHostMethod` — 6 个受限宿主 API 方法
  - `PluginManifest` — manifest.json schema 类型

### Plugin Registry (React Context)

- [x] I-2: **PluginRegistry 上下文** — `frontend/src/lib/plugin/PluginRegistry.tsx`
  - `PluginRegistryProvider` — React Context Provider，管理全局插件注册表
  - `usePluginRegistryContext()` — 获取 register/unregister 方法
  - `usePluginRegistry(slotName)` — 按 slot 查询已注册插件，按 priority 排序
  - `buildSkillRendererRegistration()` — SkillUIRenderer → PluginRegistration 转换
  - `buildDashboardWidgetRegistration()` — DashboardWidgetPlugin → PluginRegistration 转换

### 核心组件

- [x] I-3: **PluginSlot 组件** — `frontend/src/components/PluginSlot.tsx`
  - 接收 `name: SlotName`, `context`, `fallback` props
  - 查询 registry 中匹配的插件，按 priority 排序渲染
  - 无插件时显示 fallback

- [x] I-4: **PluginSandbox 组件** — `frontend/src/components/PluginSandbox.tsx`
  - 两级隔离策略：core/domain 直接渲染，community 使用 iframe sandbox
  - iframe sandbox: `sandbox="allow-scripts"` 限制
  - postMessage 通信协议: 6 个 AllowedHostMethod 处理器
  - 初始化时向 iframe 发送 context 数据

### 内置插件注册

- [x] I-5: **Dashboard 内置插件** — `frontend/src/lib/plugin/DashboardPlugins.tsx`
  - 将 RadarChart 包装为 `knowledge-radar` 插件 (priority: 10)
  - 将 MasteryBarChart 包装为 `mastery-bar` 插件 (priority: 20)
  - 将 SkillEffectivenessChart 包装为 `skill-effectiveness` 插件 (priority: 30)
  - `useBuiltinDashboardPlugins()` hook — 自动注册/注销所有内置仪表盘插件

### 系统集成

- [x] I-6: **Dashboard 页面集成** — `frontend/src/app/teacher/dashboard/page.tsx`
  - 调用 `useBuiltinDashboardPlugins()` 注册内置仪表盘插件
  - 在技能评估区和活动表之间添加 `<PluginSlot name="teacher.dashboard.widget">` 扩展点
  - 传递 courseId + courseTitle 作为插件上下文

- [x] I-7: **布局层集成** — `frontend/src/components/DashboardLayout.tsx`
  - 用 `<PluginRegistryProvider>` 包裹整个布局
  - 所有 teacher 和 student 页面自动获得插件上下文

- [x] I-8: **构建验证** — `go vet` ✅, `go build` ✅, `go test` ✅, `next build` ✅, `next lint` ✅

### Phase I 文件清单

**新建文件 (4):**
- `frontend/src/lib/plugin/types.ts`
- `frontend/src/lib/plugin/PluginRegistry.tsx`
- `frontend/src/lib/plugin/DashboardPlugins.tsx`
- `frontend/src/components/PluginSlot.tsx`
- `frontend/src/components/PluginSandbox.tsx`

**修改文件 (2):**
- `frontend/src/components/DashboardLayout.tsx` — 包裹 PluginRegistryProvider
- `frontend/src/app/teacher/dashboard/page.tsx` — 添加 PluginSlot + useBuiltinDashboardPlugins

**V2.0 Phase I 总计: 5 个新文件, 2 个修改文件 ✅**

---

## WebSocket Robustness Enhancement ✅

> **目标**: 增强 WebSocket 连接的可靠性 — 心跳保活、自动重连、并发保护、连接状态 UI。

### Backend (`internal/delivery/http/handler/session.go`)

- [x] WS-3: **心跳与超时** — 服务端 ping/pong 保活机制
  - `wsPongWait = 60s`: 等待客户端 pong 的最大时间
  - `wsPingInterval = 30s`: 服务端每 30s 发送一次 ping
  - `wsWriteWait = 10s`: 写操作超时
  - 使用 `SetReadDeadline` + `SetPongHandler` 检测客户端存活
  - 独立 goroutine 运行 ping ticker，通过 `done` channel 优雅退出

- [x] WS-4: **并发保护** — 防止同一会话并发执行 Agent 流水线
  - 新增 `wsConn` 结构体，包裹 `*websocket.Conn` + `sync.Mutex`
  - `writeJSON()` 和 `writePing()` 均加写锁 + 写超时
  - 使用 `turnMu.TryLock()` 非阻塞获取 turn 锁，重复发送时返回友好提示
  - 所有 helper 方法 (`sendEvent`, `sendWSError`) 改用 `*wsConn`

### Frontend (`frontend/src/app/student/session/[id]/page.tsx`)

- [x] WS-1: **自动重连** — 指数退避 + 最大重试次数
  - 基础延迟 1s，最大延迟 30s，最多 8 次重试
  - 使用 `reconnectCountRef` 跟踪重试次数
  - `intentionalCloseRef` 区分主动断开 vs 意外断开
  - 提取 `connectWebSocket()` 函数供重连复用

- [x] WS-2: **心跳 ping** — 30s 间隔向服务端发送 ping 消息
  - `onopen` 时启动 `setInterval`，`onclose` 时清除
  - cleanup 函数同时清除 reconnect timer 和 ping timer

- [x] WS-5: **DRY 修复** — 使用 `createWSUrl()` 替代内联 URL 构造
  - 删除 `getToken` 导入，改用 `api.ts` 中的 `createWSUrl(sessionId)`
  - URL 构建逻辑统一收敛到一处

- [x] WS-6: **连接状态 UI** — 实时连接状态指示器
  - 新增 `WSStatus` 类型: `connecting | connected | reconnecting | disconnected`
  - 状态为 `connected` 时隐藏指示器（不打扰）
  - 重连中显示当前重试次数 / 最大次数
  - 对应 CSS: `.connectionStatus`, `.connectionDot`, 颜色编码（黄/橙/红）

- [x] WS-7: **构建验证** — `go vet` ✅, `go build` ✅, `go test` ✅, `next build` ✅, `next lint` ✅

### WS Robustness 文件清单

**修改文件 (3):**
- `internal/delivery/http/handler/session.go` — 心跳、超时、wsConn 写锁、turnMu 并发保护
- `frontend/src/app/student/session/[id]/page.tsx` — 自动重连、心跳 ping、createWSUrl、连接状态 UI
- `frontend/src/app/student/session/[id]/page.module.css` — 连接状态指示器样式

**WebSocket Robustness Enhancement 总计: 0 个新文件, 3 个修改文件 ✅**

---

## Feature 2: 学生知识图谱 (Student Knowledge Map)

> 交互式力导向图，展示课程知识点之间的前置/关联关系，以掌握度着色。

### 后端

- [x] KM-1: **Neo4j GetCourseGraphEdges()** — 新增图数据库查询
  - `GraphEdge` 结构体: `Source`, `Target`, `Type` (REQUIRES / RELATES_TO)
  - Cypher 查询: Course→Chapter→KP 遍历，收集 REQUIRES 和 RELATES_TO 边
  - 双向 RELATES_TO 去重 (`kp2.id < linked.id`)

- [x] KM-2: **后端 API** — `GET /api/v1/student/knowledge-map?course_id=N`
  - Handler 挂在 `KnowledgeGraphHandler` (有 DB + Neo4j 双访问权限)
  - 合并 PG 数据 (课程/章节/KP 层级 + StudentKPMastery) 与 Neo4j 边
  - 响应类型: `KnowledgeMapResponse` (nodes, edges, avg_mastery, mastered_count, weak_count)
  - 路由注册: `student.GET("/knowledge-map", ...)`, RBAC: STUDENT + SYS_ADMIN
  - Mastery 使用 -1 哨兵值表示「暂无数据」

### 前端

- [x] KM-3: **API 类型 + 函数** — `api.ts` 新增
  - `KnowledgeMapNode`, `KnowledgeMapEdge`, `KnowledgeMapData` 接口
  - `getStudentKnowledgeMap(courseId)` 调用函数

- [x] KM-4: **知识图谱页面** — `/student/knowledge-map/`
  - ECharts 力导向图 (`GraphChart` + force layout)
  - 节点颜色编码: 绿(≥80%) / 黄(≥50%) / 红(<50%) / 灰(无数据)
  - 节点大小: 重点知识更大，难度越高越大
  - REQUIRES 边 → 实线箭头, RELATES_TO 边 → 虚线无箭头
  - 摘要统计卡片: 知识点总数、平均掌握度、已掌握、待加强
  - 课程选择器 (多课程时显示)
  - 图例说明 + ECharts 力导向交互 (缩放/拖拽/聚焦)
  - Tooltip 展示: 知识点名称、章节、难度星级、掌握度、练习次数

- [x] KM-5: **导航链接** — DashboardLayout 侧边栏新增「知识图谱」入口

- [x] KM-6: **构建验证** — `go vet` ✅, `go build` ✅, `go test` ✅, `next build` ✅

### Knowledge Map 文件清单

**新增文件 (2):**
- `frontend/src/app/student/knowledge-map/page.tsx` — 知识图谱页面 (ECharts 力导向图)
- `frontend/src/app/student/knowledge-map/page.module.css` — 页面样式

**修改文件 (4):**
- `internal/repository/neo4j/client.go` — 新增 `GetCourseGraphEdges()` + `GraphEdge` 类型
- `internal/delivery/http/handler/knowledge_graph.go` — 新增 `GetStudentKnowledgeMap` handler
- `internal/delivery/http/router.go` — 新增路由注册
- `frontend/src/lib/api.ts` — 新增类型 + API 函数
- `frontend/src/components/DashboardLayout.tsx` — 侧边栏新增导航项

**Student Knowledge Map 总计: 2 个新文件, 5 个修改文件 ✅**

---

## Feature 3: 错题本自动归档 (Error Notebook Auto-Archiving)

> 交互中暴露的错误和 AI 引导过程自动归档为结构化错题记录，关联知识图谱节点，
> 支持后续复习时的定向 RAG 检索。掌握度达到 0.8 时自动标记为已解决。

### 数据模型

- [x] EN-1: **ErrorNotebookEntry 模型** — `domain/model/session.go`
  - 字段: student_id, kp_id, session_id, student_input, coach_guidance, error_type, mastery_at_error, resolved, resolved_at, archived_at
  - AutoMigrate 注册到 `database.go`

### 后端自动归档逻辑

- [x] EN-2: **archiveErrorIfIncorrect()** — `orchestrator.go` Stage 7
  - 当 `inferCorrectness()` 返回 false 时自动归档当前轮次的学生输入 + Coach 回复
  - 自动解决: 当 BKT 掌握度 >= 0.8 时，批量标记该 KP 的未解决错题为已解决

### 后端 API

- [x] EN-3: **GET /api/v1/student/error-notebook** — `DashboardHandler`
  - 查询参数: `resolved` (bool 过滤), `kp_id` (知识点过滤)
  - 响应: items (含 KP/章节标题) + total_count + unresolved_count + resolved_count
  - RBAC: STUDENT + SYS_ADMIN

### 前端

- [x] EN-4: **API 类型 + 函数** — `api.ts` 新增
  - `ErrorNotebookItem`, `ErrorNotebookData` 接口
  - `getErrorNotebook(opts?)` 调用函数

- [x] EN-5: **错题本页面** — `/student/error-notebook/`
  - 摘要统计卡片: 总错题数、待解决、已解决
  - 筛选按钮: 全部 / 待解决 / 已解决
  - 错题卡片: KP 标题、章节、掌握度 badge、解决状态 badge
  - 对话展示: 学生回答 (红色左边框) + AI 引导 (紫色左边框)
  - 展开/收起长文本
  - 空状态提示

- [x] EN-6: **导航链接** — DashboardLayout 侧边栏新增「错题本」入口

- [x] EN-7: **构建验证** — `go vet` ✅, `go build` ✅, `go test` ✅, `next build` ✅

### Error Notebook 文件清单

**新增文件 (2):**
- `frontend/src/app/student/error-notebook/page.tsx` — 错题本页面
- `frontend/src/app/student/error-notebook/page.module.css` — 页面样式

**修改文件 (5):**
- `internal/domain/model/session.go` — 新增 `ErrorNotebookEntry` 模型
- `internal/repository/postgres/database.go` — AutoMigrate 注册
- `internal/agent/orchestrator.go` — 新增 `archiveErrorIfIncorrect()` + Stage 7 调用
- `internal/delivery/http/handler/dashboard.go` — 新增 `GetErrorNotebook` handler
- `internal/delivery/http/router.go` — 新增路由注册
- `frontend/src/lib/api.ts` — 新增类型 + API 函数
- `frontend/src/components/DashboardLayout.tsx` — 侧边栏新增导航项

**Error Notebook Auto-Archiving 总计: 2 个新文件, 7 个修改文件 ✅**

---

## Feature 4: Cross-Encoder 精重排 (Cross-Encoder Reranking)

**参考:** design.md §8.1.1 (Two-Stage Retrieve & Rerank — Stage 2)

### 概述

在 RRF 粗排之后、CRAG 质量网关之前插入 Cross-Encoder 精重排阶段。
使用 LLM-as-judge 方式对每个 [Query, Chunk] 对做深度语义相关性评分（0-10），
归一化为 [0.0, 1.0] 后替换 RRF 分数，精选 Top-5 最相关知识块。

Pipeline 变更:
```
Before: RRF Top-10 → CRAG → Truncator → Prompt
After:  RRF Top-20 → Cross-Encoder Top-5 → CRAG → Truncator → Prompt
```

### 任务

- [x] CR-1: **CrossEncoderReranker 核心实现** — `internal/agent/reranker.go`
  - `CrossEncoderReranker` struct: llm + topK
  - `Rerank(ctx, query, chunks)` — 逐一评分 + 排序 + Top-K
  - `scoreChunk(ctx, query, content)` — LLM-as-judge 单次评分
  - `parseScore(response)` — 解析 LLM 输出的数字评分 (0-10 → 0.0-1.0)
  - `truncateContent(s, maxChars)` — 防止 chunk 过长
  - 降级策略: LLM nil → 跳过; 全部失败 → 回退 RRF 顺序; 部分失败 → 未评分 chunk 排后

- [x] CR-2: **Designer Agent 集成** — `internal/agent/designer.go`
  - `DesignerAgent` struct 新增 `reranker *CrossEncoderReranker` 字段
  - `NewDesignerAgent` 初始化 reranker
  - Pipeline Step 4 RRF topN 从 10 → 20 (扩大粗排候选池)
  - Pipeline Step 4.5 插入 `a.reranker.Rerank(ctx, userInput, mergedChunks)`
  - Pipeline 注释更新: 新增 Step 4.5 说明

- [x] CR-3: **CRAG 阈值适配** — `internal/agent/crag.go`
  - `defaultRelevanceThreshold` 从 0.015 (RRF 量级) → 0.4 (Cross-Encoder 量级)
  - CRAG 测试用例更新为新阈值范围

- [x] CR-4: **测试** — `internal/agent/reranker_test.go`
  - `TestReranker_BasicReranking` — 基本排序正确性
  - `TestReranker_TopK_Truncation` — Top-K 截断
  - `TestReranker_TopK_LargerThanInput` — Top-K > 输入数
  - `TestReranker_NilLLM_GracefulDegradation` — LLM nil 降级
  - `TestReranker_EmptyChunks` — 空输入
  - `TestReranker_AllScoringFails_FallbackToRRF` — 全部失败回退
  - `TestReranker_PartialScoringFailure` — 部分失败处理
  - `TestReranker_ScoresReplaceRRFScores` — 分数替换验证
  - `TestReranker_DefaultTopK` / `InvalidTopK_Defaults` — 配置校验
  - `TestReranker_LLMCallCount` — LLM 调用次数验证
  - `TestReranker_PreservesChunkMetadata` — 元数据保留
  - `TestParseScore_*` (8 cases) — 评分解析: 整数/浮点/前缀/后缀/越界/无效
  - `TestTruncateContent_*` (4 cases) — 内容截断
  - CRAG 测试更新 (6 cases) — 适配新阈值

- [x] CR-5: **构建验证** — `go vet` ✅, `go build` ✅, `go test ./...` ✅ (全部通过)

### Cross-Encoder Reranking 文件清单

**新增文件 (2):**
- `internal/agent/reranker.go` — Cross-Encoder 重排核心实现
- `internal/agent/reranker_test.go` — 重排器测试 (27 cases)

**修改文件 (3):**
- `internal/agent/designer.go` — DesignerAgent struct + pipeline 集成
- `internal/agent/crag.go` — CRAG 阈值适配 (0.015 → 0.4)
- `internal/agent/crag_test.go` — CRAG 测试适配新阈值

**Cross-Encoder Reranking 总计: 2 个新文件, 3 个修改文件 ✅**

---

## Feature Improvement 5: 角色扮演技能 (Role-Play Skill) ✅

> 参考: design.md §2 Step 2 (L241), §5.1 (L241), §6.2 (L429), §7.13 (L1035), §8.2.1 (L1352)
> AI 模拟历史人物、科学家或外语语伴，让学生在沉浸式情境中巩固知识。

- [x] RP-1: **Skill Plugin 目录与 metadata.json** — `plugins/skills/role-play/backend/metadata.json`
  - Skill ID: `general_review_roleplay` (3 段式命名规范: general_review_roleplay)
  - Category: `role-play` (前端已有占位: CATEGORY_MAP + CATEGORY_ICONS)
  - Subjects: 全学科 (math, physics, chemistry, biology, chinese, english, history, geography)
  - Constraints: `stay_in_character`, `knowledge_must_be_accurate`, `never_break_immersion_unless_asked`, `must_weave_knowledge_naturally`, `max_scenario_switches_per_session: 3`
  - Progressive triggers: activate_when mastery >= 0.5, deactivate_when mastery >= 0.95
  - Evaluation dimensions: immersion_quality, knowledge_integration, student_engagement, learning_transfer

- [x] RP-2: **SKILL.md 约束指令** — `plugins/skills/role-play/backend/SKILL.md`
  - 核心身份: 沉浸式角色扮演导师
  - 绝对约束 (5 条): 角色一致性、知识准确性、自然融入知识点、不主动打破沉浸、最多 3 次切换
  - 角色选择策略: 按学科自动匹配 (数学家/物理学家/化学家/生物学家/历史人物/文学家/语伴/探险家)
  - 引导策略: 高/中/低支架三级递进 (倾听者→对话伙伴→批判性挑战)
  - 退出与总结机制: 角色告别→导师视角→知识点总结→表现评估

- [x] RP-3: **RolePlaySessionState** — `internal/agent/types.go`
  - `RolePlaySessionState` struct: CharacterName, CharacterRole, ScenarioDesc, ScenarioSwitches, MaxSwitches, Active
  - 序列化为 JSONB 存储在 `StudentSession.SkillState`
  - 采用声明式模式（如 socratic-questioning），无需复杂状态机

- [x] RP-4: **Coach Agent 集成** — `internal/agent/coach.go`
  - `isRolePlayActive(skillID)` — 角色扮演技能 ID 判断
  - `loadRolePlayState(sessionID)` / `saveRolePlayState(sessionID, state)` — 状态持久化
  - `defaultRolePlayState()` — 初始状态 (MaxSwitches=3, Active=true)
  - `buildRolePlayContext(state)` — 系统 Prompt 上下文注入
    - 初始状态: 指示 LLM 选择合适角色
    - 角色已选定: 指示继续角色对话
    - 最大切换: 限制进一步情境切换
    - 已退出: 指示总结学习成果
  - `rolePlayActiveLabel(active)` — 状态标签中文化
  - `buildMessages()` 中新增 role-play 状态注入 (与 fallacy-detective 并行)

- [x] RP-5: **Orchestrator 集成** — `internal/agent/orchestrator.go`
  - Stage 6.6: `updateRolePlayStateIfActive()` — 角色扮演状态更新
  - 首轮对话后持久化初始状态
  - `EventRolePlayCharacter` WebSocket 事件常量 (types.go)

- [x] RP-6: **测试** — `internal/agent/roleplay_test.go` (13 cases)
  - `TestIsRolePlayActive` — 技能 ID 判断 (7 cases)
  - `TestDefaultRolePlayState` — 默认状态验证
  - `TestRolePlayActiveLabel` — 状态标签 (2 cases)
  - `TestBuildRolePlayContext_*` (5 cases) — 初始/角色选定/最大切换/已退出/无情境
  - `TestEvaluateProgressiveTriggers_*RolePlay*` (3 cases) — 渐进式触发集成
  - `TestRolePlaySkill*` (2 cases) — 插件注册与列表

- [x] RP-7: **构建验证** — `go vet` ✅, `go build` ✅, `go test ./...` ✅ (全部通过)

### 角色扮演技能文件清单

**新增文件 (3):**
- `plugins/skills/role-play/backend/metadata.json` — 技能元数据
- `plugins/skills/role-play/backend/SKILL.md` — LLM 约束指令 (~70 行)
- `internal/agent/roleplay_test.go` — 角色扮演测试 (13 cases)

**修改文件 (3):**
- `internal/agent/types.go` — 新增 RolePlaySessionState + EventRolePlayCharacter
- `internal/agent/coach.go` — 角色扮演状态管理 + 系统 Prompt 注入
- `internal/agent/orchestrator.go` — Stage 6.6 角色扮演状态更新

**角色扮演技能总计: 3 个新文件, 3 个修改文件 ✅**

## Feature Improvement 6: 前端技能渲染器 (Frontend Skill Renderers) ✅

> 设计文档: §7.10–7.16 插件系统  
> 为三种教学技能 (苏格拉底对话、谬误侦探、角色扮演) 创建专属 React 渲染组件，通过插件系统集成到学生会话页面

- [x] FR-1: **苏格拉底渲染器** — `SocraticRenderer.tsx` + `.module.css`
  - 多轮对话式 UI，支架感知 (high=步骤+关键词+填空, medium=关键词标签, low=纯输入)
  - 知识点上下文头部、流式支持、思考指示器
  - 系统消息展示支架变化通知

- [x] FR-2: **谬误侦探渲染器** — `FallacyRenderer.tsx` + `.module.css`
  - 挑战式 UI：谬误卡片展示、文本高亮/标注工具
  - 阶段进度指示器 (present→identify→explain→correct→reflect)
  - 推理输入区域 + 阶段特定标签、谬误识别动画覆盖

- [x] FR-3: **角色扮演渲染器** — `RolePlayRenderer.tsx` + `.module.css`
  - 角色选择阶段 (头像卡片网格)、沉浸式对话 UI
  - 情境面板 (背景、目标、角色切换计数)、学习总结覆盖
  - 默认角色: 苏格拉底, 爱因斯坦, 李白, 居里夫人

- [x] FR-4: **useBuiltinSkillRenderers Hook** — `SkillRendererPlugins.tsx`
  - 遵循 `DashboardPlugins.tsx` 模式
  - 注册 3 个渲染器: `general_concept_socratic`, `general_assessment_fallacy`, `general_review_roleplay`
  - useEffect 注册/注销清理

- [x] FR-5: **会话页面集成** — `page.tsx` 修改
  - `useBuiltinSkillRenderers()` + `usePluginRegistry('student.interaction.main')`
  - `AgentWebSocketChannel` 适配器封装 WebSocket
  - `matchedPlugin` 按 `skill-renderer-${activeSkill}` 过滤
  - 默认聊天提取为 `renderDefaultChat()`，技能渲染器在 `renderSkillRenderer()` 中

- [x] FR-6: **构建验证** — `npm run build` ✅ (TypeScript 编译通过, 无错误)

### 前端技能渲染器文件清单

**新增文件 (7):**
- `frontend/src/lib/plugin/renderers/SocraticRenderer.tsx` — 苏格拉底对话渲染器
- `frontend/src/lib/plugin/renderers/SocraticRenderer.module.css` — 苏格拉底样式
- `frontend/src/lib/plugin/renderers/FallacyRenderer.tsx` — 谬误侦探渲染器
- `frontend/src/lib/plugin/renderers/FallacyRenderer.module.css` — 谬误侦探样式
- `frontend/src/lib/plugin/renderers/RolePlayRenderer.tsx` — 角色扮演渲染器
- `frontend/src/lib/plugin/renderers/RolePlayRenderer.module.css` — 角色扮演样式
- `frontend/src/lib/plugin/SkillRendererPlugins.tsx` — 内置渲染器注册 Hook

**修改文件 (1):**
- `frontend/src/app/student/session/[id]/page.tsx` — 插件集成 + AgentWebSocketChannel 适配器

**前端技能渲染器总计: 7 个新文件, 1 个修改文件 ✅**

## Feature Improvement 7: 技能商店 UX 增强 (Skill Store UX Enhancement) ✅

> 设计文档: §6.1–6.4 技能生命周期, §7.10 插件扩展点  
> 增强技能商店页面: 搜索/多维筛选、教学阶段分组视图、工具配置与渐进策略展示、完善交互体验

- [x] SS-1: **SkillMetadata 接口扩展** — `api.ts`
  - 新增 `SkillToolConfig` 接口 (enabled + description)
  - 新增 `SkillProgressiveTriggers` 接口 (activate_when + deactivate_when)
  - `SkillMetadata` 增加 `tools?: Record<string, SkillToolConfig>` 和 `progressive_triggers?: SkillProgressiveTriggers` 字段
  - 与 Go 后端 `plugin.SkillMetadata` 完全对齐

- [x] SS-2: **搜索与多维筛选** — `page.tsx`
  - 文本搜索输入框: 按名称、描述、标签、ID 搜索
  - 类型筛选下拉框: 探究式教学/批判性思维/协作学习/角色扮演
  - 学科筛选下拉框: 8 个学科 (已有)
  - 客户端 `useMemo` 过滤，避免重复 API 调用

- [x] SS-3: **教学阶段分组标签页** — `page.tsx`
  - 4 个标签页: 全部技能 / 概念引入 / 练习巩固 / 复习评估
  - 每个标签显示技能数量徽章
  - 按 category 映射到教学阶段 (设计文档 §6.1 表格)

- [x] SS-4: **增强详情弹窗** — `page.tsx` + `page.module.css`
  - 配置工具展示: 工具名、启用状态、描述
  - 渐进策略展示: 激活条件/退出条件 (代码格式)
  - 支架等级可视化: 带圆点和箭头的流程图
  - SKILL.md 使用 MarkdownRenderer 富文本渲染 (替代 `<pre>`)
  - 弹窗加载状态: detailLoading 时显示 spinner 覆盖层
  - slideUp 入场动画

- [x] SS-5: **UX 改进** — `page.tsx` + `page.module.css`
  - 错误状态 UI: 红色提示条 + 重试按钮 (替代 console-only)
  - 空搜索结果: 显示搜索关键词 + 清除按钮
  - 技能卡片工具徽章: 已启用的工具显示在卡片底部
  - 修复 lint: `fetchSkills` 提取为 `useCallback`，`detailLoading` 正确使用
  - 构建验证: `npm run build` ✅

### 技能商店 UX 增强文件清单

**修改文件 (3):**
- `frontend/src/lib/api.ts` — 新增 SkillToolConfig, SkillProgressiveTriggers; 扩展 SkillMetadata
- `frontend/src/app/teacher/skills/page.tsx` — 全面重构: 搜索+筛选+分组+增强弹窗
- `frontend/src/app/teacher/skills/page.module.css` — 新增 ~180 行样式 (标签页、工具、触发器、搜索等)

**技能商店 UX 增强总计: 0 个新文件, 3 个修改文件 ✅**

---

## Feature 8: 数据导出 (Data Export) ✅

> 教师可将学习分析数据导出为 CSV 文件，用于离线分析和报告。
> 设计文档参考: §7.10 DataExporter (Integration-level plugin type)

### 完成任务

- [x] F8-1: **后端 CSV 导出处理器** — `internal/delivery/http/handler/export.go` (新文件, ~320 行)
  - `ExportHandler` 结构体 + `NewExportHandler(db)` 构造函数
  - 4 个导出端点:
    - `GET /export/activity-sessions?course_id=` — 学习活动会话 CSV (学生姓名、开始/结束时间、轮次、RAGAS 均分)
    - `GET /export/class-mastery?course_id=` — 班级掌握度矩阵 CSV (学生×知识点, bloom_level + mastery_score)
    - `GET /export/error-notebook?course_id=` — 错题本 CSV (学生、知识点、错误类型、原始/纠正答案、状态)
    - `GET /export/interaction-log?session_id=` — 交互日志 CSV (轮次、角色、消息、支架等级、RAGAS 5 维评分)
  - UTF-8 BOM (`0xEF, 0xBB, 0xBF`) 确保 Excel 正确打开中文
  - 教师课程归属权限校验 (通过 `teacher_id` 匹配)
  - 批量加载学生姓名、知识点名称，避免 N+1 查询

- [x] F8-2: **路由注册** — `internal/delivery/http/router.go`
  - 新增 `exportHandler := handler.NewExportHandler(db)`
  - 在 TEACHER RBAC 组下添加 `/export` 子路由组 (4 个 GET 端点)
  - `go vet` ✅, `go build` ✅

- [x] F8-3: **前端导出 API** — `frontend/src/lib/api.ts`
  - `downloadCSV(endpoint, params)` 通用下载函数:
    - JWT Bearer 认证
    - 从 `Content-Disposition` 头提取文件名
    - 创建临时 `<a>` 元素触发浏览器下载
  - 4 个导出函数: `exportActivitySessions()`, `exportClassMastery()`, `exportErrorNotebookCSV()`, `exportInteractionLog()`

- [x] F8-4: **前端导出按钮** — `frontend/src/app/teacher/dashboard/page.tsx` + `page.module.css`
  - 仪表盘头部: "导出掌握度" 和 "导出错题本" 按钮 (按选中课程导出)
  - 学习活动弹窗底部: "导出会话 CSV" 按钮
  - 每行会话记录: 小型 "导出" 按钮 (导出交互日志)
  - `exporting` 状态管理: 加载中禁用按钮 + 文字提示
  - CSS 样式: `.exportGroup`, `.exportBtn`, `.exportSmallBtn`

- [x] F8-5: **构建验证**
  - `go vet ./...` ✅
  - `go build` ✅
  - `npm run build` ✅ (Next.js 16.1.6 Turbopack, 12 页面全部通过)

### 数据导出文件清单

**新增文件 (1):**
- `internal/delivery/http/handler/export.go` — CSV 导出处理器 (4 端点, ~320 行)

**修改文件 (4):**
- `internal/delivery/http/router.go` — 新增 exportHandler + 4 条导出路由
- `frontend/src/lib/api.ts` — 新增 downloadCSV 辅助函数 + 4 个导出函数
- `frontend/src/app/teacher/dashboard/page.tsx` — 导出按钮 UI + 状态管理
- `frontend/src/app/teacher/dashboard/page.module.css` — 导出按钮样式

**数据导出总计: 1 个新文件, 4 个修改文件 ✅**

---

## 🎉 所有 8 个功能全部完成

| # | 功能 | 状态 |
|---|------|------|
| 1 | AI 生成课程大纲 | ✅ 已有 |
| 2 | 学生知识图谱 | ✅ 完成 |
| 3 | 错题本自动归档 | ✅ 完成 |
| 4 | Cross-Encoder 重排序 | ✅ 完成 |
| 5 | 角色扮演技能 | ✅ 完成 |
| 6 | 前端技能渲染器 | ✅ 完成 |
| 7 | 技能商店 UX 增强 | ✅ 完成 |
| 8 | 数据导出 | ✅ 完成 |
