package repository

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"restaurantsaas/internal/apps/reviews/model"
	mongoRepo "restaurantsaas/internal/database/abstractrepository/mongodb"
)

type ReviewRepository struct {
	*mongoRepo.MongoRepository[model.Review]
}

func NewReviewRepository(db *mongo.Database) *ReviewRepository {
	return &ReviewRepository{
		MongoRepository: mongoRepo.NewMongoRepository[model.Review](db, "reviews"),
	}
}

// Insert returns mongo.IsDuplicateKeyError(err) == true when the order already
// has a review (unique index on order_id).
func (r *ReviewRepository) Insert(ctx context.Context, rv *model.Review) (*model.Review, error) {
	return r.Create(ctx, rv)
}

func (r *ReviewRepository) ListForRestaurant(ctx context.Context, restaurantID primitive.ObjectID, limit, offset int64) ([]*model.Review, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(limit).
		SetSkip(offset)
	return r.FindMany(ctx, bson.D{{Key: "restaurant_id", Value: restaurantID}}, opts)
}

func (r *ReviewRepository) ExistsForOrder(ctx context.Context, orderID primitive.ObjectID) (bool, error) {
	err := r.Collection.FindOne(ctx, bson.D{{Key: "order_id", Value: orderID}}).Err()
	if err == mongo.ErrNoDocuments {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("ReviewRepository.ExistsForOrder: %w", err)
	}
	return true, nil
}
