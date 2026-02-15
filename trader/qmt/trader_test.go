package qmt

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type qmtTestServer struct {
	server      *httptest.Server
	orderBodies []map[string]any
}

func newQMTTestServer(balanceResp any, positionsResp any, snapshotResp any, orderResp any) *qmtTestServer {
	state := &qmtTestServer{}
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/account/balance", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, balanceResp)
	})
	mux.HandleFunc("/v1/account/positions", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, positionsResp)
	})
	mux.HandleFunc("/v1/market/snapshot", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, snapshotResp)
	})
	mux.HandleFunc("/v1/orders", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		state.orderBodies = append(state.orderBodies, body)
		writeJSON(w, orderResp)
	})

	state.server = httptest.NewServer(mux)
	return state
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	if payload == nil {
		payload = map[string]any{}
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func fixedTradingTime() time.Time {
	// Monday 10:00 Asia/Shanghai
	return time.Date(2026, 1, 5, 10, 0, 0, 0, shanghaiLocation)
}

func TestQMTTraderOpenLongSuccess(t *testing.T) {
	ts := newQMTTestServer(
		map[string]any{"data": map[string]any{"total_equity": 100000, "available_balance": 80000}},
		map[string]any{"data": map[string]any{"positions": []any{}}},
		map[string]any{"data": map[string]any{"symbol": "600519.SH", "last_price": 10.0, "upper_limit": 11.0, "lower_limit": 9.0}},
		map[string]any{"data": map[string]any{"order_id": "OID-1", "avg_price": 10.2, "commission": 1.5}},
	)
	defer ts.server.Close()

	tr, err := NewQMTTrader(ts.server.URL, "acct-1", "token-1", "CN-A")
	if err != nil {
		t.Fatalf("NewQMTTrader error: %v", err)
	}
	tr.nowFn = fixedTradingTime

	result, err := tr.OpenLong("600519.SH", 250, 1)
	if err != nil {
		t.Fatalf("OpenLong error: %v", err)
	}

	if got := result["orderId"]; got != "OID-1" {
		t.Fatalf("unexpected orderId: %v", got)
	}

	if got := result["executedQty"].(float64); got != 200 {
		t.Fatalf("expected executedQty=200, got %.0f", got)
	}

	if len(ts.orderBodies) != 1 {
		t.Fatalf("expected one order request, got %d", len(ts.orderBodies))
	}

	if qty := int(toFloat(ts.orderBodies[0]["quantity"])); qty != 200 {
		t.Fatalf("expected order quantity=200, got %d", qty)
	}
}

func TestQMTTraderOpenLongRejectsSmallLot(t *testing.T) {
	ts := newQMTTestServer(
		nil,
		nil,
		map[string]any{"data": map[string]any{"symbol": "600519.SH", "last_price": 10.0, "upper_limit": 11.0, "lower_limit": 9.0}},
		nil,
	)
	defer ts.server.Close()

	tr, err := NewQMTTrader(ts.server.URL, "acct-1", "token-1", "CN-A")
	if err != nil {
		t.Fatalf("NewQMTTrader error: %v", err)
	}
	tr.nowFn = fixedTradingTime

	_, err = tr.OpenLong("600519.SH", 99, 1)
	if !errors.Is(err, ErrInvalidLotSize) {
		t.Fatalf("expected ErrInvalidLotSize, got %v", err)
	}
}

func TestQMTTraderOpenShortUnsupported(t *testing.T) {
	ts := newQMTTestServer(nil, nil, nil, nil)
	defer ts.server.Close()

	tr, err := NewQMTTrader(ts.server.URL, "acct-1", "token-1", "CN-A")
	if err != nil {
		t.Fatalf("NewQMTTrader error: %v", err)
	}

	_, err = tr.OpenShort("600519.SH", 100, 1)
	if !errors.Is(err, ErrUnsupportedForCashAccount) {
		t.Fatalf("expected ErrUnsupportedForCashAccount, got %v", err)
	}
}

func TestQMTTraderCloseLongRejectsTPlusOne(t *testing.T) {
	ts := newQMTTestServer(
		nil,
		map[string]any{"data": map[string]any{"positions": []any{
			map[string]any{
				"symbol":         "600519.SH",
				"quantity":       500,
				"available_qty":  0,
				"entry_price":    10.0,
				"last_price":     10.1,
				"unrealized_pnl": 50.0,
			},
		}}},
		map[string]any{"data": map[string]any{"symbol": "600519.SH", "last_price": 10.1, "upper_limit": 11.0, "lower_limit": 9.0}},
		nil,
	)
	defer ts.server.Close()

	tr, err := NewQMTTrader(ts.server.URL, "acct-1", "token-1", "CN-A")
	if err != nil {
		t.Fatalf("NewQMTTrader error: %v", err)
	}
	tr.nowFn = fixedTradingTime

	_, err = tr.CloseLong("600519.SH", 0)
	if !errors.Is(err, ErrTPlusOneRestricted) {
		t.Fatalf("expected ErrTPlusOneRestricted, got %v", err)
	}
}
