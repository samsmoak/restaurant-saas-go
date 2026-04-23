package controller

import (
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"

	inviteModel "restaurantsaas/internal/apps/invite/model"
	inviteSvc "restaurantsaas/internal/apps/invite/service"
	"restaurantsaas/internal/middleware"
)

type InviteController struct {
	svc inviteSvc.InviteService
}

func New(svc inviteSvc.InviteService) *InviteController {
	return &InviteController{svc: svc}
}

func (ctl *InviteController) RegisterAdminRoutes(r fiber.Router) {
	r.Get("/", ctl.List)
	r.Post("/", ctl.Create)
	r.Patch("/:id/revoke", ctl.Revoke)
	r.Delete("/:id", ctl.Delete)
}

func (ctl *InviteController) List(c *fiber.Ctx) error {
	rid := middleware.TenantIDFromCtx(c)
	rows, err := ctl.svc.List(c.UserContext(), rid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if rows == nil {
		rows = []*inviteModel.AdminInvite{}
	}
	return c.JSON(fiber.Map{"invites": rows})
}

func (ctl *InviteController) Create(c *fiber.Ctx) error {
	rid := middleware.TenantIDFromCtx(c)
	uid := middleware.UserIDFromCtx(c)
	createdBy, err := primitive.ObjectIDFromHex(uid)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
	}
	var req inviteSvc.InviteCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	if err := req.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	row, err := ctl.svc.Create(c.UserContext(), rid, createdBy, &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(row)
}

func (ctl *InviteController) Revoke(c *fiber.Ctx) error {
	rid := middleware.TenantIDFromCtx(c)
	id := c.Params("id")
	if err := ctl.svc.Revoke(c.UserContext(), rid, id); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (ctl *InviteController) Delete(c *fiber.Ctx) error {
	rid := middleware.TenantIDFromCtx(c)
	id := c.Params("id")
	if err := ctl.svc.Delete(c.UserContext(), rid, id); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}
