package repository

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"restaurantsaas/internal/apps/category/model"
	mongoRepo "restaurantsaas/internal/database/abstractrepository/mongodb"
)

type CategoryRepository struct {
	*mongoRepo.MongoRepository[model.Category]
}

func NewCategoryRepository(db *mongo.Database) *CategoryRepository {
	return &CategoryRepository{
		MongoRepository: mongoRepo.NewMongoRepository[model.Category](db, "categories"),
	}
}

func (r *CategoryRepository) ListActive(ctx context.Context, restaurantID primitive.ObjectID) ([]*model.Category, error) {
	opts := options.Find().SetSort(bson.D{{Key: "display_order", Value: 1}})
	return r.FindMany(ctx, bson.D{
		{Key: "restaurant_id", Value: restaurantID},
		{Key: "is_active", Value: true},
	}, opts)
}

func (r *CategoryRepository) ListAll(ctx context.Context, restaurantID primitive.ObjectID) ([]*model.Category, error) {
	opts := options.Find().SetSort(bson.D{{Key: "display_order", Value: 1}})
	return r.FindMany(ctx, bson.D{{Key: "restaurant_id", Value: restaurantID}}, opts)
}

func (r *CategoryRepository) GetScopedByID(ctx context.Context, restaurantID, id primitive.ObjectID) (*model.Category, error) {
	return r.FindOne(ctx, bson.D{
		{Key: "_id", Value: id},
		{Key: "restaurant_id", Value: restaurantID},
	})
}

func (r *CategoryRepository) DeleteScoped(ctx context.Context, restaurantID, id primitive.ObjectID) error {
	_, err := r.Collection.DeleteOne(ctx, bson.D{
		{Key: "_id", Value: id},
		{Key: "restaurant_id", Value: restaurantID},
	})
	return err
}
