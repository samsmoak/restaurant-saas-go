package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type OpeningHoursDay struct {
	Open   string `bson:"open" json:"open"`
	Close  string `bson:"close" json:"close"`
	Closed bool   `bson:"closed" json:"closed"`
}

type Restaurant struct {
	ID                    primitive.ObjectID         `bson:"_id,omitempty" json:"id"`
	Slug                  string                     `bson:"slug" json:"slug"`
	OwnerID               primitive.ObjectID         `bson:"owner_id" json:"owner_id"`
	Name                  string                     `bson:"name" json:"name"`
	Description           string                     `bson:"description,omitempty" json:"description,omitempty"`
	LogoURL               string                     `bson:"logo_url,omitempty" json:"logo_url,omitempty"`
	Phone                 string                     `bson:"phone,omitempty" json:"phone,omitempty"`
	Email                 string                     `bson:"email,omitempty" json:"email,omitempty"`
	Address               string                     `bson:"address,omitempty" json:"address,omitempty"`
	Timezone              string                     `bson:"timezone,omitempty" json:"timezone,omitempty"`
	DeliveryFee           float64                    `bson:"delivery_fee" json:"delivery_fee"`
	MinOrderAmount        float64                    `bson:"min_order_amount" json:"min_order_amount"`
	EstimatedPickupTime   int                        `bson:"estimated_pickup_time" json:"estimated_pickup_time"`
	EstimatedDeliveryTime int                        `bson:"estimated_delivery_time" json:"estimated_delivery_time"`
	Currency              string                     `bson:"currency" json:"currency"`
	OpeningHours          map[string]OpeningHoursDay `bson:"opening_hours" json:"opening_hours"`
	ManualClosed          bool                       `bson:"manual_closed" json:"manual_closed"`
	StripeAccountID       string                     `bson:"stripe_account_id,omitempty" json:"-"`
	CreatedAt             time.Time                  `bson:"created_at" json:"created_at"`
	UpdatedAt             time.Time                  `bson:"updated_at" json:"updated_at"`
}

// PublicView strips fields customers should never see.
type PublicView struct {
	ID                    primitive.ObjectID         `json:"id"`
	Slug                  string                     `json:"slug"`
	Name                  string                     `json:"name"`
	Description           string                     `json:"description,omitempty"`
	LogoURL               string                     `json:"logo_url,omitempty"`
	Phone                 string                     `json:"phone,omitempty"`
	Address               string                     `json:"address,omitempty"`
	DeliveryFee           float64                    `json:"delivery_fee"`
	MinOrderAmount        float64                    `json:"min_order_amount"`
	EstimatedPickupTime   int                        `json:"estimated_pickup_time"`
	EstimatedDeliveryTime int                        `json:"estimated_delivery_time"`
	Currency              string                     `json:"currency"`
	OpeningHours          map[string]OpeningHoursDay `json:"opening_hours"`
	ManualClosed          bool                       `json:"manual_closed"`
}

func (r *Restaurant) PublicView() *PublicView {
	return &PublicView{
		ID:                    r.ID,
		Slug:                  r.Slug,
		Name:                  r.Name,
		Description:           r.Description,
		LogoURL:               r.LogoURL,
		Phone:                 r.Phone,
		Address:               r.Address,
		DeliveryFee:           r.DeliveryFee,
		MinOrderAmount:        r.MinOrderAmount,
		EstimatedPickupTime:   r.EstimatedPickupTime,
		EstimatedDeliveryTime: r.EstimatedDeliveryTime,
		Currency:              r.Currency,
		OpeningHours:          r.OpeningHours,
		ManualClosed:          r.ManualClosed,
	}
}

func DefaultOpeningHours() map[string]OpeningHoursDay {
	return map[string]OpeningHoursDay{
		"monday":    {Open: "09:00", Close: "22:00", Closed: false},
		"tuesday":   {Open: "09:00", Close: "22:00", Closed: false},
		"wednesday": {Open: "09:00", Close: "22:00", Closed: false},
		"thursday":  {Open: "09:00", Close: "22:00", Closed: false},
		"friday":    {Open: "09:00", Close: "23:00", Closed: false},
		"saturday":  {Open: "10:00", Close: "23:00", Closed: false},
		"sunday":    {Open: "10:00", Close: "21:00", Closed: false},
	}
}

func NewRestaurant(slug, name string, ownerID primitive.ObjectID) *Restaurant {
	now := time.Now().UTC()
	return &Restaurant{
		Slug:                  slug,
		OwnerID:               ownerID,
		Name:                  name,
		DeliveryFee:           0,
		MinOrderAmount:        0,
		EstimatedPickupTime:   20,
		EstimatedDeliveryTime: 45,
		Currency:              "USD",
		OpeningHours:          DefaultOpeningHours(),
		ManualClosed:          false,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
}
