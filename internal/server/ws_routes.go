package server

import (
	adminRepoPkg "restaurantsaas/internal/apps/admin/repository"
	wsCtrl "restaurantsaas/internal/apps/realtime/controller"
	restaurantRepoPkg "restaurantsaas/internal/apps/restaurant/repository"
	"restaurantsaas/internal/middleware"
)

func RegisterWSRoutes(srv *FiberServer, restRepo *restaurantRepoPkg.RestaurantRepository, adminRepo *adminRepoPkg.AdminRepository) {
	ws := wsCtrl.New(srv.Hub)

	ws1 := srv.App.Group("/ws")
	ws1.Use(wsCtrl.HTTPUpgrade)

	// Admin firehose for a specific tenant. Requires an admin token scoped
	// to a restaurant (JWT carries restaurant_id).
	ws1.Get("/admin/orders",
		middleware.JWTAuth(),
		middleware.ResolveTenantFromToken(restRepo),
		middleware.RequireAdminForTenant(adminRepo),
		ws.AdminHandler(),
	)

	// Public per-order channel.
	ws1.Get("/orders/:order_number", ws.OrderHandler())
}
