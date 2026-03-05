# Hanfledge

AI-Native EdTech platform with multi-agent orchestration, knowledge graphs,
and Socratic learning. Built for K-12 classrooms in China.

Teachers upload course materials, and the system automatically extracts a
knowledge graph. Students learn through AI-guided Socratic dialogues where
scaffolding adapts in real time based on Bayesian Knowledge Tracing (BKT).

## Architecture

```
                    ┌─────────────────────────────────┐
                    │          Nginx (prod)            │
                    │    /api/* → backend:8080         │
                    │    /*     → frontend:3000        │
                    └────────┬───────────┬────────────┘
                             │           │
              ┌──────────────┘           └──────────────┐
              ▼                                         ▼
┌──────────────────────┐                 ┌──────────────────────┐
│   Go Backend (Gin)   │                 │  Next.js Frontend    │
│                      │                 │  (App Router)        │
│  JWT Auth + RBAC     │                 │                      │
│  KA-RAG Pipeline     │                 │  Admin Dashboard     │
│  Agent Orchestrator  │                 │  Teacher Dashboard   │
│  WebSocket Streaming │                 │  Student Dialogue    │
│  Teacher Intervention│                 │  ECharts Analytics   │
│  WeKnora Integration │                 │  Help Center         │
└──┬────┬────┬────┬──┬─┘                 └──────────────────────┘
   │    │    │    │  │
   ▼    ▼    ▼    ▼  ▼
┌────┐┌────┐┌────┐┌────────┐┌─────────┐
│ PG ││Neo4││Redis││ Ollama ││ WeKnora │
│    ││j   ││     ││        ││ (opt.)  │
│pgv.││    ││     ││qwen2.5 ││ KB svc  │
└────┘└────┘└────┘└────────┘└─────────┘
```

**Backend** (Go 1.25): Gin HTTP framework, GORM ORM, JWT authentication with
role-based access control (SYS_ADMIN, SCHOOL_ADMIN, TEACHER, STUDENT). Includes
teacher real-time intervention (takeover & whisper), dynamic AI provider
configuration, and optional WeKnora knowledge base integration.

**Frontend** (Next.js 16): React 19 with App Router, CSS Modules, ECharts for
analytics dashboards, WebSocket for real-time AI dialogue streaming. Features
an admin dashboard, dynamic AI settings page, in-app help center with
role-based user manuals, and i18n support (zh-CN / en-US).

**AI Pipeline**: Multi-agent orchestration (Strategist -> Designer -> Coach ->
Critic) with KA-RAG (Knowledge-Augmented Retrieval-Augmented Generation),
RRF hybrid retrieval (pgvector + Neo4j), BKT-driven scaffold fading, and
teacher whisper injection for real-time pedagogical intervention.

**Skills (Plugins)**: Pluggable skill system with 8 built-in skills — Socratic
Questioning, Quiz Generation, Role Play, Fallacy Detective, Error Diagnosis,
Cross-Disciplinary, Learning Survey, and Presentation Generator.

## Prerequisites

- Go 1.25+
- Node.js 22+
- Docker and Docker Compose
- Ollama with `qwen2.5:7b` and `bge-m3` models

Pull the required Ollama models:

```sh
ollama pull qwen2.5:7b
ollama pull bge-m3
```

## Quick start

### 1. Start infrastructure

```sh
docker compose -f deployments/docker-compose.yml up -d
```

This starts PostgreSQL (pgvector), Neo4j, and Redis with the following ports:

| Service    | Image                  | Host port | Purpose          |
|------------|------------------------|-----------|------------------|
| PostgreSQL | pgvector/pgvector:pg16 | 5433      | Data + vectors   |
| Neo4j      | neo4j:5-community      | 7475/7688 | Knowledge graph  |
| Redis      | redis:7-alpine         | 6381      | Cache / sessions |

To also start the optional WeKnora knowledge base service:

```sh
docker compose -f deployments/docker-compose.yml --profile weknora up -d
```

| Service    | Image                             | Host port | Purpose             |
|------------|-----------------------------------|-----------|---------------------|
| WeKnora    | wechatopenai/weknora-app:latest   | 9380      | Knowledge base svc  |
| DocReader  | wechatopenai/weknora-docreader    | 50051     | Document parser     |

### 2. Configure environment

```sh
cp .env.example .env
```

