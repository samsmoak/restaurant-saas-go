package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Review struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"            json:"id"`
	RestaurantID  primitive.ObjectID `bson:"restaurant_id"            json:"restaurant_id"`
	OrderID       primitive.ObjectID `bson:"order_id"                 json:"order_id"`
	CustomerID    primitive.ObjectID `bson:"customer_id"              json:"customer_id"`
	UserName      string             `bson:"user_name,omitempty"      json:"user_name"`
	Rating        int                `bson:"rating"                   json:"rating"`
	Tags          []string           `bson:"tags,omitempty"           json:"tags"`
	Comment       string             `bson:"comment,omitempty"        json:"comment,omitempty"`
	Photos        []string           `bson:"photos,omitempty"         json:"photos"`
	CourierThumbs string             `bson:"courier_thumbs,omitempty" json:"courier_thumbs,omitempty"`
	CreatedAt     time.Time          `bson:"created_at"               json:"created_at"`
}

// EnsureSlices makes the JSON shape consistent for the customer client
// even when no tags / photos were supplied (it expects empty arrays
// rather than null).
func (r *Review) EnsureSlices() {
	if r.Tags == nil {
		r.Tags = []string{}
	}
	if r.Photos == nil {
		r.Photos = []string{}
	}
}
