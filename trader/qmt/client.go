package qmt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"nofx/market"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHTTPTimeout = 10 * time.Second
)

type gatewayClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func newGatewayClient(baseURL, token string) *gatewayClient {
	return &gatewayClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}
}

func (c *gatewayClient) doJSON(ctx context.Context, method, path string, query url.Values, payload any) (any, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	fullURL := c.baseURL + path
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}

	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		msg := strings.TrimSpace(string(respData))
		if msg == "" {
			msg = resp.Status
		}
		return nil, fmt.Errorf("qmt gateway %s %s failed: %s", method, path, msg)
	}

	if len(respData) == 0 {
		return map[string]any{}, nil
	}

	var out any
	if err := json.Unmarshal(respData, &out); err != nil {
		return nil, fmt.Errorf("invalid qmt gateway response: %w", err)
	}
	return out, nil
}

func (c *gatewayClient) getBalance(ctx context.Context, accountID string) (map[string]any, error) {
	query := url.Values{}
	if accountID != "" {
		query.Set("account_id", accountID)
	}

	raw, err := c.doJSON(ctx, http.MethodGet, "/v1/account/balance", query, nil)
	if err != nil {
		return nil, err
	}
	return extractObject(raw), nil
}

func (c *gatewayClient) getPositions(ctx context.Context, accountID string) ([]map[string]any, error) {
	query := url.Values{}
	if accountID != "" {
		query.Set("account_id", accountID)
	}

	raw, err := c.doJSON(ctx, http.MethodGet, "/v1/account/positions", query, nil)
	if err != nil {
		return nil, err
	}

	arr := extractArray(raw, "positions", "items")
	positions := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		if obj, ok := item.(map[string]any); ok {
			positions = append(positions, obj)
		}
	}
	return positions, nil
}

func (c *gatewayClient) getSnapshot(ctx context.Context, symbol string) (map[string]any, error) {
	query := url.Values{}
	query.Set("symbol", symbol)

	raw, err := c.doJSON(ctx, http.MethodGet, "/v1/market/snapshot", query, nil)
	if err != nil {
		return nil, err
	}
	return extractObject(raw), nil
}

func (c *gatewayClient) getKlines(ctx context.Context, symbol, interval string, limit int) ([]market.Kline, error) {
	query := url.Values{}
	query.Set("symbol", symbol)
	if interval != "" {
		query.Set("interval", interval)
	}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}

	raw, err := c.doJSON(ctx, http.MethodGet, "/v1/market/klines", query, nil)
	if err != nil {
		return nil, err
	}

	arr := extractArray(raw, "klines", "items", "data")
	if len(arr) == 0 {
		if direct, ok := raw.([]any); ok {
			arr = direct
		}
	}

	klines := make([]market.Kline, 0, len(arr))
	for _, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}

		openTime := toInt64(obj["openTime"])
		if openTime == 0 {
			openTime = toInt64(obj["open_time"])
		}
		if openTime == 0 {
			openTime = toInt64(obj["timestamp"])
		}

		closeTime := toInt64(obj["closeTime"])
		if closeTime == 0 {
			closeTime = toInt64(obj["close_time"])
		}
		if closeTime == 0 {
			closeTime = openTime
		}

		open := toFloat(obj["open"])
		high := toFloat(obj["high"])
		low := toFloat(obj["low"])
		closePrice := toFloat(obj["close"])
		volume := toFloat(obj["volume"])
		quoteVolume := toFloat(obj["quoteVolume"])
		if quoteVolume == 0 {
			quoteVolume = toFloat(obj["quote_volume"])
		}
		if quoteVolume == 0 {
			quoteVolume = closePrice * volume
		}

		klines = append(klines, market.Kline{
			OpenTime:    openTime,
			Open:        open,
			High:        high,
			Low:         low,
			Close:       closePrice,
			Volume:      volume,
			QuoteVolume: quoteVolume,
			CloseTime:   closeTime,
		})
	}

	return klines, nil
}

func (c *gatewayClient) getSymbols(ctx context.Context, scope, sector string) ([]string, error) {
	query := url.Values{}
	if scope != "" {
		query.Set("scope", scope)
	}
	if sector != "" {
		query.Set("sector", sector)
	}

	raw, err := c.doJSON(ctx, http.MethodGet, "/v1/market/symbols", query, nil)
	if err != nil {
		return nil, err
	}

	arr := extractArray(raw, "symbols", "items", "data")
	if len(arr) == 0 {
		if direct, ok := raw.([]any); ok {
			arr = direct
		}
	}

	symbols := make([]string, 0, len(arr))
	for _, item := range arr {
		switch v := item.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				symbols = append(symbols, v)
			}
		case map[string]any:
			symbol := toString(v["symbol"])
			if symbol == "" {
				symbol = toString(v["code"])
			}
			if strings.TrimSpace(symbol) != "" {
				symbols = append(symbols, symbol)
			}
		}
	}

	return symbols, nil
}

func (c *gatewayClient) placeOrder(ctx context.Context, req map[string]any) (map[string]any, error) {
	raw, err := c.doJSON(ctx, http.MethodPost, "/v1/orders", nil, req)
	if err != nil {
		return nil, err
	}
	return extractObject(raw), nil
}

func extractObject(raw any) map[string]any {
	if m, ok := raw.(map[string]any); ok {
		// Common response envelope: {"data": {...}}
		if data, ok := m["data"]; ok {
			if obj, ok := data.(map[string]any); ok {
				return obj
			}
		}
		return m
	}
	return map[string]any{}
}

func extractArray(raw any, keys ...string) []any {
	if raw == nil {
		return nil
	}
	if arr, ok := raw.([]any); ok {
		return arr
	}

	m, ok := raw.(map[string]any)
	if !ok {
		return nil
	}

	for _, key := range keys {
		if key == "" {
			continue
		}
		if val, exists := m[key]; exists {
			if arr, ok := val.([]any); ok {
				return arr
			}
			if obj, ok := val.(map[string]any); ok {
				for _, nestedKey := range []string{"items", "list", "symbols", "klines"} {
					if nestedVal, exists := obj[nestedKey]; exists {
						if arr, ok := nestedVal.([]any); ok {
							return arr
						}
					}
				}
			}
		}
	}

	if data, exists := m["data"]; exists {
		if arr, ok := data.([]any); ok {
			return arr
		}
		if obj, ok := data.(map[string]any); ok {
			for _, key := range keys {
				if key == "" {
					continue
				}
				if val, exists := obj[key]; exists {
					if arr, ok := val.([]any); ok {
						return arr
					}
				}
			}
			for _, nestedKey := range []string{"items", "list", "symbols", "klines"} {
				if val, exists := obj[nestedKey]; exists {
					if arr, ok := val.([]any); ok {
						return arr
					}
				}
			}
		}
	}

	return nil
}

func toFloat(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case int32:
		return float64(val)
	case json.Number:
		f, _ := val.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(val), 64)
		return f
	default:
		return 0
	}
}

func toInt64(v any) int64 {
	switch val := v.(type) {
	case int64:
		return val
	case int:
		return int64(val)
	case int32:
		return int64(val)
	case float64:
		return int64(val)
	case float32:
		return int64(val)
	case json.Number:
		i, _ := val.Int64()
		return i
	case string:
		i, _ := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
		return i
	default:
		return 0
	}
}

func toString(v any) string {
	switch val := v.(type) {
	case string:
		return strings.TrimSpace(val)
	case fmt.Stringer:
		return strings.TrimSpace(val.String())
	case json.Number:
		return val.String()
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(val, 10)
	case int:
		return strconv.Itoa(val)
	default:
		return ""
	}
}