Edit `.env` to set your JWT secret and verify database credentials. The
defaults work with the Docker Compose setup above.

Key configuration options:

| Variable              | Default           | Description                        |
|-----------------------|-------------------|------------------------------------|
| `LLM_PROVIDER`        | `ollama`          | `ollama`, `dashscope`, or `gemini` |
| `EMBEDDING_PROVIDER`  | `ollama`          | `ollama` or `dashscope`            |
| `WEKNORA_ENABLED`     | `false`           | Enable WeKnora KB integration      |

### 3. Run the backend

```sh
go run cmd/server/main.go
```

The server starts on `http://localhost:8080`. It automatically runs database
migrations and creates default roles on first startup.

### 4. Seed test data

```sh
go run scripts/seed.go
```

This creates a test school, classes, teachers, and students:

| Role               | Phone        | Password   |
|--------------------|--------------|------------|
| SYS_ADMIN          | 13800000001  | admin123   |
| TEACHER (+ admin)  | 13800000010  | teacher123 |
| TEACHER            | 13800000011  | teacher123 |
| STUDENT (class 1)  | 13800000100  | student123 |
| STUDENT (class 2)  | 13800000105  | student123 |

### 5. Run the frontend

```sh
cd frontend
npm install
npm run dev
```

Opens on `http://localhost:3000`. Log in with any test account above.

## Production deployment

A full-stack Docker Compose configuration is provided for production:

```sh
docker compose -f deployments/docker-compose.prod.yml up -d
```

This runs 7 services: Nginx reverse proxy, Go backend, Next.js frontend,
PostgreSQL, Neo4j, Redis. Nginx handles routing (`/api/*` to backend,
everything else to frontend) and WebSocket upgrades for session streaming.

## Project structure

```
cmd/server/main.go                 # Entry point
internal/
  config/                          # Environment-based configuration
  domain/model/                    # GORM models (user, knowledge, session, skill, weknora)
  delivery/http/                   # Gin router, handlers, middleware (JWT, RBAC)
  usecase/                         # Business logic (KA-RAG pipeline)
  repository/postgres/             # PostgreSQL connection + migrations
  repository/neo4j/                # Neo4j graph client
  infrastructure/llm/              # LLM clients (Ollama, DashScope, dynamic provider)
  infrastructure/safety/           # Prompt injection guard, PII redactor
  infrastructure/weknora/          # WeKnora KB client + token manager
  infrastructure/cache/            # Redis cache layer
  infrastructure/i18n/             # Internationalization (zh-CN, en-US)
  infrastructure/asr/              # Speech-to-text (ASR) provider
  agent/                           # Multi-agent orchestrator (4-agent pipeline)
  agent/orchestrator_intervention  # Teacher whisper & takeover handling
plugins/skills/                    # 8 pluggable skill definitions
  socratic-questioning/            #   Socratic dialogue
  quiz-generation/                 #   Auto quiz from knowledge points
  role-play/                       #   Role-play scenarios
  fallacy-detective/               #   Logical fallacy identification
  error-diagnosis/                 #   Error analysis & remediation
  cross-disciplinary/              #   Cross-subject connections
  learning-survey/                 #   Learning diagnostic survey
  presentation-generator/          #   Slide deck generation
frontend/                          # Next.js 16 app (React 19, TypeScript)
  src/app/admin/                   #   Admin dashboard (schools, classes, users)
  src/app/teacher/                 #   Teacher dashboard, settings, WeKnora
  src/app/student/                 #   Student activities & sessions
  src/app/help/                    #   In-app help center
  src/lib/plugin/                  #   Skill renderers (Socratic, Quiz, Survey, etc.)
locales/                           # i18n message bundles (zh-CN, en-US)
docs/
  manuals/                         # Role-based user manuals
  swagger.yaml                     # OpenAPI / Swagger specification
deployments/
  docker-compose.yml               # Dev infrastructure (+ optional WeKnora profile)
  docker-compose.prod.yml          # Production full-stack
  nginx.conf                       # Reverse proxy config
scripts/seed.go                    # Test data seeder
```

## API reference

All endpoints use the `/api/v1/` prefix. Authentication uses Bearer JWT tokens.
Swagger UI is available at `/swagger/index.html` in dev mode.

### Public

