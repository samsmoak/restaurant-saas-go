package service

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"restaurantsaas/internal/apps/favorites/repository"
	restaurantModel "restaurantsaas/internal/apps/restaurant/model"
	restaurantRepoPkg "restaurantsaas/internal/apps/restaurant/repository"
)

var ErrRestaurantNotFound = errors.New("restaurant not found")

type FavoriteService interface {
	Add(ctx context.Context, customerID, restaurantID primitive.ObjectID) error
	Remove(ctx context.Context, customerID, restaurantID primitive.ObjectID) error
	List(ctx context.Context, customerID primitive.ObjectID) ([]*restaurantModel.PublicView, error)
}

type favoriteService struct {
	repo     *repository.FavoriteRepository
	restRepo *restaurantRepoPkg.RestaurantRepository
}

func NewFavoriteService(repo *repository.FavoriteRepository, restRepo *restaurantRepoPkg.RestaurantRepository) FavoriteService {
	return &favoriteService{repo: repo, restRepo: restRepo}
}

func (s *favoriteService) Add(ctx context.Context, customerID, restaurantID primitive.ObjectID) error {
	rest, err := s.restRepo.GetByID(ctx, restaurantID)
	if err != nil {
		return fmt.Errorf("FavoriteService.Add: %w", err)
	}
	if rest == nil {
		return ErrRestaurantNotFound
	}
	if _, err := s.repo.Add(ctx, customerID, restaurantID); err != nil {
		return fmt.Errorf("FavoriteService.Add: %w", err)
	}
	return nil
}

func (s *favoriteService) Remove(ctx context.Context, customerID, restaurantID primitive.ObjectID) error {
	return s.repo.Remove(ctx, customerID, restaurantID)
}

func (s *favoriteService) List(ctx context.Context, customerID primitive.ObjectID) ([]*restaurantModel.PublicView, error) {
	rows, err := s.repo.ListForCustomer(ctx, customerID)
	if err != nil {
		return nil, err
	}
	out := make([]*restaurantModel.PublicView, 0, len(rows))
	for _, row := range rows {
		raw, err := bson.Marshal(row)
		if err != nil {
			continue
		}
		var r restaurantModel.Restaurant
		if err := bson.Unmarshal(raw, &r); err != nil {
			continue
		}
		out = append(out, r.PublicView())
	}
	return out, nil
}
