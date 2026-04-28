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
)

var (
	ErrOrderNotFound      = errors.New("order not found")
	ErrOrderNotEligible   = errors.New("order is not eligible for review")
	ErrOrderNotOwned      = errors.New("order does not belong to caller")
	ErrAlreadyReviewed    = errors.New("order already reviewed")
	ErrInvalidRatingValue = errors.New("rating must be between 1 and 5")
)

type CreateRequest struct {
	OrderID string `json:"order_id"`
	Rating  int    `json:"rating"`
	Comment string `json:"comment"`
}

type ReviewService interface {
	Create(ctx context.Context, customerID primitive.ObjectID, req *CreateRequest) (*model.Review, error)
	ListForRestaurant(ctx context.Context, restaurantID primitive.ObjectID, limit, offset int64) ([]*model.Review, error)
}

type reviewService struct {
	repo      *repository.ReviewRepository
	orderRepo *orderRepoPkg.OrderRepository
	restSvc   restaurantSvc.RestaurantService
}

func NewReviewService(repo *repository.ReviewRepository, orderRepo *orderRepoPkg.OrderRepository, restSvc restaurantSvc.RestaurantService) ReviewService {
	return &reviewService{repo: repo, orderRepo: orderRepo, restSvc: restSvc}
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

	rv := &model.Review{
		RestaurantID: order.RestaurantID,
		OrderID:      orderID,
		CustomerID:   customerID,
		Rating:       req.Rating,
		Comment:      strings.TrimSpace(req.Comment),
		CreatedAt:    time.Now().UTC(),
	}
	if _, err := s.repo.Insert(ctx, rv); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, ErrAlreadyReviewed
		}
		return nil, fmt.Errorf("ReviewService.Create insert: %w", err)
	}

	// Best-effort recompute — don't fail the request on a recompute glitch.
	if err := s.restSvc.RecomputeRatingAggregates(ctx, order.RestaurantID); err != nil {
		log.Printf("ReviewService.Create: recompute aggregates: %v", err)
	}
	return rv, nil
}

func (s *reviewService) ListForRestaurant(ctx context.Context, restaurantID primitive.ObjectID, limit, offset int64) ([]*model.Review, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.ListForRestaurant(ctx, restaurantID, limit, offset)
}
