package controller

import (
	"github.com/gofiber/fiber/v2"

	menuModel "restaurantsaas/internal/apps/menu/model"
	menuSvc "restaurantsaas/internal/apps/menu/service"
	"restaurantsaas/internal/middleware"
)

type MenuController struct {
	svc menuSvc.MenuService
}

func New(svc menuSvc.MenuService) *MenuController {
	return &MenuController{svc: svc}
}

func (ctl *MenuController) RegisterPublicRoutes(r fiber.Router) {
	r.Get("/", ctl.PublicMenu)
}

func (ctl *MenuController) RegisterAdminRoutes(r fiber.Router) {
	r.Get("/", ctl.ListAll)
	r.Post("/", ctl.Create)
	r.Put("/:id", ctl.Update)
	r.Delete("/:id", ctl.Delete)
}

func (ctl *MenuController) PublicMenu(c *fiber.Ctx) error {
	rid := middleware.TenantIDFromCtx(c)
	groups, err := ctl.svc.PublicMenu(c.UserContext(), rid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"categories": groups})
}

func (ctl *MenuController) ListAll(c *fiber.Ctx) error {
	rid := middleware.TenantIDFromCtx(c)
	items, err := ctl.svc.ListAllItems(c.UserContext(), rid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if items == nil {
		items = []*menuModel.MenuItem{}
	}
	return c.JSON(fiber.Map{"items": items})
}

func (ctl *MenuController) Create(c *fiber.Ctx) error {
	rid := middleware.TenantIDFromCtx(c)
	var req menuSvc.MenuItemRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	if err := req.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	row, err := ctl.svc.Create(c.UserContext(), rid, &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(row)
}

func (ctl *MenuController) Update(c *fiber.Ctx) error {
	rid := middleware.TenantIDFromCtx(c)
	id := c.Params("id")
	var req menuSvc.MenuItemRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	if err := req.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	row, err := ctl.svc.Update(c.UserContext(), rid, id, &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if row == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "menu item not found"})
	}
	return c.JSON(row)
}

func (ctl *MenuController) Delete(c *fiber.Ctx) error {
	rid := middleware.TenantIDFromCtx(c)
	id := c.Params("id")
	if err := ctl.svc.Delete(c.UserContext(), rid, id); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}
