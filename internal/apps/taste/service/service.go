package service

import (
	"context"
	"errors"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"restaurantsaas/internal/apps/taste/model"
	"restaurantsaas/internal/apps/taste/repository"
)

// UpdateRequest is the body for PUT /api/me/taste-profile.
type UpdateRequest struct {
	Spice     int      `json:"spice"`
	Dietary   []string `json:"dietary"`
	Cuisines  []string `json:"cuisines"`
	Allergens []string `json:"allergens"`
}

func (r *UpdateRequest) Validate() error {
	if r.Spice < 0 || r.Spice > 10 {
		return errors.New("spice must be between 0 and 10")
	}
	return nil
}

type TasteService interface {
	Get(ctx context.Context, userID primitive.ObjectID) (*model.TasteProfile, error)
	Update(ctx context.Context, userID primitive.ObjectID, req *UpdateRequest) (*model.TasteProfile, error)
}

type tasteService struct {
	repo *repository.TasteRepository
}

func NewTasteService(repo *repository.TasteRepository) TasteService {
	return &tasteService{repo: repo}
}

func (s *tasteService) Get(ctx context.Context, userID primitive.ObjectID) (*model.TasteProfile, error) {
	row, err := s.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return model.Default(), nil
	}
	row.EnsureSlices()
	return row, nil
}

func (s *tasteService) Update(ctx context.Context, userID primitive.ObjectID, req *UpdateRequest) (*model.TasteProfile, error) {
	doc := &model.TasteProfile{
		Spice:     req.Spice,
		Dietary:   normaliseList(req.Dietary),
		Cuisines:  normaliseList(req.Cuisines),
		Allergens: normaliseList(req.Allergens),
	}
	row, err := s.repo.Upsert(ctx, userID, doc)
	if err != nil {
		return nil, err
	}
	if row != nil {
		row.EnsureSlices()
	}
	return row, nil
}

func normaliseList(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
