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
	usage *mongo.Collection
}

func NewBillingRepository(db *mongo.Database) *BillingRepository {
	return &BillingRepository{
		MongoRepository: mongoRepo.NewMongoRepository[model.Billing](db, "billing"),
		usage:           db.Collection("billing_usage"),
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

// RecordUsageOrder atomically increments the current period's order_count and
// per_order_fee_total for the given restaurant. The period is keyed on the
// first day of the current UTC month so concurrent webhook events converge
// onto the same row.
func (r *BillingRepository) RecordUsageOrder(ctx context.Context, restaurantID primitive.ObjectID, fee float64) error {
	now := time.Now().UTC()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.AddDate(0, 1, 0)
	_, err := r.usage.UpdateOne(ctx,
		bson.D{
			{Key: "restaurant_id", Value: restaurantID},
			{Key: "period_start", Value: periodStart},
		},
		bson.D{
			{Key: "$inc", Value: bson.D{
				{Key: "order_count", Value: 1},
				{Key: "per_order_fee_total", Value: fee},
			}},
			{Key: "$setOnInsert", Value: bson.D{
				{Key: "period_end", Value: periodEnd},
				{Key: "currency", Value: "usd"},
				{Key: "created_at", Value: now},
			}},
			{Key: "$set", Value: bson.D{{Key: "updated_at", Value: now}}},
		},
		options.Update().SetUpsert(true),
	)
	return err
}

// GetCurrentUsage returns the usage row for the current UTC-month period.
// Returns a zero-valued row (not nil) if no row exists yet so callers don't
// need a special case for "no orders this month".
func (r *BillingRepository) GetCurrentUsage(ctx context.Context, restaurantID primitive.ObjectID) (*model.BillingUsage, error) {
	now := time.Now().UTC()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.AddDate(0, 1, 0)
	var u model.BillingUsage
	err := r.usage.FindOne(ctx, bson.D{
		{Key: "restaurant_id", Value: restaurantID},
		{Key: "period_start", Value: periodStart},
	}).Decode(&u)
	if err == mongo.ErrNoDocuments {
		return &model.BillingUsage{
			RestaurantID: restaurantID,
			PeriodStart:  periodStart,
			PeriodEnd:    periodEnd,
			Currency:     "usd",
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}
