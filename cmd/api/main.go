package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"restaurantsaas/internal/database"
	"restaurantsaas/internal/server"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Fatalf("restaurantsaas: panic: %v", r)
		}
	}()

	logMissingEnv()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	mongoClient, db, err := database.Connect(ctx)
	if err != nil {
		log.Fatalf("restaurantsaas: mongo connect: %v", err)
	}
	if err := database.EnsureIndexes(ctx, db); err != nil {
		log.Printf("restaurantsaas: ensureIndexes: %v", err)
	}

	var redisClient *redis.Client
	if url := os.Getenv("REDIS_URL"); url != "" {
		opts, err := redis.ParseURL(url)
		if err != nil {
			log.Printf("restaurantsaas: invalid REDIS_URL: %v", err)
		} else {
			redisClient = redis.NewClient(opts)
			if err := redisClient.Ping(ctx).Err(); err != nil {
				log.Printf("restaurantsaas: redis ping: %v (continuing)", err)
				redisClient = nil
			}
		}
	}

	srv := server.New(mongoClient, db, redisClient)
	server.RegisterRoutes(srv)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	go func() {
		fmt.Printf("restaurantsaas: listening on :%s\n", port)
		if err := srv.App.Listen(":" + port); err != nil {
			log.Printf("restaurantsaas: server error: %v", err)
		}
	}()

	<-quit
	fmt.Println("restaurantsaas: shutting down…")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.App.Shutdown(); err != nil {
		log.Printf("restaurantsaas: fiber shutdown: %v", err)
	}
	if err := mongoClient.Disconnect(shutdownCtx); err != nil {
		log.Printf("restaurantsaas: mongo disconnect: %v", err)
	}
	if redisClient != nil {
		if err := redisClient.Close(); err != nil {
			log.Printf("restaurantsaas: redis close: %v", err)
		}
	}
	fmt.Println("restaurantsaas: shutdown complete")
}

func logMissingEnv() {
	critical := []string{"MONGO_URI", "MONGO_DB_NAME", "JWT_SECRET"}
	for _, k := range critical {
		if os.Getenv(k) == "" {
			log.Fatalf("restaurantsaas: required env %s is not set", k)
		}
	}
	warn := []string{
		"GOOGLE_OAUTH_CLIENT_ID",
		"STRIPE_SECRET_KEY", "STRIPE_WEBHOOK_SECRET",
		"AWS_S3_BUCKET", "AWS_S3_REGION",
		"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY",
		"REDIS_URL",
	}
	for _, k := range warn {
		if os.Getenv(k) == "" {
			log.Printf("restaurantsaas: env %s is not set (feature degraded)", k)
		}
	}
}
