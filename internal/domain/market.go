package domain

// MarketCode is a 2-letter ISO market identifier. Persisted as VARCHAR(2)
// across the schema. Source of truth for currency selection — never store
// currency separately from market.
type MarketCode string

const (
	MarketSN MarketCode = "SN"
	MarketUS MarketCode = "US"
)

// Currency is the ISO 4217 code (XOF / USD / …) derived from a market.
type Currency string

const (
	CurrencyXOF Currency = "XOF"
	CurrencyUSD Currency = "USD"
)

// CurrencyForMarket returns the canonical currency for a market.
//
// We never store currency next to market — it is *always* derived. This
// avoids the desync where someone writes market='US' alongside
// currency='XOF', which would produce nonsense at display time.
//
// Default for unknown codes is XOF (preserves SN behaviour for backfilled
// rows). New markets must be added here when their plans ship.
func CurrencyForMarket(m MarketCode) Currency {
	switch m {
	case MarketUS:
		return CurrencyUSD
	default:
		return CurrencyXOF
	}
}

// IsValidMarket returns true if the code matches a market we currently
// operate. Used by handlers when parsing user input (e.g. ?market= query
// param on admin endpoints).
func IsValidMarket(m string) bool {
	switch MarketCode(m) {
	case MarketSN, MarketUS:
		return true
	default:
		return false
	}
}
