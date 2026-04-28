package controller

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"

	reviewSvc "restaurantsaas/internal/apps/reviews/service"
	"restaurantsaas/internal/middleware"
)

type ReviewController struct {
	svc reviewSvc.ReviewService
}

func New(svc reviewSvc.ReviewService) *ReviewController {
	return &ReviewController{svc: svc}
}

// RegisterMeRoutes attaches the customer-write routes (POST /me/reviews).
func (ctl *ReviewController) RegisterMeRoutes(r fiber.Router) {
	r.Post("/reviews", ctl.Create)
}

// RegisterTenantRoutes attaches the public read route under /api/r/:restaurant_id.
// The tenant-resolver middleware has already populated middleware.LocalRestaurantID.
func (ctl *ReviewController) RegisterTenantRoutes(r fiber.Router) {
	r.Get("/", ctl.List)
}

func (ctl *ReviewController) Create(c *fiber.Ctx) error {
	uid, err := customerObjectID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}
	var req reviewSvc.CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	rv, err := ctl.svc.Create(c.UserContext(), uid, &req)
	if err != nil {
		switch {
		case errors.Is(err, reviewSvc.ErrInvalidRatingValue),
			errors.Is(err, reviewSvc.ErrOrderNotEligible):
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		case errors.Is(err, reviewSvc.ErrOrderNotFound):
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		case errors.Is(err, reviewSvc.ErrOrderNotOwned):
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
		case errors.Is(err, reviewSvc.ErrAlreadyReviewed):
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}
	return c.Status(fiber.StatusCreated).JSON(rv)
}

func (ctl *ReviewController) List(c *fiber.Ctx) error {
	rid := middleware.TenantIDFromCtx(c)
	if rid.IsZero() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "restaurant context missing"})
	}
	limit, _ := strconv.ParseInt(c.Query("limit", "20"), 10, 64)
	offset, _ := strconv.ParseInt(c.Query("offset", "0"), 10, 64)
	rows, err := ctl.svc.ListForRestaurant(c.UserContext(), rid, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"reviews": rows})
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