| Method | Path               | Description           |
|--------|--------------------|-----------------------|
| GET    | `/health`          | Liveness check        |
| GET    | `/health/ready`    | Readiness check       |
| POST   | `/api/v1/auth/login` | Login (phone + password) |

### Authenticated (JWT required)

| Method | Path                              | Roles                        | Description                    |
|--------|-----------------------------------|------------------------------|--------------------------------|
| GET    | `/api/v1/auth/me`                 | Any                          | Current user + roles           |
| GET    | `/api/v1/schools`                 | SYS_ADMIN                    | List schools                   |
| POST   | `/api/v1/schools`                 | SYS_ADMIN                    | Create school                  |
| GET    | `/api/v1/classes`                 | SYS_ADMIN, SCHOOL_ADMIN      | List classes                   |
| POST   | `/api/v1/classes`                 | SYS_ADMIN, SCHOOL_ADMIN      | Create class                   |
| GET    | `/api/v1/users`                   | SYS_ADMIN, SCHOOL_ADMIN      | List users                     |
| POST   | `/api/v1/users`                   | SYS_ADMIN, SCHOOL_ADMIN      | Create user                    |
| POST   | `/api/v1/users/batch`             | SYS_ADMIN, SCHOOL_ADMIN      | Batch create users             |
| GET    | `/api/v1/courses`                 | TEACHER+                     | List courses (filtered by teacher) |
| POST   | `/api/v1/courses`                 | TEACHER+                     | Create course                  |
| POST   | `/api/v1/courses/:id/materials`   | TEACHER+                     | Upload PDF, trigger KA-RAG     |
| GET    | `/api/v1/courses/:id/outline`     | TEACHER+                     | Course outline (chapters + KPs)|
| GET    | `/api/v1/courses/:id/documents`   | TEACHER+                     | Document processing status     |
| POST   | `/api/v1/courses/:id/search`      | TEACHER+                     | Semantic search (pgvector)     |
| GET    | `/api/v1/skills`                  | TEACHER+                     | List available skills          |
| GET    | `/api/v1/skills/:id`              | TEACHER+                     | Skill detail                   |
| POST   | `/api/v1/chapters/:id/skills`     | TEACHER+                     | Mount skill to chapter         |
| PATCH  | `/api/v1/chapters/:id/skills/:mount_id` | TEACHER+              | Update skill config            |
| DELETE | `/api/v1/chapters/:id/skills/:mount_id` | TEACHER+              | Unmount skill                  |
| POST   | `/api/v1/activities`              | TEACHER+                     | Create learning activity       |
| GET    | `/api/v1/activities`              | TEACHER+                     | List activities                |
| POST   | `/api/v1/activities/:id/publish`  | TEACHER+                     | Publish activity               |
| POST   | `/api/v1/activities/:id/preview`  | TEACHER+                     | Preview activity (sandbox)     |
| GET    | `/api/v1/activities/:id/sessions` | TEACHER+                     | Activity session analytics     |
| POST   | `/api/v1/activities/:id/join`     | Any                          | Student joins activity         |
| GET    | `/api/v1/sessions/:id`           | Any                          | Get session details            |
| GET    | `/api/v1/sessions/:id/stream`    | Any (WebSocket)              | Real-time AI dialogue stream   |
| POST   | `/api/v1/sessions/:id/intervention` | TEACHER+                  | Teacher intervention (takeover/whisper) |
| GET    | `/api/v1/student/activities`     | STUDENT                      | List available activities      |
| GET    | `/api/v1/student/mastery`        | STUDENT                      | Self mastery data              |
| GET    | `/api/v1/dashboard/knowledge-radar` | TEACHER+                  | Class knowledge radar          |
| GET    | `/api/v1/students/:id/mastery`   | TEACHER+                     | Student mastery details        |
| GET    | `/api/v1/system/config`          | Any                          | Get system configuration       |
| PUT    | `/api/v1/system/config`          | Any                          | Update system configuration    |
| POST   | `/api/v1/system/config/test-chat-model` | Any                    | Test chat model availability   |
| POST   | `/api/v1/system/config/test-embedding-model` | Any              | Test embedding model availability |

### WeKnora integration (optional)

Enabled when `WEKNORA_ENABLED=true`. All require TEACHER+ role.

