package controller

import (
	"errors"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"

	orderModel "restaurantsaas/internal/apps/order/model"
	orderSvc "restaurantsaas/internal/apps/order/service"
	"restaurantsaas/internal/middleware"
)

type OrderController struct {
	svc orderSvc.OrderService
}

func New(svc orderSvc.OrderService) *OrderController {
	return &OrderController{svc: svc}
}

func (ctl *OrderController) RegisterPublicRoutes(r fiber.Router) {
	r.Get("/:order_number", ctl.GetByNumber)
}

func (ctl *OrderController) RegisterMeRoutes(r fiber.Router) {
	r.Get("/orders", ctl.ListMine)
}

func (ctl *OrderController) RegisterAdminRoutes(r fiber.Router) {
	r.Get("/", ctl.ListAdmin)
	r.Get("/analytics", ctl.Analytics)
	r.Put("/:id", ctl.UpdateStatus)
	r.Delete("/:id", ctl.Delete)
}

func (ctl *OrderController) GetByNumber(c *fiber.Ctx) error {
	n := c.Params("order_number")
	order, err := ctl.svc.GetByNumberPublic(c.UserContext(), n)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if order == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "order not found"})
	}
	return c.JSON(order)
}

func (ctl *OrderController) ListMine(c *fiber.Ctx) error {
	uid := middleware.UserIDFromCtx(c)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not signed in"})
	}
	var restrictTo *primitive.ObjectID
	if q := c.Query("restaurant_id"); q != "" {
		oid, err := primitive.ObjectIDFromHex(q)
		if err == nil {
			restrictTo = &oid
		}
	}
	rows, err := ctl.svc.ListForCustomerPublic(c.UserContext(), uid, restrictTo)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if rows == nil {
		rows = []*orderModel.OrderPublicView{}
	}
	return c.JSON(fiber.Map{"orders": rows})
}

func (ctl *OrderController) ListAdmin(c *fiber.Ctx) error {
	rid := middleware.TenantIDFromCtx(c)
	status := c.Query("status", "all")
	if status != "all" && !orderModel.IsValidStatus(status) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid status"})
	}
	rows, err := ctl.svc.ListAdmin(c.UserContext(), rid, status)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if rows == nil {
		rows = []*orderModel.Order{}
	}
	return c.JSON(fiber.Map{"orders": rows})
}

// Analytics returns orders within a time window for client-side aggregation.
// Accepts ?from=<unix_ms>&to=<unix_ms>. Missing bounds = no bound.
func (ctl *OrderController) Analytics(c *fiber.Ctx) error {
	rid := middleware.TenantIDFromCtx(c)
	from := parseUnixMsParam(c.Query("from"))
	to := parseUnixMsParam(c.Query("to"))
	rows, err := ctl.svc.ListBetween(c.UserContext(), rid, from, to)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if rows == nil {
		rows = []*orderModel.Order{}
	}
	return c.JSON(fiber.Map{"orders": rows})
}

func (ctl *OrderController) UpdateStatus(c *fiber.Ctx) error {
	rid := middleware.TenantIDFromCtx(c)
	id := c.Params("id")
	var req orderSvc.UpdateStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	row, err := ctl.svc.UpdateStatus(c.UserContext(), rid, id, &req)
	if err != nil {
		var he *orderSvc.HTTPError
		if errors.As(err, &he) {
			return c.Status(he.Status).JSON(fiber.Map{"error": he.Message})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if row == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "order not found"})
	}
	return c.JSON(row)
}

func (ctl *OrderController) Delete(c *fiber.Ctx) error {
	rid := middleware.TenantIDFromCtx(c)
	id := c.Params("id")
	if err := ctl.svc.Delete(c.UserContext(), rid, id); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func parseUnixMsParam(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(n).UTC()
}
