# Development setup

This guide walks you through setting up a local development environment
for Hanfledge. By the end, you will have the backend, frontend, and all
infrastructure services running on your machine.

## Prerequisites

Install the following tools before proceeding:

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.25+ | Backend server |
| Node.js | 22+ | Frontend application |
| Docker | 24+ | Infrastructure services |
| Docker Compose | v2+ | Service orchestration |
| Git | 2.30+ | Version control |

Optional tools for enhanced development:

| Tool | Purpose |
|------|---------|
| Ollama | Local LLM inference (required for AI features) |
| `gh` CLI | GitHub CLI for PR management |

## Step 1: Clone the repository

Clone the repository and navigate to the project root:

```sh
git clone https://github.com/hflms/hanfledge.git
cd hanfledge
```

## Step 2: Start infrastructure services

Hanfledge requires PostgreSQL (with pgvector), Neo4j, and Redis. The
Docker Compose file manages all three services:

```sh
docker compose -f deployments/docker-compose.yml up -d
```

Verify that all services are healthy:

```sh
docker compose -f deployments/docker-compose.yml ps
```

Expected output shows three services running:

| Service | Container | Ports | Purpose |
|---------|-----------|-------|---------|
| PostgreSQL | `hanfledge-postgres` | `5433:5432` | Primary database with pgvector |
| Neo4j | `hanfledge-neo4j` | `7475:7474`, `7688:7687` | Knowledge graph |
| Redis | `hanfledge-redis` | `6381:6379` | Caching layer |

> **Note:** Port numbers are mapped to non-standard host ports to
> avoid conflicts with existing services on your machine.

### Default credentials

| Service | User | Password | Database |
|---------|------|----------|----------|
| PostgreSQL | `hanfledge` | `hanfledge_secret` | `hanfledge` |
| Neo4j | `neo4j` | `neo4j_secret` | -- |
| Redis | -- | -- (no auth) | -- |

To customize credentials, create a
`deployments/.env` file. See `deployments/.env.example` for all
available variables.

### Optional: Start WeKnora services

WeKnora is an optional knowledge base service. To enable it, start the
services with the `weknora` profile:

```sh
docker compose -f deployments/docker-compose.yml --profile weknora up -d
```

This adds three additional containers:

| Service | Container | Ports |
|---------|-----------|-------|
| WeKnora API | `hanfledge-weknora` | `9380:8080` |
| WeKnora Frontend | `hanfledge-weknora-frontend` | `9381:80` |
| DocReader (gRPC) | `hanfledge-weknora-docreader` | `50051:50051` |

## Step 3: Install Ollama (for AI features)

