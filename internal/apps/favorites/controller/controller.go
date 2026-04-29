package controller

import (
	"context"
	"errors"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"

	favoriteSvc "restaurantsaas/internal/apps/favorites/service"
	"restaurantsaas/internal/middleware"
)

// DishHydrator hydrates a list of menu_item ObjectIDs into the
// AI-shaped Dish payload.  Implemented by aiService — passed in via
// New() so favorites stays import-free of the AI app.  When nil, the
// dishes[] arm of the favorites response is just an empty array.
type DishHydrator func(ctx context.Context, ids []primitive.ObjectID) []any

type FavoriteController struct {
	svc      favoriteSvc.FavoriteService
	hydrator DishHydrator
}

func New(svc favoriteSvc.FavoriteService) *FavoriteController {
	return &FavoriteController{svc: svc}
}

// SetDishHydrator wires the AI-side hydrator after both controllers
// are constructed (avoids a circular import).
func (ctl *FavoriteController) SetDishHydrator(h DishHydrator) {
	ctl.hydrator = h
}

func (ctl *FavoriteController) RegisterMeRoutes(r fiber.Router) {
	r.Get("/favorites", ctl.List)
	// Restaurant favorites
	r.Post("/favorites", ctl.Add)                            // legacy body shape
	r.Put("/favorites/:restaurant_id", ctl.AddByPath)        // spec shape
	r.Delete("/favorites/:restaurant_id", ctl.Remove)
	// Dish favorites (Savorar §6 dishes branch)
	r.Put("/favorites/dishes/:dish_id", ctl.AddDish)
	r.Delete("/favorites/dishes/:dish_id", ctl.RemoveDish)
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

// AddByPath is the spec-shaped PUT /api/me/favorites/{restaurantId}.
// Returns 204 on success per BACKEND_REQUIREMENTS.md §6.
func (ctl *FavoriteController) AddByPath(c *fiber.Ctx) error {
	uid, err := customerObjectID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}
	rid, err := primitive.ObjectIDFromHex(c.Params("restaurant_id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid restaurant_id"})
	}
	if err := ctl.svc.Add(c.UserContext(), uid, rid); err != nil {
		if errors.Is(err, favoriteSvc.ErrRestaurantNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
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

func (ctl *FavoriteController) AddDish(c *fiber.Ctx) error {
	uid, err := customerObjectID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}
	dishOID, err := primitive.ObjectIDFromHex(c.Params("dish_id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid dish_id"})
	}
	if err := ctl.svc.AddDish(c.UserContext(), uid, dishOID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (ctl *FavoriteController) RemoveDish(c *fiber.Ctx) error {
	uid, err := customerObjectID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}
	dishOID, err := primitive.ObjectIDFromHex(c.Params("dish_id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid dish_id"})
	}
	if err := ctl.svc.RemoveDish(c.UserContext(), uid, dishOID); err != nil {
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
	dishes := []any{}
	if ctl.hydrator != nil && len(out.DishIDs) > 0 {
		dishes = ctl.hydrator(c.UserContext(), out.DishIDs)
	}
	// Emit BOTH the spec-shaped keys and the legacy `favorites` key so
	// the existing customer Flutter app keeps working.
	return c.JSON(fiber.Map{
		"restaurants": out.Restaurants,
		"dishes":      dishes,
		"favorites":   out.Restaurants,
	})
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
