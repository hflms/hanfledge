# AGENTS.md - Hanfledge Developer Guide

## Project Overview

Hanfledge is an AI-Native EdTech platform with multi-agent orchestration, knowledge graphs,
and Socratic learning. It consists of a Go backend (Gin + GORM) and a Next.js frontend,
backed by PostgreSQL (pgvector), Neo4j, and Redis, with Ollama for LLM inference.

Module path: `github.com/hflms/hanfledge`

## Architecture

```
cmd/server/main.go             # Entry point
internal/
  config/                      # Env-based config (godotenv)
  domain/model/                # GORM models (user, knowledge, document, session, skill)
  delivery/http/               # Gin router, handlers, middleware (JWT, RBAC)
  usecase/                     # Business logic (KA-RAG pipeline)
  repository/postgres/         # PostgreSQL connection + migrations
  repository/neo4j/            # Neo4j graph client
  infrastructure/llm/          # Ollama LLM client (chat + embedding)
  agent/                       # Multi-agent orchestration (not yet implemented)
frontend/                      # Next.js 16 app (React 19, TypeScript)
deployments/docker-compose.yml # Dev infrastructure (Postgres, Neo4j, Redis)
scripts/seed.go                # Test data seeder
```

## Build & Run Commands

### Prerequisites

Start infrastructure services first:

```sh
docker-compose -f deployments/docker-compose.yml up -d
```

Copy `.env.example` to `.env` and adjust values as needed.

### Go Backend

```sh
# Run the server
go run cmd/server/main.go

# Build
go build -o bin/hanfledge cmd/server/main.go

# Run tests (none exist yet - add them under *_test.go files)
go test ./...

# Run a single test
go test ./internal/usecase/ -run TestFunctionName -v

# Run tests with race detection
go test -race ./...

# Vet / static analysis
go vet ./...
```

### Frontend (Next.js)

All frontend commands run from the `frontend/` directory:

```sh
# Install dependencies
npm install

# Dev server
npm run dev

# Production build
npm run build

# Lint
npm run lint
```

### Seed Data

```sh
go run scripts/seed.go
```

## Code Style - Go

### Imports

Group imports in this order, separated by blank lines:
1. Standard library
2. Third-party packages
3. Internal packages (`github.com/hflms/hanfledge/...`)

Use named imports to resolve ambiguity (e.g., `neo4jRepo`, `delivery`).

### Naming

- Handler structs: `XxxHandler` with constructor `NewXxxHandler`
- Use case structs: `XxxEngine` with constructor `NewXxxEngine`
- Enum types: typed `string` constants (e.g., `type UserStatus string`)
- Enum values: `TypeValueName` pattern (e.g., `UserStatusActive`, `RoleSysAdmin`)
- GORM models: plain structs with `gorm:"..."` and `json:"..."` tags

### Error Handling

- Wrap errors with context: `fmt.Errorf("operation failed: %w", err)`
- Return errors up the call stack; handle at the handler level
- In handlers, return JSON errors using `c.JSON(status, gin.H{"error": "message"})`
- Non-fatal errors: log with `log.Printf` and continue
- Fatal startup errors: use `log.Fatalf`

### Logging

- Use `log.Printf` / `log.Fatalf` (stdlib)
- Prefix log lines with emoji indicators: `"..."`, `"..."`, `"..."`, `"..."`, etc.
- Include context in log messages: `log.Printf("[KA-RAG] Slicing document: %s", doc.FileName)`

### JSON Responses

- Success: `c.JSON(http.StatusOK, data)` or `c.JSON(http.StatusOK, gin.H{"message": "..."})`
- Error: `c.JSON(http.StatusXxx, gin.H{"error": "user-facing message"})`
- User-facing error messages are in Chinese (the target audience)

### Struct Tags

Always include both `gorm` and `json` tags on model fields:
```go
Phone string `gorm:"uniqueIndex;size:20" json:"phone"`
```
Use `json:"-"` for sensitive fields (passwords, reverse associations).
Use `json:"field_name,omitempty"` for optional fields.

### Code Organization

- Use section dividers: `// -- Section Name ----------------------------------------`
- Each handler method has a doc comment with the HTTP method and path:
  ```go
  // Login handles user authentication and returns a JWT token.
  // POST /api/v1/auth/login
  ```
- Request/response types are defined near their handler

### Concurrency

- Use goroutines with `context.Context` for background processing
- Always pass context through the call chain
- Use `context.WithTimeout` for operations with external services

## Code Style - Frontend (TypeScript)

### Framework & Config

- Next.js 16 with App Router (`src/app/` directory)
- React 19 with React Compiler enabled
- TypeScript strict mode
- ESLint: `eslint-config-next` (core-web-vitals + typescript)
- No Prettier config (use defaults)
- Path alias: `@/*` maps to `./src/*`

### Styling

- CSS Modules only (`.module.css` files) -- no CSS-in-JS, no Tailwind
- Co-locate styles with components

### API Client

- All API calls go through `src/lib/api.ts` using the `apiFetch<T>()` wrapper
- JWT tokens stored in `localStorage` under `hanfledge_token`
- Auto-redirect to `/login` on 401 responses

### Naming

- Components: PascalCase filenames and exports
- Pages: `page.tsx` (Next.js App Router convention)
- Interfaces: PascalCase, no `I` prefix (e.g., `Course`, `User`, `LoginResponse`)
- API functions: camelCase verbs (e.g., `listCourses`, `createCourse`, `getMe`)

### Section Dividers

Use the same divider style as the backend:
```ts
// -- Auth API -----------------------------------------------
```

## Infrastructure

| Service    | Image                    | Host Port | Internal Port |
|------------|--------------------------|-----------|---------------|
| PostgreSQL | pgvector/pgvector:pg16   | 5433      | 5432          |
| Neo4j      | neo4j:5-community        | 7475/7688 | 7474/7687     |
| Redis      | redis:7-alpine           | 6381      | 6379          |

## Key Technical Details

- **Auth**: JWT (HS256) with Bearer tokens; RBAC via `UserSchoolRole` join table
- **Roles**: SYS_ADMIN, SCHOOL_ADMIN, TEACHER, STUDENT
- **Embedding**: bge-m3 model (1024-dim vectors) via Ollama
- **Chat model**: qwen2.5:7b via Ollama
- **Vector search**: pgvector extension in PostgreSQL
- **Knowledge graph**: Neo4j for concept relationships
- **API prefix**: `/api/v1/`
