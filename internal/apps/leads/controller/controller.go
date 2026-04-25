package controller

import (
	"github.com/gofiber/fiber/v2"

	leadsSvc "restaurantsaas/internal/apps/leads/service"
)

type LeadsController struct {
	svc leadsSvc.LeadsService
}

func New(svc leadsSvc.LeadsService) *LeadsController {
	return &LeadsController{svc: svc}
}

func (ctl *LeadsController) RegisterRoutes(r fiber.Router) {
	r.Post("/leads", ctl.Submit)
}

func (ctl *LeadsController) Submit(c *fiber.Ctx) error {
	var req leadsSvc.LeadRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	if err := req.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if _, err := ctl.svc.Submit(c.UserContext(), &req); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"success": true})
}
