# API reference

Hanfledge exposes a REST API at `/api/v1` for all client-server
communication. This document catalogs every endpoint with its HTTP
method, path, authentication requirements, and handler function.

All endpoints return JSON. Error responses use the format:

```json
{
  "error": "User-facing error message in Chinese"
}
```

Paginated endpoints return:

```json
{
  "items": [],
  "total": 100,
  "page": 1,
  "limit": 20
}
```

## Authentication

Most endpoints require a JWT Bearer token in the `Authorization`
header:

```
Authorization: Bearer <token>
```

WebSocket endpoints accept the token as a `?token=` query parameter.

Obtain a token by calling `POST /api/v1/auth/login` with valid
credentials. Tokens expire after 72 hours by default (configurable via
`JWT_EXPIRY_HOURS`).

## Roles

The API uses four roles for access control:

| Role | Description |
|------|-------------|
| `SYS_ADMIN` | Platform-wide administrator |
| `SCHOOL_ADMIN` | School-level administrator |
| `TEACHER` | Course creator and session monitor |
| `STUDENT` | Learner in AI-guided sessions |

Routes marked "Any (JWT)" require authentication but accept any role.

---

## Health and system

These endpoints are public and do not require authentication:

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Liveness probe (returns 200 if server is running) |
| `GET` | `/health/ready` | Readiness probe (checks DB, Neo4j, Redis connections) |
| `GET` | `/swagger/*any` | Swagger UI (development mode only) |

---

## Authentication (`/api/v1/auth`)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/v1/auth/login` | None | Log in with phone and password, returns JWT token |
| `GET` | `/api/v1/auth/me` | Any (JWT) | Get current user profile and roles |

### Login request body

```json
{
  "phone": "13800000001",
  "password": "admin123"
}
```

### Login response

```json
{
  "token": "eyJhbGciOi...",
  "user": {
    "id": 1,
    "phone": "13800000001",
    "display_name": "Admin",
    "status": "active",
    "school_roles": [...]
  }
}
```

---

## Admin (`/api/v1`)

These endpoints manage platform-wide resources. All require
`SYS_ADMIN` or `SCHOOL_ADMIN` role unless noted.

| Method | Path | Roles | Description |
|--------|------|-------|-------------|
| `GET` | `/api/v1/schools` | SYS_ADMIN | List all schools |
| `POST` | `/api/v1/schools` | SYS_ADMIN | Create a school |
| `GET` | `/api/v1/classes` | SYS_ADMIN, SCHOOL_ADMIN | List classes (filtered by school) |
| `POST` | `/api/v1/classes` | SYS_ADMIN, SCHOOL_ADMIN | Create a class |
| `GET` | `/api/v1/users` | SYS_ADMIN, SCHOOL_ADMIN | List users with pagination |
| `POST` | `/api/v1/users` | SYS_ADMIN, SCHOOL_ADMIN | Create a single user |
| `POST` | `/api/v1/users/batch` | SYS_ADMIN, SCHOOL_ADMIN | Batch create multiple users |

---

## Courses (`/api/v1/courses`)

Course management and knowledge graph operations. All require
`TEACHER` or higher role.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/courses` | List courses for the current teacher |
| `POST` | `/api/v1/courses` | Create a new course |
| `POST` | `/api/v1/courses/:id/materials` | Upload a document (PDF) for processing |
| `GET` | `/api/v1/courses/:id/outline` | Get course outline (chapters and KPs) |
| `GET` | `/api/v1/courses/:id/graph` | Get course knowledge graph from Neo4j |
| `GET` | `/api/v1/courses/:id/documents` | List documents and their processing status |
| `POST` | `/api/v1/courses/:id/search` | Semantic search across course materials |
| `DELETE` | `/api/v1/courses/:id/documents/:doc_id` | Delete a document |
| `POST` | `/api/v1/courses/:id/documents/:doc_id/retry` | Retry failed document processing |
| `POST` | `/api/v1/courses/:id/skills/recommend` | AI-recommend skills for course KPs |
| `POST` | `/api/v1/courses/:id/skills/batch-mount` | Batch mount recommended skills |

---

## Skills (`/api/v1/skills`)

Skill catalog and chapter-level skill mounting. Requires `TEACHER`
or higher role.

### Skill store

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/skills` | List all available skills in the registry |
| `GET` | `/api/v1/skills/:id` | Get skill detail (metadata, templates, config) |

