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

// RegisterMeRoutes wires the customer-facing avatar upload route under
// /api/me/uploads. The route enforces a 4MB cap and writes to the
// customer-avatars/ prefix only.
//
// Both GET (Savorar client per BACKEND_REQUIREMENTS.md §2) and POST
// (legacy customer Flutter app) are accepted.
func (ctl *UploadController) RegisterMeRoutes(r fiber.Router) {
	r.Get("/uploads/presign", ctl.CustomerPresignGet)
	r.Post("/uploads/presign", ctl.CustomerPresign)
}

func (ctl *UploadController) CustomerPresign(c *fiber.Ctx) error {
	var req uploadSvc.CustomerPresignRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	if err := req.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	res, err := ctl.svc.PresignCustomerAvatar(c.UserContext(), &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{
		"upload_url": res.UploadURL,
		"public_url": res.PublicURL,
		"key":        res.Key,
		"fields":     fiber.Map{},
	})
}

// CustomerPresignGet is the spec-shaped GET variant.  Reads
// content_type and filename from the query string; size is unknown
// (clients can't supply it via GET) so we relax the validation.
func (ctl *UploadController) CustomerPresignGet(c *fiber.Ctx) error {
	req := uploadSvc.CustomerPresignRequest{
		Filename:    c.Query("filename"),
		ContentType: c.Query("content_type"),
	}
	if err := req.ValidateForGet(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	// PresignCustomerAvatar enforces size internally only when set.
	// Leave Size=0 — the upstream S3 PresignedPutURL doesn't need it.
	res, err := ctl.svc.PresignCustomerAvatar(c.UserContext(), &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{
		"upload_url": res.UploadURL,
		"public_url": res.PublicURL,
		"key":        res.Key,
		"fields":     fiber.Map{},
	})
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
