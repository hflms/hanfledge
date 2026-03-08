# WeKnora Integration Guide

## Overview

Hanfledge integrates with [WeKnora](https://github.com/WeChat-OpenAI/WeKnora), an open-source knowledge base service, to provide external knowledge management capabilities for courses.

## Architecture

```
┌─────────────────┐         ┌──────────────────┐
│  Hanfledge API  │────────▶│  WeKnora API     │
│  (Go Backend)   │  HTTP   │  (Knowledge Base)│
└─────────────────┘         └──────────────────┘
        │                            │
        ▼                            ▼
┌─────────────────┐         ┌──────────────────┐
│  PostgreSQL     │         │  PostgreSQL      │
│  (hanfledge DB) │         │  (weknora DB)    │
└─────────────────┘         └──────────────────┘
```

## Features

- **Knowledge Base Management**: List and browse WeKnora knowledge bases
- **Course Binding**: Bind external knowledge bases to Hanfledge courses
- **Semantic Search**: Search across bound knowledge bases
- **User Synchronization**: Automatic user sync with role mapping
- **Single Sign-On**: Shared bcrypt password hashing

## Setup

### 1. Start WeKnora Service

```bash
# Start with WeKnora profile
docker compose -f deployments/docker-compose.yml --profile weknora up -d

# Verify services
docker ps | grep weknora
```

This starts:
- **WeKnora App** (`localhost:9380`) - Main API service
- **DocReader** (`localhost:50051`) - Document parsing service
- **ParadeDB** - PostgreSQL 18 with `pg_search` and `pgvector` extensions

### 2. Sync Users

Run the user synchronization script to create WeKnora accounts for all Hanfledge users:

```bash
go run scripts/sync_weknora_users.go
```

This script:
- Reads all users from Hanfledge database
- Creates corresponding accounts in WeKnora
- Maps Hanfledge admins (SYS_ADMIN, SCHOOL_ADMIN) to WeKnora admins (`can_access_all_tenants=true`)
- Reuses bcrypt password hashes for SSO

### 3. Create Tenant

WeKnora requires at least one tenant. Create a default tenant:

```sql
-- Connect to weknora database
docker exec -i hanfledge-postgres psql -U hanfledge -d weknora << 'EOF'
INSERT INTO tenants (name, description, api_key, business, retriever_engines, created_at, updated_at)
VALUES (
  'Hanfledge',
  'Default tenant for Hanfledge integration',
  'hanfledge-tenant-api-key-' || substr(md5(random()::text), 1, 16),
  'education',
  '{}'::jsonb,
  NOW(),
  NOW()
)
RETURNING id, name, api_key;

-- Update users to use this tenant
UPDATE users SET tenant_id = (SELECT id FROM tenants WHERE name = 'Hanfledge');
EOF
```

### 4. Configure Hanfledge Backend

Get a WeKnora access token and add it to `.env`:

```bash
# Login to WeKnora
WK_TOKEN=$(curl -s http://localhost:9380/api/v1/auth/login -X POST \
  -H "Content-Type: application/json" \
  -d '{"email":"13800000010@hanfledge.local","password":"teacher123"}' | jq -r '.token')

# Add to .env
echo "WEKNORA_API_KEY=$WK_TOKEN" >> .env
```

Update `.env` to enable WeKnora:

```env
WEKNORA_ENABLED=true
WEKNORA_BASE_URL=http://localhost:9380/api/v1
WEKNORA_API_KEY=<token from above>
```

### 5. Restart Hanfledge Backend

```bash
# Kill existing process
pkill -f "go run.*server"

# Start with WeKnora enabled
go run cmd/server/main.go
```

Check logs for successful initialization:

```bash
grep "WeKnora integration enabled" /tmp/hanfledge.log
```

## API Endpoints

All WeKnora endpoints require authentication (JWT Bearer token) and TEACHER+ role.

### List Knowledge Bases

```bash
GET /api/v1/weknora/knowledge-bases
```

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "id": "f1280bfd-a894-4a8a-9c65-87443bc68704",
      "name": "Test Knowledge Base",
      "description": "Created from Hanfledge",
      "embedding_model": "bge-m3",
      "created_at": "2026-03-08T09:00:00Z"
    }
  ]
}
```

### Get Knowledge Base Details

```bash
GET /api/v1/weknora/knowledge-bases/:kb_id
```

### List Knowledge Entries

```bash
GET /api/v1/weknora/knowledge-bases/:kb_id/knowledge
```

### Bind Knowledge Base to Course

```bash
POST /api/v1/courses/:id/weknora-refs
Content-Type: application/json

