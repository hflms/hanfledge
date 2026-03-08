# Hanfledge

*[English Version (英文版) 🇺🇸](README.md)*

面向 K-12 课堂的 AI 原生智慧教育平台，集成多智能体协同编排、知识图谱与苏格拉底式学习。

教师上传课程资料，系统自动提取知识图谱。学生通过 AI 引导的苏格拉底式对话进行学习，
脚手架（Scaffold）基于贝叶斯知识追踪（BKT）实时自适应调整。

## 系统架构

```
                    ┌─────────────────────────────────┐
                    │          Nginx (生产环境)         │
                    │    /api/* → backend:8080         │
                    │    /*     → frontend:3000        │
                    └────────┬───────────┬────────────┘
                             │           │
              ┌──────────────┘           └──────────────┐
              ▼                                         ▼
┌──────────────────────┐                 ┌──────────────────────┐
│   Go 后端 (Gin)      │                 │  Next.js 前端        │
│                      │                 │  (App Router)        │
│  JWT 认证 + RBAC     │                 │                      │
│  KA-RAG 管线         │                 │  管理员后台          │
│  多智能体编排器       │                 │  教师工作台          │
│  WebSocket 流式推送   │                 │  学生对话界面        │
│  教师实时干预         │                 │  ECharts 学情分析    │
│  WeKnora 知识库集成   │                 │  帮助中心            │
└──┬────┬────┬────┬──┬─┘                 └──────────────────────┘
   │    │    │    │  │
   ▼    ▼    ▼    ▼  ▼
┌────┐┌────┐┌────┐┌────────┐┌─────────┐
│ PG ││Neo4││Redis││ Ollama ││ WeKnora │
│    ││j   ││     ││        ││ (可选)  │
│pgv.││    ││     ││qwen2.5 ││ 知识库  │
└────┘└────┘└────┘└────────┘└─────────┘
```

**后端**（Go 1.25）：Gin HTTP 框架、GORM ORM、JWT 认证 + 四级角色权限控制
（SYS_ADMIN / SCHOOL_ADMIN / TEACHER / STUDENT）。支持教师实时干预（接管与悄悄话）、
动态 AI 模型配置，以及可选的 WeKnora 知识库集成。

**前端**（Next.js 16）：React 19 + App Router、CSS Modules、ECharts 学情可视化、
WebSocket 实时流式对话。包含管理员后台、动态 AI 配置页面、角色化应用内帮助中心，
以及 i18n 国际化支持（zh-CN / en-US）。

**AI 管线**：多智能体协同编排（策略师 → 设计师 → 教练 → 评审员），搭配 KA-RAG
（知识增强检索增强生成）、RRF 混合检索（pgvector + Neo4j）、BKT 驱动的脚手架渐隐，
以及教师悄悄话注入机制实现实时教学干预。跨会话学习分析体系提供学生画像聚合、
学习路径追踪和可扩展维度记录，支持个性化学情洞察。

**技能插件**：可插拔技能系统，内置 8 个技能 —— 苏格拉底提问、智能出题、角色扮演、
谬误侦探、错题诊断、跨学科关联、学情问卷调查、演示文稿生成。

## 前置要求

- Go 1.25+
- Node.js 22+
- Docker 与 Docker Compose
- Ollama 并安装 `qwen2.5:7b` 和 `bge-m3` 模型

拉取所需的 Ollama 模型：

```sh
ollama pull qwen2.5:7b
ollama pull bge-m3
```

## 快速开始

### 自动化启动（推荐）

启动整个开发栈（基础设施、后端和前端）的最简方式是运行提供的开发脚本：

```sh
bash scripts/dev.sh
```

此脚本会自动处理以下任务：
- 启动 Docker 容器（Postgres、Neo4j、Redis）
- 若不存在则自动将 `.env.example` 复制为 `.env`
- 安装前端依赖
- 并行启动 Go 后端和 Next.js 前端

**支持的参数：**
- `--seed`：在基础设施启动后自动运行种子数据脚本创建测试账号。
- `--backend-only`：仅启动基础设施和 Go 后端（跳过前端）。
- `--frontend-only`：仅启动 Next.js 前端（跳过基础设施和后端）。
- `--weknora`：同时启动可选的 WeKnora 知识库服务。

---

### 手动启动

#### 1. 启动基础设施

