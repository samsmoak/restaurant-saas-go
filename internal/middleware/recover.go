package middleware

import (
	"log"

	"github.com/gofiber/fiber/v2"
)

func Recover() fiber.Handler {
	return func(c *fiber.Ctx) (err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("middleware.Recover: panic: %v", r)
				err = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
			}
		}()
		return c.Next()
	}
}
