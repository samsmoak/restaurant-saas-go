package service

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"restaurantsaas/internal/apps/promos/model"
	"restaurantsaas/internal/apps/promos/repository"
)

// PromoService is the narrow interface order/service depends on at
// checkout time to translate a user-typed code into a discount in
// cents.  Unknown / expired / inactive codes return 0 — the spec is
// silent about surfacing "invalid promo" errors so we honour the
// principle of least surprise and just skip the discount.
type PromoService interface {
	// LookupDiscountCents returns the cents to subtract from total
	// for the given code and current subtotal.  The returned int64
	// is always >= 0; the caller stores it as a negative discount.
	LookupDiscountCents(ctx context.Context, code string, subtotalCents int64) int64
	// SeedFromEnv is best-effort and only runs once at boot.  See
	// PROMO_WELCOME10_PERCENT.
	SeedFromEnv(ctx context.Context)
}

type promoService struct {
	repo *repository.PromoRepository
}

func NewPromoService(repo *repository.PromoRepository) PromoService {
	return &promoService{repo: repo}
}

func (s *promoService) LookupDiscountCents(ctx context.Context, code string, subtotalCents int64) int64 {
	code = strings.TrimSpace(code)
	if code == "" || s == nil || s.repo == nil {
		return 0
	}
	p, err := s.repo.FindByCode(ctx, code)
	if err != nil || p == nil {
		return 0
	}
	if p.ExpiresAt != nil && time.Now().After(*p.ExpiresAt) {
		return 0
	}
	if p.MinSubtotalCents > 0 && subtotalCents < p.MinSubtotalCents {
		return 0
	}
	if p.AmountOffCents > 0 {
		if p.AmountOffCents > subtotalCents {
			return subtotalCents
		}
		return p.AmountOffCents
	}
	if p.PercentOff > 0 {
		off := int64(float64(subtotalCents) * p.PercentOff / 100.0)
		if off > subtotalCents {
			off = subtotalCents
		}
		return off
	}
	return 0
}

// SeedFromEnv upserts a single WELCOME10 row when PROMO_WELCOME10_PERCENT
// is set (defaults to 10 when the variable is empty but the seed file
// is invoked anyway). Safe to call repeatedly.
func (s *promoService) SeedFromEnv(ctx context.Context) {
	if s == nil || s.repo == nil {
		return
	}
	pctRaw := strings.TrimSpace(os.Getenv("PROMO_WELCOME10_PERCENT"))
	if pctRaw == "" {
		pctRaw = "10"
	}
	pct, err := strconv.ParseFloat(pctRaw, 64)
	if err != nil || pct <= 0 || pct > 100 {
		return
	}
	if err := s.repo.Upsert(ctx, &model.Promo{
		Code:       "WELCOME10",
		PercentOff: pct,
		Active:     true,
		CreatedAt:  time.Now().UTC(),
	}); err != nil {
		log.Printf("promoService.SeedFromEnv: %v", err)
	}
}
