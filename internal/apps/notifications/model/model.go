package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Notification kinds + colours from BACKEND_REQUIREMENTS.md §8.
const (
	KindCourier     = "courier"
	KindPromo       = "promo"
	KindReview      = "review"
	KindAITip       = "ai_tip"
	KindOrderStatus = "order_status"

	ColorPrimary = "primary"
	ColorBlue    = "blue"
	ColorGreen   = "green"
)

type Notification struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"     json:"id"`
	UserID    primitive.ObjectID `bson:"user_id"           json:"-"`
	Kind      string             `bson:"kind"              json:"kind"`
	Title     string             `bson:"title"             json:"title"`
	Body      string             `bson:"body,omitempty"    json:"body"`
	IconEmoji string             `bson:"icon_emoji,omitempty" json:"icon_emoji"`
	Color     string             `bson:"color,omitempty"   json:"color"`
	DeepLink  string             `bson:"deep_link,omitempty" json:"deep_link,omitempty"`
	Read      bool               `bson:"read"              json:"read"`
	CreatedAt time.Time          `bson:"created_at"        json:"created_at"`
}
