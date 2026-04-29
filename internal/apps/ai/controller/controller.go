package controller

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
	"go.mongodb.org/mongo-driver/bson/primitive"

	aiSvc "restaurantsaas/internal/apps/ai/service"
	"restaurantsaas/internal/middleware"
)

type AIController struct {
	svc aiSvc.AIService
}

func New(svc aiSvc.AIService) *AIController {
	return &AIController{svc: svc}
}

// Search is the legacy POST /api/ai/search handler — returns
// {intent, restaurants}. The Savorar client uses the GET variant
// (SearchDishes) which returns {dishes:[]}.
func (ctl *AIController) Search(c *fiber.Ctx) error {
	var req aiSvc.SearchRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	resp, err := ctl.svc.Search(c.UserContext(), &req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(resp)
}

// SearchDishes is GET /api/ai/search?q=&spicy=&citrus=&under_15=&under_30_min=&rating_4_5_plus=
// per BACKEND_REQUIREMENTS.md §7.  Returns {dishes:[]}.
func (ctl *AIController) SearchDishes(c *fiber.Ctx) error {
	q := c.Query("q")
	filter := aiSvc.DishFilter{
		Spicy:        truthy(c.Query("spicy")),
		Citrus:       truthy(c.Query("citrus")),
		Under15:      truthy(c.Query("under_15")),
		Under30Min:   truthy(c.Query("under_30_min")),
		Rating45Plus: truthy(c.Query("rating_4_5_plus")),
	}
	lat := parseLatLng(c.Query("lat"))
	lng := parseLatLng(c.Query("lng"))
	dishes := ctl.svc.ListDishes(c.UserContext(), q, lat, lng, nil, filter, 25)
	if dishes == nil {
		dishes = []*aiSvc.Dish{}
	}
	return c.JSON(fiber.Map{"dishes": dishes})
}

// Recommend is POST /api/ai/recommend (BACKEND_REQUIREMENTS.md §7).
func (ctl *AIController) Recommend(c *fiber.Ctx) error {
	var req struct {
		Taste    *aiSvc.TasteFingerprint `json:"taste"`
		Location *struct {
			Lat float64 `json:"lat"`
			Lng float64 `json:"lng"`
		} `json:"location"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	var lat, lng *float64
	if req.Location != nil {
		la, ln := req.Location.Lat, req.Location.Lng
		lat, lng = &la, &ln
	}
	dishes := ctl.svc.Recommend(c.UserContext(), req.Taste, lat, lng)
	if dishes == nil {
		dishes = []*aiSvc.Dish{}
	}
	return c.JSON(fiber.Map{"dishes": dishes})
}

// DishByID is GET /api/ai/dishes/:id (BACKEND_REQUIREMENTS.md §7).
func (ctl *AIController) DishByID(c *fiber.Ctx) error {
	idHex := c.Params("id")
	oid, err := primitive.ObjectIDFromHex(idHex)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	dish, err := ctl.svc.GetDishByID(c.UserContext(), oid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if dish == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "dish not found"})
	}
	return c.JSON(dish)
}

// Chat streams Server-Sent Events.  Each event is a single
//
//	data: <json>\n\n
//
// frame; the client picks the type apart on receipt
// (BACKEND_REQUIREMENTS.md §7).
func (ctl *AIController) Chat(c *fiber.Ctx) error {
	var req aiSvc.StreamChatRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	uid := middleware.UserIDFromCtx(c)
	userOID, _ := primitive.ObjectIDFromHex(uid)
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")
	c.Status(fiber.StatusOK)

	ctx := c.UserContext()
	c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
		ctl.svc.StreamChat(ctx, userOID, &req, func(ev any) {
			b, err := json.Marshal(ev)
			if err != nil {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", b)
			_ = w.Flush()
		})
	}))
	return nil
}

func truthy(s string) bool {
	switch s {
	case "1", "true", "TRUE", "True", "yes", "y":
		return true
	}
	return false
}

func parseLatLng(s string) *float64 {
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}
