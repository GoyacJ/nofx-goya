# NOFX OpenClaw Integration (V1 + V2)

## 1. Overview

NOFX integrates OpenClaw as a unified AI control plane in two phases:

- **V1 (Gateway-first):** Route AI inference through OpenClaw across trader runtime, strategy test, debate, and backtest.
- **V2 (Governance):** Add webhook ingestion, risk classification, approval workflow, and auditable execution records.

Design goals:

1. Minimal schema impact for V1 (reuse existing AI model config fields).
2. Explicit configuration (no hidden defaults for OpenClaw base URL/model).
3. Fail-closed behavior for trading decisions when OpenClaw is unavailable.
4. Backward compatibility with existing providers.

## 2. V1 Scope

### 2.1 Provider and model config

OpenClaw is added as provider ID `openclaw` in:

- `/api/supported-models`
- `/api/models` default list
- front-end model config UI and provider rendering

Validation rule:

- If `provider=openclaw` and `enabled=true`, final merged config must include:
  - `api_key`
  - `custom_api_url`
  - `custom_model_name`

### 2.2 Runtime chains covered

OpenClaw provider branch is wired into:

1. Trader runtime (`AutoTrader`)
2. Strategy real-AI test
3. Debate engine client init
4. Backtest AI client

### 2.3 MCP client behavior

`OpenClawClient` is implemented as OpenAI-compatible transport:

- Bearer auth
- explicit provider `openclaw`
- base URL normalization to `/v1` when root endpoint is provided
- supports full-URL mode (`#` suffix convention)

## 3. V2 Scope

### 3.1 Webhook + governance APIs

Added endpoints:

- `POST /api/openclaw/webhooks/events`
- `GET /api/openclaw/approvals`
- `POST /api/openclaw/approvals/:id/approve`
- `POST /api/openclaw/approvals/:id/reject`

Webhook behavior:

1. Signature verification (HMAC-SHA256) with replay-window timestamp checks.
2. Event idempotency by `event_id`.
3. `tool.call.requested` is risk-classified:
   - `read_only`
   - `write_low_risk`
   - `write_high_risk`
4. High-risk requests create pending approvals; lower-risk requests create execution records directly.

### 3.2 Data model (V2)

New tables:

1. `openclaw_events`
2. `openclaw_approval_requests`
3. `openclaw_tool_executions`

Status model:

- Approval: `pending_approval -> approved/rejected` (plus reserved executed/failed states)
- Execution: `requested/approved/executed/rejected/failed`

### 3.3 Approval semantics

- Approve/reject actions are user-scoped.
- Reject requires a reason.
- Approve/reject decisions are persisted and mirrored into execution audit records.
- On approve, NOFX now attempts immediate tool execution using trader context:
  - `open_long` / `open_short`
  - `close_long` / `close_short`
  - `set_leverage` / `set_margin_mode`
  and updates execution audit status to `executed` or `failed`.

## 4. Frontend Changes

OpenClaw is visible end-to-end in UI:

1. AI provider cards and icons (`openclaw.svg`)
2. Model name aliases in trader/debate pages
3. i18n notes and input hints
4. Required-field validation for OpenClaw base URL/model
5. OpenClaw webhook secret input in model config UI (stored encrypted)

## 5. Security and Failure Policy

1. API key storage continues to use existing encrypted field flow.
2. Sensitive values are never returned in plaintext model list responses.
3. Webhook signature secret resolution order:
   - user-level OpenClaw config (`webhook_secret`) first
   - environment fallback `OPENCLAW_WEBHOOK_SECRET` second
4. OpenClaw runtime misconfiguration or request failure prevents decision execution for that cycle (fail-closed at decision stage).

## 6. Test Coverage

Implemented tests include:

1. Supported/default model lists include OpenClaw.
2. Model update validation for OpenClaw required fields.
3. OpenClaw MCP client URL/model/provider behavior.
4. Backtest OpenClaw config success/failure paths.
5. Webhook signature/risk classification.
6. Webhook high-risk approval creation and idempotency.
7. Approval approve/reject status transitions and execution record creation.
8. User-scoped approval listing.

## 7. Milestone View

- **M1 (V1 backend):** complete
- **M2 (V1 frontend):** complete
- **M3 (V1 production hardening):** pending rollout policy/config toggles
- **M4 (V2 workflow):** foundational APIs and data model complete
- **M5 (V2 production):** pending tool bridge execution wiring and ops policy

## 8. Notes for Next Iteration

Recommended next steps:

1. Add a dedicated Tool Bridge executor to bind approved requests to actual NOFX trading actions.
2. Add execution idempotency keys for tool executions.
3. Add index tuning for large audit datasets (by `user_id`, `trader_id`, `event_type`, `created_at`).
4. Add observability dashboard panels for approval latency and reject reasons.
