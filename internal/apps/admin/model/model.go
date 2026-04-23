package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AdminUser binds a global user to a specific restaurant.
// The uniqueness key is (user_id, restaurant_id).
type AdminUser struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID       primitive.ObjectID `bson:"user_id" json:"user_id"`
	RestaurantID primitive.ObjectID `bson:"restaurant_id" json:"restaurant_id"`
	Email        string             `bson:"email" json:"email"`
	Role         string             `bson:"role" json:"role"` // owner | admin | staff
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
}

const (
	RoleOwner = "owner"
	RoleAdmin = "admin"
	RoleStaff = "staff"
)
