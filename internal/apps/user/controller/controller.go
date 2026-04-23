package controller

import (
	"github.com/gofiber/fiber/v2"

	userSvc "restaurantsaas/internal/apps/user/service"
)

type UserController struct {
	svc userSvc.UserService
}

func New(svc userSvc.UserService) *UserController {
	return &UserController{svc: svc}
}

func (ctl *UserController) RegisterMeRoutes(r fiber.Router) {
	r.Get("/profile", ctl.GetProfile)
	r.Put("/profile", ctl.UpdateProfile)
}

func (ctl *UserController) GetProfile(c *fiber.Ctx) error {
	uid, _ := c.Locals("user_id").(string)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not signed in"})
	}
	p, err := ctl.svc.GetProfile(c.UserContext(), uid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if p == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "profile not found"})
	}
	return c.JSON(p)
}

func (ctl *UserController) UpdateProfile(c *fiber.Ctx) error {
	uid, _ := c.Locals("user_id").(string)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not signed in"})
	}
	var req userSvc.ProfileUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	if err := req.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	p, err := ctl.svc.UpdateProfile(c.UserContext(), uid, &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if p == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "profile not found"})
	}
	return c.JSON(p)
}
