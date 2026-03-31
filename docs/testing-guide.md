# Testing guide

Hanfledge uses Go's standard `testing` package for backend tests and
Vitest with React Testing Library for frontend tests. This guide
explains the test strategy, infrastructure, patterns, and commands
for running tests.

## Test strategy

The test suite focuses on three layers:

| Layer | Tool | Scope |
|-------|------|-------|
| Backend handlers | Go `testing` + SQLite | HTTP handler behavior with mocked DB |
| Backend middleware | Go `testing` | JWT validation, RBAC checks |
| Frontend units | Vitest + RTL | Components, hooks, API client, utilities |

Integration tests and end-to-end tests exist in limited scope. The
primary coverage focus is on handler and middleware logic.

## Backend testing

### Running tests

Run all backend tests from the project root:

```sh
# Run all tests
go test ./...

# Run with race detection
go test -race ./...

# Run a specific package
go test ./internal/delivery/http/handler/ -count=1

# Run a single test function
go test ./internal/usecase/ -run TestFunctionName -v

# Run with verbose output
go test ./internal/delivery/http/handler/ -v -count=1
```

The `-count=1` flag disables test caching, which is useful when
debugging.

### Test infrastructure

The handler test suite uses a shared test helper file at
`internal/delivery/http/handler/testhelper_test.go`. This file
provides:

#### Database setup

```go
func setupTestDB(t *testing.T) *gorm.DB
```

Creates an in-memory SQLite database with AutoMigrate for 17+ model
types. Use this in every test that needs database access:

```go
func TestMyHandler(t *testing.T) {
    db := setupTestDB(t)
    h := handler.NewMyHandler(db)
    // ... test logic
}
```

> **Note:** Some models (`Notification`, `SoulVersion`,
> `SystemConfig`) are not included in the shared `setupTestDB`
> helper. If your handler uses these models, add
> `db.AutoMigrate(&model.YourModel{})` in your test setup.

#### Seed helpers

Pre-built functions to populate test data:

| Function | Creates | Returns |
|----------|---------|---------|
| `seedUser(t, db, phone, name)` | User with password hash | `*model.User` |
| `seedCourse(t, db, teacherID, schoolID)` | Course with "draft" status | `*model.Course` |
| `seedChapter(t, db, courseID, title)` | Chapter in a course | `*model.Chapter` |
| `seedKP(t, db, chapterID, title)` | Knowledge point in a chapter | `*model.KnowledgePoint` |
| `seedActivity(t, db, courseID, teacherID)` | Learning activity | `*model.LearningActivity` |
| `seedSession(t, db, studentID, activityID, kpID)` | Student session | `*model.StudentSession` |

#### Gin test context creators

Create Gin test contexts for handler method calls:

```go
// Basic context with HTTP method and path
w, c := newTestContext("GET", "/test")

// Context with URL parameters
w, c := newTestContextWithParams("GET", "/test/:id",
    gin.Param{Key: "id", Value: "42"})

// Context with query parameters
w, c := newTestContextWithQuery("GET", "/test", "page=2&limit=10")
```

All context creators inject a `user_id` of `1` into the Gin context
to simulate an authenticated user.

#### Assertion helpers

| Function | Description |
|----------|-------------|
| `assertStatus(t, w, code)` | Assert HTTP status code |
| `assertBodyContains(t, w, substr)` | Assert response body contains a substring |
| `assertBodyNotContains(t, w, substr)` | Assert response body does not contain a substring |
| `assertHeader(t, w, key, value)` | Assert response header value |
| `assertContentType(t, w, ct)` | Assert Content-Type header |
| `assertCSVResponse(t, w)` | Assert response is valid CSV |

> **Warning:** `assertBodyNotContains` uses substring matching. Be
> careful with names that are substrings of other names (for example,
> `"read1"` matches inside `"unread1"`). Use unique, non-overlapping
> test data names.

### Test file conventions

- Test files are co-located with their source files using the
  `_test.go` suffix.
- Test functions follow Go conventions: `TestXxx(t *testing.T)`.
- Use table-driven tests for exhaustive case coverage.
- Each test function must call `setupTestDB(t)` to get a fresh
  database.

### Writing a new handler test

Follow this pattern to add a new handler test:

```go
package handler

import (
    "net/http"
    "testing"

    "github.com/hflms/hanfledge/internal/domain/model"
)

func TestMyHandler_GetItems_Empty(t *testing.T) {
    db := setupTestDB(t)
    h := NewMyHandler(db)

    w, c := newTestContext("GET", "/items")
    h.GetItems(c)

    assertStatus(t, w, http.StatusOK)
    assertBodyContains(t, w, `"items":[]`)
}

func TestMyHandler_GetItems_WithData(t *testing.T) {
    db := setupTestDB(t)
    h := NewMyHandler(db)

    // Seed test data
    user := seedUser(t, db, "13800000001", "Test User")
    // ... create items

    w, c := newTestContext("GET", "/items")
    c.Set("user_id", user.ID)
    h.GetItems(c)

    assertStatus(t, w, http.StatusOK)
    assertBodyContains(t, w, `"total":`)
}
```

