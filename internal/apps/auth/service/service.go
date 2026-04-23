package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"

	adminModel "restaurantsaas/internal/apps/admin/model"
	adminRepoPkg "restaurantsaas/internal/apps/admin/repository"
	authModel "restaurantsaas/internal/apps/auth/model"
	inviteRepoPkg "restaurantsaas/internal/apps/invite/repository"
	restaurantRepoPkg "restaurantsaas/internal/apps/restaurant/repository"
	userModel "restaurantsaas/internal/apps/user/model"
	userRepoPkg "restaurantsaas/internal/apps/user/repository"
	"restaurantsaas/internal/jwtutil"
	"restaurantsaas/internal/oauth"
)

type Membership struct {
	RestaurantID   string `json:"restaurant_id"`
	RestaurantName string `json:"restaurant_name"`
	Role           string `json:"role"`
}

type AuthResponse struct {
	Token       string                     `json:"token"`
	User        *userModel.User            `json:"user"`
	Profile     *userModel.CustomerProfile `json:"profile,omitempty"`
	IsAdmin     bool                       `json:"is_admin"`
	Memberships []Membership               `json:"memberships"`
}

type AuthService interface {
	SignupCustomer(ctx context.Context, req *authModel.SignupRequest) (*AuthResponse, error)
	Login(ctx context.Context, req *authModel.LoginRequest) (*AuthResponse, error)
	GoogleSignIn(ctx context.Context, req *authModel.GoogleAuthRequest) (*AuthResponse, error)
	AdminFinalize(ctx context.Context, userID string, userEmail string, inviteCode string) (*FinalizeResult, error)
	ActivateAdmin(ctx context.Context, userID, email, restaurantID string) (*AuthResponse, error)
	ListMemberships(ctx context.Context, userID primitive.ObjectID) ([]Membership, error)
}

type FinalizeResult struct {
	RestaurantID string `json:"restaurant_id"`
	Token        string `json:"token"`
	Role         string `json:"role"`
}

type authService struct {
	client      *mongo.Client
	userRepo    *userRepoPkg.UserRepository
	profileRepo *userRepoPkg.CustomerProfileRepository
	adminRepo   *adminRepoPkg.AdminRepository
	inviteRepo  *inviteRepoPkg.InviteRepository
	restRepo    *restaurantRepoPkg.RestaurantRepository
}

func NewAuthService(
	client *mongo.Client,
	userRepo *userRepoPkg.UserRepository,
	profileRepo *userRepoPkg.CustomerProfileRepository,
	adminRepo *adminRepoPkg.AdminRepository,
	inviteRepo *inviteRepoPkg.InviteRepository,
	restRepo *restaurantRepoPkg.RestaurantRepository,
) AuthService {
	return &authService{
		client:      client,
		userRepo:    userRepo,
		profileRepo: profileRepo,
		adminRepo:   adminRepo,
		inviteRepo:  inviteRepo,
		restRepo:    restRepo,
	}
}

var ErrEmailTaken = errors.New("email already registered")
var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrInviteInvalid = errors.New("invalid or already-used invite code")
var ErrInviteEmailMismatch = errors.New("this invite is tied to a different email")
var ErrNotAdmin = errors.New("user is not an admin of that restaurant")

func (s *authService) SignupCustomer(ctx context.Context, req *authModel.SignupRequest) (*AuthResponse, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	existing, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("AuthService.SignupCustomer: %w", err)
	}
	if existing != nil {
		return nil, ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return nil, fmt.Errorf("AuthService.SignupCustomer: bcrypt: %w", err)
	}

	now := time.Now().UTC()
	user := &userModel.User{
		Email:         email,
		PasswordHash:  string(hash),
		FullName:      strings.TrimSpace(req.FullName),
		Phone:         strings.TrimSpace(req.Phone),
		EmailVerified: false,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if _, err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("AuthService.SignupCustomer: insert user: %w", err)
	}

	profile := &userModel.CustomerProfile{
		UserID:    user.ID,
		Email:     user.Email,
		FullName:  user.FullName,
		Phone:     user.Phone,
		CreatedAt: now,
	}
	if _, err := s.profileRepo.Create(ctx, profile); err != nil {
		_ = s.userRepo.Delete(ctx, user.ID)
		return nil, fmt.Errorf("AuthService.SignupCustomer: insert profile: %w", err)
	}

	token, err := jwtutil.Sign(user.ID.Hex(), user.Email)
	if err != nil {
		return nil, fmt.Errorf("AuthService.SignupCustomer: sign token: %w", err)
	}

	return &AuthResponse{
		Token:       token,
		User:        user,
		Profile:     profile,
		IsAdmin:     false,
		Memberships: []Membership{},
	}, nil
}