{
  "kb_id": "f1280bfd-a894-4a8a-9c65-87443bc68704",
  "kb_name": "Test Knowledge Base"
}
```

**Response:**
```json
{
  "id": 1,
  "course_id": 2,
  "kb_id": "f1280bfd-a894-4a8a-9c65-87443bc68704",
  "kb_name": "Test Knowledge Base",
  "added_by_id": 2,
  "created_at": "2026-03-08T17:04:18.199050142+08:00"
}
```

### List Bound Knowledge Bases

```bash
GET /api/v1/courses/:id/weknora-refs
```

**Response:**
```json
{
  "data": [
    {
      "id": 1,
      "course_id": 2,
      "kb_id": "f1280bfd-a894-4a8a-9c65-87443bc68704",
      "kb_name": "Test Knowledge Base",
      "added_by_id": 2,
      "created_at": "2026-03-08T17:04:18.19905+08:00"
    }
  ]
}
```

### Unbind Knowledge Base

```bash
DELETE /api/v1/courses/:id/weknora-refs/:ref_id
```

**Response:**
```json
{
  "message": "knowledge base unbound successfully"
}
```

### Search Knowledge Base

```bash
POST /api/v1/courses/:id/weknora-search
Content-Type: application/json

{
  "query": "photosynthesis"
}
```

**Response:**
```json
{
  "data": [
    {
      "id": "entry-123",
      "content": "Photosynthesis is the process...",
      "score": 0.95,
      "metadata": {...}
    }
  ]
}
```

## Testing

### Manual Testing

```bash
# 1. Login
TOKEN=$(curl -s http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"phone":"13800000010","password":"teacher123"}' | jq -r '.token')

# 2. List knowledge bases
curl -s http://localhost:8080/api/v1/weknora/knowledge-bases \
  -H "Authorization: Bearer $TOKEN" | jq .

# 3. Bind KB to course
curl -s http://localhost:8080/api/v1/courses/2/weknora-refs -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"kb_id":"<kb-id>","kb_name":"Test KB"}' | jq .

# 4. Search
curl -s http://localhost:8080/api/v1/courses/2/weknora-search -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"query":"test"}' | jq .
```

### Automated Testing

Run the integration test script:

```bash
bash scripts/test_weknora_integration.sh
```

## User Synchronization

### Automatic Sync

The `sync_weknora_users.go` script syncs users from Hanfledge to WeKnora:

```go
// Hanfledge roles → WeKnora permissions
RoleSysAdmin    → can_access_all_tenants = true
RoleSchoolAdmin → can_access_all_tenants = true
RoleTeacher     → can_access_all_tenants = false
RoleStudent     → can_access_all_tenants = false
```

### Manual Sync

```bash
go run scripts/sync_weknora_users.go
```

Output:
```
✓ Updated WeKnora user: 13800000001 (admin=true)
✓ Updated WeKnora user: 13800000010 (admin=true)
✓ Updated WeKnora user: 13800000011 (admin=false)
...
✅ Synced 13 users to WeKnora
```

### Password Compatibility

Both systems use bcrypt for password hashing, enabling SSO:

```go
// Hanfledge
passwordHash := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

// WeKnora (reuses the same hash)
wkUser.PasswordHash = hanfledgeUser.PasswordHash
```

Users can login to both systems with the same credentials.

## Token Management

Hanfledge uses a `TokenManager` to maintain per-user WeKnora tokens:

```go
// Get user-specific WeKnora client
wkClient := tokenManager.GetClientForUser(ctx, userID)

