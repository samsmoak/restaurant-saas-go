package server

import (
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"

	"restaurantsaas/internal/apps/realtime"
)

type FiberServer struct {
	App         *fiber.App
	MongoClient *mongo.Client
	DB          *mongo.Database
	Redis       *redis.Client
	Hub         *realtime.Hub
}

func New(client *mongo.Client, db *mongo.Database, rc *redis.Client) *FiberServer {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		},
		BodyLimit: 12 * 1024 * 1024,
	})
	return &FiberServer{
		App:         app,
		MongoClient: client,
		DB:          db,
		Redis:       rc,
		Hub:         realtime.NewHub(),
	}
}
