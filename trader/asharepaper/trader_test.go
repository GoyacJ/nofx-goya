package asharepaper

import (
	"errors"
	ashareprovider "nofx/provider/ashare"
	"testing"
	"time"
)

type mockSnapshotProvider struct {
	snapshot *ashareprovider.Snapshot
	err      error
}

func (m *mockSnapshotProvider) GetSnapshot(symbol string) (*ashareprovider.Snapshot, string, error) {
	if m.err != nil {
		return nil, "", m.err
	}
	s := *m.snapshot
	s.Symbol = symbol
	return &s, "mock", nil
}

func newTestTrader(now time.Time) *ASharePaperTrader {
	return &ASharePaperTrader{
		provider: &mockSnapshotProvider{
			snapshot: &ashareprovider.Snapshot{
				Symbol:     "600519.SH",
				LastPrice:  100,
				UpperLimit: 110,
				LowerLimit: 90,
				PreClose:   100,
			},
		},
		accountID: "test",
		market:    "CN-A",
		cash:      100000,
		positions: make(map[string]*positionState),
		orders:    make(map[string]orderState),
		nowFn:     func() time.Time { return now },
	}
}

func TestASharePaperTrader_OpenLong_Success(t *testing.T) {
	now := time.Date(2026, 2, 16, 10, 0, 0, 0, shanghaiLocation) // Monday trading session
	tr := newTestTrader(now)

	result, err := tr.OpenLong("600519.SH", 250, 1)
	if err != nil {
		t.Fatalf("OpenLong failed: %v", err)
	}
	if result["status"] != "FILLED" {
		t.Fatalf("expected FILLED, got %v", result["status"])
	}
	if got := int(result["executedQty"].(float64)); got != 200 {
		t.Fatalf("expected executedQty=200, got %d", got)
	}
}

func TestASharePaperTrader_OpenLong_InvalidLotSize(t *testing.T) {
	now := time.Date(2026, 2, 16, 10, 0, 0, 0, shanghaiLocation)
	tr := newTestTrader(now)

	_, err := tr.OpenLong("600519.SH", 99, 1)
	if !errors.Is(err, ErrInvalidLotSize) {
		t.Fatalf("expected ErrInvalidLotSize, got %v", err)
	}
}

func TestASharePaperTrader_OpenShort_Unsupported(t *testing.T) {
	now := time.Date(2026, 2, 16, 10, 0, 0, 0, shanghaiLocation)
	tr := newTestTrader(now)

	_, err := tr.OpenShort("600519.SH", 100, 1)
	if !errors.Is(err, ErrUnsupportedForCashAccount) {
		t.Fatalf("expected ErrUnsupportedForCashAccount, got %v", err)
	}
}

func TestASharePaperTrader_CloseLong_TPlusOneRestricted(t *testing.T) {
	now := time.Date(2026, 2, 16, 10, 0, 0, 0, shanghaiLocation)
	tr := newTestTrader(now)
	if _, err := tr.OpenLong("600519.SH", 200, 1); err != nil {
		t.Fatalf("OpenLong failed: %v", err)
	}

	_, err := tr.CloseLong("600519.SH", 0)
	if !errors.Is(err, ErrTPlusOneRestricted) {
		t.Fatalf("expected ErrTPlusOneRestricted, got %v", err)
	}
}
