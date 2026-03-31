# Hanfledge frontend

The Hanfledge frontend is a Next.js 16 application built with React 19,
TypeScript, and CSS Modules. It provides the user interface for all four
roles: system admin, school admin, teacher, and student.

## Tech stack

| Technology | Version | Purpose |
|-----------|---------|---------|
| Next.js | 16.1.6 | React framework with App Router |
| React | 19.2.3 | UI library with React Compiler enabled |
| TypeScript | 5.x | Type-safe JavaScript |
| CSS Modules | -- | Scoped component styling (`.module.css`) |
| ECharts | 6.0.0 | Analytics dashboards and data visualization |
| SWR | 2.4.1 | Data fetching and caching |
| Zustand | 5.0.11 | Lightweight state management |
| Vitest | 4.0.18 | Unit and integration testing |

## Project structure

```
frontend/
  src/
    app/                          # Next.js App Router pages
      admin/                      # Admin dashboard
        classes/                  # Class management
        overview/                 # System overview
        schools/                  # School management
        soul/                     # Soul system management
        users/                    # User management
      help/                       # Help center
        [role]/                   # Role-specific help pages
      login/                      # Authentication
      student/                    # Student-facing pages
        achievements/             # Gamification badges
        activities/               # Activity list
        error-notebook/           # Error review notebook
        knowledge-map/            # Knowledge graph visualization
        mastery/                  # Mastery dashboard
        session/[id]/             # AI dialogue session
      teacher/                    # Teacher-facing pages
        activities/[id]/design/   # Activity design editor
        courses/                  # Course management
          [id]/graph/             # Knowledge graph editor
          [id]/materials/         # Document upload
          [id]/outline/           # Course outline editor
        dashboard/                # Teacher analytics dashboard
          session/[id]/           # Session monitoring
        settings/                 # AI provider settings
        skills/                   # Skill management and creation
          create/                 # Custom skill creator
        weknora/                  # WeKnora knowledge base
    components/                   # Shared components
      skill-ui/                   # Skill-specific UI components
      ui/                         # Generic UI primitives (reserved)
    lib/                          # Shared utilities
      api.ts                      # API client (apiFetch<T>)
      useApi.ts                   # React hook for API calls
      constants.ts                # UI constants and label maps
      cache/                      # IndexedDB caching layer
      plugin/                     # Plugin system (parsers, hooks)
        hooks/                    # Plugin React hooks
    styles/                       # Global styles
  public/                         # Static assets
    docs/                         # User manual markdown files
```

## Development

### Prerequisites

- Node.js 22 or later
- The backend server running at `http://localhost:8080`
- Infrastructure services (PostgreSQL, Neo4j, Redis) running via
  Docker Compose

### Getting started

Install dependencies and start the development server:

```sh
npm install
npm run dev
```

The application starts at `http://localhost:3000` with Turbopack for
fast hot module replacement.

### Available scripts

| Command | Description |
|---------|-------------|
| `npm run dev` | Start development server with Turbopack |
| `npm run build` | Build for production |
| `npm run start` | Start production server |
| `npm run lint` | Run ESLint |
| `npx vitest run` | Run all tests |
| `npx vitest --watch` | Run tests in watch mode |

## Coding conventions

### Styling

Hanfledge uses CSS Modules exclusively for component styling. Do not
use Tailwind, styled-components, or other CSS-in-JS solutions.

```tsx
// Correct
import styles from './MyComponent.module.css';

export default function MyComponent() {
  return <div className={styles.container}>...</div>;
}
```

Every component's styles live in a co-located `.module.css` file with
the same name as the component.

### API integration

All API calls must use the `apiFetch<T>()` wrapper defined in
`src/lib/api.ts`. This wrapper handles:

- Automatic Bearer token injection from `localStorage`
- 401 response handling (clears token, redirects to `/login`)
- Content-Type management (skips header for `FormData` uploads)
- Typed responses via TypeScript generics

```tsx
import { apiFetch, PaginatedResponse } from '@/lib/api';

interface Course {
  id: number;
  title: string;
}

const data = await apiFetch<PaginatedResponse<Course>>('/courses');
```

For React components that need data fetching with loading and error
states, use the `useApi` hook:

```tsx
import { useApi } from '@/lib/useApi';

function CourseList() {
  const { data, error, isLoading } = useApi<Course[]>('/courses');

  if (isLoading) return <p>Loading...</p>;
  if (error) return <p>Error: {error.message}</p>;

  return <ul>{data?.map(c => <li key={c.id}>{c.title}</li>)}</ul>;
}
```

### Naming conventions

- **Components:** PascalCase (`CourseOutline.tsx`)
- **Interfaces and types:** PascalCase, no `I` prefix
  (`Course`, not `ICourse`)
- **API functions:** camelCase (`getCourses`, `createActivity`)
- **CSS Module classes:** camelCase (`.containerWrapper`)
- **File names:** Match the exported component name

### State management

The application uses a layered approach to state:

- **Server state:** SWR for API data fetching and caching
- **Client state:** Zustand for cross-component client state
- **Local state:** React `useState`/`useReducer` for component-local
  state
- **URL state:** Next.js router params for navigation state

### Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `NEXT_PUBLIC_API_URL` | `http://localhost:8080/api/v1` | Backend API base URL |

Create a `.env.local` file for local overrides. See `.env.example`
for all available variables.

## Testing

Frontend tests use Vitest with React Testing Library. Test files are
co-located with the source files using the `.test.ts` or `.test.tsx`
extension.

```sh
npx vitest run                     # Run all tests
npx vitest run src/lib/            # Run tests in a directory
npx vitest run --coverage          # Generate coverage report
```

### Testing patterns

- Use `vi.fn()` for mock functions
- Use `vi.mock('./module')` for module mocking
- Use `@testing-library/react` for component rendering
- Use `@testing-library/user-event` for simulating user interactions
- Use `fake-indexeddb/auto` for IndexedDB tests (configured globally)

### Current test coverage

| Directory | Test files | Tests |
|-----------|-----------|-------|
| `src/lib/` | 5 | 49 |
| `src/components/` | 5 | 16 |
| `src/app/student/session/` | 3 | 13 |
| **Total** | **16** | **94** |

## Key dependencies

| Package | Purpose |
|---------|---------|
| `react-markdown` | Render markdown in AI dialogue messages |
| `katex` | Render LaTeX math expressions |
| `mermaid` | Render Mermaid diagrams in messages |
| `reveal.js` | Presentation slide rendering |
| `dompurify` | Sanitize HTML to prevent XSS |
| `@tanstack/react-virtual` | Virtualized message lists |
| `@ricky0123/vad-web` | Voice Activity Detection for ASR input |
| `zustand` | Client-side state management |
| `swr` | Data fetching with caching |
| `echarts` | Analytics chart rendering |

## Learn more

- [Architecture documentation](../ARCHITECTURE.md)
- [API Reference](../docs/api-reference.md)
- [Contributing Guide](../CONTRIBUTING.md)
- [Testing Guide](../docs/testing-guide.md)
