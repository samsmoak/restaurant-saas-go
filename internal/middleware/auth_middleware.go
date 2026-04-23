package middleware

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"

	"restaurantsaas/internal/jwtutil"
)

const (
	LocalUserID       = "user_id"
	LocalEmail        = "email"
	LocalTokenRestID  = "token_restaurant_id"
	LocalTokenRole    = "token_role"
)

// JWTAuth verifies the bearer token and stores user_id + email + token claims.
// Handshake fallback: ?token=<jwt> for WebSocket upgrades.
func JWTAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		tok, err := extractBearer(c)
		if err != nil {
			if q := c.Query("token"); q != "" {
				tok = q
			} else {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
			}
		}
		claims, err := jwtutil.Parse(tok)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
		}
		if claims.Subject == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
		}
		c.Locals(LocalUserID, claims.Subject)
		c.Locals(LocalEmail, claims.Email)
		c.Locals(LocalTokenRestID, claims.RestaurantID)
		c.Locals(LocalTokenRole, claims.Role)
		return c.Next()
	}
}

// OptionalJWTAuth is a softer variant — sets locals if a valid token is present,
// ignores missing/invalid tokens. Use on public endpoints that benefit from
// knowing the caller (e.g. customer-optional tracking).
func OptionalJWTAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		tok, err := extractBearer(c)
		if err != nil {
			if q := c.Query("token"); q != "" {
				tok = q
			} else {
				return c.Next()
			}
		}
		if claims, err := jwtutil.Parse(tok); err == nil && claims.Subject != "" {
			c.Locals(LocalUserID, claims.Subject)
			c.Locals(LocalEmail, claims.Email)
			c.Locals(LocalTokenRestID, claims.RestaurantID)
			c.Locals(LocalTokenRole, claims.Role)
		}
		return c.Next()
	}
}

func extractBearer(c *fiber.Ctx) (string, error) {
	h := c.Get("Authorization")
	if h == "" || !strings.HasPrefix(h, "Bearer ") {
		return "", errors.New("missing or malformed Authorization header")
	}
	return strings.TrimPrefix(h, "Bearer "), nil
}

// UserIDFromCtx is a helper — returns "" if not signed in.
func UserIDFromCtx(c *fiber.Ctx) string {
	v, _ := c.Locals(LocalUserID).(string)
	return v
}

// EmailFromCtx returns the JWT's email claim or "".
func EmailFromCtx(c *fiber.Ctx) string {
	v, _ := c.Locals(LocalEmail).(string)
	return v
}
