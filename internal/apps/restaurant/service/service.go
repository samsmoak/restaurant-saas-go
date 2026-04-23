package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	adminModel "restaurantsaas/internal/apps/admin/model"
	adminRepoPkg "restaurantsaas/internal/apps/admin/repository"
	"restaurantsaas/internal/apps/restaurant/model"
	"restaurantsaas/internal/apps/restaurant/repository"
)

// CreateRestaurantRequest is sent by step 2 of the wizard. The backend no
// longer asks for a slug — the restaurant's ObjectID becomes the tenant key.
type CreateRestaurantRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Phone       string `json:"phone"`
	Email       string `json:"email"`
}

func (r *CreateRestaurantRequest) Validate() error {
	if len(strings.TrimSpace(r.Name)) < 2 {
		return errors.New("name must be at least 2 characters")
	}
	return nil
}

// SettingsRequest is the PATCH payload from the wizard steps + settings page.
// Every field is optional; nil pointers / empty slices mean "don't touch".
type SettingsRequest struct {
	Name                  *string                          `json:"name"`
	LogoURL               *string                          `json:"logo_url"`
	Phone                 *string                          `json:"phone"`
	FormattedAddress      *string                          `json:"formatted_address"`
	Latitude              *float64                         `json:"latitude"`
	Longitude             *float64                         `json:"longitude"`
	PlaceID               *string                          `json:"place_id"`
	Timezone              *string                          `json:"timezone"`
	DeliveryFee           *float64                         `json:"delivery_fee"`
	MinOrderAmount        *float64                         `json:"min_order_amount"`
	EstimatedPickupTime   *int                             `json:"estimated_pickup_time"`
	EstimatedDeliveryTime *int                             `json:"estimated_delivery_time"`
	Currency              *string                          `json:"currency"`
	OpeningHours          map[string]model.OpeningHoursDay `json:"opening_hours"`
	ManualClosed          *bool                            `json:"manual_closed"`
	// Optional: when non-empty, appended to onboarding_completed_steps atomically.
	CompletedStep *string `json:"completed_step"`
}

func (r *SettingsRequest) Validate() error {
	if r.Name != nil && strings.TrimSpace(*r.Name) == "" {
		return errors.New("name cannot be empty")
	}
	if r.DeliveryFee != nil && *r.DeliveryFee < 0 {
		return errors.New("delivery_fee must be >= 0")
	}
	if r.MinOrderAmount != nil && *r.MinOrderAmount < 0 {
		return errors.New("min_order_amount must be >= 0")
	}
	if r.EstimatedPickupTime != nil && *r.EstimatedPickupTime < 1 {
		return errors.New("estimated_pickup_time must be >= 1")
	}
	if r.EstimatedDeliveryTime != nil && *r.EstimatedDeliveryTime < 1 {
		return errors.New("estimated_delivery_time must be >= 1")
	}
	return nil
}

type RestaurantService interface {
	Create(ctx context.Context, ownerID primitive.ObjectID, ownerEmail string, req *CreateRestaurantRequest) (*model.Restaurant, error)
	GetByID(ctx context.Context, id primitive.ObjectID) (*model.Restaurant, error)
	ListMine(ctx context.Context, userID primitive.ObjectID) ([]*model.Restaurant, error)
	Update(ctx context.Context, id primitive.ObjectID, req *SettingsRequest) (*model.Restaurant, error)
	ToggleManualClosed(ctx context.Context, id primitive.ObjectID, closed bool) (*model.Restaurant, error)
	MarkStepComplete(ctx context.Context, id primitive.ObjectID, step string) (*model.Restaurant, error)
}

type restaurantService struct {
	repo      *repository.RestaurantRepository
	adminRepo *adminRepoPkg.AdminRepository
}

func NewRestaurantService(repo *repository.RestaurantRepository, adminRepo *adminRepoPkg.AdminRepository) RestaurantService {
	return &restaurantService{repo: repo, adminRepo: adminRepo}
}

func (s *restaurantService) Create(ctx context.Context, ownerID primitive.ObjectID, ownerEmail string, req *CreateRestaurantRequest) (*model.Restaurant, error) {
	r := model.NewRestaurant(strings.TrimSpace(req.Name), ownerID)
	r.Description = strings.TrimSpace(req.Description)
	r.Phone = strings.TrimSpace(req.Phone)
	r.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if _, err := s.repo.Create(ctx, r); err != nil {
		return nil, fmt.Errorf("RestaurantService.Create insert: %w", err)
	}
	// The creator becomes the owner-level admin for this restaurant.
	adminRow := &adminModel.AdminUser{
		UserID:       ownerID,
		RestaurantID: r.ID,
		Email:        strings.ToLower(strings.TrimSpace(ownerEmail)),
		Role:         adminModel.RoleOwner,
		CreatedAt:    r.CreatedAt,
	}
	if _, err := s.adminRepo.Create(ctx, adminRow); err != nil {
		return nil, fmt.Errorf("RestaurantService.Create bootstrap admin: %w", err)
	}
	return r, nil
}

