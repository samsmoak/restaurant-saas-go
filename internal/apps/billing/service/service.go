package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/stripe/stripe-go/v76"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"restaurantsaas/internal/apps/billing/model"
	"restaurantsaas/internal/apps/billing/repository"
	restaurantRepoPkg "restaurantsaas/internal/apps/restaurant/repository"
	stripeutil "restaurantsaas/internal/stripe"
)

var (
	ErrSetupFeeRequired = errors.New("setup fee required")
	ErrNoCustomer       = errors.New("no stripe customer")
)

// PerOrderFee is the flat $0.99 transaction fee charged on each paid order.
const PerOrderFee = 0.99

// UsageView is the response shape for GET /api/admin/billing/usage.
type UsageView struct {
	PeriodStart      time.Time      `json:"period_start"`
	PeriodEnd        time.Time      `json:"period_end"`
	OrderCount       int            `json:"order_count"`
	PerOrderFeeTotal float64        `json:"per_order_fee_total"`
	Tier             int            `json:"tier"`
	BasePrice        int            `json:"base_price"`
	TierThresholds   TierThresholds `json:"tier_thresholds"`
	ProjectedTotal   float64        `json:"projected_total"`
	Currency         string         `json:"currency"`
}

// TierView is the response shape for GET /api/admin/billing/tier.
type TierView struct {
	CurrentTier     int  `json:"current_tier"`
	BasePrice       int  `json:"base_price"`
	IncludesOrders  int  `json:"includes_orders"`
	NextTierAt      *int `json:"next_tier_at"`
	OrderCount      int  `json:"order_count"`
}

type TierBand struct {
	Min int  `json:"min"`
	Max *int `json:"max"`
}

type TierThresholds struct {
	Tier1 TierBand `json:"tier1"`
	Tier2 TierBand `json:"tier2"`
	Tier3 TierBand `json:"tier3"`
}

type BillingService interface {
	GetStatus(ctx context.Context, restaurantID primitive.ObjectID) (*model.Billing, error)
	CreateSetupCheckout(ctx context.Context, restaurantID primitive.ObjectID, returnURL string) (string, error)
	CreateSubscriptionCheckout(ctx context.Context, restaurantID primitive.ObjectID, returnURL string) (string, error)
	CreatePortal(ctx context.Context, restaurantID primitive.ObjectID, returnURL string) (string, error)
	HandleWebhook(ctx context.Context, rawBody []byte, signature string) error

	// RecordOrder is invoked from the Stripe payment_intent.succeeded
	// webhook to track the per-order fee against the current period.
	RecordOrder(ctx context.Context, restaurantID primitive.ObjectID, fee float64) error
	// GetUsage returns the current month's order count + accumulated fee.
	GetUsage(ctx context.Context, restaurantID primitive.ObjectID) (*UsageView, error)
	// GetTier returns the restaurant's current pricing tier.
	GetTier(ctx context.Context, restaurantID primitive.ObjectID) (*TierView, error)
}

type billingService struct {
	repo     *repository.BillingRepository
	restRepo *restaurantRepoPkg.RestaurantRepository
}

func NewBillingService(repo *repository.BillingRepository, restRepo *restaurantRepoPkg.RestaurantRepository) BillingService {
	return &billingService{repo: repo, restRepo: restRepo}
}

func (s *billingService) GetStatus(ctx context.Context, restaurantID primitive.ObjectID) (*model.Billing, error) {
	b, err := s.repo.FindByRestaurantID(ctx, restaurantID)
	if err != nil {
		return nil, fmt.Errorf("GetStatus: %w", err)
	}
	if b == nil {
		return &model.Billing{RestaurantID: restaurantID, SubscriptionStatus: "none"}, nil
	}
	if b.SubscriptionStatus == "" {
		b.SubscriptionStatus = "none"
	}
	return b, nil
}

func (s *billingService) getOrCreateStripeCustomer(ctx context.Context, restaurantID primitive.ObjectID) (string, error) {
	b, err := s.repo.FindByRestaurantID(ctx, restaurantID)
	if err != nil {
		return "", fmt.Errorf("getOrCreateStripeCustomer: %w", err)
	}
	if b != nil && b.StripeCustomerID != "" {
		return b.StripeCustomerID, nil
	}
	if !stripeutil.IsConfigured() {
		return "", fmt.Errorf("stripe is not configured")
	}
	rest, err := s.restRepo.GetByID(ctx, restaurantID)
	if err != nil {
		return "", fmt.Errorf("getOrCreateStripeCustomer: lookup restaurant: %w", err)
	}
	sc, err := stripeutil.NewClient()
	if err != nil {
		return "", err
	}
	cust, err := sc.Customers.New(&stripe.CustomerParams{
		Email: stripe.String(rest.Email),
		Name:  stripe.String(rest.Name),
		Params: stripe.Params{
			Metadata: map[string]string{"restaurant_id": restaurantID.Hex()},
		},
	})
	if err != nil {
		return "", fmt.Errorf("getOrCreateStripeCustomer: stripe: %w", err)
	}
	now := time.Now().UTC()
	if _, err := s.repo.Upsert(ctx, restaurantID, bson.D{
		{Key: "stripe_customer_id", Value: cust.ID},
		{Key: "updated_at", Value: now},
	}); err != nil {
		log.Printf("billingService.getOrCreateStripeCustomer: save stripe_customer_id: %v", err)
	}
	return cust.ID, nil
}