```sh
docker compose -f deployments/docker-compose.yml up -d
```

这将启动 PostgreSQL（pgvector）、Neo4j 和 Redis，端口映射如下：

| 服务       | 镜像                   | 宿主端口  | 用途            |
|------------|------------------------|-----------|-----------------|
| PostgreSQL | pgvector/pgvector:pg16 | 5433      | 数据 + 向量存储 |
| Neo4j      | neo4j:5-community      | 7475/7688 | 知识图谱        |
| Redis      | redis:7-alpine         | 6381      | 缓存 / 会话     |

如需同时启动可选的 WeKnora 知识库服务：

```sh
docker compose -f deployments/docker-compose.yml --profile weknora up -d
```

WeKnora 现已完全集成，包含以下服务：

| 服务       | 镜像                              | 宿主端口 | 用途           |
|------------|-----------------------------------|----------|----------------|
| WeKnora    | wechatopenai/weknora-app:latest   | 9380     | 知识库服务     |
| DocReader  | wechatopenai/weknora-docreader    | 50051    | 文档解析服务   |

PostgreSQL 服务使用 `paradedb/paradedb:latest` 镜像，同时包含 `pg_search`
（WeKnora 所需）和 `pgvector` 扩展。

### 2. 配置环境变量

```sh
cp .env.example .env
```

编辑 `.env` 文件，设置 JWT 密钥并确认数据库连接凭据。默认配置与上述 Docker Compose
环境匹配。

核心配置项：

| 变量                  | 默认值            | 说明                           |
|-----------------------|-------------------|---------------------------------|
| `LLM_PROVIDER`        | `ollama`          | `ollama`、`dashscope` 或 `gemini` |
| `EMBEDDING_PROVIDER`  | `ollama`          | `ollama` 或 `dashscope`          |
| `WEKNORA_ENABLED`     | `false`           | 启用 WeKnora 知识库集成           |

### 3. 启动后端

```sh
go run cmd/server/main.go
```

服务运行在 `http://localhost:8080`。首次启动时自动执行数据库迁移并创建默认角色。

### 4. 初始化测试数据

```sh
go run scripts/seed.go
```

将创建测试学校、班级、教师和学生账号：

| 角色               | 手机号       | 密码       |
|--------------------|--------------|------------|
| 系统管理员         | 13800000001  | admin123   |
| 教师（兼校管理员） | 13800000010  | teacher123 |
| 教师               | 13800000011  | teacher123 |
| 学生（1班）        | 13800000100  | student123 |
| 学生（2班）        | 13800000105  | student123 |

### 5. 启动前端

```sh
cd frontend
npm install
npm run dev
```

打开 `http://localhost:3000`，使用上述任一测试账号登录。

## 生产部署

提供了完整的生产环境 Docker Compose 配置：

```sh
docker compose -f deployments/docker-compose.prod.yml up -d
```

包含 7 个服务：Nginx 反向代理、Go 后端、Next.js 前端、PostgreSQL、Neo4j、Redis。
Nginx 负责路由分发（`/api/*` 转发到后端，其余转发到前端）以及 WebSocket 升级。

## 项目结构

