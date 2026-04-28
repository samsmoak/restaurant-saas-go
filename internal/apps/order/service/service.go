package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	menuModel "restaurantsaas/internal/apps/menu/model"
	menuSvc "restaurantsaas/internal/apps/menu/service"
	"restaurantsaas/internal/apps/order/model"
	"restaurantsaas/internal/apps/order/repository"
	"restaurantsaas/internal/apps/realtime"
	restaurantSvc "restaurantsaas/internal/apps/restaurant/service"
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

type CheckoutRequest struct {
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
}

func (r *CheckoutRequest) Validate() error {
	if r.OrderType != model.OrderTypePickup && r.OrderType != model.OrderTypeDelivery {
		return errors.New("order_type must be pickup or delivery")
	}
	if len([]rune(strings.TrimSpace(r.CustomerName))) < 2 {
		return errors.New("customer_name must be at least 2 characters")
	}
	if len(strings.TrimSpace(r.CustomerPhone)) < 10 {
		return errors.New("customer_phone must be at least 10 characters")
	}
	if r.OrderType == model.OrderTypeDelivery {
		if strings.TrimSpace(r.DeliveryAddress) == "" ||
			strings.TrimSpace(r.DeliveryCity) == "" ||
			strings.TrimSpace(r.DeliveryState) == "" ||
			strings.TrimSpace(r.DeliveryZip) == "" {
			return errors.New("Full delivery address is required.")
		}
	}
	if len(r.Items) == 0 {
		return errors.New("Cart is empty.")
	}
	return nil
}

type CheckoutResult struct {
	Order    *model.Order
	Amount   int64 // cents
	Currency string
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
	GetByNumber(ctx context.Context, n string) (*model.Order, error)
	ListAdmin(ctx context.Context, restaurantID primitive.ObjectID, status string) ([]*model.Order, error)
	UpdateStatus(ctx context.Context, restaurantID primitive.ObjectID, id string, req *UpdateStatusRequest) (*model.Order, error)
	Delete(ctx context.Context, restaurantID primitive.ObjectID, id string) error

	UpdatePaymentStatusByIntent(ctx context.Context, intentID, status string) (*model.Order, error)
	BroadcastCreated(order *model.Order)

	ListBetween(ctx context.Context, restaurantID primitive.ObjectID, from, to time.Time) ([]*model.Order, error)
}

type orderService struct {
	repo    *repository.OrderRepository
	menuSvc menuSvc.MenuService
	restSvc restaurantSvc.RestaurantService
	hub     *realtime.Hub
	metrics restaurantSvc.MetricsRecomputer
}

func NewOrderService(repo *repository.OrderRepository, menu menuSvc.MenuService, rest restaurantSvc.RestaurantService, hub *realtime.Hub, metrics restaurantSvc.MetricsRecomputer) OrderService {
	return &orderService{repo: repo, menuSvc: menu, restSvc: rest, hub: hub, metrics: metrics}
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

	ids := make([]primitive.ObjectID, 0, len(req.Items))
	seen := make(map[primitive.ObjectID]bool)
	for _, line := range req.Items {
		oid, err := primitive.ObjectIDFromHex(line.MenuItemID)
		if err != nil {
			return nil, errors.New("invalid menu_item_id")
		}
		if !seen[oid] {
			seen[oid] = true
			ids = append(ids, oid)
		}
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

	lines := make([]model.OrderLine, 0, len(req.Items))
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

		lines = append(lines, model.OrderLine{
			ID:                  fmt.Sprintf("%s-%d", menuItem.ID.Hex(), idx),
			Name:                menuItem.Name,
			Quantity:            qty,
			BasePrice:           menuItem.BasePrice,
			SelectedSize:        selectedSize,
			SelectedExtras:      selectedExtras,
			SpecialInstructions: line.SpecialInstructions,
			ItemTotal:           lineTotal,
		})
	}

	subtotal := 0.0
	for _, l := range lines {
		subtotal += l.ItemTotal
	}
	deliveryFee := 0.0
	if req.OrderType == model.OrderTypeDelivery {
		deliveryFee = rest.DeliveryFee
	}
	total := round2(subtotal + deliveryFee)
	if total <= 0 {
		return nil, errStatusBadRequest("Order total is zero.")
	}
	if rest.MinOrderAmount > 0 && subtotal < rest.MinOrderAmount {
		return nil, errStatusBadRequest(fmt.Sprintf("Minimum order is %.2f.", rest.MinOrderAmount))
	}

	fullAddress := ""
	if req.OrderType == model.OrderTypeDelivery {
		parts := []string{}
		for _, p := range []string{req.DeliveryAddress, req.DeliveryCity, req.DeliveryState, req.DeliveryZip} {
			if strings.TrimSpace(p) != "" {
				parts = append(parts, strings.TrimSpace(p))
			}
		}
		fullAddress = strings.Join(parts, ", ")
	}

	orderNumber, err := generateOrderNumber(ctx, s.repo)
	if err != nil {
		return nil, fmt.Errorf("OrderService.Create: %w", err)
	}

	customerEmail := strings.TrimSpace(req.CustomerEmail)
	if customerEmail == "" {
		customerEmail = userEmail
	}

	order := &model.Order{
		RestaurantID:        restaurantID,
		OrderNumber:         orderNumber,
		Status:              model.OrderStatusNew,
		OrderType:           req.OrderType,
		CustomerID:          &userOID,
		CustomerName:        strings.TrimSpace(req.CustomerName),
		CustomerPhone:       strings.TrimSpace(req.CustomerPhone),
		CustomerEmail:       customerEmail,
		DeliveryAddress:     fullAddress,
		DeliveryNotes:       req.DeliveryNotes,
		Items:               lines,
		Subtotal:            round2(subtotal),
		DeliveryFee:         round2(deliveryFee),
		Total:               total,
		PaymentStatus:       model.PaymentStatusPending,
		SpecialInstructions: req.SpecialInstructions,
		CreatedAt:           time.Now().UTC(),
	}
	if _, err := s.repo.Create(ctx, order); err != nil {
		return nil, fmt.Errorf("OrderService.Create insert: %w", err)
	}

	currency := strings.ToLower(strings.TrimSpace(rest.Currency))
	if currency == "" {
		currency = "usd"
	}

	return &CheckoutResult{
		Order:    order,
		Amount:   int64(math.Round(total * 100)),
		Currency: currency,
	}, nil
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
