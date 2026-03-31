# Hanfledge – Copilot Coding Agent Instructions

## Project Overview

Hanfledge is an AI-Native EdTech platform featuring multi-agent orchestration, knowledge graphs, and Socratic learning. It has a **Go backend** (Gin + GORM + PostgreSQL/pgvector + Neo4j + Redis + Ollama) and a **Next.js 16 frontend** (React 19, TypeScript, CSS Modules).

---

## Repository Layout

```
hanfledge/
├── cmd/server/main.go             # Backend entry point
├── internal/
│   ├── agent/                     # Multi-agent pipeline (Strategist, Designer, Coach, Critic)
│   ├── config/config.go           # Config loader & validation
│   ├── delivery/http/             # Gin router, handlers, middleware (JWT, RBAC)
│   │   ├── router.go              # RouterDeps + route registration
│   │   ├── routes_*.go            # Domain-scoped route files
│   │   ├── handler/               # Handler structs per domain
│   │   └── middleware/            # JWT, RBAC, CORS, i18n
│   ├── domain/model/              # GORM models (User, Course, Session, …)
│   ├── infrastructure/            # External adapters (LLM, cache, safety, search, storage)
│   ├── plugin/                    # Plugin system & event bus
│   ├── repository/                # postgres/ and neo4j/ clients
│   └── usecase/                   # Business logic (KA-RAG pipeline)
├── frontend/
│   ├── src/
│   │   ├── app/                   # Next.js App Router pages (admin, teacher, student, login)
│   │   ├── components/            # Shared React components
│   │   ├── lib/                   # API client, auth store, i18n, a11y helpers
│   │   └── types/                 # TypeScript type definitions
│   ├── package.json               # pnpm workspace; pnpm v9 required
│   └── pnpm-lock.yaml
├── plugins/                       # Built-in skill plugins
├── scripts/                       # dev.sh, seed.go, benchmark scripts
├── deployments/docker-compose.yml # PostgreSQL, Neo4j, Redis, (optional WeKnora)
├── locales/                       # i18n JSON files (zh-CN, en-US)
├── docs/                          # Architecture and user guides
├── go.mod                         # Go 1.25 module (github.com/hflms/hanfledge)
└── .env.example                   # All supported environment variables
```

---

## Build, Lint & Test

### Go Backend (run from project root)

```bash
# Build
go build -o bin/hanfledge cmd/server/main.go

# Run all tests (no live infra needed for unit tests)
go test ./internal/... -v

# Race detector
go test -race ./internal/...

# Run a single named test
go test ./internal/usecase/ -run TestFunctionName -v

# Static analysis
go vet ./cmd/... ./internal/...

# Development server
go run cmd/server/main.go
```

### Frontend (run from `frontend/` directory)

> **Use pnpm v9**, not npm or yarn. The CI workflow uses `pnpm install`.

```bash
cd frontend

# Install dependencies
pnpm install

# Dev server
pnpm dev          # next dev --webpack

# Production build
pnpm build

# Lint (ESLint)
pnpm lint

# Unit tests – watch mode
pnpm test

# Unit tests – single run (used in CI)
pnpm test:run
```

### Full-stack dev (one command)

```bash
bash scripts/dev.sh            # starts infra + backend + frontend
bash scripts/dev.sh --seed     # also seeds test data
bash scripts/dev.sh --backend-only
bash scripts/dev.sh --frontend-only
```

### Docker infrastructure

```bash
docker compose -f deployments/docker-compose.yml up -d   # PostgreSQL, Neo4j, Redis
```

---

## CI Workflow

`.github/workflows/ci.yml` runs on push/PR to `main` and `develop`:

| Job | Steps |
|-----|-------|
| **backend** | `go mod download` → `go test ./internal/... -v` → `go vet` → `go build` |
| **frontend** | `pnpm install` → `pnpm lint` → `pnpm test:run` → `pnpm build` |

Go version in CI: **1.23** (note: `go.mod` declares `go 1.25.0`; local dev requires Go ≥ 1.25). Node version: **22**. pnpm version: **9**.

---

## Infrastructure Services

