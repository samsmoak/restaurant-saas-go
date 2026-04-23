package controller

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"

	authModel "restaurantsaas/internal/apps/auth/model"
	authSvc "restaurantsaas/internal/apps/auth/service"
	"restaurantsaas/internal/middleware"
)

type AuthController struct {
	svc authSvc.AuthService
}

func New(svc authSvc.AuthService) *AuthController {
	return &AuthController{svc: svc}
}

func (ctl *AuthController) RegisterRoutes(r fiber.Router, jwtAuth fiber.Handler) {
	// Public
	r.Post("/signup/customer", ctl.SignupCustomer)
	r.Post("/login", ctl.Login)
	r.Post("/google", ctl.Google)
	r.Post("/signout", ctl.Signout)
	// Authenticated
	r.Post("/admin/finalize", jwtAuth, ctl.AdminFinalize)
	r.Post("/admin/activate", jwtAuth, ctl.ActivateAdmin)
	r.Get("/memberships", jwtAuth, ctl.ListMemberships)
}

func (ctl *AuthController) SignupCustomer(c *fiber.Ctx) error {
	var req authModel.SignupRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	if err := req.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	resp, err := ctl.svc.SignupCustomer(c.UserContext(), &req)
	if err != nil {
		if errors.Is(err, authSvc.ErrEmailTaken) {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "email already registered"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(resp)
}

func (ctl *AuthController) Login(c *fiber.Ctx) error {
	var req authModel.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	if err := req.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	resp, err := ctl.svc.Login(c.UserContext(), &req)
	if err != nil {
		if errors.Is(err, authSvc.ErrInvalidCredentials) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(resp)
}

func (ctl *AuthController) Google(c *fiber.Ctx) error {
	var req authModel.GoogleAuthRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	if err := req.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	resp, err := ctl.svc.GoogleSignIn(c.UserContext(), &req)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(resp)
}

func (ctl *AuthController) AdminFinalize(c *fiber.Ctx) error {
	uid := middleware.UserIDFromCtx(c)
	email := middleware.EmailFromCtx(c)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not signed in"})
	}
	var req authModel.AdminFinalizeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	if err := req.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	result, err := ctl.svc.AdminFinalize(c.UserContext(), uid, email, req.InviteCode)
	if err != nil {
		if errors.Is(err, authSvc.ErrInviteInvalid) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid or already-used invite code"})
		}
		if errors.Is(err, authSvc.ErrInviteEmailMismatch) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "this invite is tied to a different email"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

func (ctl *AuthController) ActivateAdmin(c *fiber.Ctx) error {
	uid := middleware.UserIDFromCtx(c)
	email := middleware.EmailFromCtx(c)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not signed in"})
	}
	var body struct {
		RestaurantID string `json:"restaurant_id"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	if body.RestaurantID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "restaurant_id is required"})
	}
	resp, err := ctl.svc.ActivateAdmin(c.UserContext(), uid, email, body.RestaurantID)
	if err != nil {
		if errors.Is(err, authSvc.ErrNotAdmin) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(resp)
}

func (ctl *AuthController) ListMemberships(c *fiber.Ctx) error {
	uid := middleware.UserIDFromCtx(c)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not signed in"})
	}
	oid, err := primitive.ObjectIDFromHex(uid)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
	}
	ms, err := ctl.svc.ListMemberships(c.UserContext(), oid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"memberships": ms})
}

func (ctl *AuthController) Signout(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusNoContent)
}
