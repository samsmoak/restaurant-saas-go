package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"restaurantsaas/internal/apps/cravings/model"
	mongoRepo "restaurantsaas/internal/database/abstractrepository/mongodb"
)

type CravingRepository struct {
	*mongoRepo.MongoRepository[model.Craving]
}

func NewCravingRepository(db *mongo.Database) *CravingRepository {
	return &CravingRepository{
		MongoRepository: mongoRepo.NewMongoRepository[model.Craving](db, "cravings"),
	}
}

func (r *CravingRepository) ListForUser(ctx context.Context, userID primitive.ObjectID) ([]*model.Craving, error) {
	opts := options.Find().SetSort(bson.D{
		{Key: "pinned", Value: -1},
		{Key: "created_at", Value: -1},
	}).SetLimit(100)
	return r.FindMany(ctx, bson.D{{Key: "user_id", Value: userID}}, opts)
}

func (r *CravingRepository) SetPinned(ctx context.Context, userID, id primitive.ObjectID, pinned bool) (*model.Craving, error) {
	return r.FindOneAndUpdate(ctx,
		bson.D{{Key: "_id", Value: id}, {Key: "user_id", Value: userID}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "pinned", Value: pinned}}}},
	)
}

func (r *CravingRepository) DeleteForUser(ctx context.Context, userID, id primitive.ObjectID) error {
	_, err := r.Collection.DeleteOne(ctx, bson.D{
		{Key: "_id", Value: id},
		{Key: "user_id", Value: userID},
	})
	return err
}

// InsertSafe inserts a row and stamps created_at to now when missing.
// Best-effort: any error is returned so callers (typically aiService)
// can log + ignore.
func (r *CravingRepository) InsertSafe(ctx context.Context, c *model.Craving) (*model.Craving, error) {
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now().UTC()
	}
	return r.Create(ctx, c)
}