func (s *authService) Login(ctx context.Context, req *authModel.LoginRequest) (*AuthResponse, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("AuthService.Login: %w", err)
	}
	if user == nil || user.PasswordHash == "" {
		return nil, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	return s.buildAuthResponse(ctx, user)
}

func (s *authService) GoogleSignIn(ctx context.Context, req *authModel.GoogleAuthRequest) (*AuthResponse, error) {
	payload, err := oauth.VerifyIDToken(ctx, req.IDToken)
	if err != nil {
		return nil, fmt.Errorf("AuthService.GoogleSignIn: %w", err)
	}
	if payload.Email == "" {
		return nil, errors.New("google account has no email")
	}
	email := strings.ToLower(payload.Email)
	user, err := s.userRepo.FindByGoogleSub(ctx, payload.Sub)
	if err != nil {
		return nil, fmt.Errorf("AuthService.GoogleSignIn: %w", err)
	}
	if user == nil {
		byEmail, err := s.userRepo.FindByEmail(ctx, email)
		if err != nil {
			return nil, fmt.Errorf("AuthService.GoogleSignIn: %w", err)
		}
		if byEmail != nil {
			if err := s.userRepo.AttachGoogleSub(ctx, byEmail.ID, payload.Sub); err != nil {
				return nil, err
			}
			byEmail.GoogleSub = payload.Sub
			user = byEmail
		} else {
			now := time.Now().UTC()
			nu := &userModel.User{
				Email:         email,
				GoogleSub:     payload.Sub,
				FullName:      payload.Name,
				EmailVerified: payload.EmailVerified,
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			if _, err := s.userRepo.Create(ctx, nu); err != nil {
				return nil, fmt.Errorf("AuthService.GoogleSignIn: insert user: %w", err)
			}
			user = nu
		}
	}

	profile, _ := s.profileRepo.FindByUserID(ctx, user.ID)
	if profile == nil {
		now := time.Now().UTC()
		profile = &userModel.CustomerProfile{
			UserID:    user.ID,
			Email:     user.Email,
			FullName:  user.FullName,
			CreatedAt: now,
		}
		if _, err := s.profileRepo.Create(ctx, profile); err != nil {
			p, _ := s.profileRepo.FindByUserID(ctx, user.ID)
			profile = p
		}
	}
	return s.buildAuthResponse(ctx, user)
}

func (s *authService) buildAuthResponse(ctx context.Context, user *userModel.User) (*AuthResponse, error) {
	token, err := jwtutil.Sign(user.ID.Hex(), user.Email)
	if err != nil {
		return nil, fmt.Errorf("AuthService.buildAuthResponse: sign: %w", err)
	}
	profile, _ := s.profileRepo.FindByUserID(ctx, user.ID)
	memberships, _ := s.ListMemberships(ctx, user.ID)
	if memberships == nil {
		memberships = []Membership{}
	}
	return &AuthResponse{
		Token:       token,
		User:        user,
		Profile:     profile,
		IsAdmin:     len(memberships) > 0,
		Memberships: memberships,
	}, nil
}

func (s *authService) ListMemberships(ctx context.Context, userID primitive.ObjectID) ([]Membership, error) {
	rows, err := s.adminRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []Membership{}, nil
	}
	ids := make([]primitive.ObjectID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.RestaurantID)
	}
	restaurants, err := s.restRepo.FindMany(ctx, bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: ids}}}})
	if err != nil {
		return nil, err
	}
	restMap := make(map[primitive.ObjectID]string, len(restaurants))
	for _, r := range restaurants {
		restMap[r.ID] = r.Name
	}
	out := make([]Membership, 0, len(rows))
	for _, row := range rows {
		out = append(out, Membership{
			RestaurantID:   row.RestaurantID.Hex(),
			RestaurantName: restMap[row.RestaurantID],
			Role:           row.Role,
		})
	}
	return out, nil
}

