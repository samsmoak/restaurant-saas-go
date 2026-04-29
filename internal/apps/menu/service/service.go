package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	categoryModel "restaurantsaas/internal/apps/category/model"
	categoryRepoPkg "restaurantsaas/internal/apps/category/repository"
	"restaurantsaas/internal/apps/menu/model"
	"restaurantsaas/internal/apps/menu/repository"
)

type MenuItemSizeInput struct {
	Name          string  `json:"name"`
	PriceModifier float64 `json:"price_modifier"`
	IsDefault     bool    `json:"is_default"`
}

type MenuItemExtraInput struct {
	Name        string  `json:"name"`
	Price       float64 `json:"price"`
	IsAvailable bool    `json:"is_available"`
}

type MenuItemRequest struct {
	Name         string               `json:"name"`
	CategoryID   string               `json:"category_id"`
	Description  string               `json:"description"`
	BasePrice    float64              `json:"base_price"`
	ImageURL     string               `json:"image_url"`
	IsFeatured   bool                 `json:"is_featured"`
	IsAvailable  bool                 `json:"is_available"`
	DisplayOrder int                  `json:"display_order"`
	Tags         []string             `json:"tags"`
	Sizes        []MenuItemSizeInput  `json:"sizes"`
	Extras       []MenuItemExtraInput `json:"extras"`
}

func (r *MenuItemRequest) Validate() error {
	if len(strings.TrimSpace(r.Name)) < 1 {
		return errors.New("name is required")
	}
	if r.BasePrice < 0.01 {
		return errors.New("base_price must be greater than 0.01")
	}
	if strings.TrimSpace(r.CategoryID) == "" {
		return errors.New("category_id is required")
	}
	if _, err := primitive.ObjectIDFromHex(r.CategoryID); err != nil {
		return errors.New("category_id is invalid")
	}
	for i, sz := range r.Sizes {
		if len(strings.TrimSpace(sz.Name)) < 1 {
			return fmt.Errorf("sizes[%d].name is required", i)
		}
	}
	for i, ex := range r.Extras {
		if len(strings.TrimSpace(ex.Name)) < 1 {
			return fmt.Errorf("extras[%d].name is required", i)
		}
		if ex.Price < 0 {
			return fmt.Errorf("extras[%d].price must be >= 0", i)
		}
	}
	return nil
}

type MenuCategoryWithItems struct {
	categoryModel.Category
	Items []*model.MenuItem `json:"items"`
}

// MenuCategoryWithItemsPublic is the customer-facing variant returned
// by GET /api/r/{id}/menu — items use MenuItemPublicView so prices are
// expressed in cents per BACKEND_REQUIREMENTS.md §3.
type MenuCategoryWithItemsPublic struct {
	categoryModel.Category
	Items []*model.MenuItemPublicView `json:"items"`
}

type MenuService interface {
	PublicMenu(ctx context.Context, restaurantID primitive.ObjectID) ([]MenuCategoryWithItems, error)
	PublicMenuView(ctx context.Context, restaurantID primitive.ObjectID) ([]MenuCategoryWithItemsPublic, error)
	ListAllItems(ctx context.Context, restaurantID primitive.ObjectID) ([]*model.MenuItem, error)
	GetItemByID(ctx context.Context, restaurantID primitive.ObjectID, id string) (*model.MenuItem, error)
	GetItemsByIDs(ctx context.Context, restaurantID primitive.ObjectID, ids []primitive.ObjectID) ([]*model.MenuItem, error)
	Create(ctx context.Context, restaurantID primitive.ObjectID, req *MenuItemRequest) (*model.MenuItem, error)
	Update(ctx context.Context, restaurantID primitive.ObjectID, id string, req *MenuItemRequest) (*model.MenuItem, error)
	Delete(ctx context.Context, restaurantID primitive.ObjectID, id string) error
}

type menuService struct {
	repo    *repository.MenuRepository
	catRepo *categoryRepoPkg.CategoryRepository
}

func NewMenuService(repo *repository.MenuRepository, catRepo *categoryRepoPkg.CategoryRepository) MenuService {
	return &menuService{repo: repo, catRepo: catRepo}
}

func (s *menuService) PublicMenu(ctx context.Context, restaurantID primitive.ObjectID) ([]MenuCategoryWithItems, error) {
	cats, err := s.catRepo.ListActive(ctx, restaurantID)
	if err != nil {
		return nil, fmt.Errorf("MenuService.PublicMenu: %w", err)
	}
	items, err := s.repo.ListAll(ctx, restaurantID)
	if err != nil {
		return nil, fmt.Errorf("MenuService.PublicMenu: %w", err)
	}
	bucket := make(map[primitive.ObjectID][]*model.MenuItem)
	for _, it := range items {
		it.EnsureSlices()
		if it.CategoryID == nil {
			continue
		}
		bucket[*it.CategoryID] = append(bucket[*it.CategoryID], it)
	}
	out := make([]MenuCategoryWithItems, 0, len(cats))
	for _, c := range cats {
		group := bucket[c.ID]
		if group == nil {
			group = []*model.MenuItem{}
		}
		out = append(out, MenuCategoryWithItems{Category: *c, Items: group})
	}
	return out, nil
}

