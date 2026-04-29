package repository

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"restaurantsaas/internal/apps/notifications/model"
	mongoRepo "restaurantsaas/internal/database/abstractrepository/mongodb"
)

type NotificationRepository struct {
	*mongoRepo.MongoRepository[model.Notification]
}

func NewNotificationRepository(db *mongo.Database) *NotificationRepository {
	return &NotificationRepository{
		MongoRepository: mongoRepo.NewMongoRepository[model.Notification](db, "notifications"),
	}
}

func (r *NotificationRepository) ListForUser(ctx context.Context, userID primitive.ObjectID) ([]*model.Notification, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(100)
	return r.FindMany(ctx, bson.D{{Key: "user_id", Value: userID}}, opts)
}

func (r *NotificationRepository) MarkRead(ctx context.Context, userID primitive.ObjectID, ids []primitive.ObjectID) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.Collection.UpdateMany(ctx,
		bson.D{
			{Key: "user_id", Value: userID},
			{Key: "_id", Value: bson.D{{Key: "$in", Value: ids}}},
		},
		bson.D{{Key: "$set", Value: bson.D{{Key: "read", Value: true}}}},
	)
	return err
}

func (r *NotificationRepository) MarkAllRead(ctx context.Context, userID primitive.ObjectID) error {
	_, err := r.Collection.UpdateMany(ctx,
		bson.D{{Key: "user_id", Value: userID}, {Key: "read", Value: false}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "read", Value: true}}}},
	)
	return err
}
