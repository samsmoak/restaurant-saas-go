package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	menuModel "restaurantsaas/internal/apps/menu/model"
	menuSvc "restaurantsaas/internal/apps/menu/service"
	"restaurantsaas/internal/apps/order/model"
	"restaurantsaas/internal/apps/order/repository"
	promoSvc "restaurantsaas/internal/apps/promos/service"
	"restaurantsaas/internal/apps/realtime"
	restaurantSvc "restaurantsaas/internal/apps/restaurant/service"
	userRepoPkg "restaurantsaas/internal/apps/user/repository"
	"restaurantsaas/internal/pkg/money"
)

type CheckoutRequestItem struct {
	MenuItemID          string                  `json:"menu_item_id"`
	Quantity            int                     `json:"quantity"`
	SelectedSize        *CheckoutSelectedSize   `json:"selected_size"`
	SelectedExtras      []CheckoutSelectedExtra `json:"selected_extras"`
	SpecialInstructions string                  `json:"special_instructions"`
}

type CheckoutSelectedSize struct {
	Name string `json:"name"`
}

type CheckoutSelectedExtra struct {
	Name string `json:"name"`
}

// CheckoutLine is the lines[] shape from the Savorar client
// (BACKEND_REQUIREMENTS.md §5).  size_id and extra_ids resolve
// against MenuItem.Sizes[].ID / MenuItem.Extras[].ID.
type CheckoutLine struct {
	MenuItemID          string   `json:"menu_item_id"`
	SizeID              string   `json:"size_id"`
	ExtraIDs            []string `json:"extra_ids"`
	Quantity            int      `json:"quantity"`
	SpecialInstructions string   `json:"special_instructions"`
}

type CheckoutRequest struct {
	// Legacy admin/customer-app fields. Either Items[] or Lines[] must
	// be set; Lines[] takes precedence when both are provided.
	OrderType           string                `json:"order_type"`
	CustomerName        string                `json:"customer_name"`
	CustomerPhone       string                `json:"customer_phone"`
	CustomerEmail       string                `json:"customer_email"`
	DeliveryAddress     string                `json:"delivery_address"`
	DeliveryCity        string                `json:"delivery_city"`
	DeliveryState       string                `json:"delivery_state"`
	DeliveryZip         string                `json:"delivery_zip"`
	DeliveryNotes       string                `json:"delivery_notes"`
	SpecialInstructions string                `json:"special_instructions"`
	Items               []CheckoutRequestItem `json:"items"`

	// Savorar-client v2 fields (BACKEND_REQUIREMENTS.md §5).
	Lines             []CheckoutLine `json:"lines"`
	DeliveryAddressID string         `json:"delivery_address_id"`
	DeliveryMode      string         `json:"delivery_mode"`
	PromoCode         string         `json:"promo_code"`
	TipPercent        int            `json:"tip_percent"`
	GroupNote         string         `json:"group_note"`
}

func (r *CheckoutRequest) Validate() error {
	// Default order_type to delivery for v2 callers that omit it.
	if r.OrderType == "" && len(r.Lines) > 0 {
		r.OrderType = model.OrderTypeDelivery
	}
	if r.OrderType != model.OrderTypePickup && r.OrderType != model.OrderTypeDelivery {
		return errors.New("order_type must be pickup or delivery")
	}
	usingLines := len(r.Lines) > 0
	if !usingLines {
		if len([]rune(strings.TrimSpace(r.CustomerName))) < 2 {
			return errors.New("customer_name must be at least 2 characters")
		}
		if len(strings.TrimSpace(r.CustomerPhone)) < 10 {
			return errors.New("customer_phone must be at least 10 characters")
		}
	}
	if r.OrderType == model.OrderTypeDelivery && !usingLines && r.DeliveryAddressID == "" {
		if strings.TrimSpace(r.DeliveryAddress) == "" ||
			strings.TrimSpace(r.DeliveryCity) == "" ||
			strings.TrimSpace(r.DeliveryState) == "" ||
			strings.TrimSpace(r.DeliveryZip) == "" {
			return errors.New("Full delivery address is required.")
		}
	}
	if !usingLines && len(r.Items) == 0 {
		return errors.New("Cart is empty.")
	}
	if r.TipPercent < 0 || r.TipPercent > 100 {
		return errors.New("tip_percent must be between 0 and 100")
	}
	return nil
}

