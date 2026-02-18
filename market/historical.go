package market

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	ashareprovider "nofx/provider/ashare"
	"strings"
	"time"
)

const (
	binanceFuturesKlinesURL = "https://fapi.binance.com/fapi/v1/klines"
	binanceMaxKlineLimit    = 1500
)

type KlineSourceOptions struct {
	AShareToken     string
	AShareDataMode  string
	AShareWatchlist string
}

// GetKlinesRange fetches K-line series within specified time range (closed interval), returns data sorted by time in ascending order.
func GetKlinesRange(symbol string, timeframe string, start, end time.Time) ([]Kline, error) {
	return GetKlinesRangeWithSource(symbol, timeframe, start, end, "crypto", "binance")
}

func GetKlinesRangeWithSource(symbol string, timeframe string, start, end time.Time, marketType, exchange string, opts ...KlineSourceOptions) ([]Kline, error) {
	marketType = strings.ToLower(strings.TrimSpace(marketType))
	exchange = strings.ToLower(strings.TrimSpace(exchange))
	if marketType == "" {
		marketType = "crypto"
	}

	switch marketType {
	case "ashare":
		var option KlineSourceOptions
		if len(opts) > 0 {
			option = opts[0]
		}
		return getAShareKlinesRange(symbol, timeframe, start, end, option)
	default:
		return getCryptoKlinesRange(symbol, timeframe, start, end)
	}
}

func getCryptoKlinesRange(symbol string, timeframe string, start, end time.Time) ([]Kline, error) {
	symbol = Normalize(symbol)
	normTF, err := NormalizeTimeframe(timeframe)
	if err != nil {
		return nil, err
	}
	if !end.After(start) {
		return nil, fmt.Errorf("end time must be after start time")
	}

	startMs := start.UnixMilli()
	endMs := end.UnixMilli()

	var all []Kline
	cursor := startMs

	client := &http.Client{Timeout: 15 * time.Second}

	for cursor < endMs {
		req, err := http.NewRequest("GET", binanceFuturesKlinesURL, nil)
		if err != nil {
			return nil, err
		}

		q := req.URL.Query()
		q.Set("symbol", symbol)
		q.Set("interval", normTF)
		q.Set("limit", fmt.Sprintf("%d", binanceMaxKlineLimit))
		q.Set("startTime", fmt.Sprintf("%d", cursor))
		q.Set("endTime", fmt.Sprintf("%d", endMs))
		req.URL.RawQuery = q.Encode()

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("binance klines api returned status %d: %s", resp.StatusCode, string(body))
		}

		var raw [][]interface{}
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, err
		}
		if len(raw) == 0 {
			break
		}

		batch := make([]Kline, len(raw))
		for i, item := range raw {
			openTime := int64(item[0].(float64))
			open, _ := parseFloat(item[1])
			high, _ := parseFloat(item[2])
			low, _ := parseFloat(item[3])
			close, _ := parseFloat(item[4])
			volume, _ := parseFloat(item[5])
			closeTime := int64(item[6].(float64))

			batch[i] = Kline{
				OpenTime:  openTime,
				Open:      open,
				High:      high,
				Low:       low,
				Close:     close,
				Volume:    volume,
				CloseTime: closeTime,
			}
		}

		all = append(all, batch...)

		last := batch[len(batch)-1]
		cursor = last.CloseTime + 1

		// If returned quantity is less than request limit, reached the end, can exit early.
		if len(batch) < binanceMaxKlineLimit {
			break
		}
	}

	return all, nil
}

func getAShareKlinesRange(symbol string, timeframe string, start, end time.Time, option KlineSourceOptions) ([]Kline, error) {
	symbol = NormalizeCN(symbol)
	if !end.After(start) {
		return nil, fmt.Errorf("end time must be after start time")
	}

	interval := normalizeAShareInterval(timeframe)
	dur, err := TFDuration(interval)
	if err != nil || dur <= 0 {
		dur = 5 * time.Minute
	}

	estimatedBars := int(end.Sub(start)/dur) + 64
	if estimatedBars < 200 {
		estimatedBars = 200
	}
	if estimatedBars > 5000 {
		estimatedBars = 5000
	}

	provider := ashareprovider.NewProvider(option.AShareToken, option.AShareDataMode, option.AShareWatchlist)
	series, _, err := provider.GetKlines(symbol, interval, estimatedBars)
	if err != nil {
		return nil, err
	}

	startMs := start.UnixMilli()
	endMs := end.UnixMilli()
	result := make([]Kline, 0, len(series))
	for _, k := range series {
		if k.CloseTime < startMs {
			continue
		}
		if k.OpenTime > endMs {
			continue
		}
		result = append(result, Kline{
			OpenTime:    k.OpenTime,
			Open:        k.Open,
			High:        k.High,
			Low:         k.Low,
			Close:       k.Close,
			Volume:      k.Volume,
			CloseTime:   k.CloseTime,
			QuoteVolume: k.QuoteVolume,
		})
	}
	return result, nil
}

func normalizeAShareInterval(interval string) string {
	switch strings.ToLower(strings.TrimSpace(interval)) {
	case "1m", "5m", "15m", "30m", "1d":
		return strings.ToLower(strings.TrimSpace(interval))
	case "3m":
		return "5m"
	case "1h", "2h", "4h", "6h", "8h", "12h":
		return "30m"
	case "1w":
		return "1d"
	default:
		return "5m"
	}
}
