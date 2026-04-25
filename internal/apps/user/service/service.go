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

type AddressRequest struct {
	Label   string `json:"label"`
	Address string `json:"address"`
	City    string `json:"city"`
	State   string `json:"state"`
	Zip     string `json:"zip"`
}

func (r *AddressRequest) Validate() error {
	if strings.TrimSpace(r.Label) == "" {
		return errors.New("label is required")
	}
	if strings.TrimSpace(r.Address) == "" {
		return errors.New("address is required")
	}
	if len(strings.TrimSpace(r.Zip)) < 5 {
		return errors.New("zip must be at least 5 characters")
	}
	return nil
}

type UserService interface {
	GetProfile(ctx context.Context, userID string) (*model.CustomerProfile, error)
	UpdateProfile(ctx context.Context, userID string, req *ProfileUpdateRequest) (*model.CustomerProfile, error)
	ListAddresses(ctx context.Context, userID string) ([]model.SavedAddress, error)
	AddAddress(ctx context.Context, userID string, req *AddressRequest) (*model.SavedAddress, error)
	RemoveAddress(ctx context.Context, userID, addressID string) error
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

func (s *userService) ListAddresses(ctx context.Context, userID string) ([]model.SavedAddress, error) {
	oid, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, errors.New("invalid user id")
	}
	return s.profileRepo.ListAddresses(ctx, oid)
}

func (s *userService) AddAddress(ctx context.Context, userID string, req *AddressRequest) (*model.SavedAddress, error) {
	oid, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, errors.New("invalid user id")
	}
	addr := model.SavedAddress{
		ID:      primitive.NewObjectID(),
		Label:   strings.TrimSpace(req.Label),
		Address: strings.TrimSpace(req.Address),
		City:    strings.TrimSpace(req.City),
		State:   strings.TrimSpace(req.State),
		Zip:     strings.TrimSpace(req.Zip),
	}
	if _, err := s.profileRepo.AddAddress(ctx, oid, addr); err != nil {
		return nil, err
	}
	return &addr, nil
}

func (s *userService) RemoveAddress(ctx context.Context, userID, addressID string) error {
	uid, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return errors.New("invalid user id")
	}
	addrOID, err := primitive.ObjectIDFromHex(addressID)
	if err != nil {
		return errors.New("invalid address id")
	}
	_, err = s.profileRepo.RemoveAddress(ctx, uid, addrOID)
	return err
}
