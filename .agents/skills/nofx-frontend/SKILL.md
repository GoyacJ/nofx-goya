---
name: nofx-frontend
description: Use this skill when implementing or reviewing NOFX web UI changes in React/TypeScript (web/src).
---

# NOFX Frontend Skill

## Use when
- The task touches files under `web/src/` (components, pages, stores, i18n, types).
- The task changes UI behavior, API usage, or frontend data contracts.

## Workflow
1. Locate impacted surfaces:
   - UI components/pages.
   - Related API client calls and TS interfaces in `web/src/types.ts`.
   - Any required translation keys (`web/src/i18n/`).
2. Implement with minimal diff:
   - Preserve existing design language and interaction patterns.
   - Keep state and side effects localized.
3. Validate:
   - `cd web && npm run lint`
   - `cd web && npm run test`
   - `cd web && npm run build`
4. Cross-check backend contract:
   - If frontend payload/response shapes changed, ensure backend structs/handlers are aligned.

## Repo conventions
- Do not start services with docker compose / docker-compose for this repo.
- Use local dev server: `cd web && npm install && npm run dev`.
- Keep generated or lockfile churn minimal and intentional.

## Done criteria
- Lint/test/build pass.
- No runtime type mismatch against backend API contracts.
- New text has matching i18n entries where needed.
