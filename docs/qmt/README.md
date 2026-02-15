# NOFX QMT Integration (V1)

This document describes the V1 integration path for A-share trading:

- NOFX backend (Go)
- QMT Gateway (Python, Windows)
- Communication: internal HTTP + Bearer token
- Account mode: cash account, long-only
- `position_size_usd` is interpreted as CNY for `exchange=qmt`

## 1. Scope

Supported in V1:

- `open_long`, `close_long`
- account balance and positions
- account-scoped market data from gateway
- rule checks in trader adapter:
  - trading session
  - 100-share lot size
  - daily price-limit checks
  - T+1 sellable quantity

Not supported in V1:

- `open_short`, `close_short`
- leverage/margin mode
- options/futures/margin shorting

## 2. Gateway Runtime

Run on Windows where QMT/XtQuant is available.

No docker-compose is required or used.

### 2.1 Install

```bash
cd gateway/qmt
python -m venv .venv
.venv\Scripts\activate
pip install -r requirements.txt
```

### 2.2 Configure

```bash
copy .env.example .env
```

Set at least:

- `QMT_GATEWAY_TOKEN`
- `QMT_GATEWAY_MODE=mock` (or `xtquant` once XtQuant adapter is implemented)

### 2.3 Start

```bash
cd gateway/qmt
python main.py
```

Default listen address: `0.0.0.0:19090`

## 3. Gateway API

- `GET /health`
- `GET /v1/account/balance?account_id=...`
- `GET /v1/account/positions?account_id=...`
- `GET /v1/market/snapshot?symbol=...`
- `GET /v1/market/klines?symbol=...&interval=...&limit=...`
- `GET /v1/market/symbols?scope=watchlist|sector&sector=...`
- `POST /v1/orders`
- `POST /v1/orders/cancel`

Auth header for all `/v1/*` endpoints:

```text
Authorization: Bearer <QMT_GATEWAY_TOKEN>
```

## 4. NOFX Exchange Config

Create an exchange with:

- `exchange_type`: `qmt`
- `qmt_gateway_url`
- `qmt_account_id`
- `qmt_gateway_token`
- `qmt_market` (default `CN-A`)

## 5. Frontend Market Data

For QMT accounts, use protected endpoints:

- `GET /api/qmt/klines?exchange_id=...&symbol=...&interval=...&limit=...`
- `GET /api/qmt/symbols?exchange_id=...&scope=watchlist|sector&sector=...`

Public `/api/klines` and `/api/symbols` remain for non-QMT exchanges.

## 6. Notes

- The scaffold in `gateway/qmt/main.py` includes a `MockAdapter`.
- `XtQuantAdapter` is intentionally left unimplemented in this step.
- Implement XtQuant binding in `XtQuantAdapter` on Windows before live trading.