// PublicMenuView is the customer-facing menu — same grouping as
// PublicMenu but with each item projected through PublicView so the
// money fields are in cents and tags are populated.
func (s *menuService) PublicMenuView(ctx context.Context, restaurantID primitive.ObjectID) ([]MenuCategoryWithItemsPublic, error) {
	groups, err := s.PublicMenu(ctx, restaurantID)
	if err != nil {
		return nil, err
	}
	out := make([]MenuCategoryWithItemsPublic, 0, len(groups))
	for _, g := range groups {
		views := make([]*model.MenuItemPublicView, 0, len(g.Items))
		for _, it := range g.Items {
			views = append(views, it.PublicView())
		}
		out = append(out, MenuCategoryWithItemsPublic{Category: g.Category, Items: views})
	}
	return out, nil
}

func (s *menuService) ListAllItems(ctx context.Context, restaurantID primitive.ObjectID) ([]*model.MenuItem, error) {
	items, err := s.repo.ListAll(ctx, restaurantID)
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		it.EnsureSlices()
	}
	return items, nil
}

func (s *menuService) GetItemByID(ctx context.Context, restaurantID primitive.ObjectID, id string) (*model.MenuItem, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid id")
	}
	item, err := s.repo.GetScopedByID(ctx, restaurantID, oid)
	if err != nil {
		return nil, err
	}
	if item != nil {
		item.EnsureSlices()
	}
	return item, nil
}

func (s *menuService) GetItemsByIDs(ctx context.Context, restaurantID primitive.ObjectID, ids []primitive.ObjectID) ([]*model.MenuItem, error) {
	items, err := s.repo.FindByIDs(ctx, restaurantID, ids)
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		it.EnsureSlices()
	}
	return items, nil
}

func (s *menuService) Create(ctx context.Context, restaurantID primitive.ObjectID, req *MenuItemRequest) (*model.MenuItem, error) {
	catID, _ := primitive.ObjectIDFromHex(req.CategoryID)
	doc := &model.MenuItem{
		RestaurantID: restaurantID,
		CategoryID:   &catID,
		Name:         strings.TrimSpace(req.Name),
		Description:  req.Description,
		BasePrice:    req.BasePrice,
		ImageURL:     req.ImageURL,
		IsAvailable:  req.IsAvailable,
		IsFeatured:   req.IsFeatured,
		DisplayOrder: req.DisplayOrder,
		Tags:         normaliseTags(req.Tags),
		Sizes:        mapSizes(req.Sizes),
		Extras:       mapExtras(req.Extras),
		CreatedAt:    time.Now().UTC(),
	}
	doc.EnsureSlices()
	if _, err := s.repo.Create(ctx, doc); err != nil {
		return nil, fmt.Errorf("MenuService.Create: %w", err)
	}
	return doc, nil
}

func (s *menuService) Update(ctx context.Context, restaurantID primitive.ObjectID, id string, req *MenuItemRequest) (*model.MenuItem, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid id")
	}
	catID, _ := primitive.ObjectIDFromHex(req.CategoryID)
	sizes := mapSizes(req.Sizes)
	extras := mapExtras(req.Extras)
	if sizes == nil {
		sizes = []model.ItemSize{}
	}
	if extras == nil {
		extras = []model.ItemExtra{}
	}
	set := bson.D{
		{Key: "category_id", Value: catID},
		{Key: "name", Value: strings.TrimSpace(req.Name)},
		{Key: "description", Value: req.Description},
		{Key: "base_price", Value: req.BasePrice},
		{Key: "image_url", Value: req.ImageURL},
		{Key: "is_available", Value: req.IsAvailable},
		{Key: "is_featured", Value: req.IsFeatured},
		{Key: "display_order", Value: req.DisplayOrder},
		{Key: "tags", Value: normaliseTags(req.Tags)},
		{Key: "sizes", Value: sizes},
		{Key: "extras", Value: extras},
	}
	item, err := s.repo.FindOneAndUpdate(ctx,
		bson.D{{Key: "_id", Value: oid}, {Key: "restaurant_id", Value: restaurantID}},
		bson.D{{Key: "$set", Value: set}},
	)
	if err != nil {
		return nil, err
	}
	if item != nil {
		item.EnsureSlices()
	}
	return item, nil
}

func (s *menuService) Delete(ctx context.Context, restaurantID primitive.ObjectID, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid id")
	}
	return s.repo.DeleteScoped(ctx, restaurantID, oid)
}

func normaliseTags(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, t := range in {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

func mapSizes(inputs []MenuItemSizeInput) []model.ItemSize {
	out := make([]model.ItemSize, 0, len(inputs))
	for _, s := range inputs {
		out = append(out, model.ItemSize{
			ID:            primitive.NewObjectID(),
			Name:          strings.TrimSpace(s.Name),
			PriceModifier: s.PriceModifier,
			IsDefault:     s.IsDefault,
		})
	}
	return out
}

func mapExtras(inputs []MenuItemExtraInput) []model.ItemExtra {
	out := make([]model.ItemExtra, 0, len(inputs))
	for _, e := range inputs {
		out = append(out, model.ItemExtra{
			ID:          primitive.NewObjectID(),
			Name:        strings.TrimSpace(e.Name),
			Price:       e.Price,
			IsAvailable: e.IsAvailable,
		})
	}
	return out
}
