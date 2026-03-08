# Hanfledge Architecture

## Overview

Hanfledge is an AI-Native EdTech platform built on Clean Architecture principles with multi-agent orchestration, knowledge graphs, and adaptive learning.

## System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Client Layer                            │
│  Next.js 16 (React 19) + WebSocket + CSS Modules            │
└────────────────────────┬────────────────────────────────────┘
                         │ HTTP/WS
┌────────────────────────┴────────────────────────────────────┐
│                   API Gateway (Gin)                          │
│  JWT Auth + RBAC + CORS + i18n + Rate Limiting              │
└────────────────────────┬────────────────────────────────────┘
                         │
┌────────────────────────┴────────────────────────────────────┐
│                  Application Layer                           │
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   Handlers   │  │   Use Cases  │  │    Agents    │      │
│  │  (HTTP/WS)   │→ │   (KA-RAG)   │→ │ (4-pipeline) │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
│                                                              │
└────────────────────────┬────────────────────────────────────┘
                         │
┌────────────────────────┴────────────────────────────────────┐
│                 Infrastructure Layer                         │
│                                                              │
│  ┌──────┐  ┌──────┐  ┌───────┐  ┌──────┐  ┌──────────┐    │
│  │  DB  │  │ Neo4j│  │ Redis │  │ LLM  │  │ Storage  │    │
│  │(GORM)│  │(Bolt)│  │(Cache)│  │(API) │  │ (Local)  │    │
│  └──────┘  └──────┘  └───────┘  └──────┘  └──────────┘    │
└─────────────────────────────────────────────────────────────┘
```

## Layer Responsibilities

### 1. Domain Layer (`internal/domain/model`)
- **Pure business entities** (GORM models)
- No external dependencies
- Defines core data structures: User, Course, Session, KnowledgePoint, etc.

### 2. Use Case Layer (`internal/usecase`)
- **Business logic orchestration**
- KA-RAG pipeline: document processing, chunking, embedding, graph construction
- Independent of delivery mechanism (HTTP/gRPC/CLI)

### 3. Delivery Layer (`internal/delivery/http`)
- **HTTP handlers and WebSocket**
- Request validation and response formatting
- Middleware: JWT auth, RBAC, CORS, i18n
- Routes organized by domain (routes_*.go)

### 4. Infrastructure Layer (`internal/infrastructure`)
- **External service adapters**
- LLM providers (Ollama, DashScope, Gemini)
- Cache (Redis)
- Safety (injection guard, PII redactor, output guard)
- Storage (local filesystem)
- Search (web search fallback)

### 5. Agent Layer (`internal/agent`)
- **Multi-agent orchestration**
- 4-stage pipeline: Strategist → Designer → Coach → Critic
- BKT service for mastery tracking
- Profile service for cross-session analytics
- Skill state management (Quiz, Survey, RolePlay, Fallacy)

### 6. Plugin System (`internal/plugin`, `plugins/skills`)
- **Pluggable skill framework**
- Event bus for lifecycle hooks
- 8 built-in skills with manifest-driven metadata
- Custom skill support (teacher-created)

## Data Flow

### 1. Student Learning Session

```
Student Input (WebSocket)
    ↓
[Injection Guard] → Safety check
    ↓
[Strategist] → Analyze learning state, select target KP
    ↓ (parallel)
[Designer] → Retrieve materials (pgvector + Neo4j RRF)
    ↓
[Coach] → Generate Socratic response (LLM streaming)
    ↓
[Critic] → Review depth and correctness
    ↓ (retry if rejected)
[Coach] → Revise response
    ↓
[Output Guard] → Safety check
    ↓
[BKT] → Update mastery, fade scaffold
    ↓
WebSocket → Stream to frontend
```

### 2. KA-RAG Pipeline

```
Teacher uploads PDF
    ↓
[Document Parser] → Extract text
    ↓
[Chunker] → Split into semantic chunks
    ↓
[Embedding] → Generate vectors (bge-m3)
    ↓
[PostgreSQL] → Store chunks + vectors
    ↓
[Graph Builder] → Extract concepts and relationships
    ↓
[Neo4j] → Store knowledge graph
    ↓
[Outline Generator] → Create course structure
```

### 3. Hybrid Retrieval (RRF)

```
Student query
    ↓
[Embedding] → Convert to vector
    ↓ (parallel)
┌─────────────────┬─────────────────┐
│  Vector Search  │  Graph Search   │
│  (pgvector)     │  (Neo4j)        │
│  Top-50         │  Top-50         │
└────────┬────────┴────────┬────────┘
         │                 │
         └────────┬────────┘
                  ↓
         [RRF Fusion] → Top-10
                  ↓
         Personalized materials
