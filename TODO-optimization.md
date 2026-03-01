# Hanfledge V6.0 — 优化专项 TODO

**Created:** 2026-03-01
**Source:** 基于全量代码审计报告 (Go backend + Next.js frontend + 基础设施)
**Tech Stack:** Go (Gin+GORM) / Next.js 16 / PostgreSQL (pgvector) / Neo4j / Redis

---

## 进度总览

| Phase | 状态 | 完成度 |
|---|---|---|
| P0: 紧急安全修复 | 🟢 已完成 | 5/5 |
| P1: 高优先级修复 | 🟢 已完成 | 8/8 |
| P2: 架构改进 | 🟢 已完成 | 7/7 |
| P3: 长期质量提升 | 🟢 已完成 | 9/9 |

**总计: 29 / 29 tasks ✅**

---

## P0: 紧急安全修复 (立即处理, 预计 2-3h)

> 不修复可能导致生产安全事故

### P0-1: 生产环境默认密码拦截

- [x] **修改 `internal/config/config.go`**
  - 新增 `Validate() error` 方法
  - 当 `GIN_MODE=release` 时检查 `JWT_SECRET`、`DB_PASSWORD`、`NEO4J_PASSWORD` 是否仍为默认值
  - 默认值命中时返回错误，`main.go` 中调用 `Validate()` 失败则 `log.Fatalf`
  - 检查列表: `"dev-secret-change-me"`, `"hanfledge_secret"`, `"neo4j_secret"`

### P0-2: JWT type assertion panic 修复

- [x] **修改 `internal/delivery/http/middleware/jwt.go:68`**
  - `id.(uint)` → 安全类型断言 `id, ok := ...; if !ok { c.AbortWithStatus(401) }`
  - 注意: JWT 数字在 `MapClaims` 中解析为 `float64`，需要 `float64` → `uint` 转换
  - 添加单元测试: 畸形 claims 不导致 panic

### P0-3: WebSocket CheckOrigin 加固

- [x] **修改 `internal/delivery/http/handler/session.go:71`**
  - `CheckOrigin: func(r *http.Request) bool { return true }` → 基于配置校验 origin
  - 复用 `config.ServerConfig.CORSOrigins` (已有逗号分隔格式)
  - 新增 `isAllowedOrigin(origin string, allowed []string) bool` 辅助函数
  - `GIN_MODE=debug` 时可保留 `return true` 作为开发便利

### P0-4: Marketplace 端点 RBAC 补全

- [x] **修改 `internal/delivery/http/router.go`**
  - marketplace 路由组 (约 L245-253) 添加 `middleware.RequireRole(db, ...)` 中间件
  - `GET /marketplace/skills` — 所有角色可访问 (只读浏览)
  - `POST /marketplace/skills/:id/install`, `POST /submit` — 至少 TEACHER 角色
  - `DELETE /marketplace/skills/:id/uninstall` — TEACHER 角色

### P0-5: go mod tidy 依赖清理

- [x] **运行 `go mod tidy`**
  - 当前 `go.mod` 所有依赖均标记为 `// indirect` (包括 gin, gorm, jwt 等直接依赖)
  - 执行后确认 gin, gorm, jwt, neo4j, redis, godotenv, bcrypt 等进入直接依赖块
  - 验证: `go build ./...` 和 `go test ./...` 仍然通过

---

## P1: 高优先级修复 (本周完成, 预计 1-2d)

> 影响数据完整性、API 可用性或存在已知 bug

### P1-1: API 分页支持

- [x] **新建 `internal/delivery/http/handler/pagination.go`**
  - 定义 `PaginationParams` 结构体: `Page int`, `PageSize int` (默认 1/20, 上限 100)
  - 定义 `PaginatedResponse[T]` 结构体: `Items []T`, `Total int64`, `Page int`, `PageSize int`
  - `ParsePagination(c *gin.Context) PaginationParams` 辅助函数
- [x] **改造以下端点添加分页** (保持无参数时的向后兼容):
  - `GET /api/v1/schools` — `handler/user.go` ListSchools
  - `GET /api/v1/classes` — `handler/user.go` ListClasses
  - `GET /api/v1/users` — `handler/user.go` ListUsers
  - `GET /api/v1/courses` — `handler/course.go` ListCourses
  - `GET /api/v1/activities` — `handler/activity.go` ListActivities
  - `GET /api/v1/marketplace/skills` — `handler/marketplace.go` ListMarketplaceSkills
- [x] 前端 `api.ts` 对应函数添加分页参数支持

### P1-2: 被忽略的错误处理修复

