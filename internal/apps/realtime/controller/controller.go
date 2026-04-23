package controller

import (
	"log"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"restaurantsaas/internal/apps/realtime"
	"restaurantsaas/internal/middleware"
)

type WSController struct {
	hub *realtime.Hub
}

func New(hub *realtime.Hub) *WSController {
	return &WSController{hub: hub}
}

func HTTPUpgrade(c *fiber.Ctx) error {
	if websocket.IsWebSocketUpgrade(c) {
		return c.Next()
	}
	return c.SendStatus(fiber.StatusUpgradeRequired)
}

// AdminHandler broadcasts to admins subscribed to the tenant read from ctx.
func (ctl *WSController) AdminHandler() fiber.Handler {
	return websocket.New(func(c *websocket.Conn) {
		rid, _ := c.Locals(middleware.LocalRestaurantID).(primitive.ObjectID)
		if rid.IsZero() {
			_ = c.WriteJSON(realtime.NewEvent("error", "missing tenant"))
			return
		}
		key := rid.Hex()
		ctl.hub.AddAdmin(key, c)
		defer ctl.hub.RemoveAdmin(key, c)
		_ = c.WriteJSON(realtime.NewEvent("ready", nil))
		ctl.runPumps(c)
	})
}

func (ctl *WSController) OrderHandler() fiber.Handler {
	return websocket.New(func(c *websocket.Conn) {
		orderNumber := c.Params("order_number")
		if orderNumber == "" {
			return
		}
		ctl.hub.AddOrder(orderNumber, c)
		defer ctl.hub.RemoveOrder(orderNumber, c)
		_ = c.WriteJSON(realtime.NewEvent("ready", nil))
		ctl.runPumps(c)
	})
}

func (ctl *WSController) runPumps(c *websocket.Conn) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = c.SetReadDeadline(time.Now().Add(30 * time.Second))
		c.SetPongHandler(func(string) error {
			return c.SetReadDeadline(time.Now().Add(30 * time.Second))
		})
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				return
			}
		}
	}()
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if err := c.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
				log.Printf("WSController: ping: %v", err)
				return
			}
		}
	}
}
