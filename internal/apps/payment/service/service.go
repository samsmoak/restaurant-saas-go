package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/stripe/stripe-go/v76"
	"go.mongodb.org/mongo-driver/bson/primitive"

	orderModel "restaurantsaas/internal/apps/order/model"
	orderSvc "restaurantsaas/internal/apps/order/service"
	userRepoPkg "restaurantsaas/internal/apps/user/repository"
	stripeutil "restaurantsaas/internal/stripe"
)

type PaymentMethodInfo struct {
	ID       string `json:"id"`
	Brand    string `json:"brand"`
	Last4    string `json:"last4"`
	ExpMonth int64  `json:"exp_month"`
	ExpYear  int64  `json:"exp_year"`
}

type CreateIntentOptions struct {
	StripeCustomerID  string
	PaymentMethodID   string
	SavePaymentMethod bool
}

type PaymentService interface {
	CreateIntentForOrder(ctx context.Context, result *orderSvc.CheckoutResult, opts *CreateIntentOptions) (intentID string, clientSecret string, err error)
	HandleWebhook(ctx context.Context, rawBody []byte, signature string) error

	GetStripeCustomerID(ctx context.Context, userID primitive.ObjectID) (string, error)
	EnsureStripeCustomer(ctx context.Context, userID primitive.ObjectID, email, name string) (string, error)
	CreateSetupIntent(ctx context.Context, stripeCustomerID string) (string, error)
	ListPaymentMethods(ctx context.Context, stripeCustomerID string) ([]PaymentMethodInfo, error)
	DetachPaymentMethod(ctx context.Context, pmID string) error
}

type paymentService struct {
	orderSvc    orderSvc.OrderService
	profileRepo *userRepoPkg.CustomerProfileRepository
}

func NewPaymentService(orders orderSvc.OrderService, profileRepo *userRepoPkg.CustomerProfileRepository) PaymentService {
	return &paymentService{orderSvc: orders, profileRepo: profileRepo}
}

func (s *paymentService) CreateIntentForOrder(ctx context.Context, result *orderSvc.CheckoutResult, opts *CreateIntentOptions) (string, string, error) {
	if !stripeutil.IsConfigured() {
		return "", "", fmt.Errorf("stripe is not configured")
	}
	sc, err := stripeutil.NewClient()
	if err != nil {
		return "", "", err
	}
	order := result.Order
	params := &stripe.PaymentIntentParams{
		Amount:             stripe.Int64(result.Amount),
		Currency:           stripe.String(result.Currency),
		PaymentMethodTypes: []*string{stripe.String("card")},
	}
	params.AddMetadata("order_id", order.ID.Hex())
	params.AddMetadata("order_number", order.OrderNumber)
	if order.CustomerID != nil {
		params.AddMetadata("customer_id", order.CustomerID.Hex())
	}
	if opts != nil {
		if opts.StripeCustomerID != "" {
			params.Customer = stripe.String(opts.StripeCustomerID)
			if opts.SavePaymentMethod {
				params.SetupFutureUsage = stripe.String(string(stripe.PaymentIntentSetupFutureUsageOnSession))
			}
		}
		if opts.PaymentMethodID != "" {
			params.PaymentMethod = stripe.String(opts.PaymentMethodID)
			params.Confirm = stripe.Bool(true)
		}
	}
	intent, err := sc.PaymentIntents.New(params)
	if err != nil {
		return "", "", fmt.Errorf("stripe create intent: %w", err)
	}
	return intent.ID, intent.ClientSecret, nil
}

func (s *paymentService) HandleWebhook(ctx context.Context, rawBody []byte, signature string) error {
	event, err := stripeutil.ConstructEvent(rawBody, signature)
	if err != nil {
		return fmt.Errorf("webhook signature verification failed: %w", err)
	}
	switch event.Type {
	case "payment_intent.succeeded":
		var pi stripe.PaymentIntent
		if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
			return fmt.Errorf("unmarshal payment_intent: %w", err)
		}
		if _, err := s.orderSvc.UpdatePaymentStatusByIntent(ctx, pi.ID, orderModel.PaymentStatusPaid); err != nil {
			log.Printf("paymentService.HandleWebhook: update paid: %v", err)
		}
	case "payment_intent.payment_failed":
		var pi stripe.PaymentIntent
		if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
			return fmt.Errorf("unmarshal payment_intent: %w", err)
		}
		if _, err := s.orderSvc.UpdatePaymentStatusByIntent(ctx, pi.ID, orderModel.PaymentStatusFailed); err != nil {
			log.Printf("paymentService.HandleWebhook: update failed: %v", err)
		}
	default:
		log.Printf("paymentService.HandleWebhook: ignoring event %s", event.Type)
	}
	return nil
}

