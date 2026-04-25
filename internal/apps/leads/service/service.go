package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"restaurantsaas/internal/apps/leads/model"
	"restaurantsaas/internal/apps/leads/repository"
)

type LeadRequest struct {
	RestaurantName string `json:"restaurant_name"`
	OwnerName      string `json:"owner_name"`
	Phone          string `json:"phone"`
	Email          string `json:"email"`
	CityState      string `json:"city_state"`
	RevenueRange   string `json:"revenue_range"`
	Source         string `json:"source"`
	Message        string `json:"message"`
}

func (r *LeadRequest) Validate() error {
	if strings.TrimSpace(r.RestaurantName) == "" {
		return errors.New("restaurant_name is required")
	}
	if strings.TrimSpace(r.OwnerName) == "" {
		return errors.New("owner_name is required")
	}
	if strings.TrimSpace(r.Phone) == "" {
		return errors.New("phone is required")
	}
	if strings.TrimSpace(r.Email) == "" {
		return errors.New("email is required")
	}
	if strings.TrimSpace(r.CityState) == "" {
		return errors.New("city_state is required")
	}
	return nil
}

type LeadsService interface {
	Submit(ctx context.Context, req *LeadRequest) (*model.Lead, error)
}

type leadsService struct {
	repo *repository.LeadsRepository
}

func NewLeadsService(repo *repository.LeadsRepository) LeadsService {
	return &leadsService{repo: repo}
}

func (s *leadsService) Submit(ctx context.Context, req *LeadRequest) (*model.Lead, error) {
	lead := &model.Lead{
		RestaurantName: strings.TrimSpace(req.RestaurantName),
		OwnerName:      strings.TrimSpace(req.OwnerName),
		Phone:          strings.TrimSpace(req.Phone),
		Email:          strings.ToLower(strings.TrimSpace(req.Email)),
		CityState:      strings.TrimSpace(req.CityState),
		RevenueRange:   strings.TrimSpace(req.RevenueRange),
		Source:         strings.TrimSpace(req.Source),
		Message:        strings.TrimSpace(req.Message),
		SubmittedAt:    time.Now().UTC(),
	}
	return s.repo.Create(ctx, lead)
}
