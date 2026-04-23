package repository

import (
	"context"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"restaurantsaas/internal/apps/restaurant/model"
	mongoRepo "restaurantsaas/internal/database/abstractrepository/mongodb"
)

type RestaurantRepository struct {
	*mongoRepo.MongoRepository[model.Restaurant]
}

func NewRestaurantRepository(db *mongo.Database) *RestaurantRepository {
	return &RestaurantRepository{
		MongoRepository: mongoRepo.NewMongoRepository[model.Restaurant](db, "restaurants"),
	}
}

func (r *RestaurantRepository) FindBySlug(ctx context.Context, slug string) (*model.Restaurant, error) {
	return r.FindOne(ctx, bson.D{{Key: "slug", Value: strings.ToLower(strings.TrimSpace(slug))}})
}

func (r *RestaurantRepository) FindByOwner(ctx context.Context, ownerID primitive.ObjectID) ([]*model.Restaurant, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	return r.FindMany(ctx, bson.D{{Key: "owner_id", Value: ownerID}}, opts)
}

func (r *RestaurantRepository) ListAll(ctx context.Context) ([]*model.Restaurant, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	return r.FindMany(ctx, bson.D{}, opts)
}

func (r *RestaurantRepository) UpdateByID(ctx context.Context, id primitive.ObjectID, set bson.D) (*model.Restaurant, error) {
	return r.FindOneAndUpdate(ctx,
		bson.D{{Key: "_id", Value: id}},
		bson.D{{Key: "$set", Value: set}},
	)
}
