package model

import (
	restaurantModel "restaurantsaas/internal/apps/restaurant/model"
)

// RestaurantResult is what the discovery list/search endpoints return.
// It wraps the public-facing restaurant view with computed fields the
// ranker produces (distance, blended score).
type RestaurantResult struct {
	*restaurantModel.PublicView
	DistanceKm *float64 `json:"distance_km,omitempty"`
	Score      *float64 `json:"score,omitempty"`
}

// ListParams is the input shape for List/Search.
type ListParams struct {
	Q       string
	Lat     *float64
	Lng     *float64
	Cuisine string
	Limit   int
	Offset  int
}
