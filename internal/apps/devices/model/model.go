package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Device is a single FCM-registered handset bound to a user
// (BACKEND_REQUIREMENTS.md §8 POST /api/me/devices). The unique
// index on (fcm_token, user_id) keeps duplicate rows out.
type Device struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID    primitive.ObjectID `bson:"user_id"       json:"-"`
	FCMToken  string             `bson:"fcm_token"     json:"fcm_token"`
	Platform  string             `bson:"platform"      json:"platform"`
	CreatedAt time.Time          `bson:"created_at"    json:"created_at"`
}
