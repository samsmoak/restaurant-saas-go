package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Billing struct {
	ID                 primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	RestaurantID       primitive.ObjectID `bson:"restaurant_id" json:"restaurant_id"`
	StripeCustomerID   string             `bson:"stripe_customer_id,omitempty" json:"-"`
	SetupFeePaid       bool               `bson:"setup_fee_paid" json:"setup_fee_paid"`
	SubscriptionStatus string             `bson:"subscription_status" json:"subscription_status"`
	SubscriptionID     string             `bson:"subscription_id,omitempty" json:"-"`
	CurrentPeriodEnd   *time.Time         `bson:"current_period_end,omitempty" json:"current_period_end"`
	CreatedAt          time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt          time.Time          `bson:"updated_at" json:"updated_at"`
}

// BillingUsage tracks per-month per-restaurant order counts and the
// accumulated $0.99 per-order transaction fee.
type BillingUsage struct {
	ID               primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	RestaurantID     primitive.ObjectID `bson:"restaurant_id" json:"restaurant_id"`
	PeriodStart      time.Time          `bson:"period_start" json:"period_start"`
	PeriodEnd        time.Time          `bson:"period_end" json:"period_end"`
	OrderCount       int                `bson:"order_count" json:"order_count"`
	PerOrderFeeTotal float64            `bson:"per_order_fee_total" json:"per_order_fee_total"`
	Currency         string             `bson:"currency" json:"currency"`
	CreatedAt        time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt        time.Time          `bson:"updated_at" json:"updated_at"`
}