func (s *authService) AdminFinalize(ctx context.Context, userID string, userEmail string, inviteCode string) (*FinalizeResult, error) {
	code := strings.TrimSpace(inviteCode)
	if code == "" {
		return nil, errors.New("invite_code is required")
	}
	userOID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, errors.New("invalid user id")
	}

	// Legacy env bootstrap: claim against ADMIN_LEGACY_RESTAURANT_ID +
	// ADMIN_LEGACY_INVITE_CODE. Use only once, then unset both.
	legacyCode := os.Getenv("ADMIN_LEGACY_INVITE_CODE")
	legacyRestID := os.Getenv("ADMIN_LEGACY_RESTAURANT_ID")
	if legacyCode != "" && code == legacyCode && legacyRestID != "" {
		oid, err := primitive.ObjectIDFromHex(legacyRestID)
		if err != nil {
			return nil, fmt.Errorf("ADMIN_LEGACY_RESTAURANT_ID is not a valid ObjectID")
		}
		r, err := s.restRepo.GetByID(ctx, oid)
		if err != nil {
			return nil, err
		}
		if r == nil {
			return nil, fmt.Errorf("legacy restaurant '%s' not found", legacyRestID)
		}
		if err := s.upsertAdminUser(ctx, userOID, userEmail, r.ID, adminModel.RoleAdmin); err != nil {
			return nil, err
		}
		token, err := jwtutil.SignWithOptions(jwtutil.SignOptions{
			UserID: userID, Email: userEmail, RestaurantID: r.ID.Hex(), Role: adminModel.RoleAdmin,
		})
		if err != nil {
			return nil, err
		}
		return &FinalizeResult{RestaurantID: r.ID.Hex(), Token: token, Role: adminModel.RoleAdmin}, nil
	}

	var result *FinalizeResult
	session, err := s.client.StartSession()
	if err != nil {
		return nil, fmt.Errorf("AuthService.AdminFinalize: start session: %w", err)
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sctx mongo.SessionContext) (interface{}, error) {
		invite, err := s.inviteRepo.FindActiveByCode(sctx, code)
		if err != nil {
			return nil, err
		}
		if invite == nil {
			return nil, ErrInviteInvalid
		}
		if invite.Email != "" && !strings.EqualFold(strings.TrimSpace(invite.Email), strings.TrimSpace(userEmail)) {
			return nil, ErrInviteEmailMismatch
		}
		role := invite.Role
		if role == "" {
			role = adminModel.RoleAdmin
		}
		if err := s.upsertAdminUser(sctx, userOID, userEmail, invite.RestaurantID, role); err != nil {
			return nil, err
		}
		if err := s.inviteRepo.MarkUsed(sctx, invite.ID, userOID); err != nil {
			return nil, err
		}
		r, err := s.restRepo.GetByID(sctx, invite.RestaurantID)
		if err != nil {
			return nil, err
		}
		if r == nil {
			return nil, fmt.Errorf("invite's restaurant not found")
		}
		token, err := jwtutil.SignWithOptions(jwtutil.SignOptions{
			UserID: userID, Email: userEmail, RestaurantID: r.ID.Hex(), Role: role,
		})
		if err != nil {
			return nil, err
		}
		result = &FinalizeResult{RestaurantID: r.ID.Hex(), Token: token, Role: role}
		return nil, nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ActivateAdmin issues a new token scoped to the given restaurant, if the user
// is already an admin of it. Used when an admin has multiple tenants and
// wants to switch between them from the UI.
func (s *authService) ActivateAdmin(ctx context.Context, userID, email, restaurantID string) (*AuthResponse, error) {
	userOID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, errors.New("invalid user id")
	}
	restOID, err := primitive.ObjectIDFromHex(restaurantID)
	if err != nil {
		return nil, errors.New("invalid restaurant id")
	}
	row, err := s.adminRepo.FindForUserAndRestaurant(ctx, userOID, restOID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, ErrNotAdmin
	}
	token, err := jwtutil.SignWithOptions(jwtutil.SignOptions{
		UserID: userID, Email: email, RestaurantID: restaurantID, Role: row.Role,
	})
	if err != nil {
		return nil, err
	}
	user, _ := s.userRepo.GetByID(ctx, userOID)
	profile, _ := s.profileRepo.FindByUserID(ctx, userOID)
	memberships, _ := s.ListMemberships(ctx, userOID)
	return &AuthResponse{
		Token:       token,
		User:        user,
		Profile:     profile,
		IsAdmin:     true,
		Memberships: memberships,
	}, nil
}

func (s *authService) upsertAdminUser(ctx context.Context, userID primitive.ObjectID, email string, restaurantID primitive.ObjectID, role string) error {
	now := time.Now().UTC()
	row := adminModel.AdminUser{
		UserID:       userID,
		RestaurantID: restaurantID,
		Email:        strings.ToLower(strings.TrimSpace(email)),
		Role:         role,
		CreatedAt:    now,
	}
	_, err := s.adminRepo.Collection.UpdateOne(ctx,
		bson.D{{Key: "user_id", Value: userID}, {Key: "restaurant_id", Value: restaurantID}},
		bson.D{{Key: "$setOnInsert", Value: row}},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("AuthService.upsertAdminUser: %w", err)
	}
	return nil
}
