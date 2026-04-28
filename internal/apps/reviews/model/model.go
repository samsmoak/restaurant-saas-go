package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Review struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	RestaurantID primitive.ObjectID `bson:"restaurant_id" json:"restaurant_id"`
	OrderID      primitive.ObjectID `bson:"order_id" json:"order_id"`
	CustomerID   primitive.ObjectID `bson:"customer_id" json:"customer_id"`
	Rating       int                `bson:"rating" json:"rating"`
	Comment      string             `bson:"comment,omitempty" json:"comment,omitempty"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
}
