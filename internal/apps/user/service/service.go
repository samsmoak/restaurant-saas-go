package service

import (
	"context"
	"errors"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"restaurantsaas/internal/apps/user/model"
	"restaurantsaas/internal/apps/user/repository"
)

type ProfileUpdateRequest struct {
	FullName       string `json:"full_name"`
	Phone          string `json:"phone"`
	DefaultAddress string `json:"default_address"`
}

func (r *ProfileUpdateRequest) Validate() error {
	if len([]rune(strings.TrimSpace(r.FullName))) < 2 {
		return errors.New("full_name must be at least 2 characters")
	}
	if len(strings.TrimSpace(r.Phone)) < 10 {
		return errors.New("phone must be at least 10 characters")
	}
	return nil
}

type UserService interface {
	GetProfile(ctx context.Context, userID string) (*model.CustomerProfile, error)
	UpdateProfile(ctx context.Context, userID string, req *ProfileUpdateRequest) (*model.CustomerProfile, error)
}

type userService struct {
	profileRepo *repository.CustomerProfileRepository
}

func NewUserService(profileRepo *repository.CustomerProfileRepository) UserService {
	return &userService{profileRepo: profileRepo}
}

func (s *userService) GetProfile(ctx context.Context, userID string) (*model.CustomerProfile, error) {
	oid, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, errors.New("invalid user id")
	}
	return s.profileRepo.FindByUserID(ctx, oid)
}

func (s *userService) UpdateProfile(ctx context.Context, userID string, req *ProfileUpdateRequest) (*model.CustomerProfile, error) {
	oid, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, errors.New("invalid user id")
	}
	set := bson.D{
		{Key: "full_name", Value: strings.TrimSpace(req.FullName)},
		{Key: "phone", Value: strings.TrimSpace(req.Phone)},
		{Key: "default_address", Value: strings.TrimSpace(req.DefaultAddress)},
	}
	return s.profileRepo.UpdateForUser(ctx, oid, set)
}
