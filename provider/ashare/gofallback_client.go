package ashare

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	defaultKlineEndpoint    = "https://push2his.eastmoney.com/api/qt/stock/kline/get"
	defaultSnapshotEndpoint = "https://push2.eastmoney.com/api/qt/stock/get"
	defaultSymbolsEndpoint  = "https://push2.eastmoney.com/api/qt/clist/get"
)

var indexSymbolPresets = map[string][]string{
	"hs300": {
		"600519.SH", "000858.SZ", "601318.SH", "600036.SH", "000333.SZ",
		"300750.SZ", "601888.SH", "002594.SZ", "002415.SZ", "000651.SZ",
		"601166.SH", "600900.SH", "601899.SH", "600276.SH", "601012.SH",
		"600030.SH", "000725.SZ", "600309.SH", "601688.SH", "600887.SH",
	},
	"zz500": {
		"000625.SZ", "000738.SZ", "000768.SZ", "000783.SZ", "000997.SZ",
		"002001.SZ", "002241.SZ", "002271.SZ", "002739.SZ", "300003.SZ",
		"300122.SZ", "300124.SZ", "300142.SZ", "300223.SZ", "300274.SZ",
		"600219.SH", "600299.SH", "600406.SH", "600482.SH", "600516.SH",
	},
	"zz1000": {
		"000006.SZ", "000021.SZ", "000028.SZ", "000049.SZ", "000050.SZ",
		"000089.SZ", "000155.SZ", "000156.SZ", "000338.SZ", "000400.SZ",
		"000426.SZ", "000528.SZ", "000538.SZ", "000559.SZ", "000596.SZ",
		"600008.SH", "600021.SH", "600096.SH", "600126.SH", "600183.SH",
	},
}

type GoFallbackClient struct {
	klineURL    string
	snapshotURL string
	symbolsURL  string
	client      *http.Client
}

func NewGoFallbackClient() *GoFallbackClient {
	return &GoFallbackClient{
		klineURL:    defaultKlineEndpoint,
		snapshotURL: defaultSnapshotEndpoint,
		symbolsURL:  defaultSymbolsEndpoint,
		client: &http.Client{
			Timeout: 12 * time.Second,
		},
	}
}

func (c *GoFallbackClient) SetEndpoints(klineURL, snapshotURL, symbolsURL string) {
	if strings.TrimSpace(klineURL) != "" {
		c.klineURL = strings.TrimSpace(klineURL)
	}
	if strings.TrimSpace(snapshotURL) != "" {
		c.snapshotURL = strings.TrimSpace(snapshotURL)
	}
	if strings.TrimSpace(symbolsURL) != "" {
		c.symbolsURL = strings.TrimSpace(symbolsURL)
	}
}