func (s *billingService) CreateSetupCheckout(ctx context.Context, restaurantID primitive.ObjectID, returnURL string) (string, error) {
	if !stripeutil.IsConfigured() {
		return "", fmt.Errorf("stripe is not configured")
	}
	customerID, err := s.getOrCreateStripeCustomer(ctx, restaurantID)
	if err != nil {
		return "", err
	}
	sc, err := stripeutil.NewClient()
	if err != nil {
		return "", err
	}
	session, err := sc.CheckoutSessions.New(&stripe.CheckoutSessionParams{
		Customer:   stripe.String(customerID),
		Mode:       stripe.String(string(stripe.CheckoutSessionModePayment)),
		SuccessURL: stripe.String(returnURL),
		CancelURL:  stripe.String(returnURL),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(os.Getenv("STRIPE_SETUP_PRICE_ID")),
				Quantity: stripe.Int64(1),
			},
		},
		Params: stripe.Params{
			Metadata: map[string]string{
				"restaurant_id": restaurantID.Hex(),
				"type":          "setup_fee",
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("CreateSetupCheckout: stripe: %w", err)
	}
	return session.URL, nil
}

func (s *billingService) CreateSubscriptionCheckout(ctx context.Context, restaurantID primitive.ObjectID, returnURL string) (string, error) {
	b, err := s.repo.FindByRestaurantID(ctx, restaurantID)
	if err != nil {
		return "", fmt.Errorf("CreateSubscriptionCheckout: %w", err)
	}
	if b == nil || !b.SetupFeePaid {
		return "", ErrSetupFeeRequired
	}
	if !stripeutil.IsConfigured() {
		return "", fmt.Errorf("stripe is not configured")
	}
	customerID, err := s.getOrCreateStripeCustomer(ctx, restaurantID)
	if err != nil {
		return "", err
	}
	sc, err := stripeutil.NewClient()
	if err != nil {
		return "", err
	}
	session, err := sc.CheckoutSessions.New(&stripe.CheckoutSessionParams{
		Customer:   stripe.String(customerID),
		Mode:       stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL: stripe.String(returnURL),
		CancelURL:  stripe.String(returnURL),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(os.Getenv("STRIPE_SUBSCRIPTION_PRICE_ID")),
				Quantity: stripe.Int64(1),
			},
		},
		Params: stripe.Params{
			Metadata: map[string]string{
				"restaurant_id": restaurantID.Hex(),
				"type":          "subscription",
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("CreateSubscriptionCheckout: stripe: %w", err)
	}
	return session.URL, nil
}

func (s *billingService) CreatePortal(ctx context.Context, restaurantID primitive.ObjectID, returnURL string) (string, error) {
	b, err := s.repo.FindByRestaurantID(ctx, restaurantID)
	if err != nil {
		return "", fmt.Errorf("CreatePortal: %w", err)
	}
	if b == nil || b.StripeCustomerID == "" {
		return "", ErrNoCustomer
	}
	if !stripeutil.IsConfigured() {
		return "", fmt.Errorf("stripe is not configured")
	}
	sc, err := stripeutil.NewClient()
	if err != nil {
		return "", err
	}
	ps, err := sc.BillingPortalSessions.New(&stripe.BillingPortalSessionParams{
		Customer:  stripe.String(b.StripeCustomerID),
		ReturnURL: stripe.String(returnURL),
	})
	if err != nil {
		return "", fmt.Errorf("CreatePortal: stripe: %w", err)
	}
	return ps.URL, nil
}

func (s *billingService) HandleWebhook(ctx context.Context, rawBody []byte, signature string) error {
	event, err := stripeutil.ConstructBillingEvent(rawBody, signature)
	if err != nil {
		return fmt.Errorf("billing webhook signature verification failed: %w", err)
	}
	now := time.Now().UTC()
	switch event.Type {
	case "checkout.session.completed":
		var cs stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &cs); err != nil {
			return fmt.Errorf("unmarshal checkout.session: %w", err)
		}
		if cs.Metadata["type"] == "setup_fee" {
			rid := cs.Metadata["restaurant_id"]
			restaurantID, err := primitive.ObjectIDFromHex(rid)
			if err != nil {
				log.Printf("billingService.HandleWebhook: invalid restaurant_id in metadata: %v", err)
				break
			}
			if _, err := s.repo.Upsert(ctx, restaurantID, bson.D{
				{Key: "setup_fee_paid", Value: true},
				{Key: "updated_at", Value: now},
			}); err != nil {
				log.Printf("billingService.HandleWebhook: upsert setup_fee_paid: %v", err)
			}
		}
	case "customer.subscription.created", "customer.subscription.updated":
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			return fmt.Errorf("unmarshal subscription: %w", err)
		}
		b, err := s.repo.FindByStripeCustomerID(ctx, sub.Customer.ID)
		if err != nil || b == nil {
			log.Printf("billingService.HandleWebhook: billing row not found for customer %s", sub.Customer.ID)
			break
		}
		end := time.Unix(sub.CurrentPeriodEnd, 0).UTC()
		if _, err := s.repo.Upsert(ctx, b.RestaurantID, bson.D{
			{Key: "subscription_status", Value: string(sub.Status)},
			{Key: "subscription_id", Value: sub.ID},
			{Key: "current_period_end", Value: &end},
			{Key: "updated_at", Value: now},
		}); err != nil {
			log.Printf("billingService.HandleWebhook: upsert subscription: %v", err)
		}
	case "customer.subscription.deleted":
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			return fmt.Errorf("unmarshal subscription: %w", err)
		}
		b, err := s.repo.FindBySubscriptionID(ctx, sub.ID)
		if err != nil || b == nil {
			log.Printf("billingService.HandleWebhook: billing row not found for sub %s", sub.ID)
			break
		}
		if _, err := s.repo.Upsert(ctx, b.RestaurantID, bson.D{
			{Key: "subscription_status", Value: "canceled"},
			{Key: "subscription_id", Value: ""},
			{Key: "current_period_end", Value: nil},
			{Key: "updated_at", Value: now},
		}); err != nil {
			log.Printf("billingService.HandleWebhook: upsert canceled: %v", err)
		}
	default:
		log.Printf("billingService.HandleWebhook: ignoring event %s", event.Type)
	}
	return nil
}

