package ashare

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DefaultMarket   = "CN-A"
	DefaultDataMode = "tushare_then_go_fallback"
)

const (
	DataModeTushareThenFallback = "tushare_then_go_fallback"
	DataModeTushareOnly         = "tushare_only"
	DataModeGoFallbackOnly      = "go_fallback_only"
)

type Kline struct {
	OpenTime    int64   `json:"openTime"`
	Open        float64 `json:"open"`
	High        float64 `json:"high"`
	Low         float64 `json:"low"`
	Close       float64 `json:"close"`
	Volume      float64 `json:"volume"`
	CloseTime   int64   `json:"closeTime"`
	QuoteVolume float64 `json:"quoteVolume"`
}

type Snapshot struct {
	Symbol     string  `json:"symbol"`
	LastPrice  float64 `json:"lastPrice"`
	High       float64 `json:"high"`
	Low        float64 `json:"low"`
	PreClose   float64 `json:"preClose"`
	UpperLimit float64 `json:"upperLimit"`
	LowerLimit float64 `json:"lowerLimit"`
	UpdateTime int64   `json:"updateTime"`
}

type cacheEntry struct {
	expireAt time.Time
	source   string
	klines   []Kline
}

type Provider struct {
	tushare   *TushareClient
	fallback  *GoFallbackClient
	dataMode  string
	watchlist []string

	cacheMu sync.RWMutex
	cache   map[string]cacheEntry
}

func NewProvider(tushareToken, dataMode, watchlistRaw string) *Provider {
	mode := normalizeDataMode(dataMode)
	return &Provider{
		tushare:   NewTushareClient(tushareToken),
		fallback:  NewGoFallbackClient(),
		dataMode:  mode,
		watchlist: parseWatchlist(watchlistRaw),
		cache:     make(map[string]cacheEntry),
	}
}

func NewProviderWithClients(tushare *TushareClient, fallback *GoFallbackClient, dataMode, watchlistRaw string) *Provider {
	mode := normalizeDataMode(dataMode)
	if tushare == nil {
		tushare = NewTushareClient("")
	}
	if fallback == nil {
		fallback = NewGoFallbackClient()
	}
	return &Provider{
		tushare:   tushare,
		fallback:  fallback,
		dataMode:  mode,
		watchlist: parseWatchlist(watchlistRaw),
		cache:     make(map[string]cacheEntry),
	}
}

func (p *Provider) NormalizeSymbol(symbol string) string {
	return NormalizeSymbol(symbol)
}

func (p *Provider) GetKlines(symbol, interval string, limit int) ([]Kline, string, error) {
	normSymbol := NormalizeSymbol(symbol)
	normInterval := normalizeInterval(interval)
	normLimit := clampLimit(limit)
	cacheKey := fmt.Sprintf("%s|%s|%d", normSymbol, normInterval, normLimit)

	if klines, source, ok := p.getCached(cacheKey); ok {
		return klines, source, nil
	}

	if p.shouldTryTushare() {
		klines, err := p.tushare.GetKlines(normSymbol, normInterval, normLimit)
		if err == nil && len(klines) > 0 {
			p.setCache(cacheKey, "tushare", klines, ttlByInterval(normInterval))
			return klines, "tushare", nil
		}
		if p.dataMode == DataModeTushareOnly {
			return nil, "", fmt.Errorf("tushare provider failed: %w", err)
		}
	}

	klines, err := p.fallback.GetKlines(normSymbol, normInterval, normLimit)
	if err != nil {
		return nil, "", fmt.Errorf("fallback provider failed: %w", err)
	}
	p.setCache(cacheKey, "fallback", klines, ttlByInterval(normInterval))
	return klines, "fallback", nil
}

