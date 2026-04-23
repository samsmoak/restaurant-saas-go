package service

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"restaurantsaas/internal/apps/admin/model"
	"restaurantsaas/internal/apps/admin/repository"
)

type AdminService interface {
	ListForRestaurant(ctx context.Context, restaurantID primitive.ObjectID) ([]*model.AdminUser, error)
	Delete(ctx context.Context, restaurantID primitive.ObjectID, currentUserID, id string) error
}

type adminService struct {
	repo *repository.AdminRepository
}

func NewAdminService(repo *repository.AdminRepository) AdminService {
	return &adminService{repo: repo}
}

func (s *adminService) ListForRestaurant(ctx context.Context, restaurantID primitive.ObjectID) ([]*model.AdminUser, error) {
	return s.repo.ListForRestaurant(ctx, restaurantID)
}

func (s *adminService) Delete(ctx context.Context, restaurantID primitive.ObjectID, currentUserID, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid id")
	}
	row, err := s.repo.GetByID(ctx, oid)
	if err != nil {
		return err
	}
	if row == nil || row.RestaurantID != restaurantID {
		return errors.New("admin not found")
	}
	if row.UserID.Hex() == currentUserID {
		return errors.New("cannot delete your own admin record")
	}
	if row.Role == model.RoleOwner {
		return errors.New("cannot delete the restaurant owner")
	}
	return s.repo.DeleteScoped(ctx, restaurantID, oid)
}
