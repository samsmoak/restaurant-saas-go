package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	orderModel "restaurantsaas/internal/apps/order/model"
	orderRepoPkg "restaurantsaas/internal/apps/order/repository"
	restaurantSvc "restaurantsaas/internal/apps/restaurant/service"
	"restaurantsaas/internal/apps/reviews/model"
	"restaurantsaas/internal/apps/reviews/repository"
	userRepoPkg "restaurantsaas/internal/apps/user/repository"
)

var (
	ErrOrderNotFound      = errors.New("order not found")
	ErrOrderNotEligible   = errors.New("order is not eligible for review")
	ErrOrderNotOwned      = errors.New("order does not belong to caller")
	ErrAlreadyReviewed    = errors.New("order already reviewed")
	ErrInvalidRatingValue = errors.New("rating must be between 1 and 5")
)

type CreateRequest struct {
	OrderID       string   `json:"order_id"`
	RestaurantID  string   `json:"restaurant_id"`
	Rating        int      `json:"rating"`
	Tags          []string `json:"tags"`
	Comment       string   `json:"comment"`
	Photos        []string `json:"photos"`
	CourierThumbs string   `json:"courier_thumbs"`
}

type ReviewService interface {
	Create(ctx context.Context, customerID primitive.ObjectID, req *CreateRequest) (*model.Review, error)
	ListForRestaurant(ctx context.Context, restaurantID primitive.ObjectID, limit, offset int64) ([]*model.Review, error)
}

type reviewService struct {
	repo        *repository.ReviewRepository
	orderRepo   *orderRepoPkg.OrderRepository
	profileRepo *userRepoPkg.CustomerProfileRepository
	restSvc     restaurantSvc.RestaurantService
}

func NewReviewService(repo *repository.ReviewRepository, orderRepo *orderRepoPkg.OrderRepository, profileRepo *userRepoPkg.CustomerProfileRepository, restSvc restaurantSvc.RestaurantService) ReviewService {
	return &reviewService{repo: repo, orderRepo: orderRepo, profileRepo: profileRepo, restSvc: restSvc}
}

func (s *reviewService) Create(ctx context.Context, customerID primitive.ObjectID, req *CreateRequest) (*model.Review, error) {
	if req.Rating < 1 || req.Rating > 5 {
		return nil, ErrInvalidRatingValue
	}
	orderID, err := primitive.ObjectIDFromHex(req.OrderID)
	if err != nil {
		return nil, errors.New("invalid order_id")
	}
	order, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("ReviewService.Create lookup order: %w", err)
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}
	if order.CustomerID == nil || *order.CustomerID != customerID {
		return nil, ErrOrderNotOwned
	}
	if order.Status != orderModel.OrderStatusCompleted && order.Status != orderModel.OrderStatusDelivered {
		return nil, ErrOrderNotEligible
	}

	tags := dedupeStrings(req.Tags)
	photos := dedupeStrings(req.Photos)
	thumbs := strings.ToLower(strings.TrimSpace(req.CourierThumbs))
	if thumbs != "" && thumbs != "up" && thumbs != "down" {
		thumbs = ""
	}

	// Denormalise the reviewer's display name so list reads don't need
	// a join to render it (BACKEND_REQUIREMENTS.md §4 expects user_name).
	userName := ""
	if s.profileRepo != nil {
		if p, _ := s.profileRepo.FindByUserID(ctx, customerID); p != nil {
			userName = strings.TrimSpace(p.FullName)
		}
	}

	rv := &model.Review{
		RestaurantID:  order.RestaurantID,
		OrderID:       orderID,
		CustomerID:    customerID,
		UserName:      userName,
		Rating:        req.Rating,
		Tags:          tags,
		Comment:       strings.TrimSpace(req.Comment),
		Photos:        photos,
		CourierThumbs: thumbs,
		CreatedAt:     time.Now().UTC(),
	}
	if _, err := s.repo.Insert(ctx, rv); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, ErrAlreadyReviewed
		}
		return nil, fmt.Errorf("ReviewService.Create insert: %w", err)
	}
	rv.EnsureSlices()

	// Best-effort recompute — don't fail the request on a recompute glitch.
	if err := s.restSvc.RecomputeRatingAggregates(ctx, order.RestaurantID); err != nil {
		log.Printf("ReviewService.Create: recompute aggregates: %v", err)
	}
	return rv, nil
}

func dedupeStrings(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func (s *reviewService) ListForRestaurant(ctx context.Context, restaurantID primitive.ObjectID, limit, offset int64) ([]*model.Review, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.repo.ListForRestaurant(ctx, restaurantID, limit, offset)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		r.EnsureSlices()
	}
	return rows, nil
}
