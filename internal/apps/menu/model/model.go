package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
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
}
