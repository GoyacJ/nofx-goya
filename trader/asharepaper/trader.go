package asharepaper

import (
	"encoding/json"
	"fmt"
	"math"
	ashareprovider "nofx/provider/ashare"
	"nofx/trader/types"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type snapshotProvider interface {
	GetSnapshot(symbol string) (*ashareprovider.Snapshot, string, error)
}

type positionState struct {
	Symbol       string
	Quantity     int64
	AveragePrice float64
	TodayBuyQty  int64
	LastBuyDate  string
}

type orderState struct {
	Symbol     string
	Status     string
	Executed   float64
	AvgPrice   float64
	Commission float64
}

// ASharePaperTrader provides local A-share simulation trading (long-only).
type ASharePaperTrader struct {
	provider  snapshotProvider
	accountID string
	market    string
	statePath string

	mu        sync.RWMutex
	cash      float64
	positions map[string]*positionState
	orders    map[string]orderState

	nowFn func() time.Time
}

var shanghaiLocation = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}()

func NewASharePaperTrader(tushareToken, dataMode, watchlist, marketName, accountID string) (*ASharePaperTrader, error) {
	if strings.TrimSpace(marketName) == "" {
		marketName = ashareprovider.DefaultMarket
	}
	if strings.TrimSpace(accountID) == "" {
		accountID = "ashare-paper"
	}

	provider := ashareprovider.NewProvider(tushareToken, dataMode, watchlist)
	trader := &ASharePaperTrader{
		provider:  provider,
		accountID: accountID,
		market:    marketName,
		statePath: buildStateFilePath(accountID),
		cash:      1_000_000, // default virtual CNY balance
		positions: make(map[string]*positionState),
		orders:    make(map[string]orderState),
		nowFn:     time.Now,
	}
	trader.loadState()
	return trader, nil
}

func (t *ASharePaperTrader) GetBalance() (map[string]interface{}, error) {
	t.mu.RLock()
	cash := t.cash
	positions := make([]*positionState, 0, len(t.positions))
	for _, p := range t.positions {
		cp := *p
		positions = append(positions, &cp)
	}
	t.mu.RUnlock()

	marketValue := 0.0
	for _, p := range positions {
		price, err := t.GetMarketPrice(p.Symbol)
		if err != nil || price <= 0 {
			price = p.AveragePrice
		}
		marketValue += float64(p.Quantity) * price
	}

	totalEquity := cash + marketValue
	return map[string]interface{}{
		"totalEquity":           totalEquity,
		"total_equity":          totalEquity,
		"totalWalletBalance":    cash,
		"wallet_balance":        cash,
		"availableBalance":      cash,
		"available_balance":     cash,
		"totalUnrealizedProfit": 0.0,
		"unrealized_pnl":        0.0,
		"balance":               totalEquity,
		"market_value":          marketValue,
	}, nil
}

func (t *ASharePaperTrader) GetPositions() ([]map[string]interface{}, error) {
	t.mu.Lock()
	t.rollToNextTradingDayLocked()
	positions := make([]*positionState, 0, len(t.positions))
	for _, p := range t.positions {
		cp := *p
		positions = append(positions, &cp)
	}
	t.mu.Unlock()

	result := make([]map[string]interface{}, 0, len(positions))
	nowMS := t.nowFn().UTC().UnixMilli()
	for _, pos := range positions {
		price, err := t.GetMarketPrice(pos.Symbol)
		if err != nil || price <= 0 {
			price = pos.AveragePrice
		}
		availableQty := pos.Quantity - pos.TodayBuyQty
		if availableQty < 0 {
			availableQty = 0
		}

		result = append(result, map[string]interface{}{
			"symbol":           pos.Symbol,
			"side":             "long",
			"entryPrice":       pos.AveragePrice,
			"markPrice":        price,
			"positionAmt":      float64(pos.Quantity),
			"unRealizedProfit": (price - pos.AveragePrice) * float64(pos.Quantity),
			"leverage":         1.0,
			"liquidationPrice": 0.0,
			"updateTime":       nowMS,
			"availableQty":     float64(availableQty),
			"hasAvailableQty":  true,
		})
	}
	return result, nil
}

