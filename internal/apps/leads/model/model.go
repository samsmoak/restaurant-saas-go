package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Lead struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	RestaurantName string             `bson:"restaurant_name" json:"restaurant_name"`
	OwnerName      string             `bson:"owner_name" json:"owner_name"`
	Phone          string             `bson:"phone" json:"phone"`
	Email          string             `bson:"email" json:"email"`
	CityState      string             `bson:"city_state" json:"city_state"`
	RevenueRange   string             `bson:"revenue_range,omitempty" json:"revenue_range,omitempty"`
	Source         string             `bson:"source,omitempty" json:"source,omitempty"`
	Message        string             `bson:"message,omitempty" json:"message,omitempty"`
	SubmittedAt    time.Time          `bson:"submitted_at" json:"submitted_at"`
}
