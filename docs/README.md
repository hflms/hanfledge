# Hanfledge documentation

Hanfledge is an AI-native EdTech platform with multi-agent
orchestration, knowledge graphs, and Socratic learning. This directory
contains all project documentation organized by topic.

## Getting started

These guides help you set up and start working with Hanfledge:

| Document | Description |
|----------|-------------|
| [Development Setup](development-setup.md) | Local environment setup, infrastructure services, and first run |
| [Quick Start 2.0](QUICKSTART_2.0.md) | Condensed setup guide for experienced developers |
| [Contributing](../CONTRIBUTING.md) | Contribution workflow, coding standards, and PR guidelines |
| [Testing Guide](testing-guide.md) | Test strategy, patterns, and how to run test suites |

## Architecture and design

These documents explain how Hanfledge is built:

| Document | Description |
|----------|-------------|
| [Architecture](../ARCHITECTURE.md) | System architecture, layer diagram, data flows, and agent pipeline |
| [Database Schema](database-schema.md) | Entity-relationship diagram and model reference for all 14 domain models |
| [API Reference](api-reference.md) | Complete catalog of all 111 REST API endpoints with auth requirements |
| [WebSocket Protocol](websocket-protocol.md) | Real-time streaming protocol for AI dialogue sessions |
| [Security](security.md) | Authentication, authorization, injection guard, and PII redaction |

## Feature documentation

### Skill system

The plugin-based skill system provides 9 built-in instructional
strategies:

| Document | Description |
|----------|-------------|
| [Plugin Development Guide](plugin-development.md) | How to create custom skills with the plugin architecture |
| [Skill Optimization](SKILL_OPTIMIZATION.md) | Performance tuning for the skill execution pipeline |
| [Skill Performance Guide](SKILL_PERFORMANCE_GUIDE.md) | Benchmarks and profiling for skill handlers |
| [Skill Monitoring](SKILL_MONITORING.md) | Monitoring skill usage and error rates in production |
| [Skill Production Deployment](SKILL_PRODUCTION_DEPLOYMENT.md) | Deploying skills to production environments |
| [Skill Refactoring Guide](SKILL_REFACTORING_GUIDE.md) | Guide to the skill system refactoring |
| [Skill Refactoring Summary](SKILL_REFACTORING_SUMMARY.md) | Summary of skill system refactoring changes |

### Soul system

The Soul system manages AI behavior rules that evolve over time:

| Document | Description |
|----------|-------------|
| [Soul System](SOUL_SYSTEM.md) | Architecture, API endpoints, evolution process, and version control |

### WeKnora integration

Optional integration with the WeKnora knowledge base service:

| Document | Description |
|----------|-------------|
| [WeKnora Integration](WEKNORA_INTEGRATION.md) | Full integration guide for WeKnora knowledge base service |
| [WeKnora Quick Start](WEKNORA_QUICKSTART.md) | 5-minute setup for WeKnora integration |
| [WeKnora SSO Summary](WEKNORA_SSO_SUMMARY.md) | Single sign-on implementation details |
| [WeKnora Completion Summary](WEKNORA_COMPLETION_SUMMARY.md) | Feature completion status |

### Presentations

| Document | Description |
|----------|-------------|
| [Presentation Skill Enhancement](PRESENTATION_SKILL_ENHANCEMENT.md) | Presentation generation skill improvements |
| [Presentation Fullscreen](PRESENTATION_FULLSCREEN.md) | Fullscreen presentation mode |

### Other features

| Document | Description |
|----------|-------------|
| [Auto Start Session](FEATURE_AUTO_START_SESSION.md) | Automatic session start behavior |

## User manuals

Role-based guides for end users of the platform:

| Manual | Audience |
|--------|----------|
| [System Admin Manual](manuals/SYS_ADMIN_MANUAL.md) | Platform administrators managing schools, users, and system config |
| [School Admin Manual](manuals/SCHOOL_ADMIN_MANUAL.md) | School administrators managing classes and teachers |
| [Teacher Manual](manuals/TEACHER_MANUAL.md) | Teachers creating courses, activities, and monitoring students |
| [Student Manual](manuals/STUDENT_MANUAL.md) | Students using AI-guided learning sessions |

## Optimization and fixes

Historical records of performance optimizations and bug fixes:

| Document | Description |
|----------|-------------|
| [Optimization Recommendations](OPTIMIZATION_RECOMMENDATIONS.md) | Performance improvement recommendations |
| [Optimization Summary](OPTIMIZATION_SUMMARY.md) | Summary of applied optimizations |
| [Optimization Log](OPTIMIZATION_LOG.md) | Detailed optimization changelog |
| [Knowledge Point Flow Fix](FIX_KNOWLEDGE_POINT_FLOW.md) | Fix for knowledge point navigation flow |
| [Knowledge Point Guidance Fix](FIX_KNOWLEDGE_POINT_GUIDANCE.md) | Fix for knowledge point guidance display |
| [Prerequisite Auto-Insert Fix](FIX_DISABLE_PREREQ_AUTO_INSERT.md) | Fix for disabling automatic prerequisite insertion |

## API specification

Hanfledge provides an OpenAPI (Swagger) specification for the REST API:

| File | Description |
|------|-------------|
| [swagger.yaml](swagger.yaml) | OpenAPI 2.0 specification (YAML format) |
| [swagger.json](swagger.json) | OpenAPI 2.0 specification (JSON format) |

When running the backend in development mode, the Swagger UI is
available at `http://localhost:8080/swagger/index.html`.

## Changelog

See [CHANGELOG.md](../CHANGELOG.md) for a versioned record of all
notable changes to the project.
