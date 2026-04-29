package repository

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"restaurantsaas/internal/apps/favorites/model"
	mongoRepo "restaurantsaas/internal/database/abstractrepository/mongodb"
)

type FavoriteRepository struct {
	*mongoRepo.MongoRepository[model.Favorite]
}

func NewFavoriteRepository(db *mongo.Database) *FavoriteRepository {
	return &FavoriteRepository{
		MongoRepository: mongoRepo.NewMongoRepository[model.Favorite](db, "favorites"),
	}
}

// Add upserts a restaurant favorite. Idempotent — a duplicate POST
// returns the existing row rather than a 409.
func (r *FavoriteRepository) Add(ctx context.Context, customerID, restaurantID primitive.ObjectID) (*model.Favorite, error) {
	now := time.Now().UTC()
	return r.FindOneAndUpdate(ctx,
		bson.D{
			{Key: "customer_id", Value: customerID},
			{Key: "restaurant_id", Value: restaurantID},
			{Key: "menu_item_id", Value: bson.D{{Key: "$exists", Value: false}}},
		},
		bson.D{
			{Key: "$setOnInsert", Value: bson.D{
				{Key: "customer_id", Value: customerID},
				{Key: "restaurant_id", Value: restaurantID},
				{Key: "created_at", Value: now},
			}},
		},
		options.FindOneAndUpdate().SetUpsert(true),
	)
}

// AddDish upserts a menu-item favorite (Savorar §6 "dishes" branch).
func (r *FavoriteRepository) AddDish(ctx context.Context, customerID, menuItemID primitive.ObjectID) (*model.Favorite, error) {
	now := time.Now().UTC()
	return r.FindOneAndUpdate(ctx,
		bson.D{
			{Key: "customer_id", Value: customerID},
			{Key: "menu_item_id", Value: menuItemID},
		},
		bson.D{
			{Key: "$setOnInsert", Value: bson.D{
				{Key: "customer_id", Value: customerID},
				{Key: "menu_item_id", Value: menuItemID},
				{Key: "created_at", Value: now},
			}},
		},
		options.FindOneAndUpdate().SetUpsert(true),
	)
}

func (r *FavoriteRepository) Remove(ctx context.Context, customerID, restaurantID primitive.ObjectID) error {
	_, err := r.Collection.DeleteOne(ctx, bson.D{
		{Key: "customer_id", Value: customerID},
		{Key: "restaurant_id", Value: restaurantID},
		{Key: "menu_item_id", Value: bson.D{{Key: "$exists", Value: false}}},
	})
	if err != nil {
		return fmt.Errorf("FavoriteRepository.Remove: %w", err)
	}
	return nil
}

func (r *FavoriteRepository) RemoveDish(ctx context.Context, customerID, menuItemID primitive.ObjectID) error {
	_, err := r.Collection.DeleteOne(ctx, bson.D{
		{Key: "customer_id", Value: customerID},
		{Key: "menu_item_id", Value: menuItemID},
	})
	if err != nil {
		return fmt.Errorf("FavoriteRepository.RemoveDish: %w", err)
	}
	return nil
}

// ListForCustomer returns the joined restaurants for a customer's
// restaurant favorites. Dish favorites are listed separately via
// ListDishIDs because they need a different join target (menu_items).
func (r *FavoriteRepository) ListForCustomer(ctx context.Context, customerID primitive.ObjectID) ([]bson.M, error) {
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{
			{Key: "customer_id", Value: customerID},
			{Key: "restaurant_id", Value: bson.D{{Key: "$exists", Value: true}}},
			{Key: "menu_item_id", Value: bson.D{{Key: "$exists", Value: false}}},
		}}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "created_at", Value: -1}}}},
		bson.D{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: "restaurants"},
			{Key: "localField", Value: "restaurant_id"},
			{Key: "foreignField", Value: "_id"},
			{Key: "as", Value: "restaurant"},
		}}},
		bson.D{{Key: "$unwind", Value: "$restaurant"}},
		bson.D{{Key: "$replaceRoot", Value: bson.D{{Key: "newRoot", Value: "$restaurant"}}}},
	}
	cur, err := r.Collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("FavoriteRepository.ListForCustomer: %w", err)
	}
	defer cur.Close(ctx)
	out := make([]bson.M, 0)
	for cur.Next(ctx) {
		var row bson.M
		if err := cur.Decode(&row); err != nil {
			return nil, fmt.Errorf("FavoriteRepository.ListForCustomer decode: %w", err)
		}
		out = append(out, row)
	}
	return out, nil
}

// ListDishIDs returns just the menu_item ObjectIDs the customer has
// favorited, in created_at desc order.  Hydration to the AI Dish DTO
// happens at the service layer.
func (r *FavoriteRepository) ListDishIDs(ctx context.Context, customerID primitive.ObjectID) ([]primitive.ObjectID, error) {
	cur, err := r.Collection.Find(ctx,
		bson.D{
			{Key: "customer_id", Value: customerID},
			{Key: "menu_item_id", Value: bson.D{{Key: "$exists", Value: true}}},
		},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("FavoriteRepository.ListDishIDs: %w", err)
	}
	defer cur.Close(ctx)
	out := make([]primitive.ObjectID, 0)
	for cur.Next(ctx) {
		var row model.Favorite
		if err := cur.Decode(&row); err != nil {
			return nil, fmt.Errorf("FavoriteRepository.ListDishIDs decode: %w", err)
		}
		if row.MenuItemID != nil {
			out = append(out, *row.MenuItemID)
		}
	}
	return out, nil
}