### Middleware tests

Middleware tests create a full Gin engine with the middleware under
test and use `httptest.NewRecorder()`:

```go
func TestMyMiddleware(t *testing.T) {
    r := gin.New()
    r.Use(MyMiddleware(param))
    r.GET("/test", func(c *gin.Context) {
        c.JSON(200, gin.H{"ok": true})
    })

    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/test", nil)
    r.ServeHTTP(w, req)

    if w.Code != http.StatusOK {
        t.Errorf("expected 200, got %d", w.Code)
    }
}
```

### Current backend test coverage

| Package | Test files | Tests |
|---------|-----------|-------|
| `handler/` | 10 | ~217 |
| `middleware/` | 2 | 22 |
| `usecase/` | 1 | varies |
| `safety/` | 2 | varies |

## Frontend testing

### Running tests

Run all frontend tests from the `frontend/` directory:

```sh
# Run all tests
npx vitest run

# Run in watch mode (re-runs on file changes)
npx vitest --watch

# Run tests in a specific directory
npx vitest run src/lib/

# Run a specific test file
npx vitest run src/lib/api.test.ts

# Generate coverage report
npx vitest run --coverage
```

### Configuration

The Vitest configuration is at `frontend/vitest.config.ts`. Key
settings include:

- **Environment:** `jsdom` (simulates browser APIs)
- **Setup files:** `@testing-library/jest-dom/vitest` for DOM
  matchers, `fake-indexeddb/auto` for IndexedDB simulation
- **Globals:** Enabled (`describe`, `it`, `expect` are global)
- **CSS:** `{ modules: { classNameStrategy: 'non-scoped' } }` for
  CSS Module handling

### Testing patterns

#### Mocking functions

Use `vi.fn()` for mock functions:

```typescript
const mockHandler = vi.fn();
// ... trigger the handler
expect(mockHandler).toHaveBeenCalledWith('expected-arg');
```

#### Mocking modules

Use `vi.mock()` for module-level mocking:

```typescript
vi.mock('./api', () => ({
  apiFetch: vi.fn(),
}));
```

#### Mocking fetch

For API tests, mock the global `fetch` function:

```typescript
function mockFetchResponse(data: unknown, ok = true) {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
    ok,
    status: ok ? 200 : 500,
    json: () => Promise.resolve(data),
  }));
}

afterEach(() => {
  vi.restoreAllMocks();
});
```

#### Component tests

Use React Testing Library for component rendering:

```typescript
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import MyComponent from './MyComponent';

test('renders heading', () => {
  render(<MyComponent title="Hello" />);
  expect(screen.getByText('Hello')).toBeInTheDocument();
});

test('handles click', async () => {
  const user = userEvent.setup();
  const onClick = vi.fn();
  render(<MyComponent onClick={onClick} />);
  await user.click(screen.getByRole('button'));
  expect(onClick).toHaveBeenCalled();
});
```

#### Hook tests

Use `renderHook` for testing custom hooks:

```typescript
import { renderHook, act } from '@testing-library/react';
import { useMyHook } from './useMyHook';

test('returns initial state', () => {
  const { result } = renderHook(() => useMyHook());
  expect(result.current.value).toBe(0);
});
```

### Test file conventions

- Test files use the `.test.ts` or `.test.tsx` extension.
- Test files are co-located with their source files.
- Each test file imports the module under test directly.
- Use `describe` blocks to group related tests.
- Use `beforeEach`/`afterEach` for setup and teardown.
- Always call `vi.restoreAllMocks()` in `afterEach` to prevent mock
  leakage.

### Current frontend test coverage

| Directory | Test files | Tests |
|-----------|-----------|-------|
| `src/lib/` | 5 | 49 |
| `src/components/` | 5 | 16 |
| `src/app/student/session/` | 3 | 13 |
| **Total** | **16** | **94** |

## CI pipeline

The GitHub Actions CI pipeline runs on every push and pull request.
It executes the following checks:

1. `go test ./...` -- All backend tests
2. `go vet ./...` -- Go static analysis
3. `go build ./...` -- Build verification
4. `npm run lint` -- Frontend ESLint
5. `npx vitest run` -- Frontend tests
6. `npm run build` -- Frontend build verification

All checks must pass before a pull request can be merged.

## Areas for future testing

The following areas have limited or no test coverage and are
candidates for future work:

| Area | Status | Notes |
|------|--------|-------|
| `internal/repository/postgres/` | 0 tests | 10 repository implementation files |
| `internal/domain/model/` | 0 tests | Model validation and methods |
| `internal/usecase/` | Limited | KA-RAG pipeline, BKT engine |
| `internal/agent/` | 0 tests | Agent orchestrator, designer, coach, critic |
| Frontend pages | 1 of 26 | Only student session page has tests |
| Frontend components | 5 of 19 | Most shared components lack tests |
| WebSocket handler | 0 tests | Requires WebSocket test infrastructure |
| End-to-end | 0 tests | Full API workflow tests |
