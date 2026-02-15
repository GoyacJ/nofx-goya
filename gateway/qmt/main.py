from __future__ import annotations

import os
import time
import uuid
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

from fastapi import Depends, FastAPI, Header, HTTPException, Query
from pydantic import BaseModel, Field


# NOTE:
# - This file provides a production-shaped gateway skeleton.
# - Replace MockAdapter with a real XtQuant adapter for live trading.


@dataclass
class Settings:
    token: str
    mode: str


class BalanceResponse(BaseModel):
    total_equity: float
    available_balance: float
    wallet_balance: float
    unrealized_pnl: float = 0
    market_value: float = 0


class PositionResponse(BaseModel):
    symbol: str
    quantity: float
    available_qty: float
    entry_price: float
    last_price: float
    unrealized_pnl: float = 0


class SnapshotResponse(BaseModel):
    symbol: str
    last_price: float
    upper_limit: float = 0
    lower_limit: float = 0


class KlineItem(BaseModel):
    openTime: int
    closeTime: int
    open: float
    high: float
    low: float
    close: float
    volume: float = 0
    quoteVolume: float = 0


class OrderRequest(BaseModel):
    account_id: str = Field(..., description="QMT account id")
    market: str = Field(default="CN-A")
    symbol: str
    side: str = Field(..., description="BUY or SELL")
    quantity: int = Field(..., ge=1)
    order_type: str = Field(default="MARKET")
    client_req_id: str


class CancelOrderRequest(BaseModel):
    account_id: str
    order_id: str


class GatewayAdapter:
    def get_balance(self, account_id: str) -> Dict[str, Any]:
        raise NotImplementedError

    def get_positions(self, account_id: str) -> List[Dict[str, Any]]:
        raise NotImplementedError

    def get_snapshot(self, symbol: str) -> Dict[str, Any]:
        raise NotImplementedError

    def get_klines(self, symbol: str, interval: str, limit: int) -> List[Dict[str, Any]]:
        raise NotImplementedError

    def get_symbols(self, scope: str, sector: Optional[str]) -> List[str]:
        raise NotImplementedError

    def place_order(self, req: OrderRequest) -> Dict[str, Any]:
        raise NotImplementedError

    def cancel_order(self, req: CancelOrderRequest) -> Dict[str, Any]:
        raise NotImplementedError


class MockAdapter(GatewayAdapter):
    def __init__(self) -> None:
        self._orders: Dict[str, Dict[str, Any]] = {}

    def get_balance(self, account_id: str) -> Dict[str, Any]:
        return BalanceResponse(
            total_equity=1_000_000,
            available_balance=650_000,
            wallet_balance=650_000,
            market_value=350_000,
        ).model_dump()

    def get_positions(self, account_id: str) -> List[Dict[str, Any]]:
        return [
            PositionResponse(
                symbol="600519.SH",
                quantity=500,
                available_qty=300,
                entry_price=1680.0,
                last_price=1698.0,
                unrealized_pnl=9000.0,
            ).model_dump()
        ]

    def get_snapshot(self, symbol: str) -> Dict[str, Any]:
        return SnapshotResponse(
            symbol=symbol,
            last_price=1698.0,
            upper_limit=1867.8,
            lower_limit=1528.2,
        ).model_dump()

    def get_klines(self, symbol: str, interval: str, limit: int) -> List[Dict[str, Any]]:
        now_ms = int(datetime.now(tz=timezone.utc).timestamp() * 1000)
        step_ms = 5 * 60 * 1000
        price = 1698.0
        out: List[Dict[str, Any]] = []
        for i in range(limit):
            ts = now_ms - step_ms * (limit - i)
            open_px = price
            close_px = price + (0.8 if i % 2 == 0 else -0.5)
            high_px = max(open_px, close_px) + 0.4
            low_px = min(open_px, close_px) - 0.4
            out.append(
                KlineItem(
                    openTime=ts,
                    closeTime=ts + step_ms - 1,
                    open=open_px,
                    high=high_px,
                    low=low_px,
                    close=close_px,
                    volume=1200 + i,
                    quoteVolume=(1200 + i) * close_px,
                ).model_dump()
            )
            price = close_px
        return out

    def get_symbols(self, scope: str, sector: Optional[str]) -> List[str]:
        if scope == "sector" and sector:
            if sector.lower() == "liquor":
                return ["600519.SH", "000858.SZ"]
            if sector.lower() == "bank":
                return ["600036.SH", "601398.SH", "000001.SZ"]
        return ["600519.SH", "000001.SZ", "300750.SZ", "601318.SH"]

    def place_order(self, req: OrderRequest) -> Dict[str, Any]:
        order_id = str(uuid.uuid4())
        order = {
            "order_id": order_id,
            "status": "FILLED",
            "symbol": req.symbol,
            "side": req.side,
            "quantity": req.quantity,
            "avg_price": self.get_snapshot(req.symbol)["last_price"],
            "commission": 0.0,
            "client_req_id": req.client_req_id,
            "created_at": int(time.time() * 1000),
        }
        self._orders[order_id] = order
        return order

    def cancel_order(self, req: CancelOrderRequest) -> Dict[str, Any]:
        if req.order_id not in self._orders:
            return {"status": "NOT_FOUND", "order_id": req.order_id}
        self._orders[req.order_id]["status"] = "CANCELED"
        return {"status": "CANCELED", "order_id": req.order_id}


