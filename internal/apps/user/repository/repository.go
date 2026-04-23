package repository

import (
	"context"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"restaurantsaas/internal/apps/user/model"
	mongoRepo "restaurantsaas/internal/database/abstractrepository/mongodb"
)

type UserRepository struct {
	*mongoRepo.MongoRepository[model.User]
}

func NewUserRepository(db *mongo.Database) *UserRepository {
	return &UserRepository{
		MongoRepository: mongoRepo.NewMongoRepository[model.User](db, "users"),
	}
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	return r.FindOne(ctx, bson.D{{Key: "email", Value: strings.ToLower(email)}})
}

func (r *UserRepository) FindByGoogleSub(ctx context.Context, sub string) (*model.User, error) {
	if sub == "" {
		return nil, nil
	}
	return r.FindOne(ctx, bson.D{{Key: "google_sub", Value: sub}})
}

func (r *UserRepository) AttachGoogleSub(ctx context.Context, id primitive.ObjectID, sub string) error {
	_, err := r.Collection.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: id}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "google_sub", Value: sub}}}},
	)
	if err != nil {
		return fmt.Errorf("UserRepository.AttachGoogleSub: %w", err)
	}
	return nil
}

type CustomerProfileRepository struct {
	*mongoRepo.MongoRepository[model.CustomerProfile]
}

func NewCustomerProfileRepository(db *mongo.Database) *CustomerProfileRepository {
	return &CustomerProfileRepository{
		MongoRepository: mongoRepo.NewMongoRepository[model.CustomerProfile](db, "customer_profiles"),
	}
}

func (r *CustomerProfileRepository) FindByUserID(ctx context.Context, uid primitive.ObjectID) (*model.CustomerProfile, error) {
	return r.FindOne(ctx, bson.D{{Key: "user_id", Value: uid}})
}

func (r *CustomerProfileRepository) UpdateForUser(ctx context.Context, uid primitive.ObjectID, set bson.D) (*model.CustomerProfile, error) {
	return r.FindOneAndUpdate(ctx,
		bson.D{{Key: "user_id", Value: uid}},
		bson.D{{Key: "$set", Value: set}},
	)
}
