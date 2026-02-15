package qmt

import "errors"

var (
	ErrUnsupportedForCashAccount = errors.New("unsupported for cash account")
	ErrOutsideTradingSession     = errors.New("outside A-share trading session")
	ErrInvalidLotSize            = errors.New("quantity must be at least 100 shares and in 100-share lots")
	ErrTPlusOneRestricted        = errors.New("t+1 restriction: no sellable shares available")
	ErrPriceLimitReached         = errors.New("order rejected by daily price limit")
)