func (p *Provider) GetSnapshot(symbol string) (*Snapshot, string, error) {
	normSymbol := NormalizeSymbol(symbol)

	if p.shouldTryTushare() {
		snap, err := p.tushare.GetSnapshot(normSymbol)
		if err == nil && snap != nil && snap.LastPrice > 0 {
			return snap, "tushare", nil
		}
		if p.dataMode == DataModeTushareOnly {
			return nil, "", fmt.Errorf("tushare snapshot failed: %w", err)
		}
	}

	snap, err := p.fallback.GetSnapshot(normSymbol)
	if err != nil {
		return nil, "", err
	}
	return snap, "fallback", nil
}

func (p *Provider) GetSymbols(scope, indexName string) ([]string, string, error) {
	scope = strings.ToLower(strings.TrimSpace(scope))
	indexName = strings.ToLower(strings.TrimSpace(indexName))
	if scope == "" {
		scope = "watchlist"
	}

	switch scope {
	case "watchlist":
		if len(p.watchlist) > 0 {
			return append([]string(nil), p.watchlist...), "watchlist", nil
		}
		// Empty watchlist: fall back to index symbols (hs300 by default).
		if indexName == "" {
			indexName = "hs300"
		}
		symbols, err := p.fallback.GetIndexSymbols(indexName)
		if err == nil && len(symbols) > 0 {
			return symbols, "fallback", nil
		}
		return nil, "", fmt.Errorf("watchlist is empty and index fallback failed: %w", err)

	case "index":
		if indexName == "" {
			indexName = "hs300"
		}
		symbols, err := p.fallback.GetIndexSymbols(indexName)
		if err != nil {
			return nil, "", err
		}
		return symbols, "fallback", nil

	case "all":
		if p.shouldTryTushare() {
			symbols, err := p.tushare.GetSymbols()
			if err == nil && len(symbols) > 0 {
				return symbols, "tushare", nil
			}
			if p.dataMode == DataModeTushareOnly {
				return nil, "", fmt.Errorf("tushare symbols failed: %w", err)
			}
		}
		symbols, err := p.fallback.GetAllSymbols(5000)
		if err != nil {
			return nil, "", err
		}
		return symbols, "fallback", nil
	default:
		return nil, "", fmt.Errorf("unsupported scope: %s", scope)
	}
}

func (p *Provider) shouldTryTushare() bool {
	if p.dataMode == DataModeGoFallbackOnly {
		return false
	}
	return p.tushare != nil && p.tushare.Enabled()
}

func (p *Provider) getCached(key string) ([]Kline, string, bool) {
	p.cacheMu.RLock()
	entry, ok := p.cache[key]
	p.cacheMu.RUnlock()
	if !ok || time.Now().After(entry.expireAt) {
		return nil, "", false
	}
	clone := make([]Kline, len(entry.klines))
	copy(clone, entry.klines)
	return clone, entry.source, true
}

func (p *Provider) setCache(key, source string, klines []Kline, ttl time.Duration) {
	if ttl <= 0 {
		ttl = 15 * time.Second
	}
	clone := make([]Kline, len(klines))
	copy(clone, klines)

	p.cacheMu.Lock()
	p.cache[key] = cacheEntry{
		expireAt: time.Now().Add(ttl),
		source:   source,
		klines:   clone,
	}
	p.cacheMu.Unlock()
}

func normalizeDataMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case DataModeTushareOnly:
		return DataModeTushareOnly
	case DataModeGoFallbackOnly:
		return DataModeGoFallbackOnly
	default:
		return DataModeTushareThenFallback
	}
}

func normalizeInterval(interval string) string {
	switch strings.ToLower(strings.TrimSpace(interval)) {
	case "1m", "5m", "15m", "30m", "1d":
		return strings.ToLower(strings.TrimSpace(interval))
	default:
		return "5m"
	}
}

func clampLimit(limit int) int {
	switch {
	case limit <= 0:
		return 500
	case limit > 5000:
		return 5000
	default:
		return limit
	}
}

