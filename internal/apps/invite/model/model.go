package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AdminInvite struct {
	ID           primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	RestaurantID primitive.ObjectID  `bson:"restaurant_id" json:"restaurant_id"`
	Code         string              `bson:"code" json:"code"`
	Email        string              `bson:"email,omitempty" json:"email,omitempty"`
	Note         string              `bson:"note,omitempty" json:"note,omitempty"`
	Role         string              `bson:"role,omitempty" json:"role,omitempty"` // admin | staff (empty = admin)
	CreatedBy    primitive.ObjectID  `bson:"created_by,omitempty" json:"created_by,omitempty"`
	UsedBy       *primitive.ObjectID `bson:"used_by,omitempty" json:"used_by,omitempty"`
	UsedAt       *time.Time          `bson:"used_at,omitempty" json:"used_at,omitempty"`
	Revoked      bool                `bson:"revoked" json:"revoked"`
	CreatedAt    time.Time           `bson:"created_at" json:"created_at"`
}
