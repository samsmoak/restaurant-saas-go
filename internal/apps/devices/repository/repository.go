package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"restaurantsaas/internal/apps/devices/model"
	mongoRepo "restaurantsaas/internal/database/abstractrepository/mongodb"
)

type DeviceRepository struct {
	*mongoRepo.MongoRepository[model.Device]
}

func NewDeviceRepository(db *mongo.Database) *DeviceRepository {
	return &DeviceRepository{
		MongoRepository: mongoRepo.NewMongoRepository[model.Device](db, "devices"),
	}
}

// Upsert binds an FCM token to a user; idempotent.  Re-registering
// the same (user, token) pair just refreshes the platform string.
func (r *DeviceRepository) Upsert(ctx context.Context, userID primitive.ObjectID, token, platform string) error {
	now := time.Now().UTC()
	_, err := r.Collection.UpdateOne(ctx,
		bson.D{{Key: "fcm_token", Value: token}, {Key: "user_id", Value: userID}},
		bson.D{
			{Key: "$set", Value: bson.D{{Key: "platform", Value: platform}}},
			{Key: "$setOnInsert", Value: bson.D{
				{Key: "fcm_token", Value: token},
				{Key: "user_id", Value: userID},
				{Key: "created_at", Value: now},
			}},
		},
		options.Update().SetUpsert(true),
	)
	return err
}

func (r *DeviceRepository) Remove(ctx context.Context, userID primitive.ObjectID, token string) error {
	_, err := r.Collection.DeleteOne(ctx, bson.D{
		{Key: "user_id", Value: userID},
		{Key: "fcm_token", Value: token},
	})
	return err
}
