package controller

import (
	"errors"
	"log"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"

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

func (ctl *PaymentController) RegisterMeRoutes(r fiber.Router) {
	r.Get("/payment-methods", ctl.ListPaymentMethods)
	r.Post("/payment-methods/setup-intent", ctl.CreateSetupIntent)
	r.Delete("/payment-methods/:id", ctl.DetachPaymentMethod)
}

type createIntentBody struct {
	orderSvc.CheckoutRequest
	SavePaymentMethod bool   `json:"save_payment_method"`
	PaymentMethodID   string `json:"payment_method_id"`
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
	var req createIntentBody
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	result, err := ctl.orders.ValidateAndBuildOrder(c.UserContext(), rid, uid, email, &req.CheckoutRequest)
	if err != nil {
		var he *orderSvc.HTTPError
		if errors.As(err, &he) {
			return c.Status(he.Status).JSON(fiber.Map{"error": he.Message})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var opts *paymentSvc.CreateIntentOptions
	if req.SavePaymentMethod || req.PaymentMethodID != "" {
		userOID, _ := primitive.ObjectIDFromHex(uid)
		stripeCustomerID, err := ctl.payments.EnsureStripeCustomer(c.UserContext(), userOID, email, req.CustomerName)
		if err != nil {
			log.Printf("PaymentController.CreateIntent: ensure stripe customer: %v", err)
		}
		opts = &paymentSvc.CreateIntentOptions{
			StripeCustomerID:  stripeCustomerID,
			PaymentMethodID:   req.PaymentMethodID,
			SavePaymentMethod: req.SavePaymentMethod,
		}
	}

	intentID, clientSecret, err := ctl.payments.CreateIntentForOrder(c.UserContext(), result, opts)
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

func (ctl *PaymentController) ListPaymentMethods(c *fiber.Ctx) error {
	uid := middleware.UserIDFromCtx(c)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not signed in"})
	}
	userOID, err := primitive.ObjectIDFromHex(uid)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
	}
	stripeCustomerID, err := ctl.payments.GetStripeCustomerID(c.UserContext(), userOID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if stripeCustomerID == "" {
		return c.JSON(fiber.Map{"payment_methods": []struct{}{}})
	}
	pms, err := ctl.payments.ListPaymentMethods(c.UserContext(), stripeCustomerID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"payment_methods": pms})
}

func (ctl *PaymentController) CreateSetupIntent(c *fiber.Ctx) error {
	uid := middleware.UserIDFromCtx(c)
	email := middleware.EmailFromCtx(c)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not signed in"})
	}
	userOID, err := primitive.ObjectIDFromHex(uid)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
	}
	stripeCustomerID, err := ctl.payments.EnsureStripeCustomer(c.UserContext(), userOID, email, "")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	clientSecret, err := ctl.payments.CreateSetupIntent(c.UserContext(), stripeCustomerID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"client_secret": clientSecret})
}

func (ctl *PaymentController) DetachPaymentMethod(c *fiber.Ctx) error {
	uid := middleware.UserIDFromCtx(c)
	if uid == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not signed in"})
	}
	pmID := c.Params("id")
	if pmID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "payment method id is required"})
	}
	if err := ctl.payments.DetachPaymentMethod(c.UserContext(), pmID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
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