func (s *billingService) RecordOrder(ctx context.Context, restaurantID primitive.ObjectID, fee float64) error {
	if err := s.repo.RecordUsageOrder(ctx, restaurantID, fee); err != nil {
		return fmt.Errorf("RecordOrder: %w", err)
	}
	return nil
}

// tierFor returns (tier, basePriceUSD, includesOrders, nextTierAt) for a given count.
func tierFor(count int) (int, int, int, *int) {
	const tier1Max = 250
	const tier2Max = 750
	t2 := tier2Max + 1
	if count <= tier1Max {
		next := tier1Max + 1
		return 1, 49, tier1Max, &next
	}
	if count <= tier2Max {
		return 2, 99, tier2Max, &t2
	}
	return 3, 149, 0, nil
}

func defaultTierThresholds() TierThresholds {
	t1Max := 250
	t2Min := 251
	t2Max := 750
	t3Min := 751
	return TierThresholds{
		Tier1: TierBand{Min: 1, Max: &t1Max},
		Tier2: TierBand{Min: t2Min, Max: &t2Max},
		Tier3: TierBand{Min: t3Min, Max: nil},
	}
}

func (s *billingService) GetUsage(ctx context.Context, restaurantID primitive.ObjectID) (*UsageView, error) {
	u, err := s.repo.GetCurrentUsage(ctx, restaurantID)
	if err != nil {
		return nil, fmt.Errorf("GetUsage: %w", err)
	}
	tier, base, _, _ := tierFor(u.OrderCount)
	currency := u.Currency
	if currency == "" {
		currency = "usd"
	}
	return &UsageView{
		PeriodStart:      u.PeriodStart,
		PeriodEnd:        u.PeriodEnd,
		OrderCount:       u.OrderCount,
		PerOrderFeeTotal: u.PerOrderFeeTotal,
		Tier:             tier,
		BasePrice:        base,
		TierThresholds:   defaultTierThresholds(),
		ProjectedTotal:   float64(base) + u.PerOrderFeeTotal,
		Currency:         currency,
	}, nil
}

func (s *billingService) GetTier(ctx context.Context, restaurantID primitive.ObjectID) (*TierView, error) {
	u, err := s.repo.GetCurrentUsage(ctx, restaurantID)
	if err != nil {
		return nil, fmt.Errorf("GetTier: %w", err)
	}
	tier, base, includes, next := tierFor(u.OrderCount)
	return &TierView{
		CurrentTier:    tier,
		BasePrice:      base,
		IncludesOrders: includes,
		NextTierAt:     next,
		OrderCount:     u.OrderCount,
	}, nil
}
