package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/stripe/stripe-go/v76"

	orderModel "restaurantsaas/internal/apps/order/model"
	orderSvc "restaurantsaas/internal/apps/order/service"
	stripeutil "restaurantsaas/internal/stripe"
)

type PaymentService interface {
	CreateIntentForOrder(ctx context.Context, result *orderSvc.CheckoutResult) (string, string, error)
	HandleWebhook(ctx context.Context, rawBody []byte, signature string) error
}

type paymentService struct {
	orderSvc orderSvc.OrderService
}

func NewPaymentService(orders orderSvc.OrderService) PaymentService {
	return &paymentService{orderSvc: orders}
}

func (s *paymentService) CreateIntentForOrder(ctx context.Context, result *orderSvc.CheckoutResult) (string, string, error) {
	if !stripeutil.IsConfigured() {
		return "", "", fmt.Errorf("stripe is not configured")
	}
	sc, err := stripeutil.NewClient()
	if err != nil {
		return "", "", err
	}
	order := result.Order
	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(result.Amount),
		Currency: stripe.String(result.Currency),
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
	}
	params.AddMetadata("order_id", order.ID.Hex())
	params.AddMetadata("order_number", order.OrderNumber)
	if order.CustomerID != nil {
		params.AddMetadata("customer_id", order.CustomerID.Hex())
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
