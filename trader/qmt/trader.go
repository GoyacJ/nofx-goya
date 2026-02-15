package qmt

import (
	"context"
	"fmt"
	"math"
	"nofx/logger"
	"nofx/market"
	traderTypes "nofx/trader/types"
	"strconv"
	"strings"
	"sync"
	"time"
)

type snapshotData struct {
	LastPrice  float64
	UpperLimit float64
	LowerLimit float64
}

type cachedOrder struct {
	Symbol     string
	AvgPrice   float64
	Executed   float64
	Commission float64
	Status     string
}

// QMTTrader implements long-only A-share trading through an external QMT gateway.
type QMTTrader struct {
	client    *gatewayClient
	accountID string
	market    string
	nowFn     func() time.Time

	orderMu sync.RWMutex
	orders  map[string]cachedOrder
}

var shanghaiLocation = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}()

func NewQMTTrader(gatewayURL, accountID, gatewayToken, marketName string) (*QMTTrader, error) {
	if strings.TrimSpace(gatewayURL) == "" {
		return nil, fmt.Errorf("qmt gateway url is required")
	}
	if strings.TrimSpace(accountID) == "" {
		return nil, fmt.Errorf("qmt account id is required")
	}
	if strings.TrimSpace(gatewayToken) == "" {
		return nil, fmt.Errorf("qmt gateway token is required")
	}
	if strings.TrimSpace(marketName) == "" {
		marketName = "CN-A"
	}

	return &QMTTrader{
		client:    newGatewayClient(gatewayURL, gatewayToken),
		accountID: accountID,
		market:    marketName,
		nowFn:     time.Now,
		orders:    make(map[string]cachedOrder),
	}, nil
}

func (t *QMTTrader) GetBalance() (map[string]interface{}, error) {
	resp, err := t.client.getBalance(context.Background(), t.accountID)
	if err != nil {
		return nil, err
	}

	available := firstFloat(resp,
		"available_balance", "available", "available_cash", "cash_available", "cash")
	walletBalance := firstFloat(resp,
		"wallet_balance", "cash", "total_cash", "balance")
	marketValue := firstFloat(resp,
		"market_value", "positions_value", "position_market_value")
	unrealized := firstFloat(resp,
		"unrealized_pnl", "float_profit", "position_pnl")
	totalEquity := firstFloat(resp,
		"total_equity", "total_assets", "net_asset", "nav", "equity")

	if totalEquity == 0 {
		totalEquity = walletBalance + unrealized
	}
	if totalEquity == 0 {
		totalEquity = available + marketValue
	}
	if walletBalance == 0 {
		walletBalance = totalEquity - unrealized
		if walletBalance < 0 {
			walletBalance = 0
		}
	}
	if available == 0 {
		available = walletBalance
	}

	return map[string]interface{}{
		"totalEquity":           totalEquity,
		"total_equity":          totalEquity,
		"totalWalletBalance":    walletBalance,
		"wallet_balance":        walletBalance,
		"availableBalance":      available,
		"available_balance":     available,
		"totalUnrealizedProfit": unrealized,
		"unrealized_pnl":        unrealized,
		"balance":               totalEquity,
	}, nil
}