type CheckoutResult struct {
	Order    *model.Order
	Amount   int64 // cents
	Currency string
	// Summary mirrors the spec's `summary` object so payment/controller
	// can serialise it directly without re-deriving cents from floats.
	Summary CheckoutSummary
}

// CheckoutSummary is the spec'd `summary` object on the create-intent
// response (BACKEND_REQUIREMENTS.md §5).
type CheckoutSummary struct {
	SubtotalCents    int64  `json:"subtotal_cents"`
	DeliveryFeeCents int64  `json:"delivery_fee_cents"`
	ServiceFeeCents  int64  `json:"service_fee_cents"`
	TipCents         int64  `json:"tip_cents"`
	TaxCents         int64  `json:"tax_cents"`
	DiscountCents    int64  `json:"discount_cents"`
	TotalCents       int64  `json:"total_cents"`
	Currency         string `json:"currency"`
}

type UpdateStatusRequest struct {
	Status             *string    `json:"status"`
	EstimatedReadyTime *time.Time `json:"estimated_ready_time"`
}

type OrderService interface {
	ValidateAndBuildOrder(ctx context.Context, restaurantID primitive.ObjectID, userID string, userEmail string, req *CheckoutRequest) (*CheckoutResult, error)
	AttachPaymentIntent(ctx context.Context, orderID primitive.ObjectID, intentID string) error
	DeleteByID(ctx context.Context, orderID primitive.ObjectID) error

	ListForCustomer(ctx context.Context, userID string, restaurantID *primitive.ObjectID) ([]*model.Order, error)
	ListForCustomerPublic(ctx context.Context, userID string, restaurantID *primitive.ObjectID) ([]*model.OrderPublicView, error)
	GetByNumber(ctx context.Context, n string) (*model.Order, error)
	GetByNumberPublic(ctx context.Context, n string) (*model.OrderPublicView, error)
	ListAdmin(ctx context.Context, restaurantID primitive.ObjectID, status string) ([]*model.Order, error)
	UpdateStatus(ctx context.Context, restaurantID primitive.ObjectID, id string, req *UpdateStatusRequest) (*model.Order, error)
	Delete(ctx context.Context, restaurantID primitive.ObjectID, id string) error

	UpdatePaymentStatusByIntent(ctx context.Context, intentID, status string) (*model.Order, error)
	BroadcastCreated(order *model.Order)

	ListBetween(ctx context.Context, restaurantID primitive.ObjectID, from, to time.Time) ([]*model.Order, error)
}

type orderService struct {
	repo        *repository.OrderRepository
	menuSvc     menuSvc.MenuService
	restSvc     restaurantSvc.RestaurantService
	hub         *realtime.Hub
	metrics     restaurantSvc.MetricsRecomputer
	promos      promoSvc.PromoService           // optional; nil disables promo discount lookups
	profileRepo *userRepoPkg.CustomerProfileRepository // optional; resolves DeliveryAddressID
}

func NewOrderService(repo *repository.OrderRepository, menu menuSvc.MenuService, rest restaurantSvc.RestaurantService, hub *realtime.Hub, metrics restaurantSvc.MetricsRecomputer, promos promoSvc.PromoService, profileRepo *userRepoPkg.CustomerProfileRepository) OrderService {
	return &orderService{repo: repo, menuSvc: menu, restSvc: rest, hub: hub, metrics: metrics, promos: promos, profileRepo: profileRepo}
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}

