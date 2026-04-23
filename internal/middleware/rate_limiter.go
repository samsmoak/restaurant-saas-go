package middleware

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

func RedisRateLimit(rc *redis.Client, bucket string, limit int, window time.Duration) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if rc == nil {
			return c.Next()
		}
		ip := c.IP()
		key := fmt.Sprintf("rl:%s:%s", bucket, ip)
		ctx := c.UserContext()
		count, err := rc.Incr(ctx, key).Result()
		if err != nil {
			return c.Next()
		}
		if count == 1 {
			_ = rc.Expire(ctx, key, window).Err()
		}
		if count > int64(limit) {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "rate limit exceeded"})
		}
		return c.Next()
	}
}
