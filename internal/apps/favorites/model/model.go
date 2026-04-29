package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Favorite is one row per (customer, restaurant) or (customer,
// menu_item) pair. Exactly one of RestaurantID / MenuItemID is set;
// the unique partial index in database.EnsureIndexes enforces that.
type Favorite struct {
	ID           primitive.ObjectID  `bson:"_id,omitempty"          json:"id"`
	CustomerID   primitive.ObjectID  `bson:"customer_id"            json:"customer_id"`
	RestaurantID primitive.ObjectID  `bson:"restaurant_id,omitempty" json:"restaurant_id,omitempty"`
	MenuItemID   *primitive.ObjectID `bson:"menu_item_id,omitempty" json:"menu_item_id,omitempty"`
	CreatedAt    time.Time           `bson:"created_at"             json:"created_at"`
}
