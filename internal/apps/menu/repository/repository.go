package repository

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"restaurantsaas/internal/apps/menu/model"
	mongoRepo "restaurantsaas/internal/database/abstractrepository/mongodb"
)

type MenuRepository struct {
	*mongoRepo.MongoRepository[model.MenuItem]
}

func NewMenuRepository(db *mongo.Database) *MenuRepository {
	return &MenuRepository{
		MongoRepository: mongoRepo.NewMongoRepository[model.MenuItem](db, "menu_items"),
	}
}

func (r *MenuRepository) ListAll(ctx context.Context, restaurantID primitive.ObjectID) ([]*model.MenuItem, error) {
	opts := options.Find().SetSort(bson.D{{Key: "display_order", Value: 1}})
	return r.FindMany(ctx, bson.D{{Key: "restaurant_id", Value: restaurantID}}, opts)
}

func (r *MenuRepository) FindByIDs(ctx context.Context, restaurantID primitive.ObjectID, ids []primitive.ObjectID) ([]*model.MenuItem, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	return r.FindMany(ctx, bson.D{
		{Key: "restaurant_id", Value: restaurantID},
		{Key: "_id", Value: bson.D{{Key: "$in", Value: ids}}},
	})
}

func (r *MenuRepository) GetScopedByID(ctx context.Context, restaurantID, id primitive.ObjectID) (*model.MenuItem, error) {
	return r.FindOne(ctx, bson.D{
		{Key: "_id", Value: id},
		{Key: "restaurant_id", Value: restaurantID},
	})
}

func (r *MenuRepository) DeleteScoped(ctx context.Context, restaurantID, id primitive.ObjectID) error {
	_, err := r.Collection.DeleteOne(ctx, bson.D{
		{Key: "_id", Value: id},
		{Key: "restaurant_id", Value: restaurantID},
	})
	return err
}
