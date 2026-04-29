package controller

import (
	"time"

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
	// BACKEND_REQUIREMENTS.md §3 expects {is_open, next_open_at,
	// manual_closed}. We compute is_open from the current weekday's
	// opening_hours window in the restaurant's timezone (falls back
	// to UTC when timezone is empty).  The legacy fields are kept on
	// the response for the existing customer/admin Flutter clients
	// that already consume them.
	isOpen, nextOpenAt := computeOpenStatus(r)
	return c.JSON(fiber.Map{
		"is_open":       isOpen,
		"next_open_at":  nextOpenAt,
		"manual_closed": r.ManualClosed,
		"opening_hours": r.OpeningHours,
		"timezone":      r.Timezone,
		"currency":      r.Currency,
		"name":          r.Name,
		"id":            r.ID.Hex(),
	})
}

// computeOpenStatus returns whether the restaurant is currently
// accepting orders and, when closed, the next ISO timestamp at which
// it'll re-open.  Manual close always wins; otherwise we look up the
// row matching today's weekday in opening_hours.  When the timezone
// or hours are missing we conservatively report open.
func computeOpenStatus(r *model.Restaurant) (bool, *time.Time) {
	if r.ManualClosed {
		return false, nil
	}
	loc := time.UTC
	if r.Timezone != "" {
		if tz, err := time.LoadLocation(r.Timezone); err == nil {
			loc = tz
		}
	}
	now := time.Now().In(loc)
	keys := []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"}
	day := keys[int(now.Weekday())]
	hrs, ok := r.OpeningHours[day]
	if !ok || hrs.Closed {
		return false, nextOpenForward(r, now, loc, keys)
	}
	open, openErr := parseClock(hrs.Open, now, loc)
	close, closeErr := parseClock(hrs.Close, now, loc)
	if openErr != nil || closeErr != nil {
		return true, nil
	}
	if !now.Before(open) && now.Before(close) {
		return true, nil
	}
	if now.Before(open) {
		return false, &open
	}
	return false, nextOpenForward(r, now, loc, keys)
}

// nextOpenForward walks the next 7 days and returns the first
// non-closed open timestamp.
func nextOpenForward(r *model.Restaurant, from time.Time, loc *time.Location, keys []string) *time.Time {
	for i := 1; i <= 7; i++ {
		day := from.AddDate(0, 0, i)
		hrs, ok := r.OpeningHours[keys[int(day.Weekday())]]
		if !ok || hrs.Closed {
			continue
		}
		open, err := parseClock(hrs.Open, day, loc)
		if err != nil {
			continue
		}
		return &open
	}
	return nil
}

// parseClock combines an "HH:MM" string with the date portion of
// `day` to produce a localised time.
func parseClock(hm string, day time.Time, loc *time.Location) (time.Time, error) {
	t, err := time.ParseInLocation("15:04", hm, loc)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(day.Year(), day.Month(), day.Day(), t.Hour(), t.Minute(), 0, 0, loc), nil
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
