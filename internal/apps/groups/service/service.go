package service

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"restaurantsaas/internal/apps/groups/model"
	"restaurantsaas/internal/apps/groups/repository"
	orderSvc "restaurantsaas/internal/apps/order/service"
	restaurantSvc "restaurantsaas/internal/apps/restaurant/service"
	userRepoPkg "restaurantsaas/internal/apps/user/repository"
	"restaurantsaas/internal/pkg/money"
)

var (
	ErrNotHost      = errors.New("only the host can perform this action")
	ErrGroupMissing = errors.New("group not found")
	ErrLocked       = errors.New("group is locked")
	ErrNotLocked    = errors.New("group is not locked")
)

// GroupView is what GET /api/groups/{shareCode} returns.
type GroupView struct {
	*model.Group
	Members       []*model.GroupMember `json:"members"`
	SubtotalCents int64                `json:"subtotal_cents"`
}

// CreateResult is the POST /api/groups payload.
type CreateResult struct {
	*model.Group
	ShareURL string `json:"share_url"`
}

type GroupService interface {
	Create(ctx context.Context, hostID, restaurantID primitive.ObjectID) (*CreateResult, error)
	GetByShareCode(ctx context.Context, code string) (*GroupView, error)
	Join(ctx context.Context, hostName string, hostAvatar string, code string, userID primitive.ObjectID) error
	Lock(ctx context.Context, code string, hostID primitive.ObjectID, lockMinutes int) (*model.Group, error)
	Checkout(ctx context.Context, code string, hostID primitive.ObjectID, userEmail string) (*orderSvc.CheckoutResult, error)
}

type groupService struct {
	repo        *repository.GroupRepository
	orders      orderSvc.OrderService
	restSvc     restaurantSvc.RestaurantService
	profileRepo *userRepoPkg.CustomerProfileRepository
}

func NewGroupService(repo *repository.GroupRepository, orders orderSvc.OrderService, restSvc restaurantSvc.RestaurantService, profileRepo *userRepoPkg.CustomerProfileRepository) GroupService {
	return &groupService{repo: repo, orders: orders, restSvc: restSvc, profileRepo: profileRepo}
}

func (s *groupService) Create(ctx context.Context, hostID, restaurantID primitive.ObjectID) (*CreateResult, error) {
	rest, err := s.restSvc.GetByID(ctx, restaurantID)
	if err != nil {
		return nil, err
	}
	if rest == nil {
		return nil, errors.New("restaurant not found")
	}

	code, err := s.generateShareCode(ctx)
	if err != nil {
		return nil, err
	}
	g := &model.Group{
		ShareCode:               code,
		HostUserID:              hostID,
		RestaurantID:            restaurantID,
		MinForFreeDeliveryCents: money.ToCents(rest.MinOrderAmount),
	}
	if err := s.repo.Create(ctx, g); err != nil {
		return nil, err
	}
	// Auto-join the host.
	hostName, hostAvatar := s.lookupProfile(ctx, hostID)
	_ = s.repo.AddMember(ctx, &model.GroupMember{
		GroupID:   g.ID,
		UserID:    hostID,
		Name:      hostName,
		AvatarURL: hostAvatar,
	})
	return &CreateResult{Group: g, ShareURL: shareURLFor(code)}, nil
}

func (s *groupService) GetByShareCode(ctx context.Context, code string) (*GroupView, error) {
	g, err := s.repo.FindByShareCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if g == nil {
		return nil, ErrGroupMissing
	}
	members, err := s.repo.ListMembers(ctx, g.ID)
	if err != nil {
		return nil, err
	}
	var subtotal int64
	for _, m := range members {
		subtotal += m.SubtotalCents
	}
	return &GroupView{Group: g, Members: members, SubtotalCents: subtotal}, nil
}