- [x] **`handler/activity.go`**
  - L68: `json.Unmarshal([]byte(a.Config), &config)` — 添加 error 检查，失败跳过该条目
  - L221: `json.Unmarshal` — 同上
  - L345: `json.Unmarshal` — 同上
- [x] **`handler/course.go`**
  - L148: `h.DB.Create(&doc)` — 检查 `.Error`，失败返回 500
- [x] **`handler/dashboard.go`**
  - 多处 `h.DB.*` 调用 — 逐一补上 `.Error` 检查
- [x] **`handler/user.go`**
  - L92: `sid, _ := strconv.ParseUint(...)` — 添加 error 检查，返回 400

### P1-3: N+1 查询修复

- [x] **`handler/dashboard.go:212-231`** — 知识雷达图
  - 将循环内逐个查询 KP 信息改为批量 `WHERE id IN (?)`
  - 先收集所有 KPID → 一次查询所有 KP → 建 map 映射
- [x] **`handler/export.go:82-122`** — CSV 导出
  - 将循环内逐个查询 student/KP 改为批量预加载
  - 使用 `studentMap`/`kpMap` 模式 (部分端点已有此模式，统一到全部)

### P1-4: Context 传播修复

- [x] **`handler/session.go:303`**
  - `Ctx: context.Background()` → `Ctx: r.Context()` (从 HTTP 请求获取)
  - 确保客户端断开时 Agent 管线可取消
- [x] **`handler/course.go:163`**
  - goroutine 中 `context.WithTimeout(context.Background(), ...)` → 从请求 context 派生
  - 注意: 异步任务不能直接用请求 context (请求结束即取消)，应使用 `context.WithoutCancel()` 或独立超时 context
- [x] **`handler/knowledge_graph.go:544`**
  - `ctx := context.Background()` → 使用请求 context
- [x] **`repository/neo4j/client.go:28`**
  - `ctx := context.Background()` → 接收 context 参数

### P1-5: SQL LIKE 通配符转义

- [x] **`handler/marketplace.go:34`**
  - `"%"+search+"%"` → 转义 `search` 中的 `%` 和 `_` 字符
  - 新增 `escapeLike(s string) string` 辅助函数: `strings.ReplaceAll` 替换 `%` → `\%`, `_` → `\_`
  - GORM 查询添加 `ESCAPE '\'` 子句

### P1-6: publishEvent 函数去重

- [x] **新建 `internal/delivery/http/handler/event_helpers.go`**
  - 提取 `publishEvent` 为包级函数或公共方法
  - 当前重复位置: `auth.go:36`, `activity.go:35` (handler 级), `orchestrator.go:630`, `karag.go:376`
  - Handler 级的两个可合并为一个共享实现
  - 各文件删除本地副本，改为调用共享版本

### P1-7: .gitignore 补全

- [x] **修改 `.gitignore`**
  - 添加 `/server` (已存在的编译产物)
  - 添加 `/bin/` (go build 输出目录)
  - 添加 `*.pem`, `*.key` (防止意外提交密钥文件)
  - 删除已跟踪的 `server` 二进制: `git rm --cached server`

### P1-8: 前端 `.env.example` 创建

- [x] **新建 `frontend/.env.example`**
  ```
  # Hanfledge Frontend Environment
  NEXT_PUBLIC_API_URL=http://localhost:8080/api/v1
  ```
  - 当前 `api.ts:6` 引用 `NEXT_PUBLIC_API_URL` 但无文档说明
  - 同步在 `frontend/.gitignore` 中确认 `.env.local` 已排除

---

## P2: 架构改进 (本月完成, 预计 3-5d)

> 降低技术债务，提升可维护性和可测试性

### P2-1: Repository 接口层引入

- [x] **新建 `internal/repository/interfaces.go`** — 定义 8 个核心 Repository 接口
- [x] **新建 `internal/repository/postgres/` 下 8 个 GORM 实现文件**
- [x] **改造 Handler 构造函数** — `AuthHandler`, `CourseHandler`, `DashboardHandler` 已完成
  - 其余 handler (user, activity, session, export, marketplace, knowledge_graph, achievement) 后续迭代
- [x] **改造现有测试** — auth_test, course_test, dashboard_test 已更新
- [x] 验证: `go build ./...` + `go test ./...` 通过 (120+ tests)

### P2-2: 前端常量与组件去重

