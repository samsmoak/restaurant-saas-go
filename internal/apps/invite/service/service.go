package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/mail"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	adminModel "restaurantsaas/internal/apps/admin/model"
	"restaurantsaas/internal/apps/invite/model"
	"restaurantsaas/internal/apps/invite/repository"
	restaurantRepoPkg "restaurantsaas/internal/apps/restaurant/repository"
)

type InviteCreateRequest struct {
	Email string `json:"email"`
	Note  string `json:"note"`
	Role  string `json:"role"`
}

func (r *InviteCreateRequest) Validate() error {
	if strings.TrimSpace(r.Email) != "" {
		if _, err := mail.ParseAddress(strings.TrimSpace(r.Email)); err != nil {
			return errors.New("email is invalid")
		}
	}
	if r.Role != "" && r.Role != adminModel.RoleAdmin && r.Role != adminModel.RoleStaff {
		return errors.New("role must be admin or staff")
	}
	return nil
}

type InviteWithShare struct {
	*model.AdminInvite
	ShareURL string `json:"share_url"`
}

type InviteService interface {
	List(ctx context.Context, restaurantID primitive.ObjectID) ([]*model.AdminInvite, error)
	Create(ctx context.Context, restaurantID, createdBy primitive.ObjectID, req *InviteCreateRequest) (*InviteWithShare, error)
	Revoke(ctx context.Context, restaurantID primitive.ObjectID, id string) error
	Delete(ctx context.Context, restaurantID primitive.ObjectID, id string) error
}

type inviteService struct {
	repo     *repository.InviteRepository
	restRepo *restaurantRepoPkg.RestaurantRepository
}

func NewInviteService(repo *repository.InviteRepository, restRepo *restaurantRepoPkg.RestaurantRepository) InviteService {
	return &inviteService{repo: repo, restRepo: restRepo}
}

func (s *inviteService) List(ctx context.Context, restaurantID primitive.ObjectID) ([]*model.AdminInvite, error) {
	return s.repo.ListForRestaurant(ctx, restaurantID)
}

func (s *inviteService) Create(ctx context.Context, restaurantID, createdBy primitive.ObjectID, req *InviteCreateRequest) (*InviteWithShare, error) {
	code, err := generateInviteCode(12)
	if err != nil {
		return nil, fmt.Errorf("InviteService.Create: %w", err)
	}
	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = adminModel.RoleAdmin
	}
	doc := &model.AdminInvite{
		RestaurantID: restaurantID,
		Code:         code,
		Email:        strings.ToLower(strings.TrimSpace(req.Email)),
		Note:         strings.TrimSpace(req.Note),
		Role:         role,
		CreatedBy:    createdBy,
		Revoked:      false,
		CreatedAt:    time.Now().UTC(),
	}
	if _, err := s.repo.Create(ctx, doc); err != nil {
		return nil, fmt.Errorf("InviteService.Create: %w", err)
	}
	slug := ""
	if s.restRepo != nil {
		if r, err := s.restRepo.GetByID(ctx, restaurantID); err == nil && r != nil {
			slug = r.Slug
		}
	}
	return &InviteWithShare{AdminInvite: doc, ShareURL: buildShareURL(slug, code)}, nil
}

func (s *inviteService) Revoke(ctx context.Context, restaurantID primitive.ObjectID, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid id")
	}
	return s.repo.RevokeScoped(ctx, restaurantID, oid)
}

func (s *inviteService) Delete(ctx context.Context, restaurantID primitive.ObjectID, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid id")
	}
	return s.repo.DeleteScoped(ctx, restaurantID, oid)
}

// base32 alphabet without ambiguous chars (0, 1, I, L, O, U).
const inviteAlphabet = "ABCDEFGHJKMNPQRSTVWXYZ23456789"

func generateInviteCode(n int) (string, error) {
	out := make([]byte, n)
	for i := range out {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(inviteAlphabet))))
		if err != nil {
			return "", err
		}
		out[i] = inviteAlphabet[idx.Int64()]
	}
	return string(out), nil
}

func buildShareURL(slug, code string) string {
	adminURL := strings.TrimRight(os.Getenv("ADMIN_APP_URL"), "/")
	if adminURL == "" {
		adminURL = strings.TrimRight(os.Getenv("APP_URL"), "/")
	}
	if adminURL == "" {
		adminURL = "http://localhost:3001"
	}
	return fmt.Sprintf("%s/onboard?invite=%s&slug=%s", adminURL, code, slug)
}