func (s *orderService) ValidateAndBuildOrder(ctx context.Context, restaurantID primitive.ObjectID, userID string, userEmail string, req *CheckoutRequest) (*CheckoutResult, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	userOID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, errors.New("invalid user id")
	}

	ids, err := collectMenuItemIDs(req)
	if err != nil {
		return nil, err
	}
	items, err := s.menuSvc.GetItemsByIDs(ctx, restaurantID, ids)
	if err != nil {
		return nil, fmt.Errorf("OrderService.Create: %w", err)
	}
	itemMap := make(map[primitive.ObjectID]*menuModel.MenuItem)
	for _, it := range items {
		itemMap[it.ID] = it
	}

	rest, err := s.restSvc.GetByID(ctx, restaurantID)
	if err != nil {
		return nil, fmt.Errorf("OrderService.Create: %w", err)
	}
	if rest == nil {
		return nil, errors.New("restaurant not found")
	}
	if rest.ManualClosed {
		return nil, errStatusConflict("Restaurant is not accepting orders right now.")
	}

	lines, err := s.buildOrderLines(req, itemMap)
	if err != nil {
		return nil, err
	}

	subtotal := 0.0
	for _, l := range lines {
		subtotal += l.ItemTotal
	}
	deliveryFee := 0.0
	if req.OrderType == model.OrderTypeDelivery {
		deliveryFee = rest.DeliveryFee
	}

	// Service fee + tax: per-restaurant config takes precedence; fall back
	// to env defaults so a fresh deploy can flip them on without a DB write.
	serviceFeeBps := rest.ServiceFeeBps
	if serviceFeeBps == 0 {
		serviceFeeBps = envIntDefault("DEFAULT_SERVICE_FEE_BPS", 0)
	}
	taxBps := rest.TaxBps
	if taxBps == 0 {
		taxBps = envIntDefault("DEFAULT_TAX_BPS", 0)
	}
	serviceFee := round2(subtotal * float64(serviceFeeBps) / 10000.0)

	tip := 0.0
	if req.TipPercent > 0 {
		tip = round2(subtotal * float64(req.TipPercent) / 100.0)
	}

	// Promo discount in cents to avoid float drift; convert back for storage.
	subtotalCents := money.ToCents(subtotal)
	discountCents := int64(0)
	if s.promos != nil {
		discountCents = s.promos.LookupDiscountCents(ctx, req.PromoCode, subtotalCents)
	}
	discount := money.FromCents(discountCents)

	taxableBase := subtotal + deliveryFee + serviceFee - discount
	if taxableBase < 0 {
		taxableBase = 0
	}
	tax := round2(taxableBase * float64(taxBps) / 10000.0)

	total := round2(subtotal + deliveryFee + serviceFee + tip + tax - discount)
	if total <= 0 {
		return nil, errStatusBadRequest("Order total is zero.")
	}
	if rest.MinOrderAmount > 0 && subtotal < rest.MinOrderAmount {
		return nil, errStatusBadRequest(fmt.Sprintf("Minimum order is %.2f.", rest.MinOrderAmount))
	}

	customerName := strings.TrimSpace(req.CustomerName)
	customerPhone := strings.TrimSpace(req.CustomerPhone)
	deliveryAddress, deliveryNotes := s.resolveDeliveryAddress(ctx, req, userOID)
	if customerName == "" || customerPhone == "" {
		// v2 callers omit customer_name/customer_phone — pull from profile.
		if s.profileRepo != nil {
			if p, _ := s.profileRepo.FindByUserID(ctx, userOID); p != nil {
				if customerName == "" {
					customerName = strings.TrimSpace(p.FullName)
				}
				if customerPhone == "" {
					customerPhone = strings.TrimSpace(p.Phone)
				}
			}
		}
	}

	orderNumber, err := generateOrderNumber(ctx, s.repo)
	if err != nil {
		return nil, fmt.Errorf("OrderService.Create: %w", err)
	}

	customerEmail := strings.TrimSpace(req.CustomerEmail)
	if customerEmail == "" {
		customerEmail = userEmail
	}

	specialInstructions := strings.TrimSpace(req.SpecialInstructions)
	if specialInstructions == "" {
		specialInstructions = strings.TrimSpace(req.GroupNote)
	}

	currency := strings.ToLower(strings.TrimSpace(rest.Currency))
	if currency == "" {
		currency = "usd"
	}

	order := &model.Order{
		RestaurantID:        restaurantID,
		OrderNumber:         orderNumber,
		Status:              model.OrderStatusNew,
		OrderType:           req.OrderType,
		DeliveryMode:        strings.TrimSpace(req.DeliveryMode),
		CustomerID:          &userOID,
		CustomerName:        customerName,
		CustomerPhone:       customerPhone,
		CustomerEmail:       customerEmail,
		DeliveryAddress:     deliveryAddress,
		DeliveryNotes:       firstNonEmpty(req.DeliveryNotes, deliveryNotes),
		Items:               lines,
		Subtotal:            round2(subtotal),
		DeliveryFee:         round2(deliveryFee),
		ServiceFee:          serviceFee,
		Tip:                 tip,
		Tax:                 tax,
		Discount:            discount,
		PromoCode:           strings.ToUpper(strings.TrimSpace(req.PromoCode)),
		Total:               total,
		Currency:            currency,
		PaymentStatus:       model.PaymentStatusPending,
		SpecialInstructions: specialInstructions,
		CreatedAt:           time.Now().UTC(),
	}
	if _, err := s.repo.Create(ctx, order); err != nil {
		return nil, fmt.Errorf("OrderService.Create insert: %w", err)
	}

	summary := CheckoutSummary{
		SubtotalCents:    money.ToCents(order.Subtotal),
		DeliveryFeeCents: money.ToCents(order.DeliveryFee),
		ServiceFeeCents:  money.ToCents(order.ServiceFee),
		TipCents:         money.ToCents(order.Tip),
		TaxCents:         money.ToCents(order.Tax),
		DiscountCents:    -money.ToCents(order.Discount),
		TotalCents:       money.ToCents(order.Total),
		Currency:         currency,
	}

	return &CheckoutResult{
		Order:    order,
		Amount:   int64(math.Round(total * 100)),
		Currency: currency,
		Summary:  summary,
	}, nil
}

