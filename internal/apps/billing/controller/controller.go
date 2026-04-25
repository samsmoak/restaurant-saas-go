package controller

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	billingSvc "restaurantsaas/internal/apps/billing/service"
	"restaurantsaas/internal/middleware"
)

type BillingController struct {
	svc billingSvc.BillingService
}

func New(svc billingSvc.BillingService) *BillingController {
	return &BillingController{svc: svc}
}

func (ctl *BillingController) RegisterRoutes(r fiber.Router) {
	r.Get("/subscription", ctl.GetSubscription)
	r.Post("/checkout/setup", ctl.CreateSetupCheckout)
	r.Post("/checkout/subscription", ctl.CreateSubscriptionCheckout)
	r.Post("/portal", ctl.CreatePortal)
}

type returnURLBody struct {
	ReturnURL string `json:"return_url"`
}

func (ctl *BillingController) GetSubscription(c *fiber.Ctx) error {
	rid := middleware.TenantIDFromCtx(c)
	if rid.IsZero() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "restaurant context missing"})
	}
	b, err := ctl.svc.GetStatus(c.UserContext(), rid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	var periodEnd interface{}
	if b.CurrentPeriodEnd != nil {
		periodEnd = b.CurrentPeriodEnd.Format("2006-01-02T15:04:05Z")
	}
	return c.JSON(fiber.Map{
		"setup_fee_paid":      b.SetupFeePaid,
		"subscription_status": b.SubscriptionStatus,
		"current_period_end":  periodEnd,
	})
}

func (ctl *BillingController) CreateSetupCheckout(c *fiber.Ctx) error {
	rid := middleware.TenantIDFromCtx(c)
	if rid.IsZero() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "restaurant context missing"})
	}
	var req returnURLBody
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	url, err := ctl.svc.CreateSetupCheckout(c.UserContext(), rid, req.ReturnURL)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"url": url})
}

func (ctl *BillingController) CreateSubscriptionCheckout(c *fiber.Ctx) error {
	rid := middleware.TenantIDFromCtx(c)
	if rid.IsZero() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "restaurant context missing"})
	}
	var req returnURLBody
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	url, err := ctl.svc.CreateSubscriptionCheckout(c.UserContext(), rid, req.ReturnURL)
	if err != nil {
		if errors.Is(err, billingSvc.ErrSetupFeeRequired) {
			return c.Status(fiber.StatusPaymentRequired).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"url": url})
}

func (ctl *BillingController) CreatePortal(c *fiber.Ctx) error {
	rid := middleware.TenantIDFromCtx(c)
	if rid.IsZero() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "restaurant context missing"})
	}
	var req returnURLBody
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	url, err := ctl.svc.CreatePortal(c.UserContext(), rid, req.ReturnURL)
	if err != nil {
		if errors.Is(err, billingSvc.ErrNoCustomer) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"url": url})
}

func (ctl *BillingController) Webhook(c *fiber.Ctx) error {
	sig := c.Get("Stripe-Signature")
	if sig == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing signature."})
	}
	raw := c.Body()
	if err := ctl.svc.HandleWebhook(c.UserContext(), raw, sig); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"received": true})
}
