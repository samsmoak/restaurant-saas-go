package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"restaurantsaas/internal/apps/billing/model"
	mongoRepo "restaurantsaas/internal/database/abstractrepository/mongodb"
)

type BillingRepository struct {
	*mongoRepo.MongoRepository[model.Billing]
}

func NewBillingRepository(db *mongo.Database) *BillingRepository {
	return &BillingRepository{
		MongoRepository: mongoRepo.NewMongoRepository[model.Billing](db, "billing"),
	}
}

func (r *BillingRepository) FindByRestaurantID(ctx context.Context, id primitive.ObjectID) (*model.Billing, error) {
	return r.FindOne(ctx, bson.D{{Key: "restaurant_id", Value: id}})
}

func (r *BillingRepository) FindByStripeCustomerID(ctx context.Context, stripeID string) (*model.Billing, error) {
	return r.FindOne(ctx, bson.D{{Key: "stripe_customer_id", Value: stripeID}})
}

func (r *BillingRepository) FindBySubscriptionID(ctx context.Context, subID string) (*model.Billing, error) {
	return r.FindOne(ctx, bson.D{{Key: "subscription_id", Value: subID}})
}

// Upsert creates or updates the billing row for restaurantID.
// $setOnInsert seeds restaurant_id + created_at on first insert only.
func (r *BillingRepository) Upsert(ctx context.Context, restaurantID primitive.ObjectID, set bson.D) (*model.Billing, error) {
	return r.FindOneAndUpdate(ctx,
		bson.D{{Key: "restaurant_id", Value: restaurantID}},
		bson.D{
			{Key: "$set", Value: set},
			{Key: "$setOnInsert", Value: bson.D{
				{Key: "restaurant_id", Value: restaurantID},
				{Key: "created_at", Value: time.Now().UTC()},
			}},
		},
		options.FindOneAndUpdate().SetUpsert(true),
	)
}
