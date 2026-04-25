package server

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"go.mongodb.org/mongo-driver/bson"

	adminCtrl "restaurantsaas/internal/apps/admin/controller"
	adminRepoPkg "restaurantsaas/internal/apps/admin/repository"
	adminSvcPkg "restaurantsaas/internal/apps/admin/service"
	authCtrl "restaurantsaas/internal/apps/auth/controller"
	billingCtrl "restaurantsaas/internal/apps/billing/controller"
	billingRepoPkg "restaurantsaas/internal/apps/billing/repository"
	billingSvcPkg "restaurantsaas/internal/apps/billing/service"
	authSvcPkg "restaurantsaas/internal/apps/auth/service"
	categoryCtrl "restaurantsaas/internal/apps/category/controller"
	categoryRepoPkg "restaurantsaas/internal/apps/category/repository"
	categorySvcPkg "restaurantsaas/internal/apps/category/service"
	inviteCtrl "restaurantsaas/internal/apps/invite/controller"
	inviteRepoPkg "restaurantsaas/internal/apps/invite/repository"
	inviteSvcPkg "restaurantsaas/internal/apps/invite/service"
	menuCtrl "restaurantsaas/internal/apps/menu/controller"
	menuRepoPkg "restaurantsaas/internal/apps/menu/repository"
	menuSvcPkg "restaurantsaas/internal/apps/menu/service"
	orderCtrl "restaurantsaas/internal/apps/order/controller"
	orderRepoPkg "restaurantsaas/internal/apps/order/repository"
	orderSvcPkg "restaurantsaas/internal/apps/order/service"
	paymentCtrl "restaurantsaas/internal/apps/payment/controller"
	paymentSvcPkg "restaurantsaas/internal/apps/payment/service"
	restaurantCtrl "restaurantsaas/internal/apps/restaurant/controller"
	restaurantRepoPkg "restaurantsaas/internal/apps/restaurant/repository"
	restaurantSvcPkg "restaurantsaas/internal/apps/restaurant/service"
	uploadCtrl "restaurantsaas/internal/apps/upload/controller"
	uploadSvcPkg "restaurantsaas/internal/apps/upload/service"
	userCtrl "restaurantsaas/internal/apps/user/controller"
	userRepoPkg "restaurantsaas/internal/apps/user/repository"
	userSvcPkg "restaurantsaas/internal/apps/user/service"
	"restaurantsaas/internal/middleware"
	s3util "restaurantsaas/internal/s3"
	stripeutil "restaurantsaas/internal/stripe"
)

