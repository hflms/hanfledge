# Hanfledge

## What This Is

Hanfledge is an AI-native EdTech platform with a Go backend and a Next.js
frontend. It supports school operations and learning workflows including
authentication, teaching tools, and AI-assisted study experiences.

## Core Value

The platform must remain usable end to end for core education workflows without
regressions in the main web app.

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] Keep the frontend buildable and free of blocking TypeScript or syntax
  errors.
- [ ] Preserve the existing authentication and dashboard flows for the current
  web app.
- [ ] Maintain project conventions for the Go backend and Next.js frontend.

### Out of Scope

- Native mobile clients — the current product is web-first.
- Design system replacement — current work should follow the existing frontend
  structure.

## Context

- Backend uses Go, Gin, GORM, PostgreSQL, Neo4j, Redis, and Ollama.
- Frontend uses Next.js App Router, React 19, TypeScript, and CSS Modules.
- The repository already contains active application code and documentation but
  had not yet been initialized as a GSD project.

## Constraints

- **Tech stack**: Keep the current Go and Next.js architecture — changes must
  fit existing patterns.
- **Quality**: Avoid blocking compile errors in the frontend — they stop local
  development and validation.
- **Process**: Quick-task tracking now uses GSD artifacts under `.planning/`.

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Initialize minimal GSD planning docs before quick-task execution | The quick workflow requires ROADMAP.md and STATE.md before it can run | ✓ Good |

---
*Last updated: 2026-04-12 after initializing GSD quick-task support*
