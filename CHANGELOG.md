# Changelog

All notable changes to Hanfledge will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Cache metrics monitoring API (`GET /api/v1/metrics/cache`)
- Cache invalidation API (`POST /api/v1/metrics/cache/invalidate`)
- Standardized error response structure with i18n support
- Zustand-based toast notification store
- Performance indexes for high-frequency queries
- Custom hooks for outline page (useSkillMounting, useActivityPublish, etc.)
- Modular API structure (api/core.ts, api/auth.ts)

### Changed
- Refactored `orchestrator.go` into modular components (skill_state, cache_manager, profile_manager)
- Refactored `coach.go` by extracting skill state management (coach_skill_states.go)
- Reduced coach.go from 1092 to 447 lines (-59%)

### Fixed
- All 43 TypeScript type errors in frontend
- WeKnora service restart issue (documented pg_search extension requirement)
- Removed unused imports and variables

### Performance
- Added 7 database indexes for sessions, interactions, and mastery queries
- Cache hit/miss tracking for performance monitoring

## [2.0.0] - 2026-03-07

### Added
- Parallel agent execution (Strategist + Designer)
- Voice Activity Detection (VAD) with Silero WebAssembly
- Neo4j graph context preloading

### Performance
- TTFT (Time To First Token) reduced by ~40%
- ASR computation reduced by 50-70% with VAD

## [1.0.0] - 2026-03-01

### Added
- Multi-agent orchestration (Strategist, Designer, Coach, Critic)
- KA-RAG pipeline with hybrid retrieval (pgvector + Neo4j)
- Bayesian Knowledge Tracing (BKT) with scaffold fading
- 8 pluggable skills (Socratic, Quiz, RolePlay, Fallacy, etc.)
- Teacher intervention (takeover & whisper)
- JWT authentication with RBAC
- Admin dashboard and teacher dashboard
- Student learning interface with WebSocket streaming
- ECharts analytics and knowledge radar
- WeKnora knowledge base integration (optional)
- In-app help center with role-based manuals
- i18n support (zh-CN, en-US)

### Security
- Prompt injection guard (60 keywords + 14 regex patterns)
- PII redactor for sensitive information
- Output safety guardrail

### Infrastructure
- PostgreSQL with pgvector for embeddings
- Neo4j for knowledge graph
- Redis for caching and sessions
- Ollama integration (qwen2.5:7b, bge-m3)
- DashScope and Gemini provider support