| Method | Path                                      | Description                     |
|--------|-------------------------------------------|---------------------------------|
| GET    | `/api/v1/weknora/knowledge-bases`         | List remote knowledge bases     |
| GET    | `/api/v1/weknora/knowledge-bases/:kb_id`  | Get KB details                  |
| GET    | `/api/v1/weknora/knowledge-bases/:kb_id/knowledge` | List KB entries        |
| POST   | `/api/v1/courses/:id/weknora-refs`        | Bind KB to course               |
| GET    | `/api/v1/courses/:id/weknora-refs`        | List bound KBs for course       |
| DELETE | `/api/v1/courses/:id/weknora-refs/:ref_id`| Unbind KB from course           |
| POST   | `/api/v1/courses/:id/weknora-search`      | Search within bound KBs         |

### WebSocket protocol

Connect to `/api/v1/sessions/:id/stream` with a Bearer token (via query
parameter or header).

**Client events:**

```json
{"type": "user_message", "content": "What is photosynthesis?"}
```

**Server events:**

| Event                | Description                             |
|----------------------|-----------------------------------------|
| `agent_thinking`     | Agent pipeline stage update             |
| `token_delta`       | Streaming token from Coach LLM          |
| `ui_scaffold_change` | Scaffold level change (high/medium/low) |
| `turn_complete`      | AI turn finished                        |

## Development

### Build and test

```sh
# Build
go build -o bin/hanfledge cmd/server/main.go

# Run all tests
go test ./...

# Run tests with race detection
go test -race ./...

# Run E2E tests only
go test ./internal/delivery/http/ -v -timeout=60s

# Run benchmarks
go test ./internal/delivery/http/ -run '^$' -bench=. -benchmem -benchtime=3s

# Static analysis
go vet ./...
```

### Frontend

```sh
cd frontend
npm run dev       # Development server
npm run build     # Production build
npm run lint      # ESLint check
npm run test:run  # Unit tests (Vitest)
```

### Performance benchmarks

Measured on AMD Ryzen 9 5900HS (16 threads), PostgreSQL via Docker:

| Benchmark       | ops       | latency    | memory/op  | allocs/op |
|-----------------|-----------|------------|------------|-----------|
| Login           | 6,715     | 532 us     | 36 KB      | 314       |
| GetMe           | 3,688     | 903 us     | 68 KB      | 727       |
| ListCourses     | 2,941     | 1,370 us   | 173 KB     | 999       |
| HealthCheck     | 1,277,374 | 2.8 us     | 7.6 KB     | 47        |

Concurrent stress test: 50 workers x 20 requests = 1,155 req/s with 0% error
rate.

## Key technical details

- **Auth**: JWT (HS256) with Bearer tokens; RBAC via `UserSchoolRole` join table
- **Embedding**: bge-m3 (1024-dim vectors) via Ollama, configurable per provider
- **Chat model**: qwen2.5:7b via Ollama, configurable per provider (DashScope supported)
- **Dynamic AI config**: Chat and embedding providers can be configured independently at runtime via the admin settings UI
- **DashScope**: OpenAI-compatible chat base URL via `DASHSCOPE_COMPAT_BASE_URL` when needed
- **Vector search**: pgvector cosine similarity in PostgreSQL
- **Knowledge graph**: Neo4j for concept relationships and prerequisite chains
- **Retrieval**: RRF hybrid (pgvector semantic Top-50 + Neo4j graph Top-50 -> Top-10)
- **Mastery tracking**: Bayesian Knowledge Tracing (BKT) with scaffold fading
- **Safety**: Prompt injection guard (60 keywords + 14 regex patterns) + PII redactor
- **WeKnora**: Optional knowledge base service integration with per-user token mapping
- **Teacher intervention**: Real-time takeover and whisper injection into active sessions
- **i18n**: Backend returns localized messages (zh-CN, en-US) via `Accept-Language` header
- **Skills**: 8 pluggable skills with manifest-driven metadata and custom frontend renderers

## Documentation

Role-based user manuals are available under [`docs/manuals/`](docs/manuals/):

- [System Administrator Manual](docs/manuals/SYS_ADMIN_MANUAL.md)
- [School Administrator Manual](docs/manuals/SCHOOL_ADMIN_MANUAL.md)
- [Teacher Manual](docs/manuals/TEACHER_MANUAL.md)
- [Student Manual](docs/manuals/STUDENT_MANUAL.md)

An in-app help center is also accessible from the frontend at `/help`.

## License

All rights reserved.
