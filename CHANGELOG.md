# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **WeKnora Integration (2026-03-08)**
  - SSO single sign-on with automatic user synchronization
  - Per-user token management with Redis caching and database persistence
  - Neo4j graph database support for memory/knowledge graph features
  - Frontend "Open WeKnora" button in teacher dashboard
  - Knowledge base binding to courses with search functionality
  - `/api/v1/weknora/login-token` endpoint for SSO authentication
  - `WEKNORA_ENCRYPTION_KEY` environment variable for secure password generation
  - Automatic user registration and login on first access
  - Token refresh mechanism with 5-minute buffer

- **Performance Optimizations (2026-03-07)**
  - Parallel agent execution (Strategist + Designer run concurrently)
  - TTFT (Time To First Token) reduced by ~40%
  - Neo4j graph context preloading during Strategist analysis
  - Voice Activity Detection (VAD) using Silero VAD WebAssembly
  - ASR computation reduced by 50-70% with speech detection
  - Visual feedback for voice detection (🔴 waiting, 🟢 detected)

### Changed
- WeKnora client initialization now uses empty API key (per-user token mode)
- TokenManager uses `WEKNORA_ENCRYPTION_KEY` instead of `WEKNORA_API_KEY`
- Removed WeKnora ping check during initialization (deferred to first user access)
- Updated docker-compose.yml to use ParadeDB image with pg_search extension
- Neo4j service now includes health check dependency for WeKnora

### Fixed
- WeKnora knowledge base list API authentication issue
- SSO auto-login with correct localStorage key (`weknora_token`)
- User and tenant information persistence in localStorage
- Token expiration handling with automatic refresh
- WeKnora frontend token processing and URL parameter cleanup

### Documentation
- Updated README.md with WeKnora integration details
- Added SSO login-token API to API reference
- Added WeKnora optimization section to README
- Created comprehensive WeKnora integration guides
- Added quick start guide for WeKnora setup

## [2.0.0] - 2026-03-07

### Added
- Multi-agent orchestration with parallel execution
- Voice Activity Detection (VAD) support
- Dynamic AI provider configuration
- Teacher intervention system (takeover & whisper)
- Custom skill creation and marketplace
- Achievement system with gamification
- Error notebook for student learning
- Learning path analytics
- Cross-disciplinary knowledge linking
- Real-time session streaming via WebSocket

### Changed
- Upgraded to Go 1.25
- Upgraded to Next.js 16 with React 19
- Migrated to App Router architecture
- Improved BKT-driven scaffold fading
- Enhanced knowledge graph visualization

### Performance
- 40% reduction in Time To First Token (TTFT)
- 50-70% reduction in ASR computation
- Concurrent stress test: 1,155 req/s with 0% error rate
- Optimized database queries with performance indexes

## [1.0.0] - 2026-01-15

### Added
- Initial release with core features
- JWT authentication with RBAC
- KA-RAG pipeline for knowledge extraction
- 8 built-in skills (Socratic, Quiz, Role Play, etc.)
- PostgreSQL with pgvector for semantic search
- Neo4j for knowledge graph
- Redis for caching
- Ollama integration for LLM inference
- Admin, teacher, and student dashboards
- ECharts analytics visualization
- i18n support (zh-CN, en-US)

[Unreleased]: https://github.com/hflms/hanfledge/compare/v2.0.0...HEAD
[2.0.0]: https://github.com/hflms/hanfledge/compare/v1.0.0...v2.0.0
[1.0.0]: https://github.com/hflms/hanfledge/releases/tag/v1.0.0
