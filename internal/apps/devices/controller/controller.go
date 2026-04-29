package controller

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"

	deviceSvc "restaurantsaas/internal/apps/devices/service"
	"restaurantsaas/internal/middleware"
)

type DeviceController struct {
	svc deviceSvc.DeviceService
}

func New(svc deviceSvc.DeviceService) *DeviceController {
	return &DeviceController{svc: svc}
}

func (ctl *DeviceController) RegisterMeRoutes(r fiber.Router) {
	r.Post("/devices", ctl.Register)
	r.Delete("/devices/:token", ctl.Unregister)
}

func (ctl *DeviceController) Register(c *fiber.Ctx) error {
	uid, err := userObjectID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}
	var req deviceSvc.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	if err := req.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if err := ctl.svc.Register(c.UserContext(), uid, &req); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (ctl *DeviceController) Unregister(c *fiber.Ctx) error {
	uid, err := userObjectID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}
	token := c.Params("token")
	if token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "token is required"})
	}
	if err := ctl.svc.Unregister(c.UserContext(), uid, token); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func userObjectID(c *fiber.Ctx) (primitive.ObjectID, error) {
	raw := middleware.UserIDFromCtx(c)
	if raw == "" {
		return primitive.NilObjectID, errors.New("not signed in")
	}
	oid, err := primitive.ObjectIDFromHex(raw)
	if err != nil {
		return primitive.NilObjectID, errors.New("invalid user id")
	}
	return oid, nil
}
