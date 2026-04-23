package controller

import (
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"restaurantsaas/internal/apps/restaurant/model"
	restSvc "restaurantsaas/internal/apps/restaurant/service"
	"restaurantsaas/internal/middleware"
)

type RestaurantController struct {
	svc restSvc.RestaurantService
}

func New(svc restSvc.RestaurantService) *RestaurantController {
	return &RestaurantController{svc: svc}
}

// RegisterPublicRoutes — /api/r/:restaurant_id/restaurant (+ /status).
func (ctl *RestaurantController) RegisterPublicRoutes(r fiber.Router) {
	r.Get("/", ctl.GetPublic)
	r.Get("/status", ctl.GetStatus)
}

// RegisterTopLevelRoutes — /api/restaurants/*.
func (ctl *RestaurantController) RegisterTopLevelRoutes(r fiber.Router) {
	r.Post("/", ctl.Create)
	r.Get("/mine", ctl.ListMine)
	r.Get("/:restaurant_id", ctl.GetByID) // public, for customer env lookup
}

// RegisterAdminRoutes — /api/admin/restaurant/*.
func (ctl *RestaurantController) RegisterAdminRoutes(r fiber.Router) {
	r.Get("/", ctl.GetAdmin)
	r.Put("/", ctl.UpdateSettings)
	r.Post("/manual-closed", ctl.ToggleManualClosed)
	r.Post("/onboarding/complete-step", ctl.CompleteStep)
}

func (ctl *RestaurantController) GetPublic(c *fiber.Ctx) error {
	rid := middleware.TenantIDFromCtx(c)
	r, err := ctl.svc.GetByID(c.UserContext(), rid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if r == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "restaurant not found"})
	}
	return c.JSON(r.PublicView())
}

func (ctl *RestaurantController) GetStatus(c *fiber.Ctx) error {
	rid := middleware.TenantIDFromCtx(c)
	r, err := ctl.svc.GetByID(c.UserContext(), rid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if r == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "restaurant not found"})
	}
	return c.JSON(fiber.Map{
		"manual_closed": r.ManualClosed,
		"opening_hours": r.OpeningHours,
		"timezone":      r.Timezone,
		"currency":      r.Currency,
		"name":          r.Name,
		"id":            r.ID.Hex(),
	})
}

func (ctl *RestaurantController) GetByID(c *fiber.Ctx) error {
	raw := c.Params("restaurant_id")
	oid, err := primitive.ObjectIDFromHex(raw)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid restaurant_id"})
	}
	r, err := ctl.svc.GetByID(c.UserContext(), oid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if r == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "restaurant not found"})
	}
	return c.JSON(r.PublicView())
}

func (ctl *RestaurantController) GetAdmin(c *fiber.Ctx) error {
	rid := middleware.TenantIDFromCtx(c)
	r, err := ctl.svc.GetByID(c.UserContext(), rid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if r == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "restaurant not found"})
	}
	return c.JSON(r)
}

func (ctl *RestaurantController) Create(c *fiber.Ctx) error {
	uid := middleware.UserIDFromCtx(c)
	email := middleware.EmailFromCtx(c)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not signed in"})
	}
	ownerOID, err := primitive.ObjectIDFromHex(uid)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
	}
	var req restSvc.CreateRestaurantRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	if err := req.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	r, err := ctl.svc.Create(c.UserContext(), ownerOID, email, &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(r)
}

func (ctl *RestaurantController) ListMine(c *fiber.Ctx) error {
	uid := middleware.UserIDFromCtx(c)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not signed in"})
	}
	ownerOID, err := primitive.ObjectIDFromHex(uid)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
	}
	rows, err := ctl.svc.ListMine(c.UserContext(), ownerOID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if rows == nil {
		rows = []*model.Restaurant{}
	}
	return c.JSON(fiber.Map{"restaurants": rows})
}

func (ctl *RestaurantController) UpdateSettings(c *fiber.Ctx) error {
	rid := middleware.TenantIDFromCtx(c)
	var req restSvc.SettingsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	if err := req.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	r, err := ctl.svc.Update(c.UserContext(), rid, &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(r)
}

func (ctl *RestaurantController) ToggleManualClosed(c *fiber.Ctx) error {
	rid := middleware.TenantIDFromCtx(c)
	var body struct {
		Closed bool `json:"closed"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	r, err := ctl.svc.ToggleManualClosed(c.UserContext(), rid, body.Closed)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(r)
}

func (ctl *RestaurantController) CompleteStep(c *fiber.Ctx) error {
	rid := middleware.TenantIDFromCtx(c)
	var body struct {
		Step string `json:"step"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	r, err := ctl.svc.MarkStepComplete(c.UserContext(), rid, body.Step)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(r)
}