// Tokens are automatically refreshed when expired
kbs, err := wkClient.ListKnowledgeBases(ctx)
```

Tokens are stored in the `weknora_user_tokens` table:

```sql
CREATE TABLE weknora_user_tokens (
  id SERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id),
  access_token TEXT NOT NULL,
  refresh_token TEXT NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW()
);
```

## Troubleshooting

### WeKnora Connection Failed

**Error:** `WeKnora connection failed (non-fatal): weknora auth failed (status 401)`

**Solution:**
1. Verify `WEKNORA_API_KEY` in `.env` is valid
2. Regenerate token:
   ```bash
   WK_TOKEN=$(curl -s http://localhost:9380/api/v1/auth/login -X POST \
     -H "Content-Type: application/json" \
     -d '{"email":"13800000010@hanfledge.local","password":"teacher123"}' | jq -r '.token')
   sed -i "s/^WEKNORA_API_KEY=.*/WEKNORA_API_KEY=$WK_TOKEN/" .env
   ```
3. Restart Hanfledge backend

### Invalid Tenant Error

**Error:** `{"error":"Unauthorized: invalid tenant"}`

**Solution:**
1. Ensure tenant exists and has valid `retriever_engines`:
   ```sql
   UPDATE tenants SET retriever_engines = '{}'::jsonb WHERE id = 10000;
   ```
2. Verify users have `tenant_id` set:
   ```sql
   UPDATE users SET tenant_id = 10000 WHERE tenant_id = 0 OR tenant_id IS NULL;
   ```
3. Restart WeKnora service:
   ```bash
   docker compose -f deployments/docker-compose.yml --profile weknora restart weknora
   ```

### User Sync Failed

**Error:** `sql: Scan error on column index 4, name "retriever_engines"`

**Solution:**
This is a WeKnora internal error. Ensure tenant has proper JSON structure:
```sql
UPDATE tenants SET retriever_engines = '{}'::jsonb;
```

## Architecture Details

### Database Schema

**Hanfledge:**
```sql
-- Course-KB binding
CREATE TABLE course_weknora_refs (
  id SERIAL PRIMARY KEY,
  course_id BIGINT NOT NULL REFERENCES courses(id),
  kb_id VARCHAR(255) NOT NULL,
  kb_name VARCHAR(255) NOT NULL,
  added_by_id BIGINT NOT NULL REFERENCES users(id),
  created_at TIMESTAMP DEFAULT NOW()
);

-- User tokens
CREATE TABLE weknora_user_tokens (
  id SERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id),
  access_token TEXT NOT NULL,
  refresh_token TEXT NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW()
);
```

**WeKnora:**
```sql
-- Users (synced from Hanfledge)
CREATE TABLE users (
  id VARCHAR(36) PRIMARY KEY,
  username VARCHAR(100) UNIQUE NOT NULL,
  email VARCHAR(255) UNIQUE NOT NULL,
  password_hash VARCHAR(255) NOT NULL,
  tenant_id INTEGER REFERENCES tenants(id),
  can_access_all_tenants BOOLEAN DEFAULT false,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Tenants
CREATE TABLE tenants (
  id SERIAL PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  api_key VARCHAR(64) NOT NULL,
  business VARCHAR(255) NOT NULL,
  retriever_engines JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

### Code Structure

```
internal/
  infrastructure/weknora/
    client.go           # WeKnora HTTP client
    token_manager.go    # Per-user token management
    types.go            # Request/response types
  delivery/http/
    routes_weknora.go   # WeKnora API routes
    handler/
      weknora_handler.go # HTTP handlers
  domain/model/
    weknora.go          # GORM models (CourseWeKnoraRef, WeKnoraUserToken)
scripts/
  sync_weknora_users.go # User synchronization script
```

## Future Enhancements

- [ ] Automatic user sync on user creation/update
- [ ] Webhook integration for real-time KB updates
- [ ] Frontend UI for KB management
- [ ] Batch KB operations
- [ ] KB analytics and usage tracking
- [ ] Multi-tenant support in Hanfledge

## References

- [WeKnora GitHub](https://github.com/WeChat-OpenAI/WeKnora)
- [ParadeDB Documentation](https://docs.paradedb.com/)
- [Hanfledge API Documentation](../docs/swagger.yaml)