func (t *QMTTrader) GetPositions() ([]map[string]interface{}, error) {
	items, err := t.client.getPositions(context.Background(), t.accountID)
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0, len(items))
	nowMS := t.nowFn().UTC().UnixMilli()

	for _, pos := range items {
		symbolRaw := firstString(pos, "symbol", "code", "stock_code")
		if symbolRaw == "" {
			continue
		}
		symbol := market.NormalizeByExchange("qmt", symbolRaw)

		qtyRaw := firstFloat(pos,
			"positionAmt", "position_amt", "quantity", "qty", "volume", "total_qty")
		side := normalizeSide(firstString(pos, "side", "position_side"), qtyRaw)
		qty := math.Abs(qtyRaw)
		if side == "short" && qtyRaw == 0 {
			qty = firstFloat(pos, "short_qty")
		}

		entryPrice := firstFloat(pos,
			"entryPrice", "entry_price", "cost_price", "avg_price", "open_price")
		markPrice := firstFloat(pos,
			"markPrice", "mark_price", "last_price", "current_price", "price")
		unrealized := firstFloat(pos,
			"unRealizedProfit", "unrealized_pnl", "float_profit", "pnl")
		leverage := firstFloat(pos, "leverage")
		if leverage <= 0 {
			leverage = 1
		}
		liquidationPrice := firstFloat(pos,
			"liquidationPrice", "liquidation_price")
		availableQty, hasAvailableQty := firstFloatWithFound(pos,
			"availableQty", "available_qty", "available_quantity", "sellable_qty", "can_sell", "closeable_amount")
		if availableQty < 0 {
			availableQty = 0
		}
		updateTime := firstInt64(pos, "updateTime", "update_time", "timestamp")
		if updateTime == 0 {
			updateTime = nowMS
		}

		positionAmt := qty
		if side == "short" {
			positionAmt = -qty
		}

		result = append(result, map[string]interface{}{
			"symbol":           symbol,
			"side":             side,
			"entryPrice":       entryPrice,
			"markPrice":        markPrice,
			"positionAmt":      positionAmt,
			"unRealizedProfit": unrealized,
			"leverage":         leverage,
			"liquidationPrice": liquidationPrice,
			"updateTime":       updateTime,
			"availableQty":     availableQty,
			"hasAvailableQty":  hasAvailableQty,
		})
	}

	return result, nil
}

func (t *QMTTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	normalized := market.NormalizeByExchange("qmt", symbol)
	if err := t.ensureTradingSession(); err != nil {
		return nil, err
	}

	snapshot, err := t.getSnapshot(normalized)
	if err != nil {
		return nil, err
	}
	if snapshot.LastPrice <= 0 {
		return nil, fmt.Errorf("invalid market price for %s", normalized)
	}
	if snapshot.UpperLimit > 0 && snapshot.LastPrice >= snapshot.UpperLimit*0.999 {
		return nil, fmt.Errorf("%w (limit-up): %s", ErrPriceLimitReached, normalized)
	}

	shares := normalizeShareLot(quantity)
	if shares < 100 {
		return nil, ErrInvalidLotSize
	}

	clientReqID := fmt.Sprintf("qmt-open-long-%d", time.Now().UnixNano())
	resp, err := t.client.placeOrder(context.Background(), map[string]any{
		"account_id":    t.accountID,
		"market":        t.market,
		"symbol":        normalized,
		"side":          "BUY",
		"quantity":      shares,
		"order_type":    "MARKET",
		"client_req_id": clientReqID,
	})
	if err != nil {
		return nil, err
	}

	orderID := firstString(resp, "orderId", "order_id", "id")
	if orderID == "" {
		orderID = clientReqID
	}
	avgPrice := firstFloat(resp, "avgPrice", "avg_price", "fill_price", "price")
	if avgPrice <= 0 {
		avgPrice = snapshot.LastPrice
	}
	commission := firstFloat(resp, "commission", "fee")

	t.cacheOrder(orderID, normalized, float64(shares), avgPrice, commission)

	return map[string]interface{}{
		"orderId":     orderID,
		"status":      "FILLED",
		"symbol":      normalized,
		"executedQty": float64(shares),
		"avgPrice":    avgPrice,
		"commission":  commission,
	}, nil
}

func (t *QMTTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return nil, fmt.Errorf("%w: open_short", ErrUnsupportedForCashAccount)
}

