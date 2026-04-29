package realtime

import (
	"log"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
)

type Event struct {
	Type  string      `json:"type"`
	Order interface{} `json:"order,omitempty"`
	TS    int64       `json:"ts"`
}

func NewEvent(t string, order interface{}) Event {
	return Event{Type: t, Order: order, TS: time.Now().UnixMilli()}
}

type Hub struct {
	mu        sync.RWMutex
	adminSubs map[string]map[*websocket.Conn]struct{} // restaurant_id hex → conns
	orderSubs map[string]map[*websocket.Conn]struct{} // order_number → conns
}

func NewHub() *Hub {
	return &Hub{
		adminSubs: make(map[string]map[*websocket.Conn]struct{}),
		orderSubs: make(map[string]map[*websocket.Conn]struct{}),
	}
}

func (h *Hub) AddAdmin(restaurantID string, c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	conns, ok := h.adminSubs[restaurantID]
	if !ok {
		conns = make(map[*websocket.Conn]struct{})
		h.adminSubs[restaurantID] = conns
	}
	conns[c] = struct{}{}
}

func (h *Hub) RemoveAdmin(restaurantID string, c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if conns, ok := h.adminSubs[restaurantID]; ok {
		delete(conns, c)
		if len(conns) == 0 {
			delete(h.adminSubs, restaurantID)
		}
	}
}

func (h *Hub) AddOrder(orderNumber string, c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	conns, ok := h.orderSubs[orderNumber]
	if !ok {
		conns = make(map[*websocket.Conn]struct{})
		h.orderSubs[orderNumber] = conns
	}
	conns[c] = struct{}{}
}

func (h *Hub) RemoveOrder(orderNumber string, c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if conns, ok := h.orderSubs[orderNumber]; ok {
		delete(conns, c)
		if len(conns) == 0 {
			delete(h.orderSubs, orderNumber)
		}
	}
}

// BroadcastAdmin sends to every admin connected for the given restaurant.
func (h *Hub) BroadcastAdmin(restaurantID string, ev Event) {
	h.mu.RLock()
	subs := h.adminSubs[restaurantID]
	targets := make([]*websocket.Conn, 0, len(subs))
	for c := range subs {
		targets = append(targets, c)
	}
	h.mu.RUnlock()
	for _, c := range targets {
		if err := c.WriteJSON(ev); err != nil {
			log.Printf("Hub.BroadcastAdmin: write: %v", err)
		}
	}
}

func (h *Hub) BroadcastOrder(orderNumber string, ev Event) {
	h.mu.RLock()
	subs := h.orderSubs[orderNumber]
	targets := make([]*websocket.Conn, 0, len(subs))
	for c := range subs {
		targets = append(targets, c)
	}
	h.mu.RUnlock()
	for _, c := range targets {
		if err := c.WriteJSON(ev); err != nil {
			log.Printf("Hub.BroadcastOrder: write: %v", err)
		}
	}
}

// BroadcastOrderRaw fans out an arbitrary JSON-shaped payload (no
// Event wrapper) to all customer-side subscribers of a specific order
// number. This is what BACKEND_REQUIREMENTS.md §5 calls for: frames
// shaped like {"type":"status",...}, {"type":"courier",...},
// {"type":"delivered",...}. The legacy admin event firehose still
// uses BroadcastOrder/BroadcastAdmin with the wrapped shape.
func (h *Hub) BroadcastOrderRaw(orderNumber string, payload any) {
	h.mu.RLock()
	subs := h.orderSubs[orderNumber]
	targets := make([]*websocket.Conn, 0, len(subs))
	for c := range subs {
		targets = append(targets, c)
	}
	h.mu.RUnlock()
	for _, c := range targets {
		if err := c.WriteJSON(payload); err != nil {
			log.Printf("Hub.BroadcastOrderRaw: write: %v", err)
		}
	}
}
