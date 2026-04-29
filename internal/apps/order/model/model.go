package model

import (
	"encoding/json"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"restaurantsaas/internal/pkg/money"
)

const (
	OrderStatusNew       = "new"
	OrderStatusPreparing = "preparing"
	OrderStatusReady     = "ready"
	OrderStatusCompleted = "completed"
	OrderStatusDelivered = "delivered"
	OrderStatusCancelled = "cancelled"
)

var OrderStatusFlow = []string{
	OrderStatusNew, OrderStatusPreparing, OrderStatusReady, OrderStatusCompleted, OrderStatusDelivered,
}

var NextStatus = map[string]string{
	OrderStatusNew:       OrderStatusPreparing,
	OrderStatusPreparing: OrderStatusReady,
	OrderStatusReady:     OrderStatusCompleted,
}

var NextStatusDelivery = map[string]string{
	OrderStatusNew:       OrderStatusPreparing,
	OrderStatusPreparing: OrderStatusReady,
	OrderStatusReady:     OrderStatusDelivered,
}

var NextStatusAction = map[string]string{
	OrderStatusNew:       "Start Preparing",
	OrderStatusPreparing: "Mark as Ready",
	OrderStatusReady:     "Complete Order",
}

var NextStatusActionDelivery = map[string]string{
	OrderStatusNew:       "Start Preparing",
	OrderStatusPreparing: "Mark as Ready",
	OrderStatusReady:     "Mark as Delivered",
}

