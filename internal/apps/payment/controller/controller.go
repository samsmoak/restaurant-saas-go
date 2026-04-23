package controller

import (
	"errors"
	"log"

	"github.com/gofiber/fiber/v2"

	orderSvc "restaurantsaas/internal/apps/order/service"
	paymentSvc "restaurantsaas/internal/apps/payment/service"
	"restaurantsaas/internal/middleware"
)

type PaymentController struct {
	orders   orderSvc.OrderService
	payments paymentSvc.PaymentService
}

func New(orders orderSvc.OrderService, payments paymentSvc.PaymentService) *PaymentController {
	return &PaymentController{orders: orders, payments: payments}
}

func (ctl *PaymentController) RegisterCheckoutRoutes(r fiber.Router) {
	r.Post("/create-intent", ctl.CreateIntent)
}

func (ctl *PaymentController) CreateIntent(c *fiber.Ctx) error {
	uid := middleware.UserIDFromCtx(c)
	email := middleware.EmailFromCtx(c)
	rid := middleware.TenantIDFromCtx(c)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not signed in"})
	}
	if rid.IsZero() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "restaurant context missing"})
	}
	var req orderSvc.CheckoutRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	result, err := ctl.orders.ValidateAndBuildOrder(c.UserContext(), rid, uid, email, &req)
	if err != nil {
		var he *orderSvc.HTTPError
		if errors.As(err, &he) {
			return c.Status(he.Status).JSON(fiber.Map{"error": he.Message})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	intentID, clientSecret, err := ctl.payments.CreateIntentForOrder(c.UserContext(), result)
	if err != nil {
		_ = ctl.orders.DeleteByID(c.UserContext(), result.Order.ID)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if err := ctl.orders.AttachPaymentIntent(c.UserContext(), result.Order.ID, intentID); err != nil {
		log.Printf("PaymentController.CreateIntent: attach intent: %v", err)
	}
	result.Order.PaymentIntentID = intentID
	ctl.orders.BroadcastCreated(result.Order)

	return c.JSON(fiber.Map{
		"client_secret": clientSecret,
		"clientSecret":  clientSecret,
		"order_id":      result.Order.ID.Hex(),
		"orderId":       result.Order.ID.Hex(),
		"order_number":  result.Order.OrderNumber,
		"orderNumber":   result.Order.OrderNumber,
		"total":         result.Order.Total,
		"currency":      result.Currency,
	})
}

func (ctl *PaymentController) Webhook(c *fiber.Ctx) error {
	sig := c.Get("Stripe-Signature")
	if sig == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing signature."})
	}
	raw := c.Body()
	if err := ctl.payments.HandleWebhook(c.UserContext(), raw, sig); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"received": true})
}
