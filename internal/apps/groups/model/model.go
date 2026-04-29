package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	orderModel "restaurantsaas/internal/apps/order/model"
)

const (
	MemberStatusEditing = "editing"
	MemberStatusReady   = "ready"
)

// Group is a host-led shared cart (BACKEND_REQUIREMENTS.md §9).
type Group struct {
	ID                       primitive.ObjectID `bson:"_id,omitempty"            json:"id"`
	ShareCode                string             `bson:"share_code"               json:"share_code"`
	HostUserID               primitive.ObjectID `bson:"host_user_id"             json:"host_user_id"`
	RestaurantID             primitive.ObjectID `bson:"restaurant_id"            json:"restaurant_id"`
	MinForFreeDeliveryCents  int64              `bson:"min_for_free_delivery_cents" json:"min_for_free_delivery_cents"`
	LockExpiresAt            *time.Time         `bson:"lock_expires_at,omitempty" json:"lock_expires_at"`
	CreatedAt                time.Time          `bson:"created_at"               json:"created_at"`
}

// GroupMember is one row per (group, user). Lines[] is each member's
// sub-cart, computed sub-totals are emitted in cents per the spec.
type GroupMember struct {
	ID            primitive.ObjectID  `bson:"_id,omitempty"          json:"id"`
	GroupID       primitive.ObjectID  `bson:"group_id"               json:"-"`
	UserID        primitive.ObjectID  `bson:"user_id"                json:"user_id"`
	Name          string              `bson:"name,omitempty"         json:"name"`
	AvatarURL     string              `bson:"avatar_url,omitempty"   json:"avatar_url,omitempty"`
	Lines         []orderModel.OrderLine `bson:"lines,omitempty"     json:"lines"`
	SubtotalCents int64               `bson:"subtotal_cents"         json:"subtotal_cents"`
	Status        string              `bson:"status"                 json:"status"`
	JoinedAt      time.Time           `bson:"joined_at"              json:"-"`
}