```
cmd/server/main.go                 # 入口文件
internal/
  config/                          # 基于环境变量的配置
  domain/model/                    # GORM 模型（用户、知识点、会话、技能、学习分析、WeKnora）
  delivery/http/                   # Gin 路由、处理器、中间件（JWT、RBAC）
  usecase/                         # 业务逻辑（KA-RAG 管线）
  repository/postgres/             # PostgreSQL 连接 + 自动迁移
  repository/neo4j/                # Neo4j 图数据库客户端
  infrastructure/llm/              # LLM 客户端（Ollama、DashScope、动态提供者）
  infrastructure/safety/           # 提示注入防护、PII 脱敏
  infrastructure/weknora/          # WeKnora 知识库客户端 + Token 管理
  infrastructure/cache/            # Redis 缓存层
  infrastructure/i18n/             # 国际化（zh-CN、en-US）
  infrastructure/asr/              # 语音转文字（ASR）
  agent/                           # 多智能体编排器（四智能体管线）
  agent/profile.go                 # 跨会话学习分析服务（画像、路径、维度）
  agent/orchestrator_intervention  # 教师悄悄话与接管处理
plugins/skills/                    # 8 个可插拔技能定义
  socratic-questioning/            #   苏格拉底式对话
  quiz-generation/                 #   基于知识点的智能出题
  role-play/                       #   角色扮演情境
  fallacy-detective/               #   逻辑谬误识别
  error-diagnosis/                 #   错题分析与补救
  cross-disciplinary/              #   跨学科关联
  learning-survey/                 #   学情问卷诊断
  presentation-generator/          #   演示文稿生成
frontend/                          # Next.js 16 应用（React 19、TypeScript）
  src/app/admin/                   #   管理员后台（学校、班级、用户管理）
  src/app/teacher/                 #   教师工作台、设置、WeKnora
  src/app/student/                 #   学生活动与会话
  src/app/help/                    #   应用内帮助中心
  src/lib/plugin/                  #   技能渲染器（苏格拉底、出题、问卷等）
locales/                           # i18n 消息包（zh-CN、en-US）
docs/
  manuals/                         # 角色化操作手册
  swagger.yaml                     # OpenAPI / Swagger 接口文档
deployments/
  docker-compose.yml               # 开发环境基础设施（+ 可选 WeKnora 配置）
  docker-compose.prod.yml          # 生产环境全栈部署
  nginx.conf                       # 反向代理配置
scripts/seed.go                    # 测试数据初始化脚本
```

## API 参考

所有端点使用 `/api/v1/` 前缀。认证使用 Bearer JWT 令牌。
开发模式下可访问 `/swagger/index.html` 查看 Swagger UI。

### 公开接口

| 方法   | 路径               | 说明              |
|--------|--------------------|--------------------|
| GET    | `/health`          | 存活探针           |
| GET    | `/health/ready`    | 就绪探针           |
| POST   | `/api/v1/auth/login` | 登录（手机号 + 密码） |

### 认证接口（需 JWT）

| 方法   | 路径                                     | 角色                         | 说明                        |
|--------|------------------------------------------|------------------------------|-----------------------------|
| GET    | `/api/v1/auth/me`                        | 任意                         | 获取当前用户与角色          |
| GET    | `/api/v1/schools`                        | SYS_ADMIN                    | 学校列表                    |
| POST   | `/api/v1/schools`                        | SYS_ADMIN                    | 创建学校                    |
| GET    | `/api/v1/classes`                        | SYS_ADMIN, SCHOOL_ADMIN      | 班级列表                    |
| POST   | `/api/v1/classes`                        | SYS_ADMIN, SCHOOL_ADMIN      | 创建班级                    |
| GET    | `/api/v1/users`                          | SYS_ADMIN, SCHOOL_ADMIN      | 用户列表                    |
| POST   | `/api/v1/users`                          | SYS_ADMIN, SCHOOL_ADMIN      | 创建用户                    |
| POST   | `/api/v1/users/batch`                    | SYS_ADMIN, SCHOOL_ADMIN      | 批量创建用户                |
| GET    | `/api/v1/courses`                        | TEACHER+                     | 课程列表（按教师筛选）      |
| POST   | `/api/v1/courses`                        | TEACHER+                     | 创建课程                    |
| POST   | `/api/v1/courses/:id/materials`          | TEACHER+                     | 上传 PDF，触发 KA-RAG       |
| GET    | `/api/v1/courses/:id/outline`            | TEACHER+                     | 课程大纲（章节 + 知识点）   |
| GET    | `/api/v1/courses/:id/documents`          | TEACHER+                     | 文档处理状态                |
| POST   | `/api/v1/courses/:id/search`             | TEACHER+                     | 语义搜索（pgvector）        |
| GET    | `/api/v1/skills`                         | TEACHER+                     | 可用技能列表                |
| GET    | `/api/v1/skills/:id`                     | TEACHER+                     | 技能详情                    |
| POST   | `/api/v1/chapters/:id/skills`            | TEACHER+                     | 将技能挂载到章节            |
| PATCH  | `/api/v1/chapters/:id/skills/:mount_id`  | TEACHER+                     | 更新技能配置                |
| DELETE | `/api/v1/chapters/:id/skills/:mount_id`  | TEACHER+                     | 卸载技能                    |
| POST   | `/api/v1/activities`                     | TEACHER+                     | 创建学习活动                |
| GET    | `/api/v1/activities`                     | TEACHER+                     | 学习活动列表                |
| POST   | `/api/v1/activities/:id/publish`         | TEACHER+                     | 发布活动                    |
| POST   | `/api/v1/activities/:id/preview`         | TEACHER+                     | 预览活动（沙盒模式）        |
| GET    | `/api/v1/activities/:id/sessions`        | TEACHER+                     | 活动会话分析                |
| POST   | `/api/v1/activities/:id/join`            | 任意                         | 学生加入活动                |
| GET    | `/api/v1/sessions/:id`                  | 任意                         | 获取会话详情                |
| GET    | `/api/v1/sessions/:id/stream`           | 任意（WebSocket）            | 实时 AI 对话流              |
| POST   | `/api/v1/sessions/:id/intervention`     | TEACHER+                     | 教师干预（接管/悄悄话）     |
| GET    | `/api/v1/student/activities`            | STUDENT                      | 可参与的活动列表            |
| GET    | `/api/v1/student/mastery`               | STUDENT                      | 个人掌握度数据              |
| GET    | `/api/v1/dashboard/knowledge-radar`     | TEACHER+                     | 班级知识雷达图              |
| GET    | `/api/v1/students/:id/mastery`          | TEACHER+                     | 学生掌握度详情              |
| GET    | `/api/v1/system/config`                 | 任意                         | 获取系统配置                |
| PUT    | `/api/v1/system/config`                 | 任意                         | 更新系统配置                |
| POST   | `/api/v1/system/config/test-chat-model` | 任意                         | 测试聊天模型可用性          |
| POST   | `/api/v1/system/config/test-embedding-model` | 任意                    | 测试嵌入模型可用性          |