func (t *ASharePaperTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	normalized := ashareprovider.NormalizeSymbol(symbol)
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

	price := snapshot.LastPrice
	cost := float64(shares) * price

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.cash < cost {
		return nil, fmt.Errorf("insufficient cash: need %.2f, available %.2f", cost, t.cash)
	}
	t.rollToNextTradingDayLocked()

	pos, ok := t.positions[normalized]
	if !ok {
		pos = &positionState{Symbol: normalized}
		t.positions[normalized] = pos
	}
	totalCost := pos.AveragePrice*float64(pos.Quantity) + cost
	pos.Quantity += int64(shares)
	if pos.Quantity > 0 {
		pos.AveragePrice = totalCost / float64(pos.Quantity)
	}
	pos.TodayBuyQty += int64(shares)
	pos.LastBuyDate = t.nowFn().In(shanghaiLocation).Format("2006-01-02")
	t.cash -= cost

	orderID := fmt.Sprintf("ashare-open-long-%d", time.Now().UnixNano())
	t.orders[orderID] = orderState{
		Symbol:     normalized,
		Status:     "FILLED",
		Executed:   float64(shares),
		AvgPrice:   price,
		Commission: 0,
	}
	t.saveStateLocked()

	return map[string]interface{}{
		"orderId":     orderID,
		"status":      "FILLED",
		"symbol":      normalized,
		"executedQty": float64(shares),
		"avgPrice":    price,
		"commission":  0.0,
	}, nil
}

func (t *ASharePaperTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return nil, fmt.Errorf("%w: open_short", ErrUnsupportedForCashAccount)
}

func (t *ASharePaperTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	normalized := ashareprovider.NormalizeSymbol(symbol)
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

	t.mu.Lock()
	defer t.mu.Unlock()
	t.rollToNextTradingDayLocked()

	pos, ok := t.positions[normalized]
	if !ok || pos.Quantity <= 0 {
		return map[string]interface{}{
			"status": "NO_POSITION",
			"symbol": normalized,
		}, nil
	}

	availableQty := pos.Quantity - pos.TodayBuyQty
	if availableQty <= 0 {
		return nil, fmt.Errorf("%w: %s", ErrTPlusOneRestricted, normalized)
	}

	sellQty := normalizeShareLot(quantity)
	if quantity <= 0 {
		sellQty = int(availableQty)
	}
	if sellQty > int(availableQty) {
		sellQty = int(availableQty)
	}
	sellQty = normalizeShareLot(float64(sellQty))
	if sellQty < 100 {
		return nil, ErrInvalidLotSize
	}

	price := snapshot.LastPrice
	proceeds := float64(sellQty) * price
	commission := proceeds * 0.0013 // commission + stamp tax (simulated)

	pos.Quantity -= int64(sellQty)
	if pos.Quantity <= 0 {
		delete(t.positions, normalized)
	}
	t.cash += proceeds - commission

	orderID := fmt.Sprintf("ashare-close-long-%d", time.Now().UnixNano())
	t.orders[orderID] = orderState{
		Symbol:     normalized,
		Status:     "FILLED",
		Executed:   float64(sellQty),
		AvgPrice:   price,
		Commission: commission,
	}
	t.saveStateLocked()

	return map[string]interface{}{
		"orderId":     orderID,
		"status":      "FILLED",
		"symbol":      normalized,
		"executedQty": float64(sellQty),
		"avgPrice":    price,
		"commission":  commission,
	}, nil
}

func (t *ASharePaperTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	return nil, fmt.Errorf("%w: close_short", ErrUnsupportedForCashAccount)
}

func (t *ASharePaperTrader) SetLeverage(symbol string, leverage int) error {
	return fmt.Errorf("%w: leverage", ErrUnsupportedForCashAccount)
}

func (t *ASharePaperTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	return fmt.Errorf("%w: margin mode", ErrUnsupportedForCashAccount)
}

func (t *ASharePaperTrader) GetMarketPrice(symbol string) (float64, error) {
	snapshot, err := t.getSnapshot(ashareprovider.NormalizeSymbol(symbol))
	if err != nil {
		return 0, err
	}
	if snapshot.LastPrice <= 0 {
		return 0, fmt.Errorf("invalid market price for %s", symbol)
	}
	return snapshot.LastPrice, nil
}

func (t *ASharePaperTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	return fmt.Errorf("%w: stop loss", ErrUnsupportedForCashAccount)
}

func (t *ASharePaperTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	return fmt.Errorf("%w: take profit", ErrUnsupportedForCashAccount)
}

func (t *ASharePaperTrader) CancelStopLossOrders(symbol string) error {
	return fmt.Errorf("%w: cancel stop loss", ErrUnsupportedForCashAccount)
}