func IsValidStatus(s string) bool {
	switch s {
	case OrderStatusNew, OrderStatusPreparing, OrderStatusReady, OrderStatusCompleted, OrderStatusDelivered, OrderStatusCancelled:
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
	MenuItemID          string           `bson:"menu_item_id,omitempty" json:"menu_item_id,omitempty"`
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
	DeliveryMode        string              `bson:"delivery_mode,omitempty" json:"delivery_mode,omitempty"`
	CustomerID          *primitive.ObjectID `bson:"customer_id,omitempty" json:"customer_id,omitempty"`
	CustomerName        string              `bson:"customer_name" json:"customer_name"`
	CustomerPhone       string              `bson:"customer_phone" json:"customer_phone"`
	CustomerEmail       string              `bson:"customer_email,omitempty" json:"customer_email,omitempty"`
	DeliveryAddress     string              `bson:"delivery_address,omitempty" json:"delivery_address,omitempty"`
	DeliveryNotes       string              `bson:"delivery_notes,omitempty" json:"delivery_notes,omitempty"`
	Items               []OrderLine         `bson:"items" json:"items"`
	Subtotal            float64             `bson:"subtotal" json:"subtotal"`
	DeliveryFee         float64             `bson:"delivery_fee" json:"delivery_fee"`
	ServiceFee          float64             `bson:"service_fee,omitempty" json:"service_fee"`
	Tip                 float64             `bson:"tip,omitempty" json:"tip"`
	Tax                 float64             `bson:"tax,omitempty" json:"tax"`
	Discount            float64             `bson:"discount,omitempty" json:"discount"`
	PromoCode           string              `bson:"promo_code,omitempty" json:"promo_code,omitempty"`
	Total               float64             `bson:"total" json:"total"`
	Currency            string              `bson:"currency,omitempty" json:"currency,omitempty"`
	PaymentIntentID     string              `bson:"payment_intent_id,omitempty" json:"payment_intent_id,omitempty"`
	PaymentStatus       string              `bson:"payment_status" json:"payment_status"`
	SpecialInstructions string              `bson:"special_instructions,omitempty" json:"special_instructions,omitempty"`
	EstimatedReadyTime  *time.Time          `bson:"estimated_ready_time,omitempty" json:"estimated_ready_time,omitempty"`
	CreatedAt           time.Time           `bson:"created_at" json:"created_at"`
}

// ExternalStatus translates the internal order-state machine into the
// status enum the Savorar customer client expects (see
// BACKEND_REQUIREMENTS.md §5: confirmed | preparing | en_route |
// delivered | cancelled).
//
// Internal storage is unchanged so the existing admin app keeps reading
// new|preparing|ready|completed|delivered|cancelled.
func ExternalStatus(o *Order) string {
	if o == nil {
		return ""
	}
	switch o.Status {
	case OrderStatusNew:
		return "confirmed"
	case OrderStatusPreparing:
		return "preparing"
	case OrderStatusReady:
		if o.OrderType == OrderTypeDelivery {
			return "en_route"
		}
		return "preparing"
	case OrderStatusCompleted:
		return "delivered"
	case OrderStatusDelivered:
		return "delivered"
	case OrderStatusCancelled:
		return "cancelled"
	}
	return o.Status
}

// ItemsSummary builds a short comma-joined list of up to three line
// names for the customer-facing orders list (e.g. "Mutton biryani,
// Raita, Papadum").
func (o *Order) ItemsSummary() string {
	if o == nil || len(o.Items) == 0 {
		return ""
	}
	names := make([]string, 0, len(o.Items))
	for _, l := range o.Items {
		n := strings.TrimSpace(l.Name)
		if n == "" {
			continue
		}
		names = append(names, n)
		if len(names) == 3 {
			break
		}
	}
	return strings.Join(names, ", ")
}

// EstimatedDeliveryAt returns the timestamp the customer-facing client
// renders as the ETA. We prefer the explicit ready timestamp when
// present; otherwise we compute one from the order's creation time
// plus the restaurant-configured prep estimate.
func (o *Order) EstimatedDeliveryAt(estPickup, estDelivery int) time.Time {
	if o == nil {
		return time.Time{}
	}
	if o.EstimatedReadyTime != nil && !o.EstimatedReadyTime.IsZero() {
		return *o.EstimatedReadyTime
	}
	mins := estPickup
	if o.OrderType == OrderTypeDelivery {
		mins = estDelivery
	}
	if mins <= 0 {
		mins = 30
	}
	return o.CreatedAt.Add(time.Duration(mins) * time.Minute)
}

// PublicView is the customer-facing shape (BACKEND_REQUIREMENTS.md §5
// "GET /api/me/orders" + "GET /api/r/{restaurantId}/orders/{orderNumber}").
// It keeps every field of Order (so the legacy customer/admin apps that
// already consume the raw shape are unaffected) and adds:
//
//   - status translated via ExternalStatus
//   - items_summary
//   - placed_at, estimated_delivery_at
//   - *_cents money mirrors
//   - restaurant_name, restaurant_logo_url (joined)
type OrderPublicView struct {
	*Order
	StatusExternal      string    `json:"status_external"`
	PlacedAt            time.Time `json:"placed_at"`
	EstimatedDeliveryAt time.Time `json:"estimated_delivery_at"`
	ItemsSummary        string    `json:"items_summary"`

	SubtotalCents    int64 `json:"subtotal_cents"`
	DeliveryFeeCents int64 `json:"delivery_fee_cents"`
	ServiceFeeCents  int64 `json:"service_fee_cents"`
	TipCents         int64 `json:"tip_cents"`
	TaxCents         int64 `json:"tax_cents"`
	DiscountCents    int64 `json:"discount_cents"`
	TotalCents       int64 `json:"total_cents"`

	RestaurantName    string `json:"restaurant_name,omitempty"`
	RestaurantLogoURL string `json:"restaurant_logo_url,omitempty"`
}

// MarshalJSON overrides the embedded Order's `status` field with the
// spec-shaped external value (BACKEND_REQUIREMENTS.md §5).  The raw
// internal value is still emitted as `status_internal` so callers
// that want the storage state can read it.
func (v *OrderPublicView) MarshalJSON() ([]byte, error) {
	if v == nil || v.Order == nil {
		return json.Marshal(nil)
	}
	type Alias OrderPublicView
	raw, err := json.Marshal((*Alias)(v))
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if internal, ok := m["status"]; ok {
		m["status_internal"] = internal
	}
	m["status"] = v.StatusExternal
	return json.Marshal(m)
}

// BuildPublicView wraps an Order in its customer-facing view. The
// estPickup/estDelivery args carry the restaurant's configured prep
// times so the view can fall back to a synthetic ETA when the
// admin hasn't set estimated_ready_time yet.
func BuildPublicView(o *Order, estPickup, estDelivery int, restaurantName, restaurantLogoURL string) *OrderPublicView {
	if o == nil {
		return nil
	}
	return &OrderPublicView{
		Order:               o,
		StatusExternal:      ExternalStatus(o),
		PlacedAt:            o.CreatedAt,
		EstimatedDeliveryAt: o.EstimatedDeliveryAt(estPickup, estDelivery),
		ItemsSummary:        o.ItemsSummary(),
		SubtotalCents:       money.ToCents(o.Subtotal),
		DeliveryFeeCents:    money.ToCents(o.DeliveryFee),
		ServiceFeeCents:     money.ToCents(o.ServiceFee),
		TipCents:            money.ToCents(o.Tip),
		TaxCents:            money.ToCents(o.Tax),
		DiscountCents:       money.ToCents(o.Discount),
		TotalCents:          money.ToCents(o.Total),
		RestaurantName:      restaurantName,
		RestaurantLogoURL:   restaurantLogoURL,
	}
}