### Chapter skill mounting

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/chapters/:id/skills` | Mount a skill to a chapter |
| `PATCH` | `/api/v1/chapters/:id/skills/:mount_id` | Update skill mount config |
| `DELETE` | `/api/v1/chapters/:id/skills/:mount_id` | Unmount a skill from a chapter |

---

## Custom skills (`/api/v1/custom-skills`)

Teachers can create and manage custom skills. Requires `TEACHER` or
higher role.

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/custom-skills` | Create a custom skill |
| `GET` | `/api/v1/custom-skills` | List teacher's custom skills |
| `GET` | `/api/v1/custom-skills/:id` | Get custom skill detail |
| `PUT` | `/api/v1/custom-skills/:id` | Update a custom skill |
| `DELETE` | `/api/v1/custom-skills/:id` | Delete a custom skill |
| `POST` | `/api/v1/custom-skills/:id/publish` | Publish a custom skill |
| `POST` | `/api/v1/custom-skills/:id/share` | Share skill with school/platform |
| `POST` | `/api/v1/custom-skills/:id/archive` | Archive a custom skill |
| `GET` | `/api/v1/custom-skills/:id/versions` | List version history |

---

## Knowledge points (`/api/v1/knowledge-points`)

Manage misconceptions, skill mounts, cross-links, and prerequisites
at the knowledge point level. Requires `TEACHER` or higher role.

### Misconceptions

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/knowledge-points/:id/misconceptions` | Create a misconception |
| `GET` | `/api/v1/knowledge-points/:id/misconceptions` | List misconceptions for a KP |
| `DELETE` | `/api/v1/knowledge-points/:id/misconceptions/:misconception_id` | Delete a misconception |

### KP skill mounting

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/knowledge-points/:id/skills` | Mount a skill to a KP |
| `PATCH` | `/api/v1/knowledge-points/:id/skills/:mount_id` | Update KP skill config |
| `DELETE` | `/api/v1/knowledge-points/:id/skills/:mount_id` | Unmount a skill from a KP |

### Cross-links

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/knowledge-points/:id/cross-links` | Create a cross-link between KPs |
| `GET` | `/api/v1/knowledge-points/:id/cross-links` | List cross-links for a KP |
| `DELETE` | `/api/v1/knowledge-points/:id/cross-links/:link_id` | Delete a cross-link |

### Prerequisites

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/knowledge-points/:id/prerequisites` | Create a prerequisite relationship |
| `GET` | `/api/v1/knowledge-points/:id/prerequisites` | Get prerequisites for a KP |

---

## Activities and sessions (`/api/v1/activities`)

Learning activity management and student session control.

### Activity management (TEACHER+)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/designers` | List instructional designer templates |
| `POST` | `/api/v1/activities` | Create a learning activity |
| `GET` | `/api/v1/activities` | List teacher's activities |
| `GET` | `/api/v1/activities/:id` | Get activity detail |
| `PUT` | `/api/v1/activities/:id` | Update an activity |
| `PUT` | `/api/v1/activities/:id/steps` | Save activity steps |
| `POST` | `/api/v1/activities/:id/upload` | Upload an asset for an activity step |
| `POST` | `/api/v1/activities/:id/steps/suggest` | AI-suggest step content |
| `POST` | `/api/v1/activities/:id/publish` | Publish an activity to students |
| `POST` | `/api/v1/activities/:id/preview` | Create a sandbox preview session |
| `GET` | `/api/v1/activities/:id/sessions` | List sessions for an activity |

### Session management (Any JWT)

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/activities/:id/join` | Join an activity (creates a session) |
| `GET` | `/api/v1/sessions/:id` | Get session detail |
| `PUT` | `/api/v1/sessions/:id/step` | Update current session step |
| `GET` | `/api/v1/sessions/:id/stream` | WebSocket: stream AI dialogue |

### Teacher intervention (TEACHER+)

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/sessions/:id/intervention` | Takeover or whisper into a session |

---

## Student (`/api/v1/student`)

Student-specific endpoints. Require `STUDENT` or `SYS_ADMIN` role.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/student/activities` | List published activities for the student |
| `GET` | `/api/v1/student/mastery` | Get self mastery scores |
| `GET` | `/api/v1/student/knowledge-map` | Get personalized knowledge map |
| `GET` | `/api/v1/student/error-notebook` | Get error notebook entries |
| `GET` | `/api/v1/student/achievements` | Get earned achievements |
| `GET` | `/api/v1/student/achievements/definitions` | List all achievement definitions |

---

## Analytics and dashboard (`/api/v1/dashboard`)

Teacher analytics and dashboard endpoints. Require `TEACHER` or
higher role unless noted.

### Dashboard

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/dashboard/knowledge-radar` | Get knowledge radar chart data |
| `GET` | `/api/v1/dashboard/skill-effectiveness` | Get skill effectiveness metrics |
| `GET` | `/api/v1/dashboard/live-monitor` | Get real-time student monitoring data |
| `GET` | `/api/v1/dashboard/activities/:id/live` | Get live detail for a specific activity |

### Student mastery

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/students/:id/mastery` | Get a specific student's mastery data |

### Session analytics

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/sessions/:id/inquiry-tree` | Get inquiry tree for a session |
| `GET` | `/api/v1/sessions/:id/interactions` | Get interaction log for a session |