func (s *paymentService) GetStripeCustomerID(ctx context.Context, userID primitive.ObjectID) (string, error) {
	profile, err := s.profileRepo.FindByUserID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("GetStripeCustomerID: %w", err)
	}
	if profile == nil {
		return "", nil
	}
	return profile.StripeCustomerID, nil
}

func (s *paymentService) EnsureStripeCustomer(ctx context.Context, userID primitive.ObjectID, email, name string) (string, error) {
	stripeID, err := s.GetStripeCustomerID(ctx, userID)
	if err != nil {
		return "", err
	}
	if stripeID != "" {
		return stripeID, nil
	}
	if !stripeutil.IsConfigured() {
		return "", fmt.Errorf("stripe is not configured")
	}
	sc, err := stripeutil.NewClient()
	if err != nil {
		return "", err
	}
	cust, err := sc.Customers.New(&stripe.CustomerParams{
		Email: stripe.String(email),
		Name:  stripe.String(name),
	})
	if err != nil {
		return "", fmt.Errorf("EnsureStripeCustomer: %w", err)
	}
	if err := s.profileRepo.SetStripeCustomerID(ctx, userID, cust.ID); err != nil {
		log.Printf("EnsureStripeCustomer: save stripe_customer_id: %v", err)
	}
	return cust.ID, nil
}

func (s *paymentService) CreateSetupIntent(ctx context.Context, stripeCustomerID string) (string, error) {
	if !stripeutil.IsConfigured() {
		return "", fmt.Errorf("stripe is not configured")
	}
	sc, err := stripeutil.NewClient()
	if err != nil {
		return "", err
	}
	si, err := sc.SetupIntents.New(&stripe.SetupIntentParams{
		Customer:           stripe.String(stripeCustomerID),
		PaymentMethodTypes: []*string{stripe.String("card")},
		Usage:              stripe.String(string(stripe.SetupIntentUsageOffSession)),
	})
	if err != nil {
		return "", fmt.Errorf("CreateSetupIntent: %w", err)
	}
	return si.ClientSecret, nil
}

func (s *paymentService) ListPaymentMethods(ctx context.Context, stripeCustomerID string) ([]PaymentMethodInfo, error) {
	if !stripeutil.IsConfigured() {
		return []PaymentMethodInfo{}, nil
	}
	sc, err := stripeutil.NewClient()
	if err != nil {
		return nil, err
	}
	pmType := string(stripe.PaymentMethodTypeCard)
	iter := sc.PaymentMethods.List(&stripe.PaymentMethodListParams{
		Customer: stripe.String(stripeCustomerID),
		Type:     &pmType,
	})
	var out []PaymentMethodInfo
	for iter.Next() {
		pm := iter.PaymentMethod()
		if pm.Card == nil {
			continue
		}
		out = append(out, PaymentMethodInfo{
			ID:       pm.ID,
			Brand:    string(pm.Card.Brand),
			Last4:    pm.Card.Last4,
			ExpMonth: pm.Card.ExpMonth,
			ExpYear:  pm.Card.ExpYear,
		})
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("ListPaymentMethods: %w", err)
	}
	if out == nil {
		out = []PaymentMethodInfo{}
	}
	return out, nil
}

func (s *paymentService) DetachPaymentMethod(ctx context.Context, pmID string) error {
	if !stripeutil.IsConfigured() {
		return fmt.Errorf("stripe is not configured")
	}
	sc, err := stripeutil.NewClient()
	if err != nil {
		return err
	}
	if _, err := sc.PaymentMethods.Detach(pmID, nil); err != nil {
		return fmt.Errorf("DetachPaymentMethod: %w", err)
	}
	return nil
}
