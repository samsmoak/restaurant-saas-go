package controller

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"

	notifSvc "restaurantsaas/internal/apps/notifications/service"
	"restaurantsaas/internal/middleware"
)

type NotificationController struct {
	svc notifSvc.NotificationService
}

func New(svc notifSvc.NotificationService) *NotificationController {
	return &NotificationController{svc: svc}
}

func (ctl *NotificationController) RegisterMeRoutes(r fiber.Router) {
	r.Get("/notifications", ctl.List)
	r.Post("/notifications/read", ctl.MarkRead)
}

func (ctl *NotificationController) List(c *fiber.Ctx) error {
	uid, err := userObjectID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}
	rows, err := ctl.svc.List(c.UserContext(), uid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"notifications": rows})
}

type markReadRequest struct {
	IDs []string `json:"ids"`
	All bool     `json:"all"`
}

func (ctl *NotificationController) MarkRead(c *fiber.Ctx) error {
	uid, err := userObjectID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}
	var req markReadRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	ids := make([]primitive.ObjectID, 0, len(req.IDs))
	for _, raw := range req.IDs {
		oid, err := primitive.ObjectIDFromHex(raw)
		if err != nil {
			continue
		}
		ids = append(ids, oid)
	}
	if err := ctl.svc.MarkRead(c.UserContext(), uid, ids, req.All); err != nil {
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
