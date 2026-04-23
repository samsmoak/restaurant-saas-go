package middleware

import (
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"

	userRepoPkg "restaurantsaas/internal/apps/user/repository"
)

// RequireCustomerProfile ensures the signed-in user has a customer_profiles row.
// Used on /api/checkout/** which needs a resolvable customer identity.
func RequireCustomerProfile(repo *userRepoPkg.CustomerProfileRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		uid := UserIDFromCtx(c)
		if uid == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not signed in"})
		}
		oid, err := primitive.ObjectIDFromHex(uid)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
		}
		row, err := repo.FindByUserID(c.UserContext(), oid)
		if err != nil || row == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "finish setting up your customer profile"})
		}
		return c.Next()
	}
}
