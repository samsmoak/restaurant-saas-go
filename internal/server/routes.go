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
	"go.mongodb.org/mongo-driver/bson/primitive"

	adminCtrl "restaurantsaas/internal/apps/admin/controller"
	adminRepoPkg "restaurantsaas/internal/apps/admin/repository"
	adminSvcPkg "restaurantsaas/internal/apps/admin/service"
	aiClient "restaurantsaas/internal/apps/ai/client"
	aiCtrl "restaurantsaas/internal/apps/ai/controller"
	aiSvcPkg "restaurantsaas/internal/apps/ai/service"
	authCtrl "restaurantsaas/internal/apps/auth/controller"
	billingCtrl "restaurantsaas/internal/apps/billing/controller"
	billingRepoPkg "restaurantsaas/internal/apps/billing/repository"
	billingSvcPkg "restaurantsaas/internal/apps/billing/service"
	cravingCtrl "restaurantsaas/internal/apps/cravings/controller"
	cravingRepoPkg "restaurantsaas/internal/apps/cravings/repository"
	cravingSvcPkg "restaurantsaas/internal/apps/cravings/service"
	deviceCtrl "restaurantsaas/internal/apps/devices/controller"
	deviceRepoPkg "restaurantsaas/internal/apps/devices/repository"
	deviceSvcPkg "restaurantsaas/internal/apps/devices/service"
	discoveryCtrl "restaurantsaas/internal/apps/discovery/controller"
	discoveryRepoPkg "restaurantsaas/internal/apps/discovery/repository"
	discoverySvcPkg "restaurantsaas/internal/apps/discovery/service"
	favoriteCtrl "restaurantsaas/internal/apps/favorites/controller"
	favoriteRepoPkg "restaurantsaas/internal/apps/favorites/repository"
	favoriteSvcPkg "restaurantsaas/internal/apps/favorites/service"
	groupCtrl "restaurantsaas/internal/apps/groups/controller"
	groupRepoPkg "restaurantsaas/internal/apps/groups/repository"
	groupSvcPkg "restaurantsaas/internal/apps/groups/service"
	notifCtrl "restaurantsaas/internal/apps/notifications/controller"
	notifRepoPkg "restaurantsaas/internal/apps/notifications/repository"
	notifSvcPkg "restaurantsaas/internal/apps/notifications/service"
	tasteCtrl "restaurantsaas/internal/apps/taste/controller"
	tasteRepoPkg "restaurantsaas/internal/apps/taste/repository"
	tasteSvcPkg "restaurantsaas/internal/apps/taste/service"
	reviewCtrl "restaurantsaas/internal/apps/reviews/controller"
	reviewRepoPkg "restaurantsaas/internal/apps/reviews/repository"
	reviewSvcPkg "restaurantsaas/internal/apps/reviews/service"
	leadsCtrl "restaurantsaas/internal/apps/leads/controller"
	leadsRepoPkg "restaurantsaas/internal/apps/leads/repository"
	leadsSvcPkg "restaurantsaas/internal/apps/leads/service"
	authSvcPkg "restaurantsaas/internal/apps/auth/service"
	categoryCtrl "restaurantsaas/internal/apps/category/controller"
	categoryRepoPkg "restaurantsaas/internal/apps/category/repository"
	categorySvcPkg "restaurantsaas/internal/apps/category/service"
	inviteCtrl "restaurantsaas/internal/apps/invite/controller"
	inviteRepoPkg "restaurantsaas/internal/apps/invite/repository"
	inviteSvcPkg "restaurantsaas/internal/apps/invite/service"
	menuCtrl "restaurantsaas/internal/apps/menu/controller"
	menuModel "restaurantsaas/internal/apps/menu/model"
	menuRepoPkg "restaurantsaas/internal/apps/menu/repository"
	menuSvcPkg "restaurantsaas/internal/apps/menu/service"
	orderCtrl "restaurantsaas/internal/apps/order/controller"
	orderRepoPkg "restaurantsaas/internal/apps/order/repository"
	orderSvcPkg "restaurantsaas/internal/apps/order/service"
	paymentCtrl "restaurantsaas/internal/apps/payment/controller"
	paymentSvcPkg "restaurantsaas/internal/apps/payment/service"
	promoRepoPkg "restaurantsaas/internal/apps/promos/repository"
	promoSvcPkg "restaurantsaas/internal/apps/promos/service"
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

	// Repositories (continued)
	favoriteRepo := favoriteRepoPkg.NewFavoriteRepository(srv.DB)
	reviewRepo := reviewRepoPkg.NewReviewRepository(srv.DB)
	discoveryRepo := discoveryRepoPkg.NewDiscoveryRepository(srv.DB)

	// Services
	restService := restaurantSvcPkg.NewRestaurantService(restRepo, adminRepo)
	authService := authSvcPkg.NewAuthService(srv.MongoClient, userRepo, profileRepo, adminRepo, inviteRepo, restRepo)
	userService := userSvcPkg.NewUserService(profileRepo)
	adminService := adminSvcPkg.NewAdminService(adminRepo)
	inviteService := inviteSvcPkg.NewInviteService(inviteRepo, restRepo)
	categoryService := categorySvcPkg.NewCategoryService(catRepo)
	menuService := menuSvcPkg.NewMenuService(menuRepo, catRepo)
	promoRepo := promoRepoPkg.NewPromoRepository(srv.DB)
	promoService := promoSvcPkg.NewPromoService(promoRepo)
	// Best-effort seed; only runs if PROMO_WELCOME10_PERCENT or its
	// default flips on a row that doesn't exist yet.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		promoService.SeedFromEnv(ctx)
	}()

	orderService := orderSvcPkg.NewOrderService(orderRepo, menuService, restService, srv.Hub, restService, promoService, profileRepo)
	billingRepo := billingRepoPkg.NewBillingRepository(srv.DB)
	billingService := billingSvcPkg.NewBillingService(billingRepo, restRepo)
	paymentService := paymentSvcPkg.NewPaymentService(orderService, profileRepo, billingService)
	uploadService := uploadSvcPkg.NewUploadService()
	leadsRepo := leadsRepoPkg.NewLeadsRepository(srv.DB)
	leadsService := leadsSvcPkg.NewLeadsService(leadsRepo)
	leadsController := leadsCtrl.New(leadsService)
	discoveryService := discoverySvcPkg.NewDiscoveryService(discoveryRepo)
	favoriteService := favoriteSvcPkg.NewFavoriteService(favoriteRepo, restRepo)
	reviewService := reviewSvcPkg.NewReviewService(reviewRepo, orderRepo, profileRepo, restService)

	// Savor-AI side apps + their dependencies.
	tasteRepo := tasteRepoPkg.NewTasteRepository(srv.DB)
	tasteService := tasteSvcPkg.NewTasteService(tasteRepo)
	cravingRepo := cravingRepoPkg.NewCravingRepository(srv.DB)
	cravingService := cravingSvcPkg.NewCravingService(cravingRepo)
	notifRepo := notifRepoPkg.NewNotificationRepository(srv.DB)
	notifService := notifSvcPkg.NewNotificationService(notifRepo)
	deviceRepo := deviceRepoPkg.NewDeviceRepository(srv.DB)
	deviceService := deviceSvcPkg.NewDeviceService(deviceRepo)
	groupRepo := groupRepoPkg.NewGroupRepository(srv.DB)
	groupService := groupSvcPkg.NewGroupService(groupRepo, orderService, restService, profileRepo)

	// Stats endpoint depends on three repos that aren't in
	// userService's constructor; wire them out-of-band.
	userSvcPkg.SetStatsDeps(userService, orderRepo, favoriteRepo, reviewRepo)

	// LLM client is optional. FromEnv returns nil when LLM_API_KEY is unset
	// — the AI service handles nil by falling back to rule-based intent
	// parsing for /search and a deterministic reply for /chat.
	llm, err := aiClient.FromEnv()
	if err != nil {
		log.Printf("restaurantsaas: LLM client init: %v (AI endpoints will use fallback)", err)
		llm = nil
	}
	menuLookup := func(ctx context.Context, id primitive.ObjectID) (*menuModel.MenuItem, error) {
		return menuRepo.GetByID(ctx, id)
	}
	aiService := aiSvcPkg.NewAIService(llm, discoveryService, menuService, restService, tasteService, cravingService, menuLookup)

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
	discoveryController := discoveryCtrl.New(discoveryService)
	favoriteController := favoriteCtrl.New(favoriteService)
	reviewController := reviewCtrl.New(reviewService)
	aiController := aiCtrl.New(aiService)
	tasteController := tasteCtrl.New(tasteService)
	cravingController := cravingCtrl.New(cravingService)
	notifController := notifCtrl.New(notifService)
	deviceController := deviceCtrl.New(deviceService)
	groupController := groupCtrl.New(groupService)
	// Wire the AI dish hydrator into favorites so the {dishes:[]} arm
	// of GET /api/me/favorites is populated. Done here to avoid a
	// circular import between favorites and AI.
	favoriteController.SetDishHydrator(aiService.HydrateDishes)

	api := srv.App.Group("/api")

	// Auth (global). The /admin/* + /memberships sub-routes need JWT.
	authController.RegisterRoutes(api.Group("/auth"), middleware.JWTAuth())

	// Global restaurant endpoints: create, lookup, list-mine.
	// Discovery routes go on the same group so they share OptionalJWTAuth.
	// They MUST be registered before restController.RegisterTopLevelRoutes
	// so static paths (/, /search, /search/suggest) win over /:restaurant_id.
	restGroup := api.Group("/restaurants", middleware.OptionalJWTAuth())
	discoveryController.RegisterRoutes(restGroup)
	restController.RegisterTopLevelRoutes(restGroup)

	// Customer: /api/me/* (JWT, not tenant-scoped).
	me := api.Group("/me", middleware.JWTAuth())
	userController.RegisterMeRoutes(me)
	orderController.RegisterMeRoutes(me)
	paymentController.RegisterMeRoutes(me)
	favoriteController.RegisterMeRoutes(me)
	reviewController.RegisterMeRoutes(me)
	uploadController.RegisterMeRoutes(me)
	tasteController.RegisterMeRoutes(me)
	notifController.RegisterMeRoutes(me)
	deviceController.RegisterMeRoutes(me)

	// AI: /api/ai/* — /search (POST) is public so the legacy customer
	// app keeps working; the GET dishes-shape variant + dish detail +
	// recommend are also public (the Savorar client passes JWT
	// optionally for geo bias).  /chat (SSE) and /cravings/* require
	// JWT per spec.
	aiPublic := api.Group("/ai", middleware.OptionalJWTAuth())
	aiPublic.Post("/search", aiController.Search)
	aiPublic.Get("/search", aiController.SearchDishes)
	aiPublic.Post("/recommend", aiController.Recommend)
	aiPublic.Get("/dishes/:id", aiController.DishByID)
	api.Post("/ai/chat", middleware.JWTAuth(), aiController.Chat)
	cravingController.RegisterAIRoutes(api.Group("/ai", middleware.JWTAuth()))

	// Group order endpoints (BACKEND_REQUIREMENTS.md §9).
	groups := api.Group("/groups", middleware.JWTAuth())
	groupController.RegisterRoutes(groups)

	// Public leads endpoint (no auth)
	leadsController.RegisterRoutes(api)

	// Stripe webhook (no auth, raw body)
	api.Post("/stripe/webhook", paymentController.Webhook)
	api.Post("/billing/webhook", billingController.Webhook)

	// Per-tenant public + customer endpoints under /api/r/:restaurant_id/*
	tenantResolver := middleware.ResolveTenantFromPath(restRepo)
	tenant := api.Group("/r/:restaurant_id", tenantResolver)
	menuController.RegisterPublicRoutes(tenant.Group("/menu"))
	restController.RegisterPublicRoutes(tenant.Group("/restaurant"))
	orderController.RegisterPublicRoutes(tenant.Group("/orders"))
	reviewController.RegisterTenantRoutes(tenant.Group("/reviews"))

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
