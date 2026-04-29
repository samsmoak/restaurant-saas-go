package controller

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"

	groupSvc "restaurantsaas/internal/apps/groups/service"
	orderSvc "restaurantsaas/internal/apps/order/service"
	"restaurantsaas/internal/middleware"
)

type GroupController struct {
	svc groupSvc.GroupService
}

func New(svc groupSvc.GroupService) *GroupController {
	return &GroupController{svc: svc}
}

// RegisterRoutes wires /api/groups/* under a JWT-protected group.
func (ctl *GroupController) RegisterRoutes(r fiber.Router) {
	r.Post("/", ctl.Create)
	r.Get("/:share_code", ctl.Get)
	r.Post("/:share_code/join", ctl.Join)
	r.Post("/:share_code/lock", ctl.Lock)
	r.Post("/:share_code/checkout", ctl.Checkout)
}

type createReq struct {
	RestaurantID string `json:"restaurant_id"`
}

func (ctl *GroupController) Create(c *fiber.Ctx) error {
	uid, err := userObjectID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}
	var req createReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	rid, err := primitive.ObjectIDFromHex(req.RestaurantID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid restaurant_id"})
	}
	out, err := ctl.svc.Create(c.UserContext(), uid, rid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(out)
}

func (ctl *GroupController) Get(c *fiber.Ctx) error {
	if _, err := userObjectID(c); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}
	view, err := ctl.svc.GetByShareCode(c.UserContext(), c.Params("share_code"))
	if err != nil {
		if errors.Is(err, groupSvc.ErrGroupMissing) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(view)
}

func (ctl *GroupController) Join(c *fiber.Ctx) error {
	uid, err := userObjectID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}
	if err := ctl.svc.Join(c.UserContext(), "", "", c.Params("share_code"), uid); err != nil {
		if errors.Is(err, groupSvc.ErrGroupMissing) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		if errors.Is(err, groupSvc.ErrLocked) {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

type lockReq struct {
	LockMinutes int `json:"lock_minutes"`
}

func (ctl *GroupController) Lock(c *fiber.Ctx) error {
	uid, err := userObjectID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}
	var req lockReq
	_ = c.BodyParser(&req)
	g, err := ctl.svc.Lock(c.UserContext(), c.Params("share_code"), uid, req.LockMinutes)
	if err != nil {
		if errors.Is(err, groupSvc.ErrNotHost) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
		}
		if errors.Is(err, groupSvc.ErrGroupMissing) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(g)
}

func (ctl *GroupController) Checkout(c *fiber.Ctx) error {
	uid, err := userObjectID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}
	email := middleware.EmailFromCtx(c)
	result, err := ctl.svc.Checkout(c.UserContext(), c.Params("share_code"), uid, email)
	if err != nil {
		if errors.Is(err, groupSvc.ErrNotHost) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
		}
		if errors.Is(err, groupSvc.ErrGroupMissing) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		if errors.Is(err, groupSvc.ErrNotLocked) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		var he *orderSvc.HTTPError
		if errors.As(err, &he) {
			return c.Status(he.Status).JSON(fiber.Map{"error": he.Message})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{
		"order_draft_id":               result.Order.ID.Hex(),
		"payment_intent_client_secret": "", // payment intent flow happens via separate Stripe call when host pays
		"summary":                      result.Summary,
	})
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
