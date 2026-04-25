package stripeutil

import (
	"fmt"
	"os"

	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/client"
	"github.com/stripe/stripe-go/v76/webhook"
)

func IsConfigured() bool {
	return os.Getenv("STRIPE_SECRET_KEY") != ""
}

func NewClient() (*client.API, error) {
	key := os.Getenv("STRIPE_SECRET_KEY")
	if key == "" {
		return nil, fmt.Errorf("STRIPE_SECRET_KEY not configured")
	}
	sc := &client.API{}
	sc.Init(key, nil)
	return sc, nil
}

func ConstructEvent(rawBody []byte, signature string) (stripe.Event, error) {
	secret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	if secret == "" {
		return stripe.Event{}, fmt.Errorf("STRIPE_WEBHOOK_SECRET not configured")
	}
	return webhook.ConstructEvent(rawBody, signature, secret)
}

func ConstructBillingEvent(rawBody []byte, signature string) (stripe.Event, error) {
	secret := os.Getenv("STRIPE_BILLING_WEBHOOK_SECRET")
	if secret == "" {
		secret = os.Getenv("STRIPE_WEBHOOK_SECRET")
	}
	if secret == "" {
		return stripe.Event{}, fmt.Errorf("STRIPE_BILLING_WEBHOOK_SECRET not configured")
	}
	return webhook.ConstructEvent(rawBody, signature, secret)
}
