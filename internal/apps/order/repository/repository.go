package repository

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"restaurantsaas/internal/apps/order/model"
	mongoRepo "restaurantsaas/internal/database/abstractrepository/mongodb"
)

type OrderRepository struct {
	*mongoRepo.MongoRepository[model.Order]
}

func NewOrderRepository(db *mongo.Database) *OrderRepository {
	return &OrderRepository{
		MongoRepository: mongoRepo.NewMongoRepository[model.Order](db, "orders"),
	}
}

func (r *OrderRepository) ExistsByOrderNumber(ctx context.Context, n string) (bool, error) {
	err := r.Collection.FindOne(ctx, bson.D{{Key: "order_number", Value: n}}).Err()
	if err == mongo.ErrNoDocuments {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("OrderRepository.ExistsByOrderNumber: %w", err)
	}
	return true, nil
}

func (r *OrderRepository) GetByOrderNumber(ctx context.Context, n string) (*model.Order, error) {
	return r.FindOne(ctx, bson.D{{Key: "order_number", Value: n}})
}

func (r *OrderRepository) ListForCustomer(ctx context.Context, uid primitive.ObjectID, restaurantID *primitive.ObjectID, limit int64) ([]*model.Order, error) {
	filter := bson.D{{Key: "customer_id", Value: uid}}
	if restaurantID != nil {
		filter = append(filter, bson.E{Key: "restaurant_id", Value: *restaurantID})
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(limit)
	return r.FindMany(ctx, filter, opts)
}

func (r *OrderRepository) ListByStatus(ctx context.Context, restaurantID primitive.ObjectID, status string, limit int64) ([]*model.Order, error) {
	filter := bson.D{{Key: "restaurant_id", Value: restaurantID}}
	if status != "" && status != "all" {
		filter = append(filter, bson.E{Key: "status", Value: status})
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(limit)
	return r.FindMany(ctx, filter, opts)
}

func (r *OrderRepository) UpdatePaymentStatusByIntent(ctx context.Context, intentID, status string) (*model.Order, error) {
	return r.FindOneAndUpdate(ctx,
		bson.D{{Key: "payment_intent_id", Value: intentID}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "payment_status", Value: status}}}},
	)
}

func (r *OrderRepository) AttachPaymentIntent(ctx context.Context, id primitive.ObjectID, intentID string) error {
	_, err := r.Collection.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: id}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "payment_intent_id", Value: intentID}}}},
	)
	if err != nil {
		return fmt.Errorf("OrderRepository.AttachPaymentIntent: %w", err)
	}
	return nil
}

func (r *OrderRepository) GetScopedByID(ctx context.Context, restaurantID, id primitive.ObjectID) (*model.Order, error) {
	return r.FindOne(ctx, bson.D{
		{Key: "_id", Value: id},
		{Key: "restaurant_id", Value: restaurantID},
	})
}

func (r *OrderRepository) DeleteScoped(ctx context.Context, restaurantID, id primitive.ObjectID) error {
	_, err := r.Collection.DeleteOne(ctx, bson.D{
		{Key: "_id", Value: id},
		{Key: "restaurant_id", Value: restaurantID},
	})
	return err
}

func (r *OrderRepository) ListBetween(ctx context.Context, restaurantID primitive.ObjectID, from, to time.Time) ([]*model.Order, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(5000)
	filter := bson.D{{Key: "restaurant_id", Value: restaurantID}}
	if !from.IsZero() || !to.IsZero() {
		rng := bson.D{}
		if !from.IsZero() {
			rng = append(rng, bson.E{Key: "$gte", Value: from})
		}
		if !to.IsZero() {
			rng = append(rng, bson.E{Key: "$lte", Value: to})
		}
		filter = append(filter, bson.E{Key: "created_at", Value: rng})
	}
	return r.FindMany(ctx, filter, opts)
}