func RegisterRoutes(srv *FiberServer) {
	srv.App.Use(middleware.Recover())

	origins := os.Getenv("CORS_ALLOWED_ORIGINS")
	if strings.TrimSpace(origins) == "" {
		origins = "*"
	}
	srv.App.Use(cors.New(cors.Config{
		AllowOrigins:     origins,
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,Stripe-Signature,X-Restaurant-Slug",
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowCredentials: false,
		ExposeHeaders:    "",
	}))

	srv.App.Get("/", healthHandler(srv))
	srv.App.Get("/health", healthHandler(srv))

	// Apple Pay domain verification — served at the path Stripe/Apple requires.
	srv.App.Get("/.well-known/apple-developer-merchantid-domain-association", func(c *fiber.Ctx) error {
		return c.SendFile("./static/.well-known/apple-developer-merchantid-domain-association")
	})

	// Repositories
	userRepo := userRepoPkg.NewUserRepository(srv.DB)
	profileRepo := userRepoPkg.NewCustomerProfileRepository(srv.DB)
	adminRepo := adminRepoPkg.NewAdminRepository(srv.DB)
	inviteRepo := inviteRepoPkg.NewInviteRepository(srv.DB)
	catRepo := categoryRepoPkg.NewCategoryRepository(srv.DB)
	menuRepo := menuRepoPkg.NewMenuRepository(srv.DB)
	orderRepo := orderRepoPkg.NewOrderRepository(srv.DB)
	restRepo := restaurantRepoPkg.NewRestaurantRepository(srv.DB)

	// Services
	restService := restaurantSvcPkg.NewRestaurantService(restRepo, adminRepo)
	authService := authSvcPkg.NewAuthService(srv.MongoClient, userRepo, profileRepo, adminRepo, inviteRepo, restRepo)
	userService := userSvcPkg.NewUserService(profileRepo)
	adminService := adminSvcPkg.NewAdminService(adminRepo)
	inviteService := inviteSvcPkg.NewInviteService(inviteRepo, restRepo)
	categoryService := categorySvcPkg.NewCategoryService(catRepo)
	menuService := menuSvcPkg.NewMenuService(menuRepo, catRepo)
	orderService := orderSvcPkg.NewOrderService(orderRepo, menuService, restService, srv.Hub)
	paymentService := paymentSvcPkg.NewPaymentService(orderService, profileRepo)
	uploadService := uploadSvcPkg.NewUploadService()
	billingRepo := billingRepoPkg.NewBillingRepository(srv.DB)
	billingService := billingSvcPkg.NewBillingService(billingRepo, restRepo)

	// Controllers
	authController := authCtrl.New(authService)
	userController := userCtrl.New(userService)
	adminController := adminCtrl.New(adminService)
	inviteController := inviteCtrl.New(inviteService)
	restController := restaurantCtrl.New(restService)
	categoryController := categoryCtrl.New(categoryService)
	menuController := menuCtrl.New(menuService)
	orderController := orderCtrl.New(orderService)
	paymentController := paymentCtrl.New(orderService, paymentService)
	uploadController := uploadCtrl.New(uploadService)
	billingController := billingCtrl.New(billingService)

	api := srv.App.Group("/api")

	// Auth (global). The /admin/* + /memberships sub-routes need JWT.
	authController.RegisterRoutes(api.Group("/auth"), middleware.JWTAuth())

	// Global restaurant endpoints: create, lookup, list-mine.
	restGroup := api.Group("/restaurants", middleware.OptionalJWTAuth())
	restController.RegisterTopLevelRoutes(restGroup)

	// Customer: /api/me/* (JWT, not tenant-scoped).
	me := api.Group("/me", middleware.JWTAuth())
	userController.RegisterMeRoutes(me)
	orderController.RegisterMeRoutes(me)
	paymentController.RegisterMeRoutes(me)

	// Stripe webhook (no auth, raw body)
	api.Post("/stripe/webhook", paymentController.Webhook)
	api.Post("/billing/webhook", billingController.Webhook)

	// Per-tenant public + customer endpoints under /api/r/:restaurant_id/*
	tenantResolver := middleware.ResolveTenantFromPath(restRepo)
	tenant := api.Group("/r/:restaurant_id", tenantResolver)
	menuController.RegisterPublicRoutes(tenant.Group("/menu"))
	restController.RegisterPublicRoutes(tenant.Group("/restaurant"))
	orderController.RegisterPublicRoutes(tenant.Group("/orders"))

	// Checkout requires JWT + customer profile, still tenant-scoped by path.
	checkout := tenant.Group("/checkout",
		middleware.JWTAuth(),
		middleware.RequireCustomerProfile(profileRepo),
	)
	paymentController.RegisterCheckoutRoutes(checkout)

	// Admin: /api/admin/* — JWT whose token carries restaurant_id.
	// The tenant is resolved FROM the token and verified against admin_users.
	admin := api.Group("/admin",
		middleware.JWTAuth(),
		middleware.ResolveTenantFromToken(restRepo),
		middleware.RequireAdminForTenant(adminRepo),
	)
	categoryController.RegisterAdminRoutes(admin.Group("/categories"))
	menuController.RegisterAdminRoutes(admin.Group("/menu-items"))
	restController.RegisterAdminRoutes(admin.Group("/restaurant"))
	orderController.RegisterAdminRoutes(admin.Group("/orders"))
	inviteController.RegisterAdminRoutes(admin.Group("/invites"))
	adminController.RegisterAdminRoutes(admin.Group("/users"))
	uploadController.RegisterRoutes(admin.Group("/uploads"))
	billingController.RegisterRoutes(admin.Group("/billing"))

	// WebSocket
	RegisterWSRoutes(srv, restRepo, adminRepo)
}

func healthHandler(srv *FiberServer) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.UserContext(), 2*time.Second)
		defer cancel()

		mongoStatus := "ok"
		if err := srv.DB.RunCommand(ctx, bson.D{{Key: "ping", Value: 1}}).Err(); err != nil {
			mongoStatus = "err"
		}
		redisStatus := "not_configured"
		if srv.Redis != nil {
			if err := srv.Redis.Ping(ctx).Err(); err != nil {
				redisStatus = "err"
			} else {
				redisStatus = "ok"
			}
		}
		overall := "ok"
		if mongoStatus != "ok" {
			overall = "degraded"
		}
		code := fiber.StatusOK
		if mongoStatus == "err" {
			code = fiber.StatusServiceUnavailable
		}
		return c.Status(code).JSON(fiber.Map{
			"status": overall,
			"mongo":  mongoStatus,
			"redis":  redisStatus,
			"s3":     configuredFlag(s3util.IsConfigured()),
			"stripe": configuredFlag(stripeutil.IsConfigured()),
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	}
}

func configuredFlag(b bool) string {
	if b {
		return "ok"
	}
	return "not_configured"
}

func init() {
	log.SetFlags(log.LstdFlags | log.LUTC)
}