### WeKnora 集成（可选）

需设置 `WEKNORA_ENABLED=true` 启用。所有接口要求 TEACHER+ 角色。

| 方法   | 路径                                             | 说明                   |
|--------|--------------------------------------------------|------------------------|
| GET    | `/api/v1/weknora/knowledge-bases`                | 列出远程知识库         |
| GET    | `/api/v1/weknora/knowledge-bases/:kb_id`         | 获取知识库详情         |
| GET    | `/api/v1/weknora/knowledge-bases/:kb_id/knowledge` | 列出知识库条目       |
| POST   | `/api/v1/courses/:id/weknora-refs`               | 绑定知识库到课程       |
| GET    | `/api/v1/courses/:id/weknora-refs`               | 列出课程绑定的知识库   |
| DELETE | `/api/v1/courses/:id/weknora-refs/:ref_id`       | 解绑知识库             |
| POST   | `/api/v1/courses/:id/weknora-search`             | 在绑定的知识库中搜索   |

### WebSocket 协议

通过查询参数或 Header 携带 Bearer 令牌连接 `/api/v1/sessions/:id/stream`。

**客户端事件：**

```json
{"type": "user_message", "content": "什么是光合作用？"}
```

**服务端事件：**

| 事件                 | 说明                                 |
|----------------------|--------------------------------------|
| `agent_thinking`     | 智能体管线阶段更新                   |
| `token_delta`       | Coach LLM 流式输出的 Token           |
| `ui_scaffold_change` | 脚手架级别变更（高/中/低）           |
| `turn_complete`      | AI 回合结束                          |

## 开发指南

### 构建与测试

```sh
# 构建
go build -o bin/hanfledge cmd/server/main.go

# 运行全部测试
go test ./...

# 带竞态检测运行测试
go test -race ./...

# 仅运行端到端测试
go test ./internal/delivery/http/ -v -timeout=60s

# 运行基准测试
go test ./internal/delivery/http/ -run '^$' -bench=. -benchmem -benchtime=3s

# 静态分析
go vet ./...
```

### 前端

```sh
cd frontend
npm run dev       # 开发服务器
npm run build     # 生产构建
npm run lint      # ESLint 检查
npm run test:run  # 单元测试（Vitest）
```

### 性能基准

测试环境：AMD Ryzen 9 5900HS（16 线程），PostgreSQL 运行于 Docker：

| 基准测试     | ops       | 延迟       | 内存/op    | 分配/op   |
|-------------|-----------|------------|------------|-----------|
| Login       | 6,715     | 532 us     | 36 KB      | 314       |
| GetMe       | 3,688     | 903 us     | 68 KB      | 727       |
| ListCourses | 2,941     | 1,370 us   | 173 KB     | 999       |
| HealthCheck | 1,277,374 | 2.8 us     | 7.6 KB     | 47        |

