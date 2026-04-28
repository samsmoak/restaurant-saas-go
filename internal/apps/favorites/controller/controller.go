package controller

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"

	favoriteSvc "restaurantsaas/internal/apps/favorites/service"
	"restaurantsaas/internal/middleware"
)

type FavoriteController struct {
	svc favoriteSvc.FavoriteService
}

func New(svc favoriteSvc.FavoriteService) *FavoriteController {
	return &FavoriteController{svc: svc}
}

func (ctl *FavoriteController) RegisterMeRoutes(r fiber.Router) {
	r.Get("/favorites", ctl.List)
	r.Post("/favorites", ctl.Add)
	r.Delete("/favorites/:restaurant_id", ctl.Remove)
}

type addRequest struct {
	RestaurantID string `json:"restaurant_id"`
}

func (ctl *FavoriteController) Add(c *fiber.Ctx) error {
	uid, err := customerObjectID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}
	var req addRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	rid, err := primitive.ObjectIDFromHex(req.RestaurantID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid restaurant_id"})
	}
	if err := ctl.svc.Add(c.UserContext(), uid, rid); err != nil {
		if errors.Is(err, favoriteSvc.ErrRestaurantNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (ctl *FavoriteController) Remove(c *fiber.Ctx) error {
	uid, err := customerObjectID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}
	rid, err := primitive.ObjectIDFromHex(c.Params("restaurant_id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid restaurant_id"})
	}
	if err := ctl.svc.Remove(c.UserContext(), uid, rid); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (ctl *FavoriteController) List(c *fiber.Ctx) error {
	uid, err := customerObjectID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}
	out, err := ctl.svc.List(c.UserContext(), uid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"favorites": out})
}

func customerObjectID(c *fiber.Ctx) (primitive.ObjectID, error) {
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
