package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	adminModel "restaurantsaas/internal/apps/admin/model"
	adminRepoPkg "restaurantsaas/internal/apps/admin/repository"
	"restaurantsaas/internal/apps/restaurant/model"
	"restaurantsaas/internal/apps/restaurant/repository"
)

type CreateRestaurantRequest struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Phone       string `json:"phone"`
	Email       string `json:"email"`
	Address     string `json:"address"`
}

var slugRegex = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{1,48}[a-z0-9])?$`)

func (r *CreateRestaurantRequest) Validate() error {
	r.Slug = strings.ToLower(strings.TrimSpace(r.Slug))
	if r.Slug == "" {
		return errors.New("slug is required")
	}
	if !slugRegex.MatchString(r.Slug) {
		return errors.New("slug must be 2-50 chars, lowercase letters/digits/hyphens, no leading/trailing hyphens")
	}
	if len(strings.TrimSpace(r.Name)) < 2 {
		return errors.New("name must be at least 2 characters")
	}
	return nil
}

type SettingsRequest struct {
	Name                  string                           `json:"name"`
	LogoURL               string                           `json:"logo_url"`
	Phone                 string                           `json:"phone"`
	Address               string                           `json:"address"`
	DeliveryFee           float64                          `json:"delivery_fee"`
	MinOrderAmount        float64                          `json:"min_order_amount"`
	EstimatedPickupTime   int                              `json:"estimated_pickup_time"`
	EstimatedDeliveryTime int                              `json:"estimated_delivery_time"`
	Currency              string                           `json:"currency"`
	OpeningHours          map[string]model.OpeningHoursDay `json:"opening_hours"`
	ManualClosed          *bool                            `json:"manual_closed"`
}

func (r *SettingsRequest) Validate() error {
	if len(strings.TrimSpace(r.Name)) < 1 {
		return errors.New("name is required")
	}
	if r.DeliveryFee < 0 {
		return errors.New("delivery_fee must be >= 0")
	}
	if r.MinOrderAmount < 0 {
		return errors.New("min_order_amount must be >= 0")
	}
	if r.EstimatedPickupTime < 1 {
		return errors.New("estimated_pickup_time must be >= 1")
	}
	if r.EstimatedDeliveryTime < 1 {
		return errors.New("estimated_delivery_time must be >= 1")
	}
	return nil
}

type RestaurantService interface {
	Create(ctx context.Context, ownerID primitive.ObjectID, ownerEmail string, req *CreateRestaurantRequest) (*model.Restaurant, error)
	GetByID(ctx context.Context, id primitive.ObjectID) (*model.Restaurant, error)
	GetBySlug(ctx context.Context, slug string) (*model.Restaurant, error)
	ListMine(ctx context.Context, userID primitive.ObjectID) ([]*model.Restaurant, error)
	Update(ctx context.Context, id primitive.ObjectID, req *SettingsRequest) (*model.Restaurant, error)
	ToggleManualClosed(ctx context.Context, id primitive.ObjectID, closed bool) (*model.Restaurant, error)
}

type restaurantService struct {
	repo      *repository.RestaurantRepository
	adminRepo *adminRepoPkg.AdminRepository
}

func NewRestaurantService(repo *repository.RestaurantRepository, adminRepo *adminRepoPkg.AdminRepository) RestaurantService {
	return &restaurantService{repo: repo, adminRepo: adminRepo}
}

var ErrSlugTaken = errors.New("slug is already taken")

func (s *restaurantService) Create(ctx context.Context, ownerID primitive.ObjectID, ownerEmail string, req *CreateRestaurantRequest) (*model.Restaurant, error) {
	if existing, err := s.repo.FindBySlug(ctx, req.Slug); err != nil {
		return nil, fmt.Errorf("RestaurantService.Create: %w", err)
	} else if existing != nil {
		return nil, ErrSlugTaken
	}
	r := model.NewRestaurant(req.Slug, strings.TrimSpace(req.Name), ownerID)
	r.Description = strings.TrimSpace(req.Description)
	r.Phone = strings.TrimSpace(req.Phone)
	r.Email = strings.ToLower(strings.TrimSpace(req.Email))
	r.Address = strings.TrimSpace(req.Address)
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

func (s *restaurantService) GetBySlug(ctx context.Context, slug string) (*model.Restaurant, error) {
	return s.repo.FindBySlug(ctx, slug)
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
	currency := strings.TrimSpace(req.Currency)
	if currency == "" {
		currency = "USD"
	}
	set := bson.D{
		{Key: "name", Value: strings.TrimSpace(req.Name)},
		{Key: "logo_url", Value: req.LogoURL},
		{Key: "phone", Value: req.Phone},
		{Key: "address", Value: req.Address},
		{Key: "delivery_fee", Value: req.DeliveryFee},
		{Key: "min_order_amount", Value: req.MinOrderAmount},
		{Key: "estimated_pickup_time", Value: req.EstimatedPickupTime},
		{Key: "estimated_delivery_time", Value: req.EstimatedDeliveryTime},
		{Key: "currency", Value: currency},
	}
	if req.ManualClosed != nil {
		set = append(set, bson.E{Key: "manual_closed", Value: *req.ManualClosed})
	}
	if req.OpeningHours != nil {
		set = append(set, bson.E{Key: "opening_hours", Value: req.OpeningHours})
	}
	return s.repo.UpdateByID(ctx, id, set)
}

func (s *restaurantService) ToggleManualClosed(ctx context.Context, id primitive.ObjectID, closed bool) (*model.Restaurant, error) {
	return s.repo.UpdateByID(ctx, id, bson.D{{Key: "manual_closed", Value: closed}})
}
