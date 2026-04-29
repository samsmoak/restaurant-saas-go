package service

import (
	"context"
	"errors"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"restaurantsaas/internal/apps/devices/repository"
)

type RegisterRequest struct {
	FCMToken string `json:"fcm_token"`
	Platform string `json:"platform"`
}

func (r *RegisterRequest) Validate() error {
	if strings.TrimSpace(r.FCMToken) == "" {
		return errors.New("fcm_token is required")
	}
	p := strings.ToLower(strings.TrimSpace(r.Platform))
	if p != "ios" && p != "android" && p != "web" {
		return errors.New("platform must be ios, android or web")
	}
	r.Platform = p
	return nil
}

type DeviceService interface {
	Register(ctx context.Context, userID primitive.ObjectID, req *RegisterRequest) error
	Unregister(ctx context.Context, userID primitive.ObjectID, token string) error
}

type deviceService struct {
	repo *repository.DeviceRepository
}

func NewDeviceService(repo *repository.DeviceRepository) DeviceService {
	return &deviceService{repo: repo}
}

func (s *deviceService) Register(ctx context.Context, userID primitive.ObjectID, req *RegisterRequest) error {
	return s.repo.Upsert(ctx, userID, strings.TrimSpace(req.FCMToken), req.Platform)
}

func (s *deviceService) Unregister(ctx context.Context, userID primitive.ObjectID, token string) error {
	return s.repo.Remove(ctx, userID, strings.TrimSpace(token))
}