func (t *ASharePaperTrader) CancelTakeProfitOrders(symbol string) error {
	return fmt.Errorf("%w: cancel take profit", ErrUnsupportedForCashAccount)
}

func (t *ASharePaperTrader) CancelAllOrders(symbol string) error {
	return fmt.Errorf("%w: cancel all orders", ErrUnsupportedForCashAccount)
}

func (t *ASharePaperTrader) CancelStopOrders(symbol string) error {
	return fmt.Errorf("%w: cancel stop orders", ErrUnsupportedForCashAccount)
}

func (t *ASharePaperTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	shares := normalizeShareLot(quantity)
	if shares < 100 {
		return "", ErrInvalidLotSize
	}
	return strconv.Itoa(shares), nil
}

func (t *ASharePaperTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	order, ok := t.orders[orderID]
	if !ok {
		return nil, fmt.Errorf("order not found: %s", orderID)
	}
	return map[string]interface{}{
		"status":      order.Status,
		"avgPrice":    order.AvgPrice,
		"executedQty": order.Executed,
		"commission":  order.Commission,
		"symbol":      order.Symbol,
	}, nil
}

func (t *ASharePaperTrader) GetClosedPnL(startTime time.Time, limit int) ([]types.ClosedPnLRecord, error) {
	return []types.ClosedPnLRecord{}, nil
}

func (t *ASharePaperTrader) GetOpenOrders(symbol string) ([]types.OpenOrder, error) {
	return []types.OpenOrder{}, nil
}

func (t *ASharePaperTrader) getSnapshot(symbol string) (*ashareprovider.Snapshot, error) {
	if t.provider == nil {
		return nil, fmt.Errorf("market provider is nil")
	}
	snapshot, _, err := t.provider.GetSnapshot(symbol)
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

func (t *ASharePaperTrader) ensureTradingSession() error {
	now := t.nowFn().In(shanghaiLocation)
	weekday := now.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return ErrOutsideTradingSession
	}

	hhmm := now.Hour()*100 + now.Minute()
	inMorning := hhmm >= 930 && hhmm < 1130
	inAfternoon := hhmm >= 1300 && hhmm < 1500
	if !inMorning && !inAfternoon {
		return ErrOutsideTradingSession
	}
	return nil
}

func (t *ASharePaperTrader) rollToNextTradingDayLocked() {
	today := t.nowFn().In(shanghaiLocation).Format("2006-01-02")
	changed := false
	for _, pos := range t.positions {
		if pos.LastBuyDate != today {
			pos.TodayBuyQty = 0
			changed = true
		}
	}
	if changed {
		t.saveStateLocked()
	}
}

func normalizeShareLot(quantity float64) int {
	if quantity <= 0 {
		return 0
	}
	shares := int(math.Floor(quantity))
	return shares / 100 * 100
}

type persistedState struct {
	Cash      float64                   `json:"cash"`
	Positions map[string]*positionState `json:"positions"`
	Orders    map[string]orderState     `json:"orders"`
}

func (t *ASharePaperTrader) loadState() {
	if strings.TrimSpace(t.statePath) == "" {
		return
	}
	data, err := os.ReadFile(t.statePath)
	if err != nil {
		return
	}
	var state persistedState
	if err := json.Unmarshal(data, &state); err != nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if state.Cash > 0 {
		t.cash = state.Cash
	}
	if len(state.Positions) > 0 {
		t.positions = state.Positions
	}
	if len(state.Orders) > 0 {
		t.orders = state.Orders
	}
}

func (t *ASharePaperTrader) saveStateLocked() {
	if strings.TrimSpace(t.statePath) == "" {
		return
	}
	state := persistedState{
		Cash:      t.cash,
		Positions: t.positions,
		Orders:    t.orders,
	}
	data, err := json.Marshal(state)
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(t.statePath), 0o755)
	_ = os.WriteFile(t.statePath, data, 0o600)
}

func buildStateFilePath(accountID string) string {
	safe := strings.TrimSpace(accountID)
	if safe == "" {
		safe = "default"
	}
	var b strings.Builder
	b.Grow(len(safe))
	for _, ch := range safe {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-' {
			b.WriteRune(ch)
		} else {
			b.WriteRune('_')
		}
	}
	return filepath.Join(os.TempDir(), "nofx_asharepaper_"+b.String()+".json")
}
