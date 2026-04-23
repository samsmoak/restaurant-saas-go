package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"restaurantsaas/internal/apps/invite/model"
	mongoRepo "restaurantsaas/internal/database/abstractrepository/mongodb"
)

type InviteRepository struct {
	*mongoRepo.MongoRepository[model.AdminInvite]
}

func NewInviteRepository(db *mongo.Database) *InviteRepository {
	return &InviteRepository{
		MongoRepository: mongoRepo.NewMongoRepository[model.AdminInvite](db, "admin_invites"),
	}
}

// FindActiveByCode does NOT scope by restaurant — the code itself is globally unique.
// The service decides what restaurant the caller joins based on the invite row.
func (r *InviteRepository) FindActiveByCode(ctx context.Context, code string) (*model.AdminInvite, error) {
	return r.FindOne(ctx, bson.D{
		{Key: "code", Value: code},
		{Key: "revoked", Value: false},
		{Key: "used_at", Value: nil},
	})
}

func (r *InviteRepository) MarkUsed(ctx context.Context, id primitive.ObjectID, userID primitive.ObjectID) error {
	now := time.Now().UTC()
	_, err := r.Collection.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: id}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "used_by", Value: userID},
			{Key: "used_at", Value: now},
		}}},
	)
	return err
}

func (r *InviteRepository) RevokeScoped(ctx context.Context, restaurantID, id primitive.ObjectID) error {
	_, err := r.Collection.UpdateOne(ctx,
		bson.D{
			{Key: "_id", Value: id},
			{Key: "restaurant_id", Value: restaurantID},
		},
		bson.D{{Key: "$set", Value: bson.D{{Key: "revoked", Value: true}}}},
	)
	return err
}

func (r *InviteRepository) DeleteScoped(ctx context.Context, restaurantID, id primitive.ObjectID) error {
	_, err := r.Collection.DeleteOne(ctx, bson.D{
		{Key: "_id", Value: id},
		{Key: "restaurant_id", Value: restaurantID},
	})
	return err
}

func (r *InviteRepository) ListForRestaurant(ctx context.Context, restaurantID primitive.ObjectID) ([]*model.AdminInvite, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	return r.FindMany(ctx, bson.D{{Key: "restaurant_id", Value: restaurantID}}, opts)
}
