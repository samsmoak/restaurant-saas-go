package controller

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"

	cravingSvc "restaurantsaas/internal/apps/cravings/service"
	"restaurantsaas/internal/middleware"
)

type CravingController struct {
	svc cravingSvc.CravingService
}

func New(svc cravingSvc.CravingService) *CravingController {
	return &CravingController{svc: svc}
}

// RegisterMeRoutes wires the cravings endpoints under /api/ai/cravings
// per BACKEND_REQUIREMENTS.md §7.  Mounted on the JWT-protected /me
// group via routes.go.
func (ctl *CravingController) RegisterAIRoutes(r fiber.Router) {
	r.Get("/cravings", ctl.List)
	r.Put("/cravings/:id/pin", ctl.Pin)
	r.Delete("/cravings/:id/pin", ctl.Unpin)
	r.Delete("/cravings/:id", ctl.Delete)
}

func (ctl *CravingController) List(c *fiber.Ctx) error {
	uid, err := userObjectID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}
	rows, err := ctl.svc.List(c.UserContext(), uid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"cravings": rows})
}

func (ctl *CravingController) Pin(c *fiber.Ctx) error {
	return ctl.setPinned(c, true)
}

func (ctl *CravingController) Unpin(c *fiber.Ctx) error {
	return ctl.setPinned(c, false)
}

func (ctl *CravingController) setPinned(c *fiber.Ctx, pinned bool) error {
	uid, err := userObjectID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	row, err := ctl.svc.Pin(c.UserContext(), uid, id, pinned)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if row == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "craving not found"})
	}
	return c.JSON(row)
}

func (ctl *CravingController) Delete(c *fiber.Ctx) error {
	uid, err := userObjectID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	if err := ctl.svc.Delete(c.UserContext(), uid, id); err != nil {
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
