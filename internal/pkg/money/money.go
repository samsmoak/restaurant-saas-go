// Package money centralises the conversion between the float64 USD
// values used internally by the order/menu/restaurant models and the
// integer-cents shape required by the Savorar customer-facing JSON
// contract (see BACKEND_REQUIREMENTS.md §10).
package money

import "math"

// ToCents converts a float USD amount to integer cents using
// half-away-from-zero rounding so that 12.345 → 1235 and -12.345 → -1235.
func ToCents(usd float64) int64 {
	if usd >= 0 {
		return int64(math.Floor(usd*100 + 0.5))
	}
	return -int64(math.Floor(-usd*100 + 0.5))
}

// FromCents is the inverse of ToCents.
func FromCents(c int64) float64 {
	return float64(c) / 100
}