- [x] **新建 `frontend/src/lib/constants.ts`** — 提取共享常量
  - `STATUS_LABEL` / `STATUS_MAP` (当前在 courses, outline, dashboard, materials 页面重复)
  - `CATEGORY_MAP` / `SUBJECT_MAP` / `CATEGORY_ICONS` (skills/page.tsx 与 skills/create/page.tsx 重复)
  - 各页面改为 `import { STATUS_LABEL, ... } from '@/lib/constants'`
- [x] **新建 `frontend/src/components/LoadingSpinner.tsx`** — 提取重复的加载模式
  - 替代各页面中重复的 `<div style={{ display: 'flex', ... }}><div className="spinner" /></div>`
  - 支持 `size` prop (small/medium/large)
- [x] **合并 `error.tsx`** — 根级、student、teacher 三处几乎相同
  - 新建 `frontend/src/components/ErrorBoundaryFallback.tsx` 共享组件
  - 三个 `error.tsx` 改为调用共享组件 (可传入不同的 "返回" 路径)

### P2-3: 学生会话页面拆分

- [x] **从 `frontend/src/app/student/session/[id]/page.tsx` (~764 行) 提取:**
  - `hooks/useSessionWebSocket.ts` — WebSocket 连接管理、重连、心跳
  - `components/MessageList.tsx` — 消息列表渲染 (气泡、流式光标、Markdown)
  - `components/ScaffoldPanel.tsx` — 支架 UI (高/中/低三级)
  - `components/SessionInput.tsx` — 输入框 + 发送按钮 + 自动伸缩
  - page.tsx 精简为 ~280 行的组合容器
- [x] 验证: `npm run build` 通过

### P2-4: ECharts 初始化统一

- [x] **新建 `frontend/src/lib/echarts-setup.ts`**
  ```ts
  import * as echarts from 'echarts/core';
  import { CanvasRenderer } from 'echarts/renderers';
  // ... 所有需要的组件
  echarts.use([CanvasRenderer, ...]);
  export default echarts;
  ```
- [x] 所有图表组件 (`RadarChart`, `MasteryBarChart`, `SkillEffectivenessChart`, `KnowledgeGraph`, `MasteryTrendChart`) 改为:
  ```ts
  import echarts from '@/lib/echarts-setup';
  ```
  删除各文件中重复的 `echarts.use([...])` 调用

### P2-5: Docker Compose 安全加固

- [x] **修改 `deployments/docker-compose.yml`**
  - 硬编码密码改为环境变量引用: `POSTGRES_PASSWORD: ${DB_PASSWORD:-hanfledge_secret}`
  - 添加资源限制 (`deploy.resources.limits`) — PostgreSQL 和 Neo4j 各 1G/1CPU, Redis 256M/0.5CPU
  - 添加 `restart: unless-stopped` 策略
  - 添加 Neo4j 和 Redis 健康检查
- [x] **新建 `deployments/.env.example`** — 对应 docker-compose 所需的环境变量

### P2-6: 健康检查端点增强

- [x] **修改 `internal/delivery/http/handler/health.go`**
  - 现有 `GET /health` 重构为 `HealthHandler.Liveness` (供 Docker HEALTHCHECK 使用)
  - 新增 `GET /health/ready` 端点 (`HealthHandler.Readiness`) — 深度检查:
    - PostgreSQL: `db.PingContext(ctx)`
    - Neo4j: `client.VerifyConnectivity(ctx)`
    - Redis: `cache.Ping(ctx)`
  - 返回各组件状态: `{ "postgres": "ok", "neo4j": "ok", "redis": "ok" | "skipped" }`
  - 任一组件不可用 → HTTP 503
  - 新增 `RedisCache.Ping()` 方法

### P2-7: 前端测试基础设施

- [x] **安装测试依赖**
  ```bash
  npm install -D vitest @testing-library/react @testing-library/jest-dom jsdom
  ```
- [x] **添加 vitest 配置** — `frontend/vitest.config.ts`
- [x] **编写首批核心测试:**
  - `src/lib/api.test.ts` — apiFetch 401 重定向、token 管理、错误处理 (11 tests)
  - `src/components/Toast.test.tsx` — Toast 组件渲染、自动消失、关闭、多 toast (6 tests)
  - `src/lib/cache/indexedDBCache.test.ts` — 缓存命中/未命中/过期 (9 tests)
- [x] 在 `package.json` 添加 `"test": "vitest"`, `"test:run": "vitest run"` 脚本
- [x] 验证: 26/26 tests 全部通过

---

## P3: 长期质量提升 (下个迭代, 预计 1-2w)

> 提升用户体验、开发效率和可观测性

### P3-1: CI/CD 流水线

