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

var (
	ErrRestaurantNotFound = errors.New("restaurant not found")
	ErrDishNotFound       = errors.New("dish not found")
)

// ListResult bundles both arms of the favorites response. The dishes
// slice is left untyped (interface{}) because dish hydration happens
// in the AI service to keep the import direction one-way (favorites
// stays free of any AI dependency).
type ListResult struct {
	Restaurants []*restaurantModel.PublicView `json:"restaurants"`
	DishIDs     []primitive.ObjectID          `json:"-"` // hydrated by aiService.HydrateDishes
}

type FavoriteService interface {
	Add(ctx context.Context, customerID, restaurantID primitive.ObjectID) error
	Remove(ctx context.Context, customerID, restaurantID primitive.ObjectID) error
	AddDish(ctx context.Context, customerID, menuItemID primitive.ObjectID) error
	RemoveDish(ctx context.Context, customerID, menuItemID primitive.ObjectID) error
	List(ctx context.Context, customerID primitive.ObjectID) (*ListResult, error)
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

// AddDish saves a menu_item favorite. We don't validate the dish
// exists here because doing so would require a menu service injection
// — the unique partial index keeps duplicate rows out, and a stale
// favorite is filtered out at hydrate time.
func (s *favoriteService) AddDish(ctx context.Context, customerID, menuItemID primitive.ObjectID) error {
	if _, err := s.repo.AddDish(ctx, customerID, menuItemID); err != nil {
		return fmt.Errorf("FavoriteService.AddDish: %w", err)
	}
	return nil
}

func (s *favoriteService) RemoveDish(ctx context.Context, customerID, menuItemID primitive.ObjectID) error {
	return s.repo.RemoveDish(ctx, customerID, menuItemID)
}

func (s *favoriteService) List(ctx context.Context, customerID primitive.ObjectID) (*ListResult, error) {
	rows, err := s.repo.ListForCustomer(ctx, customerID)
	if err != nil {
		return nil, err
	}
	restaurants := make([]*restaurantModel.PublicView, 0, len(rows))
	for _, row := range rows {
		raw, err := bson.Marshal(row)
		if err != nil {
			continue
		}
		var r restaurantModel.Restaurant
		if err := bson.Unmarshal(raw, &r); err != nil {
			continue
		}
		restaurants = append(restaurants, r.PublicView())
	}
	dishIDs, err := s.repo.ListDishIDs(ctx, customerID)
	if err != nil {
		// Don't fail the whole call when only the dish leg breaks.
		dishIDs = []primitive.ObjectID{}
	}
	return &ListResult{Restaurants: restaurants, DishIDs: dishIDs}, nil
}
