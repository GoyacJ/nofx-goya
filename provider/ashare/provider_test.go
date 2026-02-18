package ashare

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProvider_GetKlines_TushareSuccess(t *testing.T) {
	tushareSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code":0,
			"msg":"",
			"data":{
				"fields":["ts_code","trade_time","open","high","low","close","vol","amount"],
				"items":[
					["600519.SH","2026-01-01 09:30:00",100,101,99,100.5,1200,120000],
					["600519.SH","2026-01-01 09:35:00",100.5,102,100,101.2,1300,130000]
				]
			}
		}`))
	}))
	defer tushareSrv.Close()

	tushareClient := NewTushareClient("demo-token")
	tushareClient.SetBaseURL(tushareSrv.URL)
	provider := NewProviderWithClients(tushareClient, NewGoFallbackClient(), DataModeTushareThenFallback, "")

	klines, source, err := provider.GetKlines("600519", "5m", 100)
	if err != nil {
		t.Fatalf("GetKlines failed: %v", err)
	}
	if source != "tushare" {
		t.Fatalf("expected source=tushare, got %s", source)
	}
	if len(klines) != 2 {
		t.Fatalf("expected 2 klines, got %d", len(klines))
	}
	if klines[0].Open <= 0 || klines[1].Close <= 0 {
		t.Fatalf("invalid parsed kline values: %+v", klines)
	}
}

func TestProvider_GetKlines_FallbackWhenTushareFails(t *testing.T) {
	tushareSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":-1,"msg":"rate limit","data":{"fields":[],"items":[]}}`))
	}))
	defer tushareSrv.Close()

	fallbackSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/kline/get"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"rc":0,
				"data":{
					"klines":[
						"2026-01-01 09:30,100,101,102,99,10000,1000000",
						"2026-01-01 09:35,101,102,103,100,12000,1300000"
					]
				}
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer fallbackSrv.Close()

	tushareClient := NewTushareClient("demo-token")
	tushareClient.SetBaseURL(tushareSrv.URL)
	fallbackClient := NewGoFallbackClient()
	fallbackClient.SetEndpoints(fallbackSrv.URL+"/kline/get", fallbackSrv.URL+"/stock/get", fallbackSrv.URL+"/clist/get")

	provider := NewProviderWithClients(tushareClient, fallbackClient, DataModeTushareThenFallback, "")
	klines, source, err := provider.GetKlines("600519.SH", "5m", 100)
	if err != nil {
		t.Fatalf("GetKlines failed: %v", err)
	}
	if source != "fallback" {
		t.Fatalf("expected source=fallback, got %s", source)
	}
	if len(klines) != 2 {
		t.Fatalf("expected 2 klines, got %d", len(klines))
	}
}
