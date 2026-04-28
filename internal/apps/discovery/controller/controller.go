package controller

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"restaurantsaas/internal/apps/discovery/model"
	discoverySvc "restaurantsaas/internal/apps/discovery/service"
)

type DiscoveryController struct {
	svc discoverySvc.DiscoveryService
}

func New(svc discoverySvc.DiscoveryService) *DiscoveryController {
	return &DiscoveryController{svc: svc}
}

// RegisterRoutes wires the discovery routes onto the existing
// /api/restaurants group. The static /search and /search/suggest paths must
// be registered BEFORE any /:restaurant_id route so Fiber's tree matches
// them first.
func (ctl *DiscoveryController) RegisterRoutes(r fiber.Router) {
	r.Get("/search/suggest", ctl.Suggest)
	r.Get("/search", ctl.Search)
	// `Get("/")` is the bare-list endpoint. The existing restaurant controller
	// already owns POST "/" (create) and GET "/:restaurant_id". Fiber matches
	// "/" exactly, so this does not collide.
	r.Get("/", ctl.List)
}

func parseListParams(c *fiber.Ctx) model.ListParams {
	p := model.ListParams{
		Q:       c.Query("q"),
		Cuisine: c.Query("cuisine"),
	}
	if v, err := strconv.ParseFloat(c.Query("lat"), 64); err == nil {
		p.Lat = &v
	}
	if v, err := strconv.ParseFloat(c.Query("lng"), 64); err == nil {
		p.Lng = &v
	}
	if v, err := strconv.Atoi(c.Query("limit")); err == nil {
		p.Limit = v
	}
	if v, err := strconv.Atoi(c.Query("offset")); err == nil {
		p.Offset = v
	}
	return p
}

func (ctl *DiscoveryController) List(c *fiber.Ctx) error {
	p := parseListParams(c)
	results, total, err := ctl.svc.List(c.UserContext(), p)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{
		"restaurants": results,
		"total":       total,
		"limit":       clampedLimit(p.Limit),
		"offset":      maxInt(p.Offset, 0),
	})
}

func (ctl *DiscoveryController) Search(c *fiber.Ctx) error {
	p := parseListParams(c)
	results, total, err := ctl.svc.Search(c.UserContext(), p)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{
		"restaurants": results,
		"total":       total,
		"limit":       clampedLimit(p.Limit),
		"offset":      maxInt(p.Offset, 0),
	})
}

func (ctl *DiscoveryController) Suggest(c *fiber.Ctx) error {
	q := c.Query("q")
	suggestions, err := ctl.svc.Suggest(c.UserContext(), q)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"suggestions": suggestions})
}

func clampedLimit(n int) int {
	if n <= 0 {
		return 25
	}
	if n > 50 {
		return 50
	}
	return n
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
