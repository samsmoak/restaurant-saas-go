package middleware

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"

	restaurantRepoPkg "restaurantsaas/internal/apps/restaurant/repository"
)

const (
	LocalRestaurantID   = "tenant_restaurant_id"   // primitive.ObjectID
	LocalRestaurantSlug = "tenant_restaurant_slug" // string
)

// ResolveTenantFromPath parses :slug from the URL and loads the restaurant.
// Use on `/api/r/:slug/...` groups. Returns 404 if slug is unknown.
func ResolveTenantFromPath(repo *restaurantRepoPkg.RestaurantRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		slug := strings.TrimSpace(c.Params("slug"))
		if slug == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "restaurant slug is required"})
		}
		r, err := repo.FindBySlug(c.UserContext(), slug)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if r == nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "restaurant not found"})
		}
		c.Locals(LocalRestaurantID, r.ID)
		c.Locals(LocalRestaurantSlug, r.Slug)
		return c.Next()
	}
}

// RequireAdminForTenant ensures the signed-in user has an admin row for the
// tenant already placed in locals (via ResolveTenantFromPath or ResolveTenantFromToken).
func RequireAdminForTenant(adminRepo AdminChecker) fiber.Handler {
	return func(c *fiber.Ctx) error {
		uid := UserIDFromCtx(c)
		if uid == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not signed in"})
		}
		restaurantID, ok := c.Locals(LocalRestaurantID).(primitive.ObjectID)
		if !ok || restaurantID.IsZero() {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "restaurant context missing"})
		}
		userOID, err := primitive.ObjectIDFromHex(uid)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
		}
		ok, err = adminRepo.IsAdmin(c.UserContext(), userOID, restaurantID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if !ok {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "admin only"})
		}
		return c.Next()
	}
}

// ResolveTenantFromToken reads restaurant_id from the JWT claim and stores it
// as the tenant context. Used for /api/admin/** where the admin's JWT is scoped.
func ResolveTenantFromToken(repo *restaurantRepoPkg.RestaurantRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		rid, _ := c.Locals(LocalTokenRestID).(string)
		if rid == "" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "token is not scoped to a restaurant"})
		}
		oid, err := primitive.ObjectIDFromHex(rid)
		if err != nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "invalid restaurant in token"})
		}
		r, err := repo.GetByID(c.UserContext(), oid)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if r == nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "restaurant not found"})
		}
		c.Locals(LocalRestaurantID, r.ID)
		c.Locals(LocalRestaurantSlug, r.Slug)
		return c.Next()
	}
}

// TenantIDFromCtx fetches the resolved tenant id. Returns zero ObjectID if unset.
func TenantIDFromCtx(c *fiber.Ctx) primitive.ObjectID {
	v, _ := c.Locals(LocalRestaurantID).(primitive.ObjectID)
	return v
}

// TenantSlugFromCtx fetches the slug.
func TenantSlugFromCtx(c *fiber.Ctx) string {
	v, _ := c.Locals(LocalRestaurantSlug).(string)
	return v
}

// AdminChecker is the narrow interface RequireAdminForTenant needs.
// AdminRepository satisfies it via an IsAdmin helper.
type AdminChecker interface {
	IsAdmin(ctx context.Context, userID, restaurantID primitive.ObjectID) (bool, error)
}