### Performance analytics (Any JWT)

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/analytics/performance` | Record frontend performance metrics |

---

## Data export (`/api/v1/export`)

CSV data export endpoints. Require `TEACHER` or higher role.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/export/activities/:id/sessions` | Export activity sessions as CSV |
| `GET` | `/api/v1/export/courses/:id/mastery` | Export class mastery data as CSV |
| `GET` | `/api/v1/export/courses/:id/error-notebook` | Export error notebook as CSV |
| `GET` | `/api/v1/export/sessions/:id/interactions` | Export session interactions as CSV |

All export endpoints return `text/csv` with appropriate
`Content-Disposition` headers for file download.

---

## System configuration (`/api/v1/system`)

Runtime system configuration. Require JWT authentication (any role).

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/system/config` | Get all system config key-value pairs |
| `PUT` | `/api/v1/system/config` | Update system config values |
| `POST` | `/api/v1/system/config/test-chat-model` | Test a chat model configuration |
| `POST` | `/api/v1/system/config/test-embedding-model` | Test an embedding model configuration |

---

## Soul system (`/api/v1/system/soul`)

AI behavior rules management. Requires `SYS_ADMIN` role.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/system/soul` | Get current soul rules |
| `PUT` | `/api/v1/system/soul` | Update soul rules (creates a version) |
| `GET` | `/api/v1/system/soul/history` | List soul version history |
| `POST` | `/api/v1/system/soul/rollback` | Rollback to a previous version |
| `POST` | `/api/v1/system/soul/evolve` | Trigger AI-driven soul evolution |

For detailed soul system documentation, see
[Soul System](SOUL_SYSTEM.md).

---

## Marketplace (`/api/v1/marketplace`)

Plugin marketplace for discovering and installing skills.

| Method | Path | Roles | Description |
|--------|------|-------|-------------|
| `GET` | `/api/v1/marketplace/plugins` | Any (JWT) | List approved plugins |
| `GET` | `/api/v1/marketplace/plugins/:plugin_id` | Any (JWT) | Get plugin detail |
| `GET` | `/api/v1/marketplace/installed` | Any (JWT) | List installed plugins |
| `POST` | `/api/v1/marketplace/plugins` | TEACHER+ | Submit a plugin for review |
| `POST` | `/api/v1/marketplace/install` | TEACHER+ | Install a plugin |
| `DELETE` | `/api/v1/marketplace/installed/:id` | TEACHER+ | Uninstall a plugin |

---

## Notifications (`/api/v1/notifications`)

System notification management. Requires JWT authentication.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/notifications/unread` | Get unread notifications (max 20) |
| `POST` | `/api/v1/notifications/:id/read` | Mark a notification as read |

---

## Metrics (`/api/v1/metrics`)

Cache performance monitoring endpoints. Public (no auth required).

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/metrics/cache` | Get Redis cache hit/miss metrics |
| `POST` | `/api/v1/metrics/cache/invalidate` | Invalidate cache entries by pattern |

---

## WeKnora integration (`/api/v1/weknora`)

Optional WeKnora knowledge base integration. These endpoints are only
available when `WEKNORA_ENABLED=true`. Require `TEACHER` or higher
role.

### Knowledge base management

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/weknora/login-token` | Get SSO token for WeKnora |
| `POST` | `/api/v1/weknora/knowledge-bases` | Create a knowledge base in WeKnora |
| `GET` | `/api/v1/weknora/knowledge-bases` | List knowledge bases |
| `GET` | `/api/v1/weknora/knowledge-bases/:kb_id` | Get knowledge base detail |
| `GET` | `/api/v1/weknora/knowledge-bases/:kb_id/knowledge` | List knowledge items in a KB |
| `DELETE` | `/api/v1/weknora/knowledge-bases/:kb_id` | Delete a knowledge base |

### Course-KB binding

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/courses/:id/weknora-refs` | Bind a KB to a course |
| `GET` | `/api/v1/courses/:id/weknora-refs` | List bound KBs for a course |
| `DELETE` | `/api/v1/courses/:id/weknora-refs/:ref_id` | Unbind a KB from a course |
| `POST` | `/api/v1/courses/:id/weknora-search` | Search across bound KBs |

---

## Endpoint summary

| Category | Endpoints |
|----------|-----------|
| Health and system | 3 |
| Authentication | 2 |
| Admin | 7 |
| Courses | 11 |
| Skills | 5 |
| Custom skills | 9 |
| Knowledge points | 11 |
| Activities and sessions | 16 |
| Student | 6 |
| Analytics and dashboard | 8 |
| Data export | 4 |
| System configuration | 4 |
| Soul system | 5 |
| Marketplace | 6 |
| Notifications | 2 |
| Metrics | 2 |
| WeKnora | 10 |
| **Total** | **111** |
