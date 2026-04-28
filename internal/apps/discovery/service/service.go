package service

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"

	"restaurantsaas/internal/apps/discovery/model"
	"restaurantsaas/internal/apps/discovery/repository"
	restaurantModel "restaurantsaas/internal/apps/restaurant/model"
)

const (
	defaultLimit = 25
	maxLimit     = 50
)

type DiscoveryService interface {
	List(ctx context.Context, p model.ListParams) ([]*model.RestaurantResult, int64, error)
	Search(ctx context.Context, p model.ListParams) ([]*model.RestaurantResult, int64, error)
	Suggest(ctx context.Context, prefix string) ([]string, error)
}

type discoveryService struct {
	repo *repository.DiscoveryRepository
}

func NewDiscoveryService(repo *repository.DiscoveryRepository) DiscoveryService {
	return &discoveryService{repo: repo}
}

func (s *discoveryService) clampParams(p *model.ListParams) {
	if p.Limit <= 0 {
		p.Limit = defaultLimit
	}
	if p.Limit > maxLimit {
		p.Limit = maxLimit
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
}

func (s *discoveryService) List(ctx context.Context, p model.ListParams) ([]*model.RestaurantResult, int64, error) {
	s.clampParams(&p)
	rows, total, err := s.repo.List(ctx, p)
	if err != nil {
		return nil, 0, err
	}
	return mapResults(rows), total, nil
}

func (s *discoveryService) Search(ctx context.Context, p model.ListParams) ([]*model.RestaurantResult, int64, error) {
	s.clampParams(&p)
	rows, total, err := s.repo.Search(ctx, p)
	if err != nil {
		return nil, 0, err
	}
	return mapResults(rows), total, nil
}

func (s *discoveryService) Suggest(ctx context.Context, prefix string) ([]string, error) {
	return s.repo.Suggest(ctx, prefix, 10)
}

// mapResults converts raw aggregation rows into RestaurantResult by decoding
// each row through bson.Marshal/Unmarshal into a Restaurant struct, taking
// PublicView, and attaching distance + score.
func mapResults(rows []bson.M) []*model.RestaurantResult {
	out := make([]*model.RestaurantResult, 0, len(rows))
	for _, row := range rows {
		raw, err := bson.Marshal(row)
		if err != nil {
			continue
		}
		var r restaurantModel.Restaurant
		if err := bson.Unmarshal(raw, &r); err != nil {
			continue
		}
		res := &model.RestaurantResult{PublicView: r.PublicView()}
		if dm, ok := row["distance_m"].(float64); ok {
			km := dm / 1000.0
			res.DistanceKm = &km
		}
		if score, ok := numericValue(row["composite_score"]); ok {
			res.Score = &score
		}
		if score, ok := numericValue(row["final_score"]); ok {
			res.Score = &score
		}
		out = append(out, res)
	}
	return out
}

func numericValue(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case int:
		return float64(x), true
	}
	return 0, false
}