// collectMenuItemIDs walks both the legacy Items[] and the v2 Lines[]
// arrays and returns a deduped set of menu_item_id ObjectIDs to fetch
// in one round-trip. v2 Lines[] takes precedence when both are set.
func collectMenuItemIDs(req *CheckoutRequest) ([]primitive.ObjectID, error) {
	usingLines := len(req.Lines) > 0
	seen := make(map[primitive.ObjectID]bool)
	out := make([]primitive.ObjectID, 0, len(req.Items)+len(req.Lines))
	add := func(raw string) error {
		oid, err := primitive.ObjectIDFromHex(raw)
		if err != nil {
			return errors.New("invalid menu_item_id")
		}
		if !seen[oid] {
			seen[oid] = true
			out = append(out, oid)
		}
		return nil
	}
	if usingLines {
		for _, l := range req.Lines {
			if err := add(l.MenuItemID); err != nil {
				return nil, err
			}
		}
		return out, nil
	}
	for _, l := range req.Items {
		if err := add(l.MenuItemID); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// buildOrderLines materialises the request's items/lines against the
// authoritative menu, computing each line's price from the server-side
// MenuItem (the client sends only IDs). Either shape is accepted; the
// v2 lines[] uses size_id / extra_ids while the legacy items[] used
// selected_size.name / selected_extras[].name.
func (s *orderService) buildOrderLines(req *CheckoutRequest, itemMap map[primitive.ObjectID]*menuModel.MenuItem) ([]model.OrderLine, error) {
	if len(req.Lines) > 0 {
		out := make([]model.OrderLine, 0, len(req.Lines))
		for idx, l := range req.Lines {
			oid, _ := primitive.ObjectIDFromHex(l.MenuItemID)
			menuItem, ok := itemMap[oid]
			if !ok {
				return nil, errStatusConflict(fmt.Sprintf("Item %d is no longer available.", idx+1))
			}
			if !menuItem.IsAvailable {
				return nil, errStatusConflict(fmt.Sprintf("'%s' is currently unavailable.", menuItem.Name))
			}
			var selectedSize *model.OrderLineSize
			sizeMod := 0.0
			if strings.TrimSpace(l.SizeID) != "" {
				szOID, err := primitive.ObjectIDFromHex(l.SizeID)
				if err != nil {
					return nil, errStatusBadRequest("invalid size_id")
				}
				matched := false
				for _, sz := range menuItem.Sizes {
					if sz.ID == szOID {
						selectedSize = &model.OrderLineSize{Name: sz.Name, PriceModifier: sz.PriceModifier}
						sizeMod = sz.PriceModifier
						matched = true
						break
					}
				}
				if !matched {
					return nil, errStatusConflict(fmt.Sprintf("Size no longer exists for '%s'.", menuItem.Name))
				}
			}
			selectedExtras := make([]model.OrderLineExtra, 0, len(l.ExtraIDs))
			extrasTotal := 0.0
			for _, eraw := range l.ExtraIDs {
				eOID, err := primitive.ObjectIDFromHex(eraw)
				if err != nil {
					return nil, errStatusBadRequest("invalid extra id")
				}
				matched := false
				for _, ex := range menuItem.Extras {
					if ex.ID == eOID && ex.IsAvailable {
						selectedExtras = append(selectedExtras, model.OrderLineExtra{Name: ex.Name, Price: ex.Price})
						extrasTotal += ex.Price
						matched = true
						break
					}
				}
				if !matched {
					return nil, errStatusConflict(fmt.Sprintf("Extra is no longer available on '%s'.", menuItem.Name))
				}
			}
			qty := l.Quantity
			if qty < 1 {
				qty = 1
			}
			lineTotal := round2((menuItem.BasePrice + sizeMod + extrasTotal) * float64(qty))
			out = append(out, model.OrderLine{
				ID:                  fmt.Sprintf("%s-%d", menuItem.ID.Hex(), idx),
				MenuItemID:          menuItem.ID.Hex(),
				Name:                menuItem.Name,
				Quantity:            qty,
				BasePrice:           menuItem.BasePrice,
				SelectedSize:        selectedSize,
				SelectedExtras:      selectedExtras,
				SpecialInstructions: l.SpecialInstructions,
				ItemTotal:           lineTotal,
			})
		}
		return out, nil
	}

	out := make([]model.OrderLine, 0, len(req.Items))
	for idx, line := range req.Items {
		oid, _ := primitive.ObjectIDFromHex(line.MenuItemID)
		menuItem, ok := itemMap[oid]
		if !ok {
			return nil, errStatusConflict(fmt.Sprintf("Item %d is no longer available.", idx+1))
		}
		if !menuItem.IsAvailable {
			return nil, errStatusConflict(fmt.Sprintf("'%s' is currently unavailable.", menuItem.Name))
		}
		var selectedSize *model.OrderLineSize
		sizeMod := 0.0
		if line.SelectedSize != nil {
			matched := false
			for _, sz := range menuItem.Sizes {
				if sz.Name == line.SelectedSize.Name {
					selectedSize = &model.OrderLineSize{Name: sz.Name, PriceModifier: sz.PriceModifier}
					sizeMod = sz.PriceModifier
					matched = true
					break
				}
			}
			if !matched {
				return nil, errStatusConflict(fmt.Sprintf("Size '%s' no longer exists for '%s'.", line.SelectedSize.Name, menuItem.Name))
			}
		}
		selectedExtras := make([]model.OrderLineExtra, 0, len(line.SelectedExtras))
		extrasTotal := 0.0
		for _, picked := range line.SelectedExtras {
			matched := false
			for _, ex := range menuItem.Extras {
				if ex.Name == picked.Name && ex.IsAvailable {
					selectedExtras = append(selectedExtras, model.OrderLineExtra{Name: ex.Name, Price: ex.Price})
					extrasTotal += ex.Price
					matched = true
					break
				}
			}
			if !matched {
				return nil, errStatusConflict(fmt.Sprintf("Extra '%s' is no longer available on '%s'.", picked.Name, menuItem.Name))
			}
		}
		qty := line.Quantity
		if qty < 1 {
			qty = 1
		}
		lineTotal := round2((menuItem.BasePrice + sizeMod + extrasTotal) * float64(qty))
		out = append(out, model.OrderLine{
			ID:                  fmt.Sprintf("%s-%d", menuItem.ID.Hex(), idx),
			MenuItemID:          menuItem.ID.Hex(),
			Name:                menuItem.Name,
			Quantity:            qty,
			BasePrice:           menuItem.BasePrice,
			SelectedSize:        selectedSize,
			SelectedExtras:      selectedExtras,
			SpecialInstructions: line.SpecialInstructions,
			ItemTotal:           lineTotal,
		})
	}
	return out, nil
}

// resolveDeliveryAddress returns the formatted address + landmark/floor
// notes string for the order.  v2 callers send delivery_address_id —
// we look it up on the customer profile.  Legacy callers send the
// individual address fields directly.
func (s *orderService) resolveDeliveryAddress(ctx context.Context, req *CheckoutRequest, userOID primitive.ObjectID) (string, string) {
	if req.OrderType != model.OrderTypeDelivery {
		return "", ""
	}
	if strings.TrimSpace(req.DeliveryAddressID) != "" && s.profileRepo != nil {
		if p, _ := s.profileRepo.FindByUserID(ctx, userOID); p != nil {
			addrOID, err := primitive.ObjectIDFromHex(req.DeliveryAddressID)
			if err == nil {
				for _, a := range p.Addresses {
					if a.ID == addrOID {
						parts := []string{}
						for _, s := range []string{a.Address, a.City, a.State, a.Zip} {
							if strings.TrimSpace(s) != "" {
								parts = append(parts, strings.TrimSpace(s))
							}
						}
						note := ""
						if a.Floor != "" || a.Landmark != "" {
							note = strings.TrimSpace(a.Floor + " " + a.Landmark)
						}
						return strings.Join(parts, ", "), note
					}
				}
			}
		}
	}
	parts := []string{}
	for _, p := range []string{req.DeliveryAddress, req.DeliveryCity, req.DeliveryState, req.DeliveryZip} {
		if strings.TrimSpace(p) != "" {
			parts = append(parts, strings.TrimSpace(p))
		}
	}
	return strings.Join(parts, ", "), ""
}

func envIntDefault(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func (s *orderService) AttachPaymentIntent(ctx context.Context, orderID primitive.ObjectID, intentID string) error {
	return s.repo.AttachPaymentIntent(ctx, orderID, intentID)
}

func (s *orderService) DeleteByID(ctx context.Context, orderID primitive.ObjectID) error {
	return s.repo.Delete(ctx, orderID)
}

func (s *orderService) ListForCustomer(ctx context.Context, userID string, restaurantID *primitive.ObjectID) ([]*model.Order, error) {
	oid, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, errors.New("invalid user id")
	}
	return s.repo.ListForCustomer(ctx, oid, restaurantID, 50)
}

func (s *orderService) GetByNumber(ctx context.Context, n string) (*model.Order, error) {
	return s.repo.GetByOrderNumber(ctx, n)
}

// ListForCustomerPublic returns the customer-facing orders list with
// restaurant_name, restaurant_logo_url and the spec'd cents fields
// joined in. The legacy ListForCustomer is kept for the existing
// admin/customer Flutter clients that still expect the raw shape.
func (s *orderService) ListForCustomerPublic(ctx context.Context, userID string, restaurantID *primitive.ObjectID) ([]*model.OrderPublicView, error) {
	rows, err := s.ListForCustomer(ctx, userID, restaurantID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []*model.OrderPublicView{}, nil
	}
	idSet := make(map[primitive.ObjectID]struct{}, len(rows))
	ids := make([]primitive.ObjectID, 0, len(rows))
	for _, o := range rows {
		if _, ok := idSet[o.RestaurantID]; ok {
			continue
		}
		idSet[o.RestaurantID] = struct{}{}
		ids = append(ids, o.RestaurantID)
	}
	rests, _ := s.restSvc.GetByIDs(ctx, ids) // best-effort join
	type rmeta struct {
		name, logo                  string
		estPickup, estDelivery      int
	}
	rmap := make(map[primitive.ObjectID]rmeta, len(rests))
	for _, r := range rests {
		rmap[r.ID] = rmeta{
			name: r.Name, logo: r.LogoURL,
			estPickup: r.EstimatedPickupTime, estDelivery: r.EstimatedDeliveryTime,
		}
	}
	out := make([]*model.OrderPublicView, 0, len(rows))
	for _, o := range rows {
		m := rmap[o.RestaurantID]
		out = append(out, model.BuildPublicView(o, m.estPickup, m.estDelivery, m.name, m.logo))
	}
	return out, nil
}

// GetByNumberPublic returns the spec'd customer-facing shape for a
// single order, joining restaurant metadata.
func (s *orderService) GetByNumberPublic(ctx context.Context, n string) (*model.OrderPublicView, error) {
	o, err := s.GetByNumber(ctx, n)
	if err != nil || o == nil {
		return nil, err
	}
	r, _ := s.restSvc.GetByID(ctx, o.RestaurantID)
	name, logo, ep, ed := "", "", 0, 0
	if r != nil {
		name, logo, ep, ed = r.Name, r.LogoURL, r.EstimatedPickupTime, r.EstimatedDeliveryTime
	}
	return model.BuildPublicView(o, ep, ed, name, logo), nil
}

func (s *orderService) ListAdmin(ctx context.Context, restaurantID primitive.ObjectID, status string) ([]*model.Order, error) {
	return s.repo.ListByStatus(ctx, restaurantID, status, 200)
}

func (s *orderService) UpdateStatus(ctx context.Context, restaurantID primitive.ObjectID, id string, req *UpdateStatusRequest) (*model.Order, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid id")
	}
	set := bson.D{}
	if req.Status != nil {
		if !model.IsValidStatus(*req.Status) {
			return nil, errStatusBadRequest("invalid status")
		}
		set = append(set, bson.E{Key: "status", Value: *req.Status})
	}
	if req.EstimatedReadyTime != nil {
		set = append(set, bson.E{Key: "estimated_ready_time", Value: *req.EstimatedReadyTime})
	}
	if len(set) == 0 {
		return s.repo.GetScopedByID(ctx, restaurantID, oid)
	}
	updated, err := s.repo.FindOneAndUpdate(ctx,
		bson.D{{Key: "_id", Value: oid}, {Key: "restaurant_id", Value: restaurantID}},
		bson.D{{Key: "$set", Value: set}},
	)
	if err != nil {
		return nil, err
	}
	if updated != nil && s.hub != nil {
		ev := realtime.NewEvent("order.updated", updated)
		s.hub.BroadcastAdmin(updated.RestaurantID.Hex(), ev)
		s.hub.BroadcastOrder(updated.OrderNumber, ev)
		s.broadcastSpecFrames(ctx, updated)
	}
	// Refresh ranking-input metrics when status transitions occur. Best-effort
	// and run in the background so the API call returns immediately.
	if updated != nil && s.metrics != nil && req.Status != nil {
		go func(rid primitive.ObjectID) {
			if err := s.metrics.RecomputeOperationalMetrics(context.Background(), rid); err != nil {
				log.Printf("orderService.UpdateStatus: RecomputeOperationalMetrics: %v", err)
			}
		}(updated.RestaurantID)
	}
	return updated, nil
}

func (s *orderService) Delete(ctx context.Context, restaurantID primitive.ObjectID, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid id")
	}
	existing, err := s.repo.GetScopedByID(ctx, restaurantID, oid)
	if err != nil {
		return err
	}
	if existing == nil {
		return nil
	}
	if err := s.repo.DeleteScoped(ctx, restaurantID, oid); err != nil {
		return err
	}
	if s.hub != nil {
		payload := map[string]any{
			"id":           existing.ID.Hex(),
			"order_number": existing.OrderNumber,
		}
		ev := realtime.NewEvent("order.deleted", payload)
		s.hub.BroadcastAdmin(existing.RestaurantID.Hex(), ev)
		s.hub.BroadcastOrder(existing.OrderNumber, ev)
	}
	return nil
}

