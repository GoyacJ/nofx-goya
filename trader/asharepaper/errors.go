package asharepaper

import "errors"

var (
	ErrUnsupportedForCashAccount = errors.New("unsupported for cash account")
	ErrInvalidLotSize            = errors.New("invalid lot size, minimum tradable lot is 100 shares")
	ErrTPlusOneRestricted        = errors.New("t+1 restriction: shares bought today are not sellable")
	ErrOutsideTradingSession     = errors.New("outside ashare trading session")
	ErrPriceLimitReached         = errors.New("price limit reached")
)
