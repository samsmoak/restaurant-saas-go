package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"restaurantsaas/internal/apps/category/model"
	"restaurantsaas/internal/apps/category/repository"
)

type CategoryRequest struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	ImageURL     string `json:"image_url"`
	DisplayOrder int    `json:"display_order"`
	IsActive     bool   `json:"is_active"`
}

func (r *CategoryRequest) Validate() error {
	if len(strings.TrimSpace(r.Name)) < 1 {
		return errors.New("name is required")
	}
	return nil
}

type CategoryService interface {
	List(ctx context.Context, restaurantID primitive.ObjectID) ([]*model.Category, error)
	ListActive(ctx context.Context, restaurantID primitive.ObjectID) ([]*model.Category, error)
	Create(ctx context.Context, restaurantID primitive.ObjectID, req *CategoryRequest) (*model.Category, error)
	Update(ctx context.Context, restaurantID primitive.ObjectID, id string, req *CategoryRequest) (*model.Category, error)
	Delete(ctx context.Context, restaurantID primitive.ObjectID, id string) error
}

type categoryService struct {
	repo *repository.CategoryRepository
}

func NewCategoryService(repo *repository.CategoryRepository) CategoryService {
	return &categoryService{repo: repo}
}

func (s *categoryService) List(ctx context.Context, restaurantID primitive.ObjectID) ([]*model.Category, error) {
	return s.repo.ListAll(ctx, restaurantID)
}

func (s *categoryService) ListActive(ctx context.Context, restaurantID primitive.ObjectID) ([]*model.Category, error) {
	return s.repo.ListActive(ctx, restaurantID)
}

func (s *categoryService) Create(ctx context.Context, restaurantID primitive.ObjectID, req *CategoryRequest) (*model.Category, error) {
	doc := &model.Category{
		RestaurantID: restaurantID,
		Name:         strings.TrimSpace(req.Name),
		Description:  req.Description,
		ImageURL:     req.ImageURL,
		DisplayOrder: req.DisplayOrder,
		IsActive:     req.IsActive,
		CreatedAt:    time.Now().UTC(),
	}
	if _, err := s.repo.Create(ctx, doc); err != nil {
		return nil, fmt.Errorf("CategoryService.Create: %w", err)
	}
	return doc, nil
}

func (s *categoryService) Update(ctx context.Context, restaurantID primitive.ObjectID, id string, req *CategoryRequest) (*model.Category, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid id")
	}
	set := bson.D{
		{Key: "name", Value: strings.TrimSpace(req.Name)},
		{Key: "description", Value: req.Description},
		{Key: "image_url", Value: req.ImageURL},
		{Key: "display_order", Value: req.DisplayOrder},
		{Key: "is_active", Value: req.IsActive},
	}
	return s.repo.FindOneAndUpdate(ctx,
		bson.D{{Key: "_id", Value: oid}, {Key: "restaurant_id", Value: restaurantID}},
		bson.D{{Key: "$set", Value: set}},
	)
}

func (s *categoryService) Delete(ctx context.Context, restaurantID primitive.ObjectID, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid id")
	}
	return s.repo.DeleteScoped(ctx, restaurantID, oid)
}
