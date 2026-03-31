# Security

Hanfledge implements multiple layers of security to protect user data,
prevent unauthorized access, and ensure safe AI interactions. This
document covers authentication, authorization, input safety, and data
protection mechanisms.

## Authentication

### JWT tokens

Hanfledge uses JSON Web Tokens (JWT) with the HS256 (HMAC-SHA256)
signing algorithm for stateless authentication.

Token structure (claims):

| Claim | Type | Description |
|-------|------|-------------|
| `user_id` | `uint` | Authenticated user ID |
| `phone` | `string` | User phone number |
| `display_name` | `string` | User display name |
| `exp` | `int64` | Token expiry (Unix timestamp) |

Token lifecycle:

1. The client sends credentials to `POST /api/v1/auth/login`.
2. The server validates credentials against bcrypt-hashed passwords.
3. On success, the server returns a signed JWT token.
4. The client stores the token in `localStorage` under the key
   `hanfledge_token`.
5. All subsequent API requests include the token in the
   `Authorization: Bearer <token>` header.
6. Tokens expire after 72 hours by default (configurable via
   `JWT_EXPIRY_HOURS`).

WebSocket connections pass the token via the `?token=` query
parameter since WebSocket does not support custom headers during the
handshake.

### Password storage

User passwords are hashed using bcrypt with the default cost factor.
Plain-text passwords are never stored or logged. The `password_hash`
field uses `json:"-"` to prevent accidental exposure in API responses.

### Production safety checks

When running in release mode (`GIN_MODE=release`), the server
validates that security-critical environment variables are not using
insecure defaults:

- `JWT_SECRET` must not be the development default
- `DB_PASSWORD` must not be the development default
- `NEO4J_PASSWORD` must not be the development default

The server refuses to start if these checks fail in production mode.

## Authorization (RBAC)

### Role hierarchy

Hanfledge uses role-based access control with four roles:

| Role | Scope | Description |
|------|-------|-------------|
| `SYS_ADMIN` | Platform-wide | Full administrative access |
| `SCHOOL_ADMIN` | School-scoped | School-level user and class management |
| `TEACHER` | School-scoped | Course, activity, and skill management |
| `STUDENT` | School-scoped | Learning sessions and self-service views |

### Middleware chain

The authorization flow for protected endpoints:

1. **JWTAuth middleware:** Validates the token and injects `user_id`,
   `phone`, and `display_name` into the Gin context.
2. **RBAC middleware:** Queries the user's roles from the
   `UserSchoolRole` table (with preloaded `Role`) and checks against
   the required roles for the endpoint.
3. The RBAC middleware uses OR logic: the user needs at least one of
   the required roles.

### Role assignment

Users can hold multiple roles across different schools. The
`UserSchoolRole` join table maps users to roles at specific schools.
`SYS_ADMIN` roles have a null `school_id` since they operate
platform-wide.

### Route protection

Every route registration function declares its required roles:

```go
// Example: only TEACHER and SYS_ADMIN can access
group.GET("/courses", middleware.RBAC(db, model.RoleTeacher, model.RoleSysAdmin), handler.ListCourses)
```

## Input safety

### Injection guard

The `InjectionGuard` protects against prompt injection attacks using
a three-layer defense strategy:

**Layer 1: Input length limit**

Student input is capped at 2,000 characters. Messages exceeding this
limit are blocked before reaching the LLM.

**Layer 2: Keyword blacklist**

A curated list of Chinese and English keywords commonly used in
prompt injection attempts:

- System prompt override commands ("ignore previous instructions",
  "forget all rules")
- Role hijacking attempts ("you are now", "act as")
- Direct instruction manipulation ("your instructions are",
  "new system prompt")

**Layer 3: Regex pattern matching**

Structured patterns that detect more sophisticated injection attempts:

- Role/persona switching patterns
- Instruction override patterns
- Base64-encoded payloads
- Markdown/HTML injection

Detection results are classified into three risk levels:

| Risk | Action | Description |
|------|--------|-------------|
| `safe` | Pass through | No risk detected |
| `warning` | Log and pass | Suspicious but not definitively malicious |
| `blocked` | Reject | High-risk injection attempt, message rejected |

### PII redactor

The `PIIRedactor` automatically strips personally identifiable
information from student messages before they reach the LLM.

Redaction targets:

| PII type | Replacement | Detection method |
|----------|-------------|------------------|
| Student names | `[学生]` | Database dictionary lookup |
| Teacher names | `[教师]` | Database dictionary lookup |
| School names | `[学校]` | Database dictionary lookup |
| Phone numbers | `[手机号]` | Regex: `1[3-9]\d{9}` |
| Email addresses | `[邮箱]` | Standard email regex |
| ID card numbers | `[证件号]` | Chinese 18-digit ID regex |

The PII dictionary is loaded from the database on startup and
refreshed periodically. Database-backed detection catches names that
regex patterns would miss.

### Output guard

The `OutputGuard` filters AI-generated responses before they reach
the student. It uses the LLM itself to evaluate whether a response
is appropriate for a K-12 educational context.

Filtered content includes:

- Inappropriate language or content
- Responses that deviate from educational objectives
- Potential information leaks from the system prompt

## Data protection

### Sensitive field handling

GORM models use `json:"-"` tags to prevent sensitive fields from
appearing in API responses:

- `User.PasswordHash`
- `WeKnoraToken.Token`
- `WeKnoraToken.RefreshToken`
- `DocumentChunk.Embedding`

### Soft deletes

The following entities support soft deletes (records are marked as
deleted but not physically removed):

- `User`
- `School`
- `Class`

Soft-deleted records are automatically excluded from queries by
GORM's built-in soft delete support.

### Token encryption

WeKnora SSO tokens are stored encrypted in the database. The
encryption key is configured via the `WEKNORA_ENCRYPTION_KEY`
environment variable.

## Frontend security

### XSS prevention

- All user-generated content rendered via `react-markdown` is
  sanitized through DOMPurify.
- Mermaid diagrams have XSS prevention applied before rendering.
- Content Security Policy headers are recommended in production
  Nginx configuration.

### Token storage

JWT tokens are stored in `localStorage` and automatically cleared on
401 responses. The `apiFetch<T>()` wrapper handles this:

1. Clears the token from `localStorage`.
2. Redirects to `/login`.

### Crypto-secure IDs

Frontend-generated IDs use `crypto.getRandomValues()` instead of
`Math.random()` for unpredictable random values.

## CORS

The backend configures CORS via the `CORS_ORIGINS` environment
variable. In development, this defaults to `http://localhost:3000`.
In production, set this to the actual frontend domain.

The CORS middleware:

- Allows specified origins
- Permits `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `OPTIONS` methods
- Allows `Authorization`, `Content-Type`, and other standard headers
- Supports credentials

## Security checklist for deployment

Follow these steps when deploying Hanfledge to production:

1. Set `GIN_MODE=release` to enable production safety checks.
2. Generate a strong, unique `JWT_SECRET` (at least 32 characters).
3. Set strong passwords for `DB_PASSWORD` and `NEO4J_PASSWORD`.
4. Configure `CORS_ORIGINS` to only allow your frontend domain.
5. Use HTTPS with a valid TLS certificate (configure in Nginx).
6. Set the `WEKNORA_ENCRYPTION_KEY` if WeKnora integration is
   enabled.
7. Restrict database ports to internal network only (do not expose
   PostgreSQL, Neo4j, or Redis to the public internet).
8. Enable Redis authentication in production.
9. Review and rotate the JWT secret periodically.
10. Monitor injection guard logs for blocked attempts.
