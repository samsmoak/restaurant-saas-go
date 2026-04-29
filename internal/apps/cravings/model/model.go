package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Craving is a saved Savor-AI conversation summary
// (BACKEND_REQUIREMENTS.md §7 "Cravings history").  Rows are written
// best-effort by aiService.StreamChat after the answer event fires.
type Craving struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"     json:"id"`
	UserID    primitive.ObjectID `bson:"user_id"           json:"-"`
	Title     string             `bson:"title"             json:"title"`
	Summary   string             `bson:"summary,omitempty" json:"summary"`
	Emoji     string             `bson:"emoji,omitempty"   json:"emoji"`
	DateLabel string             `bson:"date_label,omitempty" json:"date_label"`
	Match     int                `bson:"match"             json:"match"`
	Pinned    bool               `bson:"pinned"            json:"pinned"`
	CreatedAt time.Time          `bson:"created_at"        json:"-"`
}
