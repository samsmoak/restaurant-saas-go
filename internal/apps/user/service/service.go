package service

import (
	"context"
	"errors"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	favoriteRepoPkg "restaurantsaas/internal/apps/favorites/repository"
	orderRepoPkg "restaurantsaas/internal/apps/order/repository"
	reviewRepoPkg "restaurantsaas/internal/apps/reviews/repository"
	"restaurantsaas/internal/apps/user/model"
	"restaurantsaas/internal/apps/user/repository"
)

// Stats is the lifetime-totals payload backing the Profile screen
// (BACKEND_REQUIREMENTS.md §2 GET /api/me/stats).
type Stats struct {
	Orders  int64 `json:"orders"`
	Saved   int64 `json:"saved"`
	Reviews int64 `json:"reviews"`
}

type ProfileUpdateRequest struct {
	FullName       string  `json:"full_name"`
	Phone          string  `json:"phone"`
	DefaultAddress string  `json:"default_address"`
	PhotoURL       *string `json:"photo_url"`
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
	Label    string  `json:"label"`
	Address  string  `json:"address"`
	City     string  `json:"city"`
	State    string  `json:"state"`
	Zip      string  `json:"zip"`
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
	Floor    string  `json:"floor"`
	Landmark string  `json:"landmark"`
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
	GetStats(ctx context.Context, userID string) (*Stats, error)
}

type userService struct {
	profileRepo  *repository.CustomerProfileRepository
	orderRepo    *orderRepoPkg.OrderRepository
	favoriteRepo *favoriteRepoPkg.FavoriteRepository
	reviewRepo   *reviewRepoPkg.ReviewRepository
}

// NewUserService still accepts only the customer-profile repo so the
// existing callers don't need to change.  The optional Stats deps are
// configured via SetStatsDeps which routes.go calls after wiring.
func NewUserService(profileRepo *repository.CustomerProfileRepository) UserService {
	return &userService{profileRepo: profileRepo}
}

// SetStatsDeps wires the order / favorite / review repos that
// GetStats needs.  Done out-of-band so the existing constructor
// signature is preserved.
func SetStatsDeps(s UserService, orderRepo *orderRepoPkg.OrderRepository, favoriteRepo *favoriteRepoPkg.FavoriteRepository, reviewRepo *reviewRepoPkg.ReviewRepository) {
	if us, ok := s.(*userService); ok {
		us.orderRepo = orderRepo
		us.favoriteRepo = favoriteRepo
		us.reviewRepo = reviewRepo
	}
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
	// PhotoURL is opt-in: nil means "don't touch", "" means "clear it".
	if req.PhotoURL != nil {
		set = append(set, bson.E{Key: "photo_url", Value: strings.TrimSpace(*req.PhotoURL)})
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
		ID:       primitive.NewObjectID(),
		Label:    strings.TrimSpace(req.Label),
		Address:  strings.TrimSpace(req.Address),
		City:     strings.TrimSpace(req.City),
		State:    strings.TrimSpace(req.State),
		Zip:      strings.TrimSpace(req.Zip),
		Lat:      req.Lat,
		Lng:      req.Lng,
		Floor:    strings.TrimSpace(req.Floor),
		Landmark: strings.TrimSpace(req.Landmark),
	}
	if _, err := s.profileRepo.AddAddress(ctx, oid, addr); err != nil {
		return nil, err
	}
	return &addr, nil
}

// GetStats counts the user's orders / favorites / reviews. Failures
// in any one count default to 0 so the Profile screen always renders
// something — counts are advisory, not load-bearing.
func (s *userService) GetStats(ctx context.Context, userID string) (*Stats, error) {
	uid, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, errors.New("invalid user id")
	}
	out := &Stats{}
	if s.orderRepo != nil {
		if n, err := s.orderRepo.Collection.CountDocuments(ctx, bson.D{{Key: "customer_id", Value: uid}}); err == nil {
			out.Orders = n
		}
	}
	if s.favoriteRepo != nil {
		if n, err := s.favoriteRepo.Collection.CountDocuments(ctx, bson.D{{Key: "customer_id", Value: uid}}); err == nil {
			out.Saved = n
		}
	}
	if s.reviewRepo != nil {
		if n, err := s.reviewRepo.Collection.CountDocuments(ctx, bson.D{{Key: "customer_id", Value: uid}}); err == nil {
			out.Reviews = n
		}
	}
	return out, nil
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
