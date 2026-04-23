package repository

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"restaurantsaas/internal/apps/admin/model"
	mongoRepo "restaurantsaas/internal/database/abstractrepository/mongodb"
)

type AdminRepository struct {
	*mongoRepo.MongoRepository[model.AdminUser]
}

func NewAdminRepository(db *mongo.Database) *AdminRepository {
	return &AdminRepository{
		MongoRepository: mongoRepo.NewMongoRepository[model.AdminUser](db, "admin_users"),
	}
}

// FindForUserAndRestaurant returns the admin membership row for (user, restaurant).
func (r *AdminRepository) FindForUserAndRestaurant(ctx context.Context, userID, restaurantID primitive.ObjectID) (*model.AdminUser, error) {
	return r.FindOne(ctx, bson.D{
		{Key: "user_id", Value: userID},
		{Key: "restaurant_id", Value: restaurantID},
	})
}

// ListByUserID returns every restaurant a user is an admin of.
func (r *AdminRepository) ListByUserID(ctx context.Context, userID primitive.ObjectID) ([]*model.AdminUser, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	return r.FindMany(ctx, bson.D{{Key: "user_id", Value: userID}}, opts)
}

// ListForRestaurant returns every admin assigned to a specific restaurant.
func (r *AdminRepository) ListForRestaurant(ctx context.Context, restaurantID primitive.ObjectID) ([]*model.AdminUser, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	return r.FindMany(ctx, bson.D{{Key: "restaurant_id", Value: restaurantID}}, opts)
}

// IsAdmin returns true if the user has any admin row for the given restaurant.
func (r *AdminRepository) IsAdmin(ctx context.Context, userID, restaurantID primitive.ObjectID) (bool, error) {
	row, err := r.FindForUserAndRestaurant(ctx, userID, restaurantID)
	if err != nil {
		return false, err
	}
	return row != nil, nil
}

func (r *AdminRepository) DeleteScoped(ctx context.Context, restaurantID, id primitive.ObjectID) error {
	_, err := r.Collection.DeleteOne(ctx, bson.D{
		{Key: "_id", Value: id},
		{Key: "restaurant_id", Value: restaurantID},
	})
	return err
}