```

## Key Design Patterns

### 1. Dependency Injection
- `RouterDeps` aggregates all dependencies
- Handlers receive only what they need
- Testable and mockable

### 2. Repository Pattern
- Abstract database operations
- `postgres/` and `neo4j/` repositories
- GORM for SQL, Bolt driver for Neo4j

### 3. Strategy Pattern
- LLM providers (Ollama, DashScope, Gemini)
- ASR providers (extensible)
- Storage backends (local, S3-ready)

### 4. Observer Pattern
- Plugin event bus with lifecycle hooks
- Teacher intervention via event injection
- Achievement triggers

### 5. State Machine
- Skill session states (Quiz, Survey, Fallacy)
- Phase transitions based on content analysis

## Scalability Considerations

### Current Limits
- Single-instance deployment
- In-memory session state (Redis-backed)
- Synchronous agent pipeline

### Future Scaling
1. **Horizontal scaling**: Stateless backend + Redis session store
2. **Async processing**: Move KA-RAG to background jobs (Asynq)
3. **Microservices**: Split into Auth, Course, Session, Analytics services
4. **CDN**: Static assets and document storage
5. **Load balancing**: Nginx upstream with health checks

## Security Architecture

### Layer 1: Input Validation
- Gin binding validation
- Phone number format check
- File type whitelist

### Layer 2: Authentication & Authorization
- JWT (HS256) with Bearer tokens
- RBAC via UserSchoolRole join table
- Role hierarchy: SYS_ADMIN > SCHOOL_ADMIN > TEACHER > STUDENT

### Layer 3: Prompt Injection Guard
- 60 keyword patterns + 14 regex rules
- Blocks jailbreak attempts
- Logs suspicious inputs

### Layer 4: PII Redaction
- Removes phone numbers, emails, ID cards
- Applied before LLM processing
- Preserves conversation context

### Layer 5: Output Safety
- Content policy check
- Harmful content filtering
- Fallback to safe responses

## Performance Optimizations

### 1. Parallel Agent Execution (v2.0)
- Strategist + Designer run concurrently
- TTFT reduced by ~40%
- Graph context preloading

### 2. Voice Activity Detection (v2.0)
- Frontend Silero VAD (WebAssembly)
- Only sends audio when speech detected
- Reduces ASR computation by 50-70%

### 3. Multi-Level Caching
- **L1**: In-memory session context
- **L2**: Redis semantic cache (cosine similarity > 0.95)
- **L3**: Redis output cache (exact prompt hash)

### 4. Database Indexes
- Composite indexes for activity + status queries
- Time-series indexes for analytics
- Partial indexes for conditional queries

### 5. Connection Pooling
- PostgreSQL: 25 max connections
- Redis: 20 pool size, 5 min idle
- Neo4j: 50 max connections

## Technology Stack

### Backend
- **Language**: Go 1.25
- **Framework**: Gin (HTTP), gorilla/websocket
- **ORM**: GORM
- **Database**: PostgreSQL 16 + pgvector
- **Graph DB**: Neo4j 5 Community
- **Cache**: Redis 7
- **AI**: Ollama (qwen2.5:7b, bge-m3)

### Frontend
- **Framework**: Next.js 16 (App Router)
- **UI Library**: React 19
- **Styling**: CSS Modules
- **Charts**: ECharts
- **State**: Zustand
- **Testing**: Vitest + React Testing Library

### Infrastructure
- **Containerization**: Docker + Docker Compose
- **Reverse Proxy**: Nginx (production)
- **CI/CD**: GitHub Actions

## File Organization

```
cmd/server/main.go                 # Entry point
internal/
  domain/model/                    # Entities (GORM models)
  usecase/                         # Business logic (KA-RAG)
  delivery/http/                   # HTTP handlers + middleware
    handler/                       # Domain handlers
    middleware/                    # JWT, RBAC, CORS
    routes_*.go                    # Route registration by domain
  agent/                           # Multi-agent system
    orchestrator.go                # Pipeline coordinator
    strategist.go                  # Learning state analyzer
    designer.go                    # Material retriever
    coach.go                       # Socratic dialogue generator
    critic.go                      # Response reviewer
    skill_state.go                 # Skill state management
    cache_manager.go               # Cache operations
    profile_manager.go             # Student profile
  infrastructure/                  # External adapters
    llm/                           # LLM providers
    cache/                         # Redis client
    safety/                        # Security guards
    storage/                       # File storage
  repository/                      # Data access
    postgres/                      # SQL repository
    neo4j/                         # Graph repository
  plugin/                          # Plugin framework
plugins/skills/                    # Skill definitions
frontend/                          # Next.js app
  src/app/                         # App Router pages
  src/components/                  # Shared components
  src/lib/                         # Utilities and API client
  src/stores/                      # Zustand stores
```

## Testing Strategy

### Backend
- **Unit tests**: Agent logic, BKT, cache, safety
- **Integration tests**: HTTP handlers with test DB
- **E2E tests**: Full pipeline with mocked LLM
- **Benchmarks**: Login, GetMe, ListCourses

### Frontend
- **Unit tests**: Hooks, utilities
- **Component tests**: React Testing Library
- **E2E tests**: Playwright (planned)

## Monitoring & Observability

### Logging
- Structured logging with log/slog
- Component-scoped loggers
- JSON output in production

### Metrics (Available)
- Cache hit rate
- Cache invalidation
- (Prometheus integration planned)

### Tracing (Planned)
- OpenTelemetry integration
- Distributed tracing across agents
- LLM latency tracking

## Deployment

### Development
```bash
bash scripts/dev.sh
```

### Production
```bash
docker compose -f deployments/docker-compose.prod.yml up -d
```

### Environment Variables
See `.env.example` for all configuration options.

## Contributing

1. Follow Clean Architecture principles
2. Write tests for new features
3. Use structured logging
4. Document API changes in swagger.yaml
5. Update CHANGELOG.md

## References

- [README.md](README.md) - Quick start guide
- [AGENTS.md](AGENTS.md) - Developer guide for AI agents
- [docs/manuals/](docs/manuals/) - User manuals by role
- [OPTIMIZATION_RECOMMENDATIONS.md](docs/OPTIMIZATION_RECOMMENDATIONS.md) - Performance tuning guide