- [x] **新建 `.github/workflows/ci.yml`**
  ```yaml
  jobs:
    backend:
      - go vet ./...
      - go test -race ./...
      - go build -o /dev/null ./cmd/server/main.go
    frontend:
      - npm ci
      - npm run lint
      - npm run build
      - npm run test:run
  ```
  - 触发条件: push to main + PR
  - Go 和 Node 并行执行

### P3-2: Cross-Encoder 批量评分优化

- [x] **修改 `internal/agent/reranker.go`**
  - 当前: 对 Top-20 每个 chunk 单独调用 LLM (20 次 LLM 调用)
  - 优化: 将多个 [query, chunk] 对合并为单个 batch prompt
  - 单次 LLM 调用返回 JSON 数组 `[{"chunk_id": 1, "score": 8.5}, ...]`
  - 预期: LLM 调用从 20 次降至 2-4 次 (每批 5-10 个 chunk)
  - 保留逐个评分作为 fallback (batch 解析失败时降级)

### P3-3: RAGAS 评估器事件驱动化

- [x] **修改 `internal/agent/evaluator.go`**
  - 新增 `evalCh chan uint` 通知 channel + `Notify(id)` 非阻塞方法
  - `Start()` 改为 `select { case <-evalCh: ... case <-ticker.C: ... }` 双通道监听
  - 无事件时采用指数退避: 30s → 60s → 120s → 300s (最大)
  - 有事件立即触发评估 + 排空通道中积攒的通知
  - `EvalConfig` 新增 `MaxInterval`, `NotifyBuffer` 字段
- [x] **修改 `internal/agent/orchestrator.go`**
  - 新增 `evalNotify func(uint)` 字段 + `SetEvalNotifier()` 注入方法
  - `saveInteraction()` 保存 coach 交互后调用 `evalNotify(coachMsg.ID)` 通知评估引擎
- [x] **修改 `cmd/server/main.go`**
  - 在 evaluator 和 orchestrator 都创建后调用 `orchestrator.SetEvalNotifier(evaluator.Notify)` 连接
- [x] **更新测试** — 新增 12 个测试覆盖 Notify, backoff, drainNotifications, DefaultEvalConfig
  - 验证: 109/109 agent tests 通过

### P3-4: 前端可访问性 (a11y) 改进

- [x] **模态框 (`Modal`) 可访问性**
  - 添加 `role="dialog"`, `aria-modal="true"`, `aria-labelledby`
  - 实现焦点陷阱 (Tab 循环在模态框内)
  - ESC 键关闭
  - 新建 `useModalA11y` hook (`frontend/src/lib/a11y.ts`)
  - 应用到全部 8 个模态框 (10+ 文件)
- [x] **交互式卡片可访问性**
  - 所有 `onClick` 的 `<div>` 添加 `role="button"`, `tabIndex={0}`, `onKeyDown` (Enter/Space)
  - 新建 `handleCardKeyDown` + `cardA11yProps` 工具函数 (`frontend/src/lib/a11y.ts`)
  - 应用到全部 12 个交互式 div/span 元素
- [x] **全局可访问性**
  - 添加 skip-to-content 链接 (DashboardLayout)
  - 状态颜色指示器添加 `aria-hidden="true"` + 文本标签 (10 处修复)
  - KnowledgeGraph 节点标签添加 mastery% 文本

### P3-5: 结构化日志

- [x] **新建 `internal/infrastructure/logger/logger.go`**
  - 基于 `log/slog` (Go 1.21+ 标准库) 的统一日志包
  - `Init(level)`: 按 GIN_MODE 设置日志级别 (debug→Debug, release→Info+JSON)
  - `L(component)`: 返回带 `component` 结构化字段的子 logger
  - `Fatal(msg, args...)`: Error 级别 + `os.Exit(1)`
- [x] **迁移全部 38 个 Go 源文件** (287 个 log 调用)
  - `cmd/server/main.go` (24 calls)
  - `internal/agent/` (orchestrator, coach, evaluator, designer, strategist, critic, reranker, crag, ragfusion, truncator, bkt — ~100 calls)
  - `internal/delivery/http/handler/` (session, course, dashboard, activity, achievement — ~46 calls)
  - `internal/usecase/karag.go` (10 calls)
  - `internal/infrastructure/` (cache, llm/router, safety, search, asr, embedding, federated, i18n — ~60 calls)
  - `internal/plugin/` (registry, eventbus, grpc, validator — ~19 calls)
  - `internal/repository/` + `internal/config/` (9 calls)
  - `scripts/seed.go` (16 calls)
- [x] **日志级别分布**: Fatal 8 / Error 16 / Warn 95 / Info 117 / Debug 51
- [x] 验证: `go vet ./...` + `go build` + 全部测试通过