func ttlByInterval(interval string) time.Duration {
	switch interval {
	case "1d":
		return 60 * time.Second
	default:
		return 20 * time.Second
	}
}

func parseWatchlist(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	parts := make([]string, 0, 64)

	if strings.HasPrefix(raw, "[") {
		var arr []string
		if err := json.Unmarshal([]byte(raw), &arr); err == nil {
			parts = append(parts, arr...)
		}
	}
	if len(parts) == 0 {
		splitFn := func(r rune) bool {
			return r == ',' || r == ';' || r == '\n' || r == '\t' || r == ' '
		}
		parts = strings.FieldsFunc(raw, splitFn)
	}

	seen := make(map[string]struct{}, len(parts))
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		symbol := NormalizeSymbol(part)
		if symbol == "" {
			continue
		}
		if _, ok := seen[symbol]; ok {
			continue
		}
		seen[symbol] = struct{}{}
		result = append(result, symbol)
	}
	sort.Strings(result)
	return result
}

func NormalizeSymbol(symbol string) string {
	s := strings.ToUpper(strings.TrimSpace(symbol))
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, " ", "")

	market := ""
	code := s

	switch {
	case strings.HasPrefix(code, "SH"):
		market = "SH"
		code = strings.TrimPrefix(code, "SH")
	case strings.HasPrefix(code, "SZ"):
		market = "SZ"
		code = strings.TrimPrefix(code, "SZ")
	case strings.HasSuffix(code, ".SH"):
		market = "SH"
		code = strings.TrimSuffix(code, ".SH")
	case strings.HasSuffix(code, ".SZ"):
		market = "SZ"
		code = strings.TrimSuffix(code, ".SZ")
	case strings.HasSuffix(code, "SH"):
		market = "SH"
		code = strings.TrimSuffix(code, "SH")
	case strings.HasSuffix(code, "SZ"):
		market = "SZ"
		code = strings.TrimSuffix(code, "SZ")
	}

	if len(code) != 6 || !isDigits(code) {
		return s
	}
	if market == "" {
		if strings.HasPrefix(code, "6") || strings.HasPrefix(code, "9") {
			market = "SH"
		} else {
			market = "SZ"
		}
	}
	return code + "." + market
}

func normalizeSecID(symbol string) (string, error) {
	norm := NormalizeSymbol(symbol)
	parts := strings.Split(norm, ".")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid symbol format: %s", symbol)
	}
	code := parts[0]
	switch parts[1] {
	case "SH":
		return "1." + code, nil
	case "SZ":
		return "0." + code, nil
	default:
		return "", fmt.Errorf("unsupported symbol market: %s", symbol)
	}
}

func intervalToMinutes(interval string) int64 {
	switch normalizeInterval(interval) {
	case "1m":
		return 1
	case "5m":
		return 5
	case "15m":
		return 15
	case "30m":
		return 30
	case "1d":
		return 24 * 60
	default:
		return 5
	}
}

func parseDateTime(input string) (int64, error) {
	candidates := []string{
		"2006-01-02 15:04",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"20060102",
	}
	for _, layout := range candidates {
		t, err := time.ParseInLocation(layout, input, time.Local)
		if err == nil {
			return t.UnixMilli(), nil
		}
	}
	return 0, fmt.Errorf("unsupported datetime: %s", input)
}

func parseFloatAny(v any) float64 {
	switch x := v.(type) {
	case nil:
		return 0
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case json.Number:
		f, _ := x.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f
	default:
		return 0
	}
}

func parseIntAny(v any) int64 {
	switch x := v.(type) {
	case nil:
		return 0
	case int:
		return int64(x)
	case int64:
		return x
	case float64:
		return int64(x)
	case float32:
		return int64(x)
	case json.Number:
		i, _ := x.Int64()
		return i
	case string:
		i, _ := strconv.ParseInt(strings.TrimSpace(x), 10, 64)
		return i
	default:
		return 0
	}
}

func isDigits(s string) bool {
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}
