package controller

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"

	tasteSvc "restaurantsaas/internal/apps/taste/service"
	"restaurantsaas/internal/middleware"
)

type TasteController struct {
	svc tasteSvc.TasteService
}

func New(svc tasteSvc.TasteService) *TasteController {
	return &TasteController{svc: svc}
}

func (ctl *TasteController) RegisterMeRoutes(r fiber.Router) {
	r.Get("/taste-profile", ctl.Get)
	r.Put("/taste-profile", ctl.Update)
}

func (ctl *TasteController) Get(c *fiber.Ctx) error {
	uid, err := userObjectID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}
	row, err := ctl.svc.Get(c.UserContext(), uid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(row)
}

func (ctl *TasteController) Update(c *fiber.Ctx) error {
	uid, err := userObjectID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}
	var req tasteSvc.UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	if err := req.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	row, err := ctl.svc.Update(c.UserContext(), uid, &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(row)
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
