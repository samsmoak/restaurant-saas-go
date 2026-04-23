package controller

import (
	"github.com/gofiber/fiber/v2"

	catModel "restaurantsaas/internal/apps/category/model"
	catSvc "restaurantsaas/internal/apps/category/service"
	"restaurantsaas/internal/middleware"
)

type CategoryController struct {
	svc catSvc.CategoryService
}

func New(svc catSvc.CategoryService) *CategoryController {
	return &CategoryController{svc: svc}
}

func (ctl *CategoryController) RegisterAdminRoutes(r fiber.Router) {
	r.Get("/", ctl.List)
	r.Post("/", ctl.Create)
	r.Put("/:id", ctl.Update)
	r.Delete("/:id", ctl.Delete)
}

func (ctl *CategoryController) List(c *fiber.Ctx) error {
	rid := middleware.TenantIDFromCtx(c)
	rows, err := ctl.svc.List(c.UserContext(), rid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if rows == nil {
		rows = []*catModel.Category{}
	}
	return c.JSON(fiber.Map{"categories": rows})
}

func (ctl *CategoryController) Create(c *fiber.Ctx) error {
	rid := middleware.TenantIDFromCtx(c)
	var req catSvc.CategoryRequest
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

func (ctl *CategoryController) Update(c *fiber.Ctx) error {
	rid := middleware.TenantIDFromCtx(c)
	id := c.Params("id")
	var req catSvc.CategoryRequest
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
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "category not found"})
	}
	return c.JSON(row)
}

func (ctl *CategoryController) Delete(c *fiber.Ctx) error {
	rid := middleware.TenantIDFromCtx(c)
	id := c.Params("id")
	if err := ctl.svc.Delete(c.UserContext(), rid, id); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}
