---
name: nofx-backend
description: Use this skill when implementing or reviewing Go backend changes in NOFX (api, trader, store, market, manager, kernel).
---

# NOFX Backend Skill

## Use when
- The task touches Go backend code under `api/`, `trader/`, `store/`, `market/`, `manager/`, or `kernel/`.
- The task adds or changes API endpoints, exchange adapters, persistence models, or strategy execution logic.

## Workflow
1. Scope first:
   - Identify the exact package(s) and endpoint(s) impacted.
   - Confirm whether frontend contract sync is required (`web/src/types.ts`).
2. Implement minimally:
   - Keep behavior-local changes.
   - Avoid broad refactors unless explicitly requested.
3. Contract sync:
   - If JSON fields or response shapes changed in Go, update frontend type definitions in the same change.
4. Validate:
   - `go fmt ./...`
   - `go test ./...`
   - If API contract changed, run frontend build checks.

## Repo conventions
- Do not use docker compose / docker-compose for execution in this repo.
- Prefer local run:
  - Backend: `go run main.go`
  - Frontend: `cd web && npm install && npm run dev`
- Never commit secrets or raw credentials.

## Done criteria
- Build/test passes for affected scope.
- API shape and frontend types remain consistent.
- No unrelated files changed.
