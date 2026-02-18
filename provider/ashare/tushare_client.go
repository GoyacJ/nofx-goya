package ashare

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

const tushareDefaultBaseURL = "https://api.tushare.pro"

type TushareClient struct {
	token   string
	baseURL string
	client  *http.Client
}

type tushareRequest struct {
	APIName string         `json:"api_name"`
	Token   string         `json:"token"`
	Params  map[string]any `json:"params,omitempty"`
	Fields  string         `json:"fields,omitempty"`
}

type tushareResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Fields []string `json:"fields"`
		Items  [][]any  `json:"items"`
	} `json:"data"`
}

func NewTushareClient(token string) *TushareClient {
	return &TushareClient{
		token:   strings.TrimSpace(token),
		baseURL: tushareDefaultBaseURL,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *TushareClient) Enabled() bool {
	return c != nil && strings.TrimSpace(c.token) != ""
}

func (c *TushareClient) SetBaseURL(baseURL string) {
	if c == nil {
		return
	}
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return
	}
	c.baseURL = strings.TrimRight(baseURL, "/")
}

func (c *TushareClient) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("tushare token is empty")
	}

	freqMap := map[string]string{
		"1m":  "1min",
		"5m":  "5min",
		"15m": "15min",
		"30m": "30min",
		"1d":  "D",
	}
	freq, ok := freqMap[normalizeInterval(interval)]
	if !ok {
		freq = "5min"
	}

	fields := "ts_code,trade_time,trade_date,open,high,low,close,vol,amount"
	params := map[string]any{
		"ts_code": NormalizeSymbol(symbol),
		"freq":    freq,
		"limit":   clampLimit(limit),
	}

	resp, err := c.post("pro_bar", params, fields)
	if err != nil {
		return nil, err
	}
	if len(resp.Data.Items) == 0 {
		return nil, fmt.Errorf("empty tushare kline response")
	}

	index := make(map[string]int, len(resp.Data.Fields))
	for i, field := range resp.Data.Fields {
		index[field] = i
	}
	get := func(item []any, key string) any {
		i, ok := index[key]
		if !ok || i < 0 || i >= len(item) {
			return nil
		}
		return item[i]
	}

	klines := make([]Kline, 0, len(resp.Data.Items))
	durMs := intervalToMinutes(interval) * 60 * 1000
	for _, item := range resp.Data.Items {
		tsVal := strings.TrimSpace(fmt.Sprintf("%v", get(item, "trade_time")))
		if tsVal == "" || tsVal == "<nil>" {
			tsVal = strings.TrimSpace(fmt.Sprintf("%v", get(item, "trade_date")))
		}
		if tsVal == "" || tsVal == "<nil>" {
			continue
		}

		openTime, err := parseDateTime(tsVal)
		if err != nil {
			continue
		}
		closeTime := openTime + durMs - 1

		klines = append(klines, Kline{
			OpenTime:    openTime,
			Open:        parseFloatAny(get(item, "open")),
			High:        parseFloatAny(get(item, "high")),
			Low:         parseFloatAny(get(item, "low")),
			Close:       parseFloatAny(get(item, "close")),
			Volume:      parseFloatAny(get(item, "vol")),
			QuoteVolume: parseFloatAny(get(item, "amount")),
			CloseTime:   closeTime,
		})
	}

	if len(klines) == 0 {
		return nil, fmt.Errorf("no valid klines parsed from tushare")
	}
	sort.Slice(klines, func(i, j int) bool { return klines[i].OpenTime < klines[j].OpenTime })
	return klines, nil
}

func (c *TushareClient) GetSymbols() ([]string, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("tushare token is empty")
	}

	resp, err := c.post("stock_basic", map[string]any{"list_status": "L"}, "ts_code")
	if err != nil {
		return nil, err
	}
	if len(resp.Data.Items) == 0 {
		return nil, fmt.Errorf("empty tushare symbols response")
	}

	idx := 0
	for i, field := range resp.Data.Fields {
		if field == "ts_code" {
			idx = i
			break
		}
	}

	seen := make(map[string]struct{}, len(resp.Data.Items))
	symbols := make([]string, 0, len(resp.Data.Items))
	for _, item := range resp.Data.Items {
		if idx < 0 || idx >= len(item) {
			continue
		}
		symbol := NormalizeSymbol(fmt.Sprintf("%v", item[idx]))
		if symbol == "" {
			continue
		}
		if _, ok := seen[symbol]; ok {
			continue
		}
		seen[symbol] = struct{}{}
		symbols = append(symbols, symbol)
	}
	sort.Strings(symbols)
	return symbols, nil
}

func (c *TushareClient) GetSnapshot(symbol string) (*Snapshot, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("tushare token is empty")
	}

	fields := "ts_code,trade_time,price,pre_close,high,low"
	resp, err := c.post("realtime_quote", map[string]any{"ts_code": NormalizeSymbol(symbol)}, fields)
	if err != nil {
		return nil, err
	}
	if len(resp.Data.Items) == 0 {
		return nil, fmt.Errorf("empty tushare snapshot response")
	}

	index := make(map[string]int, len(resp.Data.Fields))
	for i, field := range resp.Data.Fields {
		index[field] = i
	}
	get := func(item []any, key string) any {
		i, ok := index[key]
		if !ok || i < 0 || i >= len(item) {
			return nil
		}
		return item[i]
	}

	item := resp.Data.Items[0]
	last := parseFloatAny(get(item, "price"))
	preClose := parseFloatAny(get(item, "pre_close"))
	if last <= 0 {
		return nil, fmt.Errorf("invalid tushare snapshot price")
	}

	updateTime := time.Now().UnixMilli()
	timeRaw := strings.TrimSpace(fmt.Sprintf("%v", get(item, "trade_time")))
	if timeRaw != "" && timeRaw != "<nil>" {
		if parsed, err := parseDateTime(timeRaw); err == nil {
			updateTime = parsed
		}
	}

	upper := 0.0
	lower := 0.0
	if preClose > 0 {
		upper = preClose * 1.10
		lower = preClose * 0.90
	}

	return &Snapshot{
		Symbol:     NormalizeSymbol(symbol),
		LastPrice:  last,
		High:       parseFloatAny(get(item, "high")),
		Low:        parseFloatAny(get(item, "low")),
		PreClose:   preClose,
		UpperLimit: upper,
		LowerLimit: lower,
		UpdateTime: updateTime,
	}, nil
}

func (c *TushareClient) post(apiName string, params map[string]any, fields string) (*tushareResponse, error) {
	reqBody := tushareRequest{
		APIName: apiName,
		Token:   c.token,
		Params:  params,
		Fields:  fields,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tushare status %d: %s", resp.StatusCode, string(body))
	}

	var decoded tushareResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, err
	}
	if decoded.Code != 0 {
		return nil, fmt.Errorf("tushare code=%d msg=%s", decoded.Code, decoded.Msg)
	}
	return &decoded, nil
}
