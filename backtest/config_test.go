package backtest

import "testing"

func baseConfig() BacktestConfig {
	return BacktestConfig{
		RunID:             "bt_test",
		Symbols:           []string{"BTC"},
		Timeframes:        []string{"5m"},
		DecisionTimeframe: "5m",
		StartTS:           1700000000,
		EndTS:             1700003600,
		InitialBalance:    1000,
		FillPolicy:        FillPolicyNextOpen,
	}
}

func TestBacktestConfigValidate_DefaultCryptoMarket(t *testing.T) {
	cfg := baseConfig()
	cfg.Symbols = []string{"btc"}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if cfg.Market != "crypto" {
		t.Fatalf("expected market=crypto, got %s", cfg.Market)
	}
	if cfg.Exchange != "binance" {
		t.Fatalf("expected exchange=binance, got %s", cfg.Exchange)
	}
	if got := cfg.Symbols[0]; got != "BTCUSDT" {
		t.Fatalf("expected symbol BTCUSDT, got %s", got)
	}
}

func TestBacktestConfigValidate_InferAShareMarket(t *testing.T) {
	cfg := baseConfig()
	cfg.Symbols = []string{"600519"}
	cfg.Timeframes = []string{"1d"}
	cfg.DecisionTimeframe = "1d"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if cfg.Market != "ashare" {
		t.Fatalf("expected market=ashare, got %s", cfg.Market)
	}
	if cfg.Exchange != "ashare" {
		t.Fatalf("expected exchange=ashare, got %s", cfg.Exchange)
	}
	if got := cfg.Symbols[0]; got != "600519.SH" {
		t.Fatalf("expected symbol 600519.SH, got %s", got)
	}
}