func (t *QMTTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	normalized := market.NormalizeByExchange("qmt", symbol)
	if err := t.ensureTradingSession(); err != nil {
		return nil, err
	}

	snapshot, err := t.getSnapshot(normalized)
	if err != nil {
		return nil, err
	}
	if snapshot.LowerLimit > 0 && snapshot.LastPrice <= snapshot.LowerLimit*1.001 {
		return nil, fmt.Errorf("%w (limit-down): %s", ErrPriceLimitReached, normalized)
	}

	positions, err := t.GetPositions()
	if err != nil {
		return nil, err
	}

	var position map[string]interface{}
	for _, pos := range positions {
		if toString(pos["symbol"]) == normalized && strings.EqualFold(toString(pos["side"]), "long") {
			position = pos
			break
		}
	}

	if position == nil {
		return map[string]interface{}{
			"status": "NO_POSITION",
			"symbol": normalized,
		}, nil
	}

	available := toFloat(position["availableQty"])
	hasAvailableQty, _ := position["hasAvailableQty"].(bool)
	if !hasAvailableQty {
		available = math.Max(0, toFloat(position["positionAmt"]))
	}
	if available < 100 {
		return nil, fmt.Errorf("%w: %s", ErrTPlusOneRestricted, normalized)
	}

	sellQty := available
	if quantity > 0 {
		sellQty = math.Min(quantity, available)
	}
	shares := normalizeShareLot(sellQty)
	if shares < 100 {
		return nil, ErrInvalidLotSize
	}

	clientReqID := fmt.Sprintf("qmt-close-long-%d", time.Now().UnixNano())
	resp, err := t.client.placeOrder(context.Background(), map[string]any{
		"account_id":    t.accountID,
		"market":        t.market,
		"symbol":        normalized,
		"side":          "SELL",
		"quantity":      shares,
		"order_type":    "MARKET",
		"client_req_id": clientReqID,
	})
	if err != nil {
		return nil, err
	}

	orderID := firstString(resp, "orderId", "order_id", "id")
	if orderID == "" {
		orderID = clientReqID
	}
	avgPrice := firstFloat(resp, "avgPrice", "avg_price", "fill_price", "price")
	if avgPrice <= 0 {
		avgPrice = snapshot.LastPrice
	}
	commission := firstFloat(resp, "commission", "fee")

	t.cacheOrder(orderID, normalized, float64(shares), avgPrice, commission)

	return map[string]interface{}{
		"orderId":     orderID,
		"status":      "FILLED",
		"symbol":      normalized,
		"executedQty": float64(shares),
		"avgPrice":    avgPrice,
		"commission":  commission,
	}, nil
}

func (t *QMTTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	return nil, fmt.Errorf("%w: close_short", ErrUnsupportedForCashAccount)
}

func (t *QMTTrader) SetLeverage(symbol string, leverage int) error {
	return fmt.Errorf("%w: leverage", ErrUnsupportedForCashAccount)
}

func (t *QMTTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	return fmt.Errorf("%w: margin mode", ErrUnsupportedForCashAccount)
}

func (t *QMTTrader) GetMarketPrice(symbol string) (float64, error) {
	normalized := market.NormalizeByExchange("qmt", symbol)
	snapshot, err := t.getSnapshot(normalized)
	if err != nil {
		return 0, err
	}
	if snapshot.LastPrice <= 0 {
		return 0, fmt.Errorf("invalid market price for %s", normalized)
	}
	return snapshot.LastPrice, nil
}

func (t *QMTTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	return fmt.Errorf("%w: stop loss", ErrUnsupportedForCashAccount)
}

func (t *QMTTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	return fmt.Errorf("%w: take profit", ErrUnsupportedForCashAccount)
}

func (t *QMTTrader) CancelStopLossOrders(symbol string) error {
	return fmt.Errorf("%w: cancel stop loss", ErrUnsupportedForCashAccount)
}

func (t *QMTTrader) CancelTakeProfitOrders(symbol string) error {
	return fmt.Errorf("%w: cancel take profit", ErrUnsupportedForCashAccount)
}

func (t *QMTTrader) CancelAllOrders(symbol string) error {
	return nil
}

func (t *QMTTrader) CancelStopOrders(symbol string) error {
	return fmt.Errorf("%w: cancel stop orders", ErrUnsupportedForCashAccount)
}

func (t *QMTTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	shares := normalizeShareLot(quantity)
	if shares < 100 {
		return "", ErrInvalidLotSize
	}
	return strconv.Itoa(shares), nil
}

