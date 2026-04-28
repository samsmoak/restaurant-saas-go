package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

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

	// RecomputeRatingAggregates re-aggregates the reviews collection for the
	// given restaurant and writes back average_rating + rating_count.
	RecomputeRatingAggregates(ctx context.Context, id primitive.ObjectID) error
	// RecomputeOperationalMetrics computes completion_rate and
	// average_prep_minutes from the last 90 days of orders and writes them
	// back to the restaurant document.
	RecomputeOperationalMetrics(ctx context.Context, id primitive.ObjectID) error
}

// MetricsRecomputer is the narrow interface order/service depends on so it
// can fire RecomputeOperationalMetrics without taking a hard dependency on
// the full RestaurantService.
type MetricsRecomputer interface {
	RecomputeOperationalMetrics(ctx context.Context, id primitive.ObjectID) error
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
	// Mirror lat/lng into the GeoJSON `location` field so the 2dsphere index
	// powering distance queries stays in sync with the legacy lat/lng pair.
	if req.Latitude != nil && req.Longitude != nil {
		set = append(set, bson.E{Key: "location", Value: model.NewGeoJSONPoint(*req.Longitude, *req.Latitude)})
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

// RecomputeRatingAggregates re-aggregates the reviews collection and writes
// average_rating + rating_count back to the restaurant document.
func (s *restaurantService) RecomputeRatingAggregates(ctx context.Context, id primitive.ObjectID) error {
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{{Key: "restaurant_id", Value: id}}}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: nil},
			{Key: "avg", Value: bson.D{{Key: "$avg", Value: "$rating"}}},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
	}
	cur, err := s.repo.Collection.Database().Collection("reviews").Aggregate(ctx, pipeline)
	if err != nil {
		return fmt.Errorf("RecomputeRatingAggregates aggregate: %w", err)
	}
	defer cur.Close(ctx)

	avg := 0.0
	count := 0
	if cur.Next(ctx) {
		var row struct {
			Avg   float64 `bson:"avg"`
			Count int     `bson:"count"`
		}
		if err := cur.Decode(&row); err != nil {
			return fmt.Errorf("RecomputeRatingAggregates decode: %w", err)
		}
		avg = row.Avg
		count = row.Count
	}

	_, err = s.repo.UpdateByID(ctx, id, bson.D{
		{Key: "average_rating", Value: avg},
		{Key: "rating_count", Value: count},
		{Key: "updated_at", Value: time.Now().UTC()},
	})
	if err != nil {
		return fmt.Errorf("RecomputeRatingAggregates update: %w", err)
	}
	return nil
}

// RecomputeOperationalMetrics computes completion_rate and
// average_prep_minutes from the last 90 days of orders for the restaurant
// and writes them back. Best-effort: errors are returned but the caller is
// expected to log + swallow them.
func (s *restaurantService) RecomputeOperationalMetrics(ctx context.Context, id primitive.ObjectID) error {
	since := time.Now().UTC().AddDate(0, 0, -90)

	// completion_rate = completed/(completed + non-cancelled non-completed)
	// = orders with status in (completed, delivered) divided by orders with
	// status != cancelled, over the last 90 days.
	completionPipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{
			{Key: "restaurant_id", Value: id},
			{Key: "created_at", Value: bson.D{{Key: "$gte", Value: since}}},
			{Key: "status", Value: bson.D{{Key: "$ne", Value: "cancelled"}}},
		}}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: nil},
			{Key: "total", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "completed", Value: bson.D{{Key: "$sum", Value: bson.D{{Key: "$cond", Value: bson.A{
				bson.D{{Key: "$in", Value: bson.A{"$status", bson.A{"completed", "delivered"}}}},
				1, 0,
			}}}}}},
		}}},
	}
	ordersColl := s.repo.Collection.Database().Collection("orders")
	cur, err := ordersColl.Aggregate(ctx, completionPipeline)
	if err != nil {
		return fmt.Errorf("RecomputeOperationalMetrics completion aggregate: %w", err)
	}
	completionRate := 1.0
	if cur.Next(ctx) {
		var row struct {
			Total     int `bson:"total"`
			Completed int `bson:"completed"`
		}
		if err := cur.Decode(&row); err != nil {
			cur.Close(ctx)
			return fmt.Errorf("RecomputeOperationalMetrics completion decode: %w", err)
		}
		if row.Total > 0 {
			completionRate = float64(row.Completed) / float64(row.Total)
		}
	}
	cur.Close(ctx)

	// average_prep_minutes = average of (estimated_ready_time - created_at)
	// for orders that reached ready+ in the window, in minutes. We use the
	// estimated_ready_time as a proxy because we don't store the actual
	// transition timestamp. Falls back to the existing value when unknown.
	prepPipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{
			{Key: "restaurant_id", Value: id},
			{Key: "created_at", Value: bson.D{{Key: "$gte", Value: since}}},
			{Key: "status", Value: bson.D{{Key: "$in", Value: bson.A{"ready", "completed", "delivered"}}}},
			{Key: "estimated_ready_time", Value: bson.D{{Key: "$ne", Value: nil}}},
		}}},
		bson.D{{Key: "$project", Value: bson.D{
			{Key: "minutes", Value: bson.D{{Key: "$divide", Value: bson.A{
				bson.D{{Key: "$subtract", Value: bson.A{"$estimated_ready_time", "$created_at"}}},
				60000, // ms → minutes
			}}}},
		}}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: nil},
			{Key: "avg", Value: bson.D{{Key: "$avg", Value: "$minutes"}}},
		}}},
	}
	cur2, err := ordersColl.Aggregate(ctx, prepPipeline)
	if err != nil {
		return fmt.Errorf("RecomputeOperationalMetrics prep aggregate: %w", err)
	}
	defer cur2.Close(ctx)

	prepMinutes := 0
	havePrep := false
	if cur2.Next(ctx) {
		var row struct {
			Avg float64 `bson:"avg"`
		}
		if err := cur2.Decode(&row); err != nil {
			return fmt.Errorf("RecomputeOperationalMetrics prep decode: %w", err)
		}
		if row.Avg > 0 {
			prepMinutes = int(row.Avg + 0.5)
			havePrep = true
		}
	}

	set := bson.D{
		{Key: "completion_rate", Value: completionRate},
		{Key: "updated_at", Value: time.Now().UTC()},
	}
	if havePrep {
		set = append(set, bson.E{Key: "average_prep_minutes", Value: prepMinutes})
	}
	if _, err := s.repo.UpdateByID(ctx, id, set); err != nil {
		return fmt.Errorf("RecomputeOperationalMetrics update: %w", err)
	}
	return nil
}
