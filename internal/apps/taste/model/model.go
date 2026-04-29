package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TasteProfile is the user's saved spice / dietary / cuisine /
// allergens preferences (BACKEND_REQUIREMENTS.md §7 "Taste profile").
// One row per user; the unique index on user_id enforces that.
type TasteProfile struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"     json:"-"`
	UserID    primitive.ObjectID `bson:"user_id"            json:"-"`
	Spice     int                `bson:"spice"              json:"spice"`
	Dietary   []string           `bson:"dietary,omitempty"  json:"dietary"`
	Cuisines  []string           `bson:"cuisines,omitempty" json:"cuisines"`
	Allergens []string           `bson:"allergens,omitempty" json:"allergens"`
	UpdatedAt time.Time          `bson:"updated_at"         json:"-"`
}

// EnsureSlices makes the JSON shape stable when fields are missing
// (the client expects empty arrays rather than null).
func (t *TasteProfile) EnsureSlices() {
	if t.Dietary == nil {
		t.Dietary = []string{}
	}
	if t.Cuisines == nil {
		t.Cuisines = []string{}
	}
	if t.Allergens == nil {
		t.Allergens = []string{}
	}
}

// Default is what GET returns when the user has no row yet — the
// Savorar client falls back to these values for the Taste Profile
// setup wizard.
func Default() *TasteProfile {
	return &TasteProfile{
		Spice:     5,
		Dietary:   []string{},
		Cuisines:  []string{},
		Allergens: []string{},
	}
}
