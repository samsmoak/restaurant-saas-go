package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Favorite struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	CustomerID   primitive.ObjectID `bson:"customer_id" json:"customer_id"`
	RestaurantID primitive.ObjectID `bson:"restaurant_id" json:"restaurant_id"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
}