func (c *GoFallbackClient) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	secid, err := normalizeSecID(symbol)
	if err != nil {
		return nil, err
	}

	kltMap := map[string]string{
		"1m":  "1",
		"5m":  "5",
		"15m": "15",
		"30m": "30",
		"1d":  "101",
	}
	klt, ok := kltMap[normalizeInterval(interval)]
	if !ok {
		klt = "5"
	}

	req, err := http.NewRequest(http.MethodGet, c.klineURL, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Set("secid", secid)
	q.Set("klt", klt)
	q.Set("fqt", "1")
	q.Set("lmt", fmt.Sprintf("%d", clampLimit(limit)))
	q.Set("end", "20500101")
	q.Set("fields1", "f1,f2,f3,f4,f5,f6")
	q.Set("fields2", "f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61")
	req.URL.RawQuery = q.Encode()

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
		return nil, fmt.Errorf("fallback kline status %d: %s", resp.StatusCode, string(body))
	}

	var decoded struct {
		Rc   int    `json:"rc"`
		Msg  string `json:"msg"`
		Data struct {
			Klines []string `json:"klines"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, err
	}
	if decoded.Rc != 0 {
		return nil, fmt.Errorf("fallback kline rc=%d msg=%s", decoded.Rc, decoded.Msg)
	}
	if len(decoded.Data.Klines) == 0 {
		return nil, fmt.Errorf("empty fallback kline data")
	}

	durMs := intervalToMinutes(interval) * 60 * 1000
	result := make([]Kline, 0, len(decoded.Data.Klines))
	for _, raw := range decoded.Data.Klines {
		parts := strings.Split(raw, ",")
		if len(parts) < 7 {
			continue
		}

		openTime, err := parseDateTime(parts[0])
		if err != nil {
			continue
		}
		open := parseFloatAny(parts[1])
		closePrice := parseFloatAny(parts[2])
		high := parseFloatAny(parts[3])
		low := parseFloatAny(parts[4])
		volume := parseFloatAny(parts[5])
		amount := parseFloatAny(parts[6])

		result = append(result, Kline{
			OpenTime:    openTime,
			Open:        open,
			High:        high,
			Low:         low,
			Close:       closePrice,
			Volume:      volume,
			QuoteVolume: amount,
			CloseTime:   openTime + durMs - 1,
		})
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no valid fallback kline entries")
	}
	sort.Slice(result, func(i, j int) bool { return result[i].OpenTime < result[j].OpenTime })
	return result, nil
}

func (c *GoFallbackClient) GetSnapshot(symbol string) (*Snapshot, error) {
	secid, err := normalizeSecID(symbol)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, c.snapshotURL, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Set("secid", secid)
	q.Set("fields", "f43,f44,f45,f46,f47,f57,f58,f60,f170")
	req.URL.RawQuery = q.Encode()

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
		return nil, fmt.Errorf("fallback snapshot status %d: %s", resp.StatusCode, string(body))
	}

	var decoded struct {
		Rc   int                    `json:"rc"`
		Msg  string                 `json:"msg"`
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, err
	}
	if decoded.Rc != 0 {
		return nil, fmt.Errorf("fallback snapshot rc=%d msg=%s", decoded.Rc, decoded.Msg)
	}
	if len(decoded.Data) == 0 {
		return nil, fmt.Errorf("empty fallback snapshot data")
	}

	last := scaledPrice(decoded.Data["f43"])
	high := scaledPrice(decoded.Data["f44"])
	low := scaledPrice(decoded.Data["f45"])
	preClose := scaledPrice(decoded.Data["f60"])
	if last <= 0 {
		return nil, fmt.Errorf("fallback snapshot invalid last price")
	}
	if preClose <= 0 {
		preClose = scaledPrice(decoded.Data["f46"])
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
		High:       high,
		Low:        low,
		PreClose:   preClose,
		UpperLimit: upper,
		LowerLimit: lower,
		UpdateTime: time.Now().UnixMilli(),
	}, nil
}

func (c *GoFallbackClient) GetAllSymbols(limit int) ([]string, error) {
	if limit <= 0 {
		limit = 2000
	}
	if limit > 5000 {
		limit = 5000
	}

	req, err := http.NewRequest(http.MethodGet, c.symbolsURL, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Set("pn", "1")
	q.Set("pz", fmt.Sprintf("%d", limit))
	q.Set("po", "1")
	q.Set("np", "1")
	q.Set("fltt", "2")
	q.Set("invt", "2")
	q.Set("fid", "f3")
	q.Set("fs", "m:0+t:6,m:0+t:80,m:1+t:2,m:1+t:23")
	q.Set("fields", "f12,f13,f14")
	req.URL.RawQuery = q.Encode()

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
		return nil, fmt.Errorf("fallback symbols status %d: %s", resp.StatusCode, string(body))
	}

	var decoded struct {
		Data struct {
			Diff []map[string]interface{} `json:"diff"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, err
	}
	if len(decoded.Data.Diff) == 0 {
		return nil, fmt.Errorf("empty fallback symbols response")
	}

	seen := make(map[string]struct{}, len(decoded.Data.Diff))
	symbols := make([]string, 0, len(decoded.Data.Diff))
	for _, item := range decoded.Data.Diff {
		code := strings.TrimSpace(fmt.Sprintf("%v", item["f12"]))
		marketID := parseIntAny(item["f13"])
		if len(code) != 6 || !isDigits(code) {
			continue
		}

		market := "SZ"
		if marketID == 1 {
			market = "SH"
		}
		symbol := fmt.Sprintf("%s.%s", code, market)
		if _, ok := seen[symbol]; ok {
			continue
		}
		seen[symbol] = struct{}{}
		symbols = append(symbols, symbol)
	}

	sort.Strings(symbols)
	return symbols, nil
}

func (c *GoFallbackClient) GetIndexSymbols(index string) ([]string, error) {
	index = strings.ToLower(strings.TrimSpace(index))
	if index == "" {
		index = "hs300"
	}
	preset, ok := indexSymbolPresets[index]
	if !ok {
		return nil, fmt.Errorf("unsupported index scope: %s", index)
	}
	clone := make([]string, len(preset))
	copy(clone, preset)
	sort.Strings(clone)
	return clone, nil
}

func scaledPrice(v any) float64 {
	raw := parseFloatAny(v)
	if raw == 0 {
		return 0
	}
	// Eastmoney stock quote fields are usually integer prices scaled by 100.
	return raw / 100.0
}

func buildURL(base string, q url.Values) string {
	return strings.TrimRight(base, "?") + "?" + q.Encode()
}