### P3-6: API 文档自动生成

- [x] **集成 `swaggo/swag`**
  - 安装: `go install github.com/swaggo/swag/cmd/swag@latest`
  - 在 handler 函数上添加 Swagger 注释 — 全部 38 个端点已覆盖:
    - `auth.go` (2/2): Login, GetMe
    - `health.go` (2/2): Liveness, Readiness
    - `course.go` (8/8): ListCourses, CreateCourse, UploadMaterial, GetOutline, GetDocumentStatus, SearchCourse, DeleteDocument, RetryDocument
    - `dashboard.go` (5/5): GetKnowledgeRadar, GetStudentMastery, GetActivitySessions, GetSelfMastery, GetErrorNotebook
    - `activity.go` (7/7): CreateActivity, ListActivities, PublishActivity, PreviewActivity, StudentListActivities, JoinActivity, GetSession
    - `user.go` (7/7): ListSchools, CreateSchool, ListClasses, CreateClass, ListUsers, CreateUser, BatchCreateUsers
    - `marketplace.go` (6/6): ListPlugins, GetPlugin, SubmitPlugin, InstallPlugin, UninstallPlugin, ListInstalled
    - `session.go` (1/1): StreamSession (WebSocket)
  - 生成 `docs/swagger.json` + `docs/swagger.yaml` + `docs/docs.go` (via `swag init`)
  - 注册路由: `GET /swagger/*any` (仅 GinMode != "release" 时启用)
  - Tags: Dashboard, Student, Activities, Sessions, Courses, Admin, Marketplace, Auth, Health

### P3-7: Router 按领域拆分

- [x] **拆分 `internal/delivery/http/router.go`**
  - 当前 `NewRouter` 注册全部路由 (约 50+ 端点)
  - 拆分为: `RegisterAuthRoutes(deps)`, `RegisterCourseRoutes(deps)`, `RegisterSessionRoutes(deps)`, `RegisterAnalyticsRoutes(deps)`, `RegisterExportRoutes(deps)`, `RegisterMarketplaceRoutes(deps)`
  - `NewRouter` 仅负责创建 gin.Engine + 中间件 + 调用各 Register 函数

### P3-8: 前端状态管理优化

- [x] **评估引入轻量状态管理**
  - 安装 `zustand` — 轻量 (1.2kB), 无 boilerplate, 原生 React 兼容
  - 新建 `frontend/src/lib/auth-store.ts` — `useAuthStore` (user, loading, fetchUser, loginUser, logout)
  - 重构 `DashboardLayout`: 从 useState+useEffect 改为 `useAuthStore`
  - 重构 `login/page.tsx`: `loginUser(token, user)` 缓存登录响应的 User
  - 修复 `knowledge-map/page.tsx` 和 `error-notebook/page.tsx` 双重嵌套 `DashboardLayout` 问题
  - 效果: `getMe()` 调用从每次布局挂载降至全局仅 1 次; 登录后 0 次 (User 已缓存)

### P3-9: NewRouter 签名扩展预防

- [x] **建立架构约束** — 防止 `RouterDeps` 继续膨胀
  - `registerXxxRoutes` 不再接收 `RouterDeps`，只接收具体 handler 和 db
  - `registerAuthRoutes` 参数从 `deps RouterDeps` 改为 `jwtSecret string`
  - `RouterDeps` 注释中增加架构约束说明
  - `AGENTS.md` 新增 "Router Architecture & Dependency Rules" 章节 (5 条规则)
  - 考虑按 handler group 注入依赖 (每个 Register 函数只接收自己需要的依赖)
  - 或引入 DI 容器 (如 `uber-go/fx`，但需评估复杂度 vs 收益)
  - 记录到 AGENTS.md: "新增依赖时优先考虑是否可以通过现有接口组合"

---

## 验证清单

每个 Phase 完成后执行:

```bash
# Go 后端
go vet ./...
go build -o /dev/null ./cmd/server/main.go
go test -race ./...

# 前端
cd frontend && npm run lint
cd frontend && npm run build
cd frontend && npm run test:run
```

---

## 文件影响矩阵 (预估)

| Phase | 新增文件 | 修改文件 | 预计变更行数 |
|-------|---------|---------|------------|
| P0 | 0 | 5 | ~80 |
| P1 | 3 | ~15 | ~400 |
| P2 | ~10 | ~20 | ~1200 |
| P3 | ~5 | ~15 | ~800 |
| **合计** | **~18** | **~55** | **~2480** |