并发压力测试：50 线程 × 20 请求 = 1,155 req/s，错误率 0%。

### 2.0 优化（2026-03-08）

**并行 Agent 执行：**
- Strategist + Designer 现在使用 goroutine 并行运行
- TTFT（首 Token 时间）减少约 40%
- Strategist 分析期间预加载 Neo4j 图谱上下文

**语音活动检测（VAD）：**
- 前端使用 Silero VAD（WebAssembly）检测语音
- 仅在检测到语音时发送音频
- 减少后端 ASR 计算量 50-70%
- 视觉反馈：🔴 等待语音，🟢 检测到语音

**WeKnora 集成（2026-03-08）：**
- SSO 单点登录与自动用户同步
- 基于 Redis 缓存的用户级 Token 管理
- Neo4j 图数据库支持记忆/知识图谱功能
- 前端集成"打开 WeKnora"按钮
- 无缝的知识库绑定到课程

**技能系统重构（2026-03-08）：**
- 统一的 Hooks 系统（useMessages、useStateMachine、useAgentChannel）
- 共享 UI 组件库（ProgressBar、PhaseIndicator、QuestionCard、LoadingState）
- Quiz 和 Presentation 技能的渐进式生成
- 虚拟化消息列表，性能提升 43 倍
- 动态渲染器加载，Bundle 大小减少 86%
- 代码量减少：重构后的渲染器平均减少 60%
- 详见 [docs/SKILL_REFACTORING_SUMMARY.md](docs/SKILL_REFACTORING_SUMMARY.md)

运行性能基准测试：
```bash
go run scripts/benchmark-parallel.go
```

## 核心技术细节

- **认证**：JWT (HS256) Bearer 令牌；RBAC 通过 `UserSchoolRole` 连接表实现
- **向量嵌入**：bge-m3（1024 维）通过 Ollama，支持按提供者配置
- **聊天模型**：qwen2.5:7b 通过 Ollama，支持按提供者配置（已适配 DashScope）
- **动态 AI 配置**：聊天与嵌入提供者可通过管理设置界面在运行时独立配置
- **DashScope**：需要时通过 `DASHSCOPE_COMPAT_BASE_URL` 使用 OpenAI 兼容接口
- **向量搜索**：PostgreSQL 中的 pgvector 余弦相似度检索
- **知识图谱**：Neo4j 维护概念关系与先修知识链路
- **混合检索**：RRF 融合（pgvector 语义 Top-50 + Neo4j 图 Top-50 → Top-10）
- **掌握度追踪**：贝叶斯知识追踪（BKT）+ 脚手架渐隐机制
- **跨会话分析**：学生画像聚合、学习路径事件日志、可扩展维度记录（Skill 可注入自定义指标）
- **安全防护**：提示注入防护（60 关键词 + 14 正则模式）+ PII 脱敏
- **WeKnora**：可选知识库服务集成，支持用户级 Token 映射
- **教师干预**：实时接管与悄悄话注入至活跃会话
- **国际化**：后端通过 `Accept-Language` Header 返回本地化消息（zh-CN、en-US）
- **技能插件**：8 个可插拔技能，基于 Manifest 驱动的元数据 + 自定义前端渲染器

## 文档

角色化操作手册位于 [`docs/manuals/`](docs/manuals/) 目录下：

- [系统管理员操作手册](docs/manuals/SYS_ADMIN_MANUAL.md)
- [学校管理员操作手册](docs/manuals/SCHOOL_ADMIN_MANUAL.md)
- [教师操作手册](docs/manuals/TEACHER_MANUAL.md)
- [学生操作手册](docs/manuals/STUDENT_MANUAL.md)

技术文档：

- [技能系统重构总结](docs/SKILL_REFACTORING_SUMMARY.md)
- [技能优化分析](docs/SKILL_OPTIMIZATION.md)
- [技能重构指南](docs/SKILL_REFACTORING_GUIDE.md)
- [技能性能指南](docs/SKILL_PERFORMANCE_GUIDE.md)
- [技能监控指南](docs/SKILL_MONITORING.md)

前端应用内帮助中心可通过 `/help` 路径访问。

## 许可证

保留所有权利。
