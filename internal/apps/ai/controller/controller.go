package controller

import (
	"github.com/gofiber/fiber/v2"

	aiSvc "restaurantsaas/internal/apps/ai/service"
)

type AIController struct {
	svc aiSvc.AIService
}

func New(svc aiSvc.AIService) *AIController {
	return &AIController{svc: svc}
}

// RegisterRoutes wires both AI endpoints onto the /api/ai group.
// /search is public (OptionalJWTAuth applied at the parent group);
// /chat is JWT-only and the caller is expected to mount it under JWTAuth().
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

func (ctl *AIController) Chat(c *fiber.Ctx) error {
	var req aiSvc.ChatRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	resp, err := ctl.svc.Chat(c.UserContext(), &req)
	if err != nil {
		// Per the non-negotiable: no 5xx for AI failures. The service
		// already swallows LLM errors and returns the fallback reply, so
		// the only path here is a request-shape failure — return 400.
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(resp)
}
