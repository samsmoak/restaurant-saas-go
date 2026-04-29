package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"restaurantsaas/internal/apps/taste/model"
	mongoRepo "restaurantsaas/internal/database/abstractrepository/mongodb"
)

type TasteRepository struct {
	*mongoRepo.MongoRepository[model.TasteProfile]
}

func NewTasteRepository(db *mongo.Database) *TasteRepository {
	return &TasteRepository{
		MongoRepository: mongoRepo.NewMongoRepository[model.TasteProfile](db, "taste_profiles"),
	}
}

func (r *TasteRepository) FindByUserID(ctx context.Context, userID primitive.ObjectID) (*model.TasteProfile, error) {
	return r.FindOne(ctx, bson.D{{Key: "user_id", Value: userID}})
}

func (r *TasteRepository) Upsert(ctx context.Context, userID primitive.ObjectID, p *model.TasteProfile) (*model.TasteProfile, error) {
	now := time.Now().UTC()
	return r.FindOneAndUpdate(ctx,
		bson.D{{Key: "user_id", Value: userID}},
		bson.D{
			{Key: "$set", Value: bson.D{
				{Key: "spice", Value: p.Spice},
				{Key: "dietary", Value: p.Dietary},
				{Key: "cuisines", Value: p.Cuisines},
				{Key: "allergens", Value: p.Allergens},
				{Key: "updated_at", Value: now},
			}},
			{Key: "$setOnInsert", Value: bson.D{{Key: "user_id", Value: userID}}},
		},
		options.FindOneAndUpdate().SetUpsert(true),
	)
}
