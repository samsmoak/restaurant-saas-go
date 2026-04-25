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
	r.Get("/addresses", ctl.ListAddresses)
	r.Post("/addresses", ctl.AddAddress)
	r.Delete("/addresses/:id", ctl.RemoveAddress)
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

func (ctl *UserController) ListAddresses(c *fiber.Ctx) error {
	uid, _ := c.Locals("user_id").(string)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not signed in"})
	}
	addrs, err := ctl.svc.ListAddresses(c.UserContext(), uid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"addresses": addrs})
}

func (ctl *UserController) AddAddress(c *fiber.Ctx) error {
	uid, _ := c.Locals("user_id").(string)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not signed in"})
	}
	var req userSvc.AddressRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	if err := req.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	addr, err := ctl.svc.AddAddress(c.UserContext(), uid, &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(addr)
}

func (ctl *UserController) RemoveAddress(c *fiber.Ctx) error {
	uid, _ := c.Locals("user_id").(string)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not signed in"})
	}
	addrID := c.Params("id")
	if addrID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "address id is required"})
	}
	if err := ctl.svc.RemoveAddress(c.UserContext(), uid, addrID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
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
