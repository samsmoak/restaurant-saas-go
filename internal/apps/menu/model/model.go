package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"restaurantsaas/internal/pkg/money"
)

type ItemSize struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name          string             `bson:"name" json:"name"`
	PriceModifier float64            `bson:"price_modifier" json:"price_modifier"`
	IsDefault     bool               `bson:"is_default" json:"is_default"`
}

type ItemExtra struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name        string             `bson:"name" json:"name"`
	Price       float64            `bson:"price" json:"price"`
	IsAvailable bool               `bson:"is_available" json:"is_available"`
}

type MenuItem struct {
	ID           primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	RestaurantID primitive.ObjectID  `bson:"restaurant_id" json:"restaurant_id"`
	CategoryID   *primitive.ObjectID `bson:"category_id,omitempty" json:"category_id,omitempty"`
	Name         string              `bson:"name" json:"name"`
	Description  string              `bson:"description,omitempty" json:"description,omitempty"`
	BasePrice    float64             `bson:"base_price" json:"base_price"`
	ImageURL     string              `bson:"image_url,omitempty" json:"image_url,omitempty"`
	IsAvailable  bool                `bson:"is_available" json:"is_available"`
	IsFeatured   bool                `bson:"is_featured" json:"is_featured"`
	DisplayOrder int                 `bson:"display_order" json:"display_order"`
	Tags         []string            `bson:"tags,omitempty" json:"tags,omitempty"`
	Sizes        []ItemSize          `bson:"sizes" json:"sizes"`
	Extras       []ItemExtra         `bson:"extras" json:"extras"`
	CreatedAt    time.Time           `bson:"created_at" json:"created_at"`
}

func (m *MenuItem) EnsureSlices() {
	if m.Sizes == nil {
		m.Sizes = []ItemSize{}
	}
	if m.Extras == nil {
		m.Extras = []ItemExtra{}
	}
	if m.Tags == nil {
		m.Tags = []string{}
	}
}

// ItemSizePublicView mirrors ItemSize with money in cents.
type ItemSizePublicView struct {
	ID                 primitive.ObjectID `json:"id"`
	Name               string             `json:"name"`
	PriceModifier      float64            `json:"price_modifier"`
	PriceModifierCents int64              `json:"price_modifier_cents"`
	IsDefault          bool               `json:"is_default"`
}

// ItemExtraPublicView mirrors ItemExtra with money in cents.
type ItemExtraPublicView struct {
	ID         primitive.ObjectID `json:"id"`
	Name       string             `json:"name"`
	Price      float64            `json:"price"`
	PriceCents int64              `json:"price_cents"`
}

// MenuItemPublicView is the customer-facing shape required by
// BACKEND_REQUIREMENTS.md §3 "GET /api/r/{id}/menu".  Money is in
// cents; the legacy float fields are also emitted for the existing
// customer/admin Flutter clients that still consume them.
type MenuItemPublicView struct {
	ID             primitive.ObjectID    `json:"id"`
	RestaurantID   primitive.ObjectID    `json:"restaurant_id"`
	CategoryID     *primitive.ObjectID   `json:"category_id,omitempty"`
	Name           string                `json:"name"`
	Description    string                `json:"description,omitempty"`
	BasePrice      float64               `json:"base_price"`
	BasePriceCents int64                 `json:"base_price_cents"`
	ImageURL       string                `json:"image_url,omitempty"`
	IsAvailable    bool                  `json:"is_available"`
	IsFeatured     bool                  `json:"is_featured"`
	DisplayOrder   int                   `json:"display_order"`
	Tags           []string              `json:"tags"`
	Sizes          []ItemSizePublicView  `json:"sizes"`
	Extras         []ItemExtraPublicView `json:"extras"`
}

// PublicView projects a MenuItem onto the customer contract.
func (m *MenuItem) PublicView() *MenuItemPublicView {
	if m == nil {
		return nil
	}
	tags := m.Tags
	if tags == nil {
		tags = []string{}
	}
	sizes := make([]ItemSizePublicView, 0, len(m.Sizes))
	for _, s := range m.Sizes {
		sizes = append(sizes, ItemSizePublicView{
			ID:                 s.ID,
			Name:               s.Name,
			PriceModifier:      s.PriceModifier,
			PriceModifierCents: money.ToCents(s.PriceModifier),
			IsDefault:          s.IsDefault,
		})
	}
	extras := make([]ItemExtraPublicView, 0, len(m.Extras))
	for _, e := range m.Extras {
		if !e.IsAvailable {
			continue
		}
		extras = append(extras, ItemExtraPublicView{
			ID:         e.ID,
			Name:       e.Name,
			Price:      e.Price,
			PriceCents: money.ToCents(e.Price),
		})
	}
	return &MenuItemPublicView{
		ID:             m.ID,
		RestaurantID:   m.RestaurantID,
		CategoryID:     m.CategoryID,
		Name:           m.Name,
		Description:    m.Description,
		BasePrice:      m.BasePrice,
		BasePriceCents: money.ToCents(m.BasePrice),
		ImageURL:       m.ImageURL,
		IsAvailable:    m.IsAvailable,
		IsFeatured:     m.IsFeatured,
		DisplayOrder:   m.DisplayOrder,
		Tags:           tags,
		Sizes:          sizes,
		Extras:         extras,
	}
}
