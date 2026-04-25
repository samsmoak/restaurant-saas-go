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