| Service | Port (host:container) | Purpose |
|---------|----------------------|---------|
| PostgreSQL (ParadeDB/pgvector) | 5433:5432 | Primary DB + vector search |
| Neo4j | 7475:7474 (HTTP), 7688:7687 (bolt) | Knowledge graph |
| Redis | 6381:6379 | Session/mastery cache |
| Ollama | 11434 | Local LLM (`qwen2.5:7b`) and embeddings (`bge-m3`, 1024-dim) |
| DashScope (Qwen Cloud) | — | Optional cloud LLM |
| Gemini | — | Optional cloud LLM |
| SearXNG | 8888 | Web search fallback |
| Whisper API | 9000 | Speech-to-text |
| Aliyun OSS | — | Optional cloud storage |
| WeKnora | 9380 / 9381 | Optional external knowledge base |

---

## Key Environment Variables

Copy `.env.example` to `.env` and adjust. Critical vars:

```
SERVER_PORT=8080
GIN_MODE=debug            # use "release" in prod (enforces strong secret checks)
CORS_ORIGINS=http://localhost:3000

DB_HOST=localhost
DB_PORT=5433
DB_USER=hanfledge
DB_PASSWORD=hanfledge_secret
DB_NAME=hanfledge

NEO4J_URI=bolt://localhost:7688
NEO4J_USER=neo4j
NEO4J_PASSWORD=neo4j_secret

REDIS_URL=redis://localhost:6381/0

JWT_SECRET=your-jwt-secret-change-in-production
JWT_EXPIRY_HOURS=24

LLM_PROVIDER=ollama        # ollama | dashscope | gemini
OLLAMA_HOST=http://localhost:11434
OLLAMA_MODEL=qwen2.5:7b
EMBEDDING_MODEL=bge-m3

STORAGE_BACKEND=local      # local | oss
STORAGE_LOCAL_ROOT=uploads

WEKNORA_ENABLED=false
```

> In `GIN_MODE=release` the server validates that `JWT_SECRET`, `DB_PASSWORD`, etc. are **not** the shipped defaults. Startup fails otherwise.

---

## Test Accounts (after `--seed`)

| Role | Phone | Password |
|------|-------|----------|
| SYS_ADMIN | 13800000001 | admin123 |
| TEACHER | 13800000010 | teacher123 |
| STUDENT | 13800000100 | student123 |

---

## Backend Code Conventions

### Imports
Group by: standard library → third-party → internal (`github.com/hflms/hanfledge/...`). Use named imports to resolve ambiguity.

### Naming
- Handlers: `XxxHandler`, constructor `NewXxxHandler`
- Engines / services: `XxxEngine`
- Typed string constants for enums: `UserStatusActive UserStatus = "active"`
- GORM models: plain structs; always include both `gorm:"..."` and `json:"..."` tags; use `json:"-"` for sensitive fields

### Error Handling
- Wrap errors: `fmt.Errorf("context: %w", err)`
- Return JSON from handlers: `c.JSON(status, gin.H{"error": "Chinese user-facing message"})`

### Logging
- Use `log.Printf` / `log.Fatalf`
- Prefix lines with emoji for context (e.g., `🔍`, `✅`, `❌`)

### Code Organization
- Section dividers: `// -- Section Name ----------------------------------------`
- Each handler must have a doc comment showing the HTTP method and path

### Router / Dependency Injection
- `RouterDeps` (in `router.go`) is the **top-level** dependency bag; it is used **only** inside `NewRouter`
- `registerXxxRoutes` functions receive only what they specifically need (e.g., `*gin.RouterGroup`, `*gorm.DB`, specific handler); **never** pass the entire `RouterDeps`
- Add a new domain by creating `routes_<domain>.go` and a matching `registerXxxRoutes` function

### Request Validation
- Use Gin binding validator tags: `` `binding:"required,oneof=..."` ``

### Database
- GORM `AutoMigrate` runs at startup via `postgres.AutoMigrate(db)` (see `cmd/server/main.go`)
- New models must be added to the `AutoMigrate` call

---

## Frontend Code Conventions

### Framework & Tooling
- Next.js 16 App Router (`src/app/`), strict TypeScript (`strict: true`)
- React Compiler enabled
- **CSS Modules only** (`.module.css`) — no Tailwind, no CSS-in-JS

### API Calls
- All API calls must use `apiFetch<T>()` from `src/lib/api.ts`
- JWT stored in `localStorage` under key `hanfledge_token`

### Naming
- PascalCase for components and interfaces (no `I` prefix)
- camelCase for utility/API functions

### Accessibility
- Modals must use the shared `useModalA11y` hook (`src/lib/a11y.ts`)
- Attach the returned `ref` and `tabIndex={-1}` to the `role="dialog"` element
- The hook handles ESC close, focus trap, and focus restore automatically