Hanfledge uses Ollama for local LLM inference. Install Ollama from
[ollama.com](https://ollama.com) and pull the required models:

```sh
# Chat model
ollama pull qwen2.5:7b

# Embedding model
ollama pull bge-m3
```

Verify Ollama is running:

```sh
curl http://localhost:11434/api/tags
```

> **Note:** AI-powered features (Socratic dialogue, quiz generation,
> skill execution) do not work without a running LLM provider. The
> rest of the application functions normally.

### Alternative LLM providers

Hanfledge supports multiple LLM providers. Set the `LLM_PROVIDER`
environment variable to switch providers:

| Provider | `LLM_PROVIDER` | Additional variables |
|----------|----------------|---------------------|
| Ollama (default) | `ollama` | `OLLAMA_HOST`, `OLLAMA_MODEL` |
| DashScope (Alibaba) | `dashscope` | `DASHSCOPE_API_KEY`, `DASHSCOPE_MODEL` |
| Gemini (Google) | `gemini` | Configured via system settings |

## Step 4: Run the backend

From the project root, start the Go backend server:

```sh
go run cmd/server/main.go
```

The server starts on `http://localhost:8080`. You see log output
confirming connections to PostgreSQL, Neo4j, and Redis.

### Backend environment variables

The backend reads configuration from environment variables with
sensible defaults for local development. Key variables include:

| Variable | Default | Description |
|----------|---------|-------------|
| `GIN_MODE` | `debug` | Gin framework mode (`debug` or `release`) |
| `SERVER_PORT` | `8080` | HTTP server port |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5433` | PostgreSQL port |
| `DB_USER` | `hanfledge` | PostgreSQL user |
| `DB_PASSWORD` | `hanfledge_secret` | PostgreSQL password |
| `DB_NAME` | `hanfledge` | PostgreSQL database name |
| `NEO4J_URI` | `bolt://localhost:7688` | Neo4j Bolt URI |
| `NEO4J_USER` | `neo4j` | Neo4j user |
| `NEO4J_PASSWORD` | `neo4j_secret` | Neo4j password |
| `REDIS_URL` | `redis://localhost:6381/0` | Redis connection URL |
| `JWT_SECRET` | `dev-jwt-secret-change-me` | JWT signing secret |
| `JWT_EXPIRY_HOURS` | `72` | JWT token expiry in hours |
| `LLM_PROVIDER` | `ollama` | LLM provider (`ollama`, `dashscope`) |
| `OLLAMA_HOST` | `http://localhost:11434` | Ollama API URL |
| `OLLAMA_MODEL` | `qwen2.5:7b` | Ollama chat model |
| `CORS_ORIGINS` | `http://localhost:3000` | Allowed CORS origins |

Create a `.env` file in the project root to override defaults. See
`.env.example` for a complete reference.

### Seed data

On first startup, the server automatically runs database migrations
and seeds default data:

- **Roles:** SYS_ADMIN, SCHOOL_ADMIN, TEACHER, STUDENT
- **Admin user:** Phone `13800000001`, password `admin123`
- **Test users:** Multiple users per role for testing

## Step 5: Run the frontend

In a separate terminal, install dependencies and start the frontend:

```sh
cd frontend
npm install
npm run dev
```

The frontend starts on `http://localhost:3000` with Turbopack for fast
hot module replacement.

### Frontend environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `NEXT_PUBLIC_API_URL` | `http://localhost:8080/api/v1` | Backend API URL |

Create `frontend/.env.local` for local overrides.

## Step 6: Verify the setup

Open `http://localhost:3000/login` in your browser and log in with:

- **Phone:** `13800000001`
- **Password:** `admin123`

After logging in, you are redirected to the admin dashboard. Navigate
to different sections to verify functionality.

### Health checks

Verify backend health endpoints:

```sh
# Liveness check
curl http://localhost:8080/health

# Readiness check (verifies DB, Neo4j, Redis connections)
curl http://localhost:8080/health/ready
```

### Swagger UI

In development mode, the Swagger UI is available at:

```
http://localhost:8080/swagger/index.html
```

## Common tasks

### Rebuild after code changes

The frontend hot-reloads automatically. For backend changes, stop the
server with `Ctrl+C` and restart:

```sh
go run cmd/server/main.go
```

### Reset the database

To reset the database, remove the Docker volume and restart:

```sh
docker compose -f deployments/docker-compose.yml down -v
docker compose -f deployments/docker-compose.yml up -d
```

### Run tests

```sh
# Backend (from project root)
go test ./...
go test -race ./...

# Frontend (from frontend/ directory)
npx vitest run
```

### Build for production

```sh
# Backend
go build -o bin/hanfledge cmd/server/main.go

# Frontend
cd frontend
npm run build
```

### Run linters

```sh
# Backend
go vet ./...

# Frontend
cd frontend
npm run lint
```

## Troubleshooting

### PostgreSQL connection refused

Verify the PostgreSQL container is running and healthy:

```sh
docker compose -f deployments/docker-compose.yml ps postgres
```

Check that port `5433` is not in use by another process:

```sh
lsof -i :5433
```

### Neo4j connection failure

Neo4j takes 20-30 seconds to start. The backend logs a warning on
startup if Neo4j is unavailable but continues running. Knowledge
graph features are disabled until Neo4j becomes available.

### Redis connection failure

Similar to Neo4j, the backend handles Redis unavailability gracefully.
Caching is disabled, and the application falls back to direct database
queries.

### Ollama not responding

Verify Ollama is running:

```sh
ollama list
```

If Ollama is not installed, AI features (dialogue, quiz generation)
are unavailable but all other features work normally.

### Frontend build errors

Clear the Next.js cache and rebuild:

```sh
cd frontend
rm -rf .next
npm run build
```

## Next steps

- Read the [Architecture documentation](../ARCHITECTURE.md) to
  understand how the system is designed.
- Review the [API Reference](api-reference.md) for the complete
  endpoint catalog.
- See the [Contributing Guide](../CONTRIBUTING.md) for development
  workflow and coding standards.
- Explore the [Testing Guide](testing-guide.md) for test patterns and
  conventions.