func (s *orderService) UpdatePaymentStatusByIntent(ctx context.Context, intentID, status string) (*model.Order, error) {
	updated, err := s.repo.UpdatePaymentStatusByIntent(ctx, intentID, status)
	if err != nil {
		return nil, err
	}
	if updated != nil && s.hub != nil {
		ev := realtime.NewEvent("order.updated", updated)
		s.hub.BroadcastAdmin(updated.RestaurantID.Hex(), ev)
		s.hub.BroadcastOrder(updated.OrderNumber, ev)
	}
	return updated, nil
}

func (s *orderService) BroadcastCreated(order *model.Order) {
	if s.hub == nil || order == nil {
		return
	}
	ev := realtime.NewEvent("order.created", order)
	s.hub.BroadcastAdmin(order.RestaurantID.Hex(), ev)
	s.hub.BroadcastOrder(order.OrderNumber, ev)
	s.broadcastSpecFrames(context.Background(), order)
}

// broadcastSpecFrames emits the customer-facing WebSocket frames the
// Savorar client expects (BACKEND_REQUIREMENTS.md §5).  We always
// emit a `status` frame translating the internal state via
// ExternalStatus; when the external state becomes "delivered" we
// also emit a `delivered` frame carrying the timestamp.
//
// The legacy `order.created` / `order.updated` Event payloads are
// still emitted on the same channel so the existing customer/admin
// Flutter clients continue to work — this is purely additive.
func (s *orderService) broadcastSpecFrames(ctx context.Context, order *model.Order) {
	if s.hub == nil || order == nil {
		return
	}
	estPickup, estDelivery := 0, 0
	if s.restSvc != nil {
		if r, _ := s.restSvc.GetByID(ctx, order.RestaurantID); r != nil {
			estPickup, estDelivery = r.EstimatedPickupTime, r.EstimatedDeliveryTime
		}
	}
	eta := order.EstimatedDeliveryAt(estPickup, estDelivery)
	mins := 0
	if !eta.IsZero() {
		d := time.Until(eta)
		if d > 0 {
			mins = int(d.Minutes() + 0.5)
		}
	}
	ext := model.ExternalStatus(order)
	s.hub.BroadcastOrderRaw(order.OrderNumber, map[string]any{
		"type":                       "status",
		"status":                     ext,
		"estimated_minutes_remaining": mins,
	})
	if ext == "delivered" {
		s.hub.BroadcastOrderRaw(order.OrderNumber, map[string]any{
			"type":         "delivered",
			"delivered_at": time.Now().UTC(),
		})
	}
}

func (s *orderService) ListBetween(ctx context.Context, restaurantID primitive.ObjectID, from, to time.Time) ([]*model.Order, error) {
	return s.repo.ListBetween(ctx, restaurantID, from, to)
}

func generateOrderNumber(ctx context.Context, repo *repository.OrderRepository) (string, error) {
	for i := 0; i < 10; i++ {
		n := rand.Intn(100000)
		candidate := fmt.Sprintf("ORD-%05d", n)
		exists, err := repo.ExistsByOrderNumber(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not generate unique order number")
}

// Typed errors to surface HTTP status from the controller.
type HTTPError struct {
	Status  int
	Message string
}

func (e *HTTPError) Error() string { return e.Message }

func errStatusBadRequest(msg string) error {
	return &HTTPError{Status: 400, Message: msg}
}
func errStatusConflict(msg string) error {
	return &HTTPError{Status: 409, Message: msg}
}
