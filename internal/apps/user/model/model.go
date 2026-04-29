package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Email         string             `bson:"email" json:"email"`
	PasswordHash  string             `bson:"password_hash,omitempty" json:"-"`
	GoogleSub     string             `bson:"google_sub,omitempty" json:"-"`
	FullName      string             `bson:"full_name" json:"full_name"`
	Phone         string             `bson:"phone,omitempty" json:"phone,omitempty"`
	EmailVerified bool               `bson:"email_verified" json:"email_verified"`
	CreatedAt     time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt     time.Time          `bson:"updated_at" json:"updated_at"`
}

type SavedAddress struct {
	ID       primitive.ObjectID `bson:"_id,omitempty"      json:"id"`
	Label    string             `bson:"label"              json:"label"`
	Address  string             `bson:"address"            json:"address"`
	City     string             `bson:"city"               json:"city"`
	State    string             `bson:"state"              json:"state"`
	Zip      string             `bson:"zip"                json:"zip"`
	Lat      float64            `bson:"lat,omitempty"      json:"lat,omitempty"`
	Lng      float64            `bson:"lng,omitempty"      json:"lng,omitempty"`
	Floor    string             `bson:"floor,omitempty"    json:"floor,omitempty"`
	Landmark string             `bson:"landmark,omitempty" json:"landmark,omitempty"`
}

type CustomerProfile struct {
	ID               primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID           primitive.ObjectID `bson:"user_id" json:"user_id"`
	Email            string             `bson:"email" json:"email"`
	FullName         string             `bson:"full_name" json:"full_name"`
	Phone            string             `bson:"phone" json:"phone"`
	DefaultAddress   string             `bson:"default_address" json:"default_address"`
	PhotoURL         string             `bson:"photo_url,omitempty" json:"photo_url,omitempty"`
	Addresses        []SavedAddress     `bson:"addresses,omitempty" json:"addresses,omitempty"`
	StripeCustomerID string             `bson:"stripe_customer_id,omitempty" json:"-"`
	CreatedAt        time.Time          `bson:"created_at" json:"created_at"`
}
