# AGENTS.md - Hanfledge Developer Guide for AI Agents

## Agent Operating Procedures
- **Understand First:** Before modifying code, use `grep`, `glob`, and `read` tools to explore the repository. Analyze surrounding context, `go.mod`, `package.json`, and related tests.
- **Self-Verification:** Use unit tests, output logs, and debug statements to verify solutions.
- **Path Construction:** Always use absolute paths (e.g., `/home/wuxf/Develop/hanfledge/...`) for file read/write operations.
- **Follow Existing Patterns:** Mimic the styling, structure, and typing of the existing code exactly.

## Project Overview
Hanfledge is an AI-Native EdTech platform with multi-agent orchestration, knowledge graphs, and Socratic learning.
- **Backend:** Go, Gin, GORM, PostgreSQL (pgvector), Neo4j, Redis, Ollama.
- **Frontend:** Next.js 16 (React 19, TypeScript), CSS Modules.

## Architecture
```text
cmd/server/main.go             # Go backend entry point
internal/                      # Go backend source
  domain/model/                # GORM models
  delivery/http/               # Gin router, handlers, middleware (JWT, RBAC)
  usecase/                     # Business logic (KA-RAG pipeline)
  repository/                  # postgres & neo4j clients
frontend/                      # Next.js 16 app
deployments/docker-compose.yml # Dev infrastructure
```

## Build, Lint, & Test Commands

### Go Backend (Run from project root)
```sh
# Build the server
go build -o bin/hanfledge cmd/server/main.go

# Run all tests / run with race detection
go test ./... 
go test -race ./...

# Run a single test (Very important for targeted fixes)
go test ./internal/usecase/ -run TestFunctionName -v

# Lint / Static Analysis
go vet ./...
```

### Frontend (Run from `frontend/` directory)
```sh
# Install dependencies
npm install

# Run the dev server
npm run dev

# Build for production
npm run build

# Run linter
npm run lint
```

## Code Style & Conventions

### Go Backend
- **Imports:** Group by standard library, third-party, and internal (`github.com/hflms/hanfledge/...`). Use named imports for ambiguity.
- **Naming Conventions:** Use `XxxHandler`, `NewXxxHandler`, `XxxEngine`. Use typed `string` constants for Enums (e.g., `UserStatusActive`). GORM models are plain structs.
- **Error Handling:** Wrap errors `fmt.Errorf("...: %w", err)`. Return JSON errors in handlers via `c.JSON(status, gin.H{"error": "user-facing Chinese message"})`.
- **Struct Tags:** Always include both `gorm:"..."` and `json:"..."` tags on models. Use `json:"-"` for sensitive fields.
- **Logging:** Use `log.Printf` / `log.Fatalf`. Prefix log lines with emoji indicators to provide context.
- **Code Organization:** Group logic using dividers like `// -- Section Name ----------------------------------------`. Handlers must include a doc comment with the HTTP method and path.

### TypeScript Frontend
- **Framework:** Next.js App Router (`src/app/`), strict TypeScript, React Compiler enabled.
- **Styling:** CSS Modules only (`.module.css`). Do NOT use Tailwind or CSS-in-JS.
- **API Handling:** All API calls must use `apiFetch<T>()` wrapper in `src/lib/api.ts`. JWT tokens are stored in `localStorage` under `hanfledge_token`.
- **Naming Conventions:** PascalCase for components and interfaces (no `I` prefix). camelCase for API functions.

## Router Architecture & Dependency Rules (Backend)
- `internal/delivery/http/router.go` uses `RouterDeps` as a top-level dependency bag during construction.
- `registerXxxRoutes` functions must **NOT** receive `RouterDeps`. Pass only what is specifically needed (e.g., `*gin.RouterGroup`, `*gorm.DB`, specific handlers).
- To add a new domain, create a `routes_<domain>.go` file and an associated `registerXxxRoutes` function. Keep endpoints grouped by domain.
- Infrastructure dependencies should be injected into the relevant handler constructor in `NewRouter`, and the handler owns it internally.

## Key Technical Details
- **Authentication:** JWT (HS256) Bearer tokens; Roles include SYS_ADMIN, SCHOOL_ADMIN, TEACHER, STUDENT.
- **Infrastructure Services:** PostgreSQL (5433:5432), Neo4j (7475:7474), Redis (6381:6379).
- **AI Stack:** `bge-m3` for embeddings (1024-dim), `qwen2.5:7b` for chat inference, running via Ollama.