func (s *restaurantService) GetByID(ctx context.Context, id primitive.ObjectID) (*model.Restaurant, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *restaurantService) ListMine(ctx context.Context, userID primitive.ObjectID) ([]*model.Restaurant, error) {
	rows, err := s.adminRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	ids := make([]primitive.ObjectID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.RestaurantID)
	}
	if len(ids) == 0 {
		return []*model.Restaurant{}, nil
	}
	return s.repo.FindMany(ctx, bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: ids}}}})
}

func (s *restaurantService) Update(ctx context.Context, id primitive.ObjectID, req *SettingsRequest) (*model.Restaurant, error) {
	set := bson.D{}
	if req.Name != nil {
		set = append(set, bson.E{Key: "name", Value: strings.TrimSpace(*req.Name)})
	}
	if req.LogoURL != nil {
		set = append(set, bson.E{Key: "logo_url", Value: *req.LogoURL})
	}
	if req.Phone != nil {
		set = append(set, bson.E{Key: "phone", Value: *req.Phone})
	}
	if req.FormattedAddress != nil {
		set = append(set, bson.E{Key: "formatted_address", Value: *req.FormattedAddress})
	}
	if req.Latitude != nil {
		set = append(set, bson.E{Key: "latitude", Value: *req.Latitude})
	}
	if req.Longitude != nil {
		set = append(set, bson.E{Key: "longitude", Value: *req.Longitude})
	}
	if req.PlaceID != nil {
		set = append(set, bson.E{Key: "place_id", Value: *req.PlaceID})
	}
	if req.Timezone != nil {
		set = append(set, bson.E{Key: "timezone", Value: *req.Timezone})
	}
	if req.DeliveryFee != nil {
		set = append(set, bson.E{Key: "delivery_fee", Value: *req.DeliveryFee})
	}
	if req.MinOrderAmount != nil {
		set = append(set, bson.E{Key: "min_order_amount", Value: *req.MinOrderAmount})
	}
	if req.EstimatedPickupTime != nil {
		set = append(set, bson.E{Key: "estimated_pickup_time", Value: *req.EstimatedPickupTime})
	}
	if req.EstimatedDeliveryTime != nil {
		set = append(set, bson.E{Key: "estimated_delivery_time", Value: *req.EstimatedDeliveryTime})
	}
	if req.Currency != nil {
		c := strings.TrimSpace(*req.Currency)
		if c == "" {
			c = "USD"
		}
		set = append(set, bson.E{Key: "currency", Value: c})
	}
	if req.ManualClosed != nil {
		set = append(set, bson.E{Key: "manual_closed", Value: *req.ManualClosed})
	}
	if req.OpeningHours != nil {
		set = append(set, bson.E{Key: "opening_hours", Value: req.OpeningHours})
	}

	var updated *model.Restaurant
	var err error
	if len(set) > 0 {
		updated, err = s.repo.UpdateByID(ctx, id, set)
		if err != nil {
			return nil, err
		}
	} else {
		updated, err = s.repo.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
	}

	// If the caller requested to also mark a step complete, do that after
	// the primary update.
	if req.CompletedStep != nil && strings.TrimSpace(*req.CompletedStep) != "" {
		step := strings.TrimSpace(*req.CompletedStep)
		r, mErr := s.repo.MarkStepComplete(ctx, id, step)
		if mErr != nil {
			return nil, mErr
		}
		if r != nil {
			updated = r
		}
	}
	return updated, nil
}

func (s *restaurantService) ToggleManualClosed(ctx context.Context, id primitive.ObjectID, closed bool) (*model.Restaurant, error) {
	return s.repo.UpdateByID(ctx, id, bson.D{{Key: "manual_closed", Value: closed}})
}

func (s *restaurantService) MarkStepComplete(ctx context.Context, id primitive.ObjectID, step string) (*model.Restaurant, error) {
	step = strings.TrimSpace(step)
	if step == "" {
		return nil, errors.New("step is required")
	}
	return s.repo.MarkStepComplete(ctx, id, step)
}
