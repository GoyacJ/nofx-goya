# AGENTS.md

## Scope
This file applies to the entire repository.

## Project Context
- Backend: Go (`main.go`, `api/`, `trader/`, `store/`, `market/`).
- Frontend: React + TypeScript (`web/`).
- Database: SQLite/Postgres via GORM.
- Multi-market support exists in code paths; avoid exchange-specific regressions.

## Working Agreements
1. Keep changes minimal and task-scoped. Avoid unrelated refactors.
2. If backend API contracts change, update frontend types in `web/src/types.ts` in the same change.
3. Preserve backward compatibility unless the user explicitly asks for breaking changes.
4. Do not use `docker compose` / `docker-compose` for local development in this repository.

## Local Run (No Docker Compose)
- Backend: `go run main.go`
- Frontend: `cd web && npm install && npm run dev`

## Validation Checklist
- Go changes:
  - `go fmt ./...`
  - `go test ./...`
- Frontend changes:
  - `cd web && npm run lint`
  - `cd web && npm run test`
  - `cd web && npm run build`

## Git and Push Policy
1. Do not push to upstream `origin`.
2. Push to fork remote `goya` (user repository) only.
3. Prefer non-destructive git operations.
4. Never run destructive commands (`git reset --hard`, `git checkout --`, `git clean -fd`) unless explicitly requested.

## Security
- Never commit secrets (`.env`, API keys, private keys).
- Keep sensitive values masked in logs, examples, and screenshots.
