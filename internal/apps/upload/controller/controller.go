package controller

import (
	"io"

	"github.com/gofiber/fiber/v2"

	uploadSvc "restaurantsaas/internal/apps/upload/service"
)

type UploadController struct {
	svc uploadSvc.UploadService
}

func New(svc uploadSvc.UploadService) *UploadController {
	return &UploadController{svc: svc}
}

func (ctl *UploadController) RegisterRoutes(r fiber.Router) {
	r.Post("/presign", ctl.Presign)
	r.Post("/direct", ctl.Direct)
}

func (ctl *UploadController) Presign(c *fiber.Ctx) error {
	var req uploadSvc.PresignRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	if err := req.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	res, err := ctl.svc.Presign(c.UserContext(), &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(res)
}

func (ctl *UploadController) Direct(c *fiber.Ctx) error {
	prefix := c.FormValue("prefix")
	fh, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "file is required"})
	}
	f, err := fh.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	defer f.Close()
	body, err := io.ReadAll(f)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	contentType := fh.Header.Get("Content-Type")
	res, err := ctl.svc.Direct(c.UserContext(), prefix, fh.Filename, contentType, fh.Size, body)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(res)
}