class XtQuantAdapter(GatewayAdapter):
    """
    Placeholder for real XtQuant integration.

    Implement this adapter on Windows with xtquant and switch `QMT_GATEWAY_MODE=xtquant`.
    """

    def __init__(self) -> None:
        raise NotImplementedError("XtQuant adapter is not implemented in this scaffold")


def load_settings() -> Settings:
    token = os.getenv("QMT_GATEWAY_TOKEN", "")
    mode = os.getenv("QMT_GATEWAY_MODE", "mock").lower().strip() or "mock"
    return Settings(token=token, mode=mode)


settings = load_settings()

if settings.mode == "xtquant":
    adapter: GatewayAdapter = XtQuantAdapter()
else:
    adapter = MockAdapter()


app = FastAPI(title="NOFX QMT Gateway", version="0.1.0")


def require_bearer(authorization: Optional[str] = Header(default=None)) -> None:
    if not settings.token:
        # Dev mode: when token is empty, skip auth.
        return
    if not authorization or not authorization.startswith("Bearer "):
        raise HTTPException(status_code=401, detail="missing bearer token")
    token = authorization.removeprefix("Bearer ").strip()
    if token != settings.token:
        raise HTTPException(status_code=401, detail="invalid bearer token")


@app.get("/health")
def health() -> Dict[str, Any]:
    return {
        "status": "ok",
        "mode": settings.mode,
        "time": int(time.time()),
    }


@app.get("/v1/account/balance", dependencies=[Depends(require_bearer)])
def get_balance(account_id: str = Query(...)) -> Dict[str, Any]:
    return {"data": adapter.get_balance(account_id)}


@app.get("/v1/account/positions", dependencies=[Depends(require_bearer)])
def get_positions(account_id: str = Query(...)) -> Dict[str, Any]:
    return {"data": {"positions": adapter.get_positions(account_id)}}


@app.get("/v1/market/snapshot", dependencies=[Depends(require_bearer)])
def get_snapshot(symbol: str = Query(...)) -> Dict[str, Any]:
    return {"data": adapter.get_snapshot(symbol)}


@app.get("/v1/market/klines", dependencies=[Depends(require_bearer)])
def get_klines(
    symbol: str = Query(...),
    interval: str = Query("5m"),
    limit: int = Query(500, ge=1, le=2000),
) -> Dict[str, Any]:
    return {"data": {"klines": adapter.get_klines(symbol, interval, limit)}}


@app.get("/v1/market/symbols", dependencies=[Depends(require_bearer)])
def get_symbols(
    scope: str = Query("watchlist", pattern="^(watchlist|sector)$"),
    sector: Optional[str] = Query(None),
) -> Dict[str, Any]:
    return {"data": {"symbols": adapter.get_symbols(scope, sector)}}


@app.post("/v1/orders", dependencies=[Depends(require_bearer)])
def place_order(req: OrderRequest) -> Dict[str, Any]:
    return {"data": adapter.place_order(req)}


@app.post("/v1/orders/cancel", dependencies=[Depends(require_bearer)])
def cancel_order(req: CancelOrderRequest) -> Dict[str, Any]:
    return {"data": adapter.cancel_order(req)}


if __name__ == "__main__":
    import uvicorn

    uvicorn.run("main:app", host="0.0.0.0", port=19090, reload=False)