### Markdown / AI Content
- Use `MarkdownRenderer` component for AI-generated text
- It suppresses custom AI tags (`slides`, `suggestions`, `presentation`, etc.) via `PassthroughTag` in `react-markdown` components

### State Management
- Zustand for global state; SWR for server data fetching
- IndexedDB for offline caching

### Testing
- Unit tests use **Vitest** + `@testing-library/react`
- Test files: `*.test.ts` / `*.test.tsx` co-located with source
- Run with `pnpm test:run` (single pass, used in CI)

---

## Architecture: AI Pipeline

```
Student Input
  └─→ InjectionGuard.Check()           (safety: lowercase + truncate)
  └─→ Strategist agent                 (analyze KP, select focus)
  └─→ Designer agent                   (hybrid retrieval: pgvector RRF + Neo4j)
  └─→ Coach agent                      (Socratic response, LLM streaming)
  └─→ Critic agent                     (review depth/correctness)
  └─→ OutputGuard                      (safety check on response)
  └─→ BKT update                       (mastery tracking, scaffold fade)
  └─→ WebSocket stream → frontend
```

### KA-RAG Pipeline (teacher upload)

```
PDF Upload
  └─→ Parser → Chunker (semantic split)
  └─→ Embedder (bge-m3, 1024-dim → pgvector)
  └─→ Graph Builder (Neo4j concepts + relationships)
  └─→ Outline Generator (course structure)
```

### Hybrid Retrieval (RRF)

```
Query → pgvector top-50 + Neo4j top-50
      → Reciprocal Rank Fusion
      → Top-10 personalized materials
```

### LLM Provider Routing
Three-tier routing (when `LLM_ROUTER_ENABLED=true`):
- **Tier 1** (`LLM_TIER1_MODEL`): Local small model (Ollama)
- **Tier 2** (`LLM_TIER2_MODEL`): Mid-range cloud (e.g., `qwen-plus`)
- **Tier 3** (`LLM_TIER3_MODEL`): Flagship model (e.g., `qwen-max`)

Infra LLM clients keep a reusable `*http.Client` on the struct; do not create a new client per request.

---

## Security & Safety

- `InjectionGuard.Check`: lowercases input, truncates `Matched` to 50 chars + `"..."`
- `cfg.WeKnora.EncryptionKey` is the shared secret for deterministic per-user WeKnora password generation in `TokenManager`
- System configs are exposed via `GET/PUT /api/v1/system/config`; keys are raw `SystemConfig.Key` strings (e.g., `LLM_PROVIDER`)
- Never commit `.env`; never log or return secrets in responses
- Do not introduce `eval`, dynamic command construction, or shell expansion exploits

---

## i18n

- `Locale` is a typed string (`LocaleZhCN` / `LocaleEnUS`); `DefaultLocale = LocaleZhCN`
- Translation files live in `locales/zh-CN/` and `locales/en-US/`
- Add new strings to **both** locale files

---

## Common Gotchas

1. **pnpm, not npm**: The CI and lock-file use pnpm v9. Running `npm install` in `frontend/` will create a `package-lock.json` and break CI.
2. **Port offsets**: Infra services run on non-standard host ports (PostgreSQL → 5433, Neo4j → 7475/7688, Redis → 6381) to avoid conflicts with local installations.
3. **DB migrations are automatic**: Add new GORM models to the `AutoMigrate` slice in `internal/repository/postgres/database.go`.
4. **Release mode secrets**: `GIN_MODE=release` enforces non-default values for `JWT_SECRET`, `DB_PASSWORD`, etc. Tests that spin up the full server must set these env vars.
5. **RouterDeps is for construction only**: `registerXxxRoutes` must **not** accept `RouterDeps`; pass only the specific deps needed.
6. **CSS Modules only in frontend**: Do not add Tailwind or global CSS classes. All styles belong in `.module.css` files.
7. **`apiFetch` wrapper**: All frontend HTTP calls must go through `apiFetch<T>()` in `src/lib/api.ts`; do not use raw `fetch` directly.
8. **Embedding dimensions**: `bge-m3` produces 1024-dim vectors. Any schema or comparison change must match this dimension.
9. **Agent pipeline is stateful**: Each stage passes a `Context` struct; mutations in Strategist are visible to Designer/Coach/Critic.
10. **Go module path**: The module is `github.com/hflms/hanfledge`; all internal imports must use this path.
