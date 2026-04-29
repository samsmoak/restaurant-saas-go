package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Promo is a coupon row used by orderService when computing discounts
// at checkout. The Savorar client does not have a "list available
// promos" endpoint; the user types a code on the cart screen and the
// server applies it silently when valid.
type Promo struct {
	ID                primitive.ObjectID `bson:"_id,omitempty"             json:"id"`
	Code              string             `bson:"code"                      json:"code"`
	PercentOff        float64            `bson:"percent_off,omitempty"     json:"percent_off,omitempty"`
	AmountOffCents    int64              `bson:"amount_off_cents,omitempty" json:"amount_off_cents,omitempty"`
	MinSubtotalCents  int64              `bson:"min_subtotal_cents,omitempty" json:"min_subtotal_cents,omitempty"`
	ExpiresAt         *time.Time         `bson:"expires_at,omitempty"      json:"expires_at,omitempty"`
	Active            bool               `bson:"active"                    json:"active"`
	CreatedAt         time.Time          `bson:"created_at"                json:"created_at"`
}
