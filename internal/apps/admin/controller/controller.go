package controller

import (
	"github.com/gofiber/fiber/v2"

	adminModel "restaurantsaas/internal/apps/admin/model"
	adminSvc "restaurantsaas/internal/apps/admin/service"
	"restaurantsaas/internal/middleware"
)

type AdminController struct {
	svc adminSvc.AdminService
}

func New(svc adminSvc.AdminService) *AdminController {
	return &AdminController{svc: svc}
}

func (ctl *AdminController) RegisterAdminRoutes(r fiber.Router) {
	r.Get("/", ctl.List)
	r.Delete("/:id", ctl.Delete)
}

func (ctl *AdminController) List(c *fiber.Ctx) error {
	rid := middleware.TenantIDFromCtx(c)
	rows, err := ctl.svc.ListForRestaurant(c.UserContext(), rid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if rows == nil {
		rows = []*adminModel.AdminUser{}
	}
	return c.JSON(fiber.Map{"users": rows})
}

func (ctl *AdminController) Delete(c *fiber.Ctx) error {
	rid := middleware.TenantIDFromCtx(c)
	id := c.Params("id")
	currentUID := middleware.UserIDFromCtx(c)
	if err := ctl.svc.Delete(c.UserContext(), rid, currentUID, id); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}