func (t *QMTTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	key := strings.TrimSpace(orderID)
	if key == "" {
		return nil, fmt.Errorf("order id is required")
	}

	t.orderMu.RLock()
	cached, ok := t.orders[key]
	t.orderMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("order %s not found in local cache", key)
	}

	return map[string]interface{}{
		"status":      cached.Status,
		"avgPrice":    cached.AvgPrice,
		"executedQty": cached.Executed,
		"commission":  cached.Commission,
		"symbol":      cached.Symbol,
	}, nil
}

func (t *QMTTrader) GetClosedPnL(startTime time.Time, limit int) ([]traderTypes.ClosedPnLRecord, error) {
	return []traderTypes.ClosedPnLRecord{}, nil
}

func (t *QMTTrader) GetOpenOrders(symbol string) ([]traderTypes.OpenOrder, error) {
	return []traderTypes.OpenOrder{}, nil
}

func (t *QMTTrader) GetKlines(symbol, interval string, limit int) ([]market.Kline, error) {
	normalized := market.NormalizeByExchange("qmt", symbol)
	return t.client.getKlines(context.Background(), normalized, interval, limit)
}

func (t *QMTTrader) GetSymbols(scope, sector string) ([]string, error) {
	symbols, err := t.client.getSymbols(context.Background(), scope, sector)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	result := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		normalized := market.NormalizeByExchange("qmt", symbol)
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		result = append(result, normalized)
	}
	return result, nil
}

func (t *QMTTrader) cacheOrder(orderID, symbol string, qty, price, commission float64) {
	t.orderMu.Lock()
	defer t.orderMu.Unlock()
	if orderID == "" {
		return
	}
	t.orders[orderID] = cachedOrder{
		Symbol:     symbol,
		AvgPrice:   price,
		Executed:   qty,
		Commission: commission,
		Status:     "FILLED",
	}
}

func (t *QMTTrader) getSnapshot(symbol string) (snapshotData, error) {
	resp, err := t.client.getSnapshot(context.Background(), symbol)
	if err != nil {
		return snapshotData{}, err
	}

	s := snapshotData{
		LastPrice:  firstFloat(resp, "last_price", "current_price", "price", "close"),
		UpperLimit: firstFloat(resp, "upper_limit", "limit_up", "up_limit"),
		LowerLimit: firstFloat(resp, "lower_limit", "limit_down", "down_limit"),
	}

	if s.LastPrice <= 0 {
		logger.Warnf("[QMT] snapshot missing price for %s: %+v", symbol, resp)
	}
	return s, nil
}

func (t *QMTTrader) ensureTradingSession() error {
	now := t.nowFn().In(shanghaiLocation)
	weekday := now.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return ErrOutsideTradingSession
	}

	hm := now.Hour()*60 + now.Minute()
	inMorning := hm >= (9*60+30) && hm < (11*60+30)
	inAfternoon := hm >= (13*60) && hm < (15*60)
	if !inMorning && !inAfternoon {
		return ErrOutsideTradingSession
	}

	return nil
}

func normalizeShareLot(quantity float64) int {
	if quantity <= 0 {
		return 0
	}
	return int(math.Floor(quantity/100.0) * 100)
}

func normalizeSide(raw string, qty float64) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	switch raw {
	case "long", "buy", "b", "1", "net_long":
		return "long"
	case "short", "sell", "s", "-1", "net_short":
		return "short"
	}
	if qty < 0 {
		return "short"
	}
	return "long"
}

func firstFloat(m map[string]any, keys ...string) float64 {
	value, _ := firstFloatWithFound(m, keys...)
	return value
}

func firstFloatWithFound(m map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		if key == "" {
			continue
		}
		if val, ok := m[key]; ok {
			return toFloat(val), true
		}
	}
	return 0, false
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if key == "" {
			continue
		}
		if val, ok := m[key]; ok {
			s := toString(val)
			if s != "" {
				return s
			}
		}
	}
	return ""
}

func firstInt64(m map[string]any, keys ...string) int64 {
	for _, key := range keys {
		if key == "" {
			continue
		}
		if val, ok := m[key]; ok {
			n := toInt64(val)
			if n != 0 {
				return n
			}
		}
	}
	return 0
}
