# Hanfledge MVP V1.0 — TODO Tasks

**Last Updated:** 2026-02-28 14:00
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
| Post-MVP: 单元测试补全 | ⏳ 待开始 | 0% | — |

**MVP 总进度: 73 / 73 tasks (100%)**

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

## Post-MVP: 单元测试补全 ⏳

- [ ] PM-6.1: **Agent 层单元测试** — Coach/Critic/Strategist/Designer mock 测试
- [ ] PM-6.2: **UseCase 层单元测试** — KA-RAG pipeline 测试
- [ ] PM-6.3: **Repository 层单元测试** — GORM/Neo4j mock 测试
