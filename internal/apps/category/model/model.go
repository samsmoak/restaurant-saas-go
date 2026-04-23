package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Category struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	RestaurantID primitive.ObjectID `bson:"restaurant_id" json:"restaurant_id"`
	Name         string             `bson:"name" json:"name"`
	Description  string             `bson:"description,omitempty" json:"description,omitempty"`
	ImageURL     string             `bson:"image_url,omitempty" json:"image_url,omitempty"`
	DisplayOrder int                `bson:"display_order" json:"display_order"`
	IsActive     bool               `bson:"is_active" json:"is_active"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
}
