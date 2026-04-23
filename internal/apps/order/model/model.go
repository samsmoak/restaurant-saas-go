package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	OrderStatusNew       = "new"
	OrderStatusPreparing = "preparing"
	OrderStatusReady     = "ready"
	OrderStatusCompleted = "completed"
	OrderStatusCancelled = "cancelled"
)

var OrderStatusFlow = []string{
	OrderStatusNew, OrderStatusPreparing, OrderStatusReady, OrderStatusCompleted,
}

var NextStatus = map[string]string{
	OrderStatusNew:       OrderStatusPreparing,
	OrderStatusPreparing: OrderStatusReady,
	OrderStatusReady:     OrderStatusCompleted,
}

var NextStatusAction = map[string]string{
	OrderStatusNew:       "Start Preparing",
	OrderStatusPreparing: "Mark as Ready",
	OrderStatusReady:     "Complete Order",
}

func IsValidStatus(s string) bool {
	switch s {
	case OrderStatusNew, OrderStatusPreparing, OrderStatusReady, OrderStatusCompleted, OrderStatusCancelled:
		return true
	}
	return false
}

const (
	PaymentStatusPending = "pending"
	PaymentStatusPaid    = "paid"
	PaymentStatusFailed  = "failed"
)

const (
	OrderTypePickup   = "pickup"
	OrderTypeDelivery = "delivery"
)

type OrderLineSize struct {
	Name          string  `bson:"name" json:"name"`
	PriceModifier float64 `bson:"price_modifier" json:"price_modifier"`
}

type OrderLineExtra struct {
	Name  string  `bson:"name" json:"name"`
	Price float64 `bson:"price" json:"price"`
}

type OrderLine struct {
	ID                  string           `bson:"id" json:"id"`
	Name                string           `bson:"name" json:"name"`
	Quantity            int              `bson:"quantity" json:"quantity"`
	BasePrice           float64          `bson:"base_price" json:"base_price"`
	SelectedSize        *OrderLineSize   `bson:"selected_size,omitempty" json:"selected_size"`
	SelectedExtras      []OrderLineExtra `bson:"selected_extras" json:"selected_extras"`
	SpecialInstructions string           `bson:"special_instructions,omitempty" json:"special_instructions,omitempty"`
	ItemTotal           float64          `bson:"item_total" json:"item_total"`
}

type Order struct {
	ID                  primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	RestaurantID        primitive.ObjectID  `bson:"restaurant_id" json:"restaurant_id"`
	OrderNumber         string              `bson:"order_number" json:"order_number"`
	Status              string              `bson:"status" json:"status"`
	OrderType           string              `bson:"order_type" json:"order_type"`
	CustomerID          *primitive.ObjectID `bson:"customer_id,omitempty" json:"customer_id,omitempty"`
	CustomerName        string              `bson:"customer_name" json:"customer_name"`
	CustomerPhone       string              `bson:"customer_phone" json:"customer_phone"`
	CustomerEmail       string              `bson:"customer_email,omitempty" json:"customer_email,omitempty"`
	DeliveryAddress     string              `bson:"delivery_address,omitempty" json:"delivery_address,omitempty"`
	DeliveryNotes       string              `bson:"delivery_notes,omitempty" json:"delivery_notes,omitempty"`
	Items               []OrderLine         `bson:"items" json:"items"`
	Subtotal            float64             `bson:"subtotal" json:"subtotal"`
	DeliveryFee         float64             `bson:"delivery_fee" json:"delivery_fee"`
	Total               float64             `bson:"total" json:"total"`
	PaymentIntentID     string              `bson:"payment_intent_id,omitempty" json:"payment_intent_id,omitempty"`
	PaymentStatus       string              `bson:"payment_status" json:"payment_status"`
	SpecialInstructions string              `bson:"special_instructions,omitempty" json:"special_instructions,omitempty"`
	EstimatedReadyTime  *time.Time          `bson:"estimated_ready_time,omitempty" json:"estimated_ready_time,omitempty"`
	CreatedAt           time.Time           `bson:"created_at" json:"created_at"`
}