func (s *groupService) Join(ctx context.Context, _ string, _ string, code string, userID primitive.ObjectID) error {
	g, err := s.repo.FindByShareCode(ctx, code)
	if err != nil {
		return err
	}
	if g == nil {
		return ErrGroupMissing
	}
	if g.LockExpiresAt != nil && g.LockExpiresAt.After(time.Now()) {
		return ErrLocked
	}
	name, avatar := s.lookupProfile(ctx, userID)
	return s.repo.AddMember(ctx, &model.GroupMember{
		GroupID:   g.ID,
		UserID:    userID,
		Name:      name,
		AvatarURL: avatar,
	})
}

func (s *groupService) Lock(ctx context.Context, code string, hostID primitive.ObjectID, lockMinutes int) (*model.Group, error) {
	if lockMinutes <= 0 || lockMinutes > 60 {
		lockMinutes = 5
	}
	g, err := s.repo.FindByShareCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if g == nil {
		return nil, ErrGroupMissing
	}
	if g.HostUserID != hostID {
		return nil, ErrNotHost
	}
	until := time.Now().Add(time.Duration(lockMinutes) * time.Minute).UTC()
	return s.repo.SetLockExpiry(ctx, g.ID, until)
}

func (s *groupService) Checkout(ctx context.Context, code string, hostID primitive.ObjectID, userEmail string) (*orderSvc.CheckoutResult, error) {
	g, err := s.repo.FindByShareCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if g == nil {
		return nil, ErrGroupMissing
	}
	if g.HostUserID != hostID {
		return nil, ErrNotHost
	}
	if g.LockExpiresAt == nil || g.LockExpiresAt.Before(time.Now()) {
		return nil, ErrNotLocked
	}
	members, err := s.repo.ListMembers(ctx, g.ID)
	if err != nil {
		return nil, err
	}
	// Flatten members → checkout lines.  Each member.Line carries
	// menu_item_id and selected size/extras names — sufficient for the
	// legacy CheckoutRequest path.
	req := &orderSvc.CheckoutRequest{
		OrderType: "delivery",
		Items:     []orderSvc.CheckoutRequestItem{},
	}
	for _, m := range members {
		for _, l := range m.Lines {
			line := orderSvc.CheckoutRequestItem{
				MenuItemID:          l.MenuItemID,
				Quantity:            l.Quantity,
				SpecialInstructions: l.SpecialInstructions,
			}
			if l.SelectedSize != nil {
				line.SelectedSize = &orderSvc.CheckoutSelectedSize{Name: l.SelectedSize.Name}
			}
			for _, ex := range l.SelectedExtras {
				line.SelectedExtras = append(line.SelectedExtras, orderSvc.CheckoutSelectedExtra{Name: ex.Name})
			}
			req.Items = append(req.Items, line)
		}
	}
	if len(req.Items) == 0 {
		return nil, errors.New("no items in group order")
	}
	return s.orders.ValidateAndBuildOrder(ctx, g.RestaurantID, hostID.Hex(), userEmail, req)
}

func (s *groupService) generateShareCode(ctx context.Context) (string, error) {
	for i := 0; i < 5; i++ {
		buf := make([]byte, 5)
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		code := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf)
		// Trim to 4 chars uppercase.
		code = strings.ToUpper(code)[:4]
		taken, err := s.repo.ShareCodeTaken(ctx, code)
		if err != nil {
			return "", err
		}
		if !taken {
			return code, nil
		}
	}
	return "", fmt.Errorf("could not generate unique share code")
}

func (s *groupService) lookupProfile(ctx context.Context, userID primitive.ObjectID) (string, string) {
	if s.profileRepo == nil {
		return "", ""
	}
	p, _ := s.profileRepo.FindByUserID(ctx, userID)
	if p == nil {
		return "", ""
	}
	return p.FullName, p.PhotoURL
}

func shareURLFor(code string) string {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("PUBLIC_GROUP_URL_BASE")), "/")
	if base == "" {
		base = "savorar.app/g"
	}
	return fmt.Sprintf("%s/%s", base, code)
}
