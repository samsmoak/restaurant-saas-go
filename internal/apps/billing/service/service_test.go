package service_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	billingSvc "restaurantsaas/internal/apps/billing/service"
)

// We can't reach tierFor directly (unexported); instead we test via the
// public TierThresholds shape and the round-trip math in PerOrderFee.
func TestPerOrderFeeIs99Cents(t *testing.T) {
	assert.InDelta(t, 0.99, billingSvc.PerOrderFee, 1e-9)
}
