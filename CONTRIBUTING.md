# Contributing to Hanfledge

Hanfledge is an AI-native EdTech platform built for K-12 classrooms in
China. Contributions are welcome from developers, educators, and
researchers who share our commitment to improving education through
technology.

This guide covers the contribution workflow, coding standards, and
review process to help you get started.

## Prerequisites

Before contributing, make sure you have the following tools installed:

- **Go** 1.25 or later
- **Node.js** 22 or later (with npm)
- **Docker** and **Docker Compose** (for infrastructure services)
- **Git** 2.30 or later

For a complete development environment setup, see the
[Development Setup Guide](docs/development-setup.md).

## Getting started

Follow these steps to set up your local development environment:

1. Fork the repository on GitHub.
2. Clone your fork locally:
   ```sh
   git clone https://github.com/<your-username>/hanfledge.git
   cd hanfledge
   ```
3. Start the infrastructure services:
   ```sh
   docker compose -f deployments/docker-compose.yml up -d
   ```
4. Run the backend:
   ```sh
   go run cmd/server/main.go
   ```
5. Run the frontend in a separate terminal:
   ```sh
   cd frontend
   npm install
   npm run dev
   ```
6. Verify everything works by visiting `http://localhost:3000`.

## Contribution workflow

Hanfledge uses a branch-based workflow with merge queue protection on
the `main` branch. Direct pushes to `main` are blocked.

Follow these steps to submit a contribution:

1. Create a feature branch from `main`:
   ```sh
   git checkout -b feat/your-feature-name
   ```
2. Make your changes, following the coding standards below.
3. Run all verification checks before committing:
   ```sh
   # Backend
   go build ./...
   go vet ./...
   go test ./...

   # Frontend (from frontend/ directory)
   npm run lint
   npm run build
   npx vitest run
   ```
4. Commit your changes with a clear, descriptive message.
5. Push your branch and open a pull request against `main`.
6. Address any review feedback promptly.

## Branch naming conventions

Use these prefixes for branch names:

| Prefix | Purpose | Example |
|--------|---------|---------|
| `feat/` | New features | `feat/quiz-analytics` |
| `fix/` | Bug fixes | `fix/session-timeout` |
| `test/` | Test additions or improvements | `test/handler-coverage` |
| `docs/` | Documentation changes | `docs/api-reference` |
| `refactor/` | Code refactoring (no behavior change) | `refactor/handler-deps` |
| `chore/` | Build, CI, or tooling changes | `chore/ci-pipeline` |

## Commit message format

Write commit messages in English. Use the conventional commit format:

```
<type>: <short description>

<optional body explaining the "why">
```

**Types:** `feat`, `fix`, `test`, `docs`, `refactor`, `chore`, `perf`

Examples:

```
feat: add real-time student monitoring dashboard

Adds a live monitoring view for teachers to track active student
sessions, including alert detection and session status indicators.
```

```
fix: correct GORM column name mapping for KPIDS field

The KPIDS model field maps to column "kp_id_s" via GORM naming
strategy, not "kpids".
```

## Coding standards

### Go backend

Refer to [AGENTS.md](AGENTS.md) for the complete Go coding
conventions. Key points include:

- **Imports:** Group by standard library, third-party, and internal
  packages (`github.com/hflms/hanfledge/...`).
- **Error handling:** Wrap errors with `fmt.Errorf("context: %w", err)`.
  Return JSON errors in handlers via
  `c.JSON(status, gin.H{"error": "user-facing Chinese message"})`.
- **Struct tags:** Always include both `gorm:"..."` and `json:"..."` tags
  on models. Use `json:"-"` for sensitive fields.
- **Naming:** Use `XxxHandler` for handler structs, `NewXxxHandler` for
  constructors, and typed `string` constants for enums.
- **Router architecture:** `registerXxxRoutes` functions must not receive
  `RouterDeps`. Pass only the specific dependencies needed.
- **Doc comments:** Every exported handler method must include a comment
  with the HTTP method and path.

### TypeScript frontend

- **Framework:** Next.js App Router (`src/app/`), strict TypeScript,
  React Compiler enabled.
- **Styling:** CSS Modules only (`.module.css`). Do not use Tailwind or
  CSS-in-JS libraries.
- **API calls:** All API calls must use the `apiFetch<T>()` wrapper in
  `src/lib/api.ts`.
- **Naming:** PascalCase for components and interfaces (no `I` prefix).
  camelCase for API functions.

## Testing

Hanfledge uses Go's standard `testing` package for backend tests and
Vitest with React Testing Library for frontend tests.

### Backend tests

Run all backend tests from the project root:

```sh
go test ./...
go test -race ./...               # With race detection
go test ./internal/usecase/ -run TestFunctionName -v  # Single test
```

Backend test infrastructure provides:

- `setupTestDB(t)` for in-memory SQLite with auto-migration
- Seed helpers: `seedUser`, `seedCourse`, `seedChapter`, `seedKP`,
  `seedActivity`, `seedSession`
- Assertion helpers: `assertStatus`, `assertBodyContains`,
  `assertCSVResponse`

### Frontend tests

Run frontend tests from the `frontend/` directory:

```sh
npx vitest run              # Run all tests
npx vitest run --watch      # Watch mode
npx vitest run src/lib/     # Run tests in a specific directory
```

Frontend tests use `vi.fn()` and `vi.mock()` for mocking. See existing
test files in `src/lib/` for patterns.

For a comprehensive testing guide, see
[Testing Guide](docs/testing-guide.md).

## Pull request guidelines

When submitting a pull request:

- Write a clear title using the same format as commit messages.
- Include a summary explaining what changed and why.
- Reference any related issues (for example, `Closes #42`).
- Ensure all CI checks pass (Go test, vet, build; frontend lint, test,
  build).
- Keep pull requests focused. Large changes are harder to review and
  more likely to introduce conflicts.
- Add tests for new functionality. Bug fixes must include a regression
  test.

## Code review

All pull requests require at least one review before merging. Reviewers
focus on:

- **Correctness:** Does the code do what it claims?
- **Tests:** Are new features and bug fixes covered by tests?
- **Style:** Does the code follow the project's coding standards?
- **Security:** Are there any injection, auth bypass, or data exposure
  risks?
- **Performance:** Are there unnecessary database queries, memory
  allocations, or blocking operations?

## Reporting issues

When reporting a bug:

1. Search existing issues to avoid duplicates.
2. Include a clear title and description.
3. Provide steps to reproduce the issue.
4. Include relevant logs, error messages, or screenshots.
5. Specify your environment (OS, Go version, Node.js version, browser).

## License

By contributing to Hanfledge, you agree that your contributions are
licensed under the same license as the project.
