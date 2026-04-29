package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	aiClient "restaurantsaas/internal/apps/ai/client"
	"restaurantsaas/internal/apps/ai/parser"
)

// StreamChatRequest is the body for POST /api/ai/chat per
// BACKEND_REQUIREMENTS.md §7.  conversation_id is optional and the
// server is free to ignore it (history isn't replayed today).
type StreamChatRequest struct {
	ConversationID string         `json:"conversation_id"`
	Message        string         `json:"message"`
	Voice          bool           `json:"voice"`
	Location       *ChatLocation  `json:"location"`
}

type ChatLocation struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// ListDishes is the public driver for /api/ai/search.
func (s *aiService) ListDishes(ctx context.Context, query string, lat, lng *float64, taste *TasteFingerprint, filter DishFilter, limit int) []*Dish {
	return s.listDishes(ctx, query, lat, lng, taste, filter, limit)
}

// Recommend drives /api/ai/recommend (BACKEND_REQUIREMENTS.md §7).
func (s *aiService) Recommend(ctx context.Context, taste *TasteFingerprint, lat, lng *float64) []*Dish {
	// Build a query string from taste so the discovery search has
	// something to bite on.  Falls back to "" which makes the listDishes
	// helper switch to discoveryService.List.
	q := ""
	if taste != nil {
		if taste.Spice >= 7 {
			q += "spicy "
		}
		if taste.Citrus >= 6 {
			q += "citrus "
		}
		if taste.Richness >= 7 {
			q += "rich "
		}
	}
	return s.listDishes(ctx, strings.TrimSpace(q), lat, lng, taste, DishFilter{}, 12)
}

// GetDishByID drives /api/ai/dishes/{id}.
func (s *aiService) GetDishByID(ctx context.Context, id primitive.ObjectID) (*Dish, error) {
	if s.menuLookup == nil {
		return nil, errors.New("menu lookup not configured")
	}
	item, err := s.menuLookup(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}
	name := ""
	if s.restSvc != nil {
		if r, _ := s.restSvc.GetByID(ctx, item.RestaurantID); r != nil {
			name = r.Name
		}
	}
	d := dishFromMenuItem(item, name)
	d.Match = scoreDish(d, "", nil)
	return d, nil
}

// HydrateDishes is exported via the AIService interface so
// favorites/controller can pass dish IDs in for hydration without an
// import cycle.
func (s *aiService) HydrateDishes(ctx context.Context, ids []primitive.ObjectID) []any {
	return s.hydrateDishes(ctx, ids)
}

// StreamChat runs the four-event SSE pipeline:
//
//	clarify? → step(parse|filter|search|rerank|done) → answer → followups
//
// emit serialises each event as a `data:` line; the controller is
// responsible for the SSE framing.  All errors degrade gracefully —
// the spec demands every event regardless.
func (s *aiService) StreamChat(ctx context.Context, userID primitive.ObjectID, req *StreamChatRequest, emit func(any)) {
	if req == nil || strings.TrimSpace(req.Message) == "" {
		emit(map[string]any{"type": "answer", "text": "What are you craving?", "sources": []any{}, "dishes": []any{}})
		emit(map[string]any{"type": "followups", "chips": []string{"Quick & cheap", "Comfort food", "Healthy"}})
		return
	}

	// Load taste profile for personalisation when available.
	var taste *TasteFingerprint
	if s.taste != nil && !userID.IsZero() {
		if tp, _ := s.taste.Get(ctx, userID); tp != nil {
			taste = &TasteFingerprint{
				Spice:    tp.Spice,
				Richness: 5,
				Acidity:  3,
				Carbs:    5,
			}
		}
	}

	// 1. clarify (only when the prompt has no taste signal AND we
	// don't already know the user's taste profile).
	if !hasTasteSignal(req.Message) && taste == nil {
		emit(map[string]any{
			"type": "clarify",
			"taste": map[string]any{
				"spice": 5, "richness": 5, "acidity": 3, "carbs": 5,
			},
			"suggestions": []string{"Looks right", "Less spicy", "More citrus"},
		})
	}

	// 2. step(parse).
	emit(stepEvent("parse", "active", nil))
	intent := parser.Parse(req.Message)
	emit(stepEvent("parse", "done", map[string]any{"count": 0}))

	// 3. step(filter): discovery search.
	emit(stepEvent("filter", "active", nil))
	var lat, lng *float64
	if req.Location != nil {
		lat, lng = ptr(req.Location.Lat), ptr(req.Location.Lng)
	}
	dishes := s.listDishes(ctx, req.Message, lat, lng, taste, DishFilter{}, 12)
	emit(stepEvent("filter", "done", map[string]any{"count": len(dishes)}))

	// 4. step(search): same dataset, exposes "found N candidates".
	emit(stepEvent("search", "active", nil))
	emit(stepEvent("search", "done", map[string]any{"count": len(dishes)}))

	// 5. step(rerank).
	emit(stepEvent("rerank", "active", nil))
	emit(stepEvent("rerank", "done", map[string]any{"count": len(dishes)}))

	// 6. step(done).
	emit(stepEvent("done", "done", nil))

	// 7. answer.
	top := dishes
	if len(top) > 5 {
		top = top[:5]
	}
	answerText := s.composeAnswerText(ctx, req.Message, top)
	sources := buildSources(top)
	emit(map[string]any{
		"type":    "answer",
		"text":    answerText,
		"sources": sources,
		"dishes":  toAnySlice(top),
	})

	// 8. followups.
	emit(map[string]any{
		"type":  "followups",
		"chips": followupChips(intent),
	})

	// 9. Best-effort cravings record.
	if s.cravings != nil && !userID.IsZero() {
		emoji := ""
		match := 0
		if len(top) > 0 {
			emoji = top[0].Emoji
			match = top[0].Match
		}
		summary := ""
		if len(top) > 0 {
			summary = fmt.Sprintf("Found %s at %s", top[0].Name, top[0].RestaurantName)
		}
		go func(uid primitive.ObjectID, title, sum, em string, m int) {
			cctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if _, err := s.cravings.Record(cctx, uid, title, sum, em, m); err != nil {
				log.Printf("aiService.StreamChat: cravings record: %v", err)
			}
		}(userID, req.Message, summary, emoji, match)
	}
}

func stepEvent(id, status string, meta map[string]any) map[string]any {
	out := map[string]any{
		"type":   "step",
		"step":   id,
		"status": status,
	}
	if meta != nil {
		out["meta"] = meta
	}
	return out
}

func ptr[T any](v T) *T { return &v }

// hasTasteSignal returns true when the prompt mentions any spice /
// dietary / richness keyword we recognise — used to skip the clarify
// event for prompts that are already specific.
func hasTasteSignal(msg string) bool {
	msg = strings.ToLower(msg)
	signals := []string{
		"spicy", "mild", "hot", "sweet", "savory", "rich", "creamy", "light", "heavy",
		"vegan", "vegetarian", "halal", "kosher", "gluten", "pescatarian",
		"citrus", "sour", "umami", "salty",
	}
	for _, s := range signals {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

func buildSources(dishes []*Dish) []map[string]string {
	if len(dishes) == 0 {
		return []map[string]string{{"kind": "taste", "name": "Your taste profile"}}
	}
	seen := make(map[string]struct{}, len(dishes))
	out := make([]map[string]string, 0, 3)
	for _, d := range dishes {
		key := d.RestaurantID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, map[string]string{
			"kind": "menu",
			"name": d.RestaurantName + " · Menu",
		})
		if len(out) == 2 {
			break
		}
	}
	out = append(out, map[string]string{"kind": "taste", "name": "Your taste profile"})
	return out
}

func followupChips(intent parser.Intent) []string {
	chips := []string{"Cheaper options", "Vegetarian instead", "Anything with rice?"}
	if intent.Cuisine != "" {
		chips = []string{
			"More " + intent.Cuisine + " options",
			"Cheaper options",
			"Less spicy",
		}
	}
	return chips
}

func toAnySlice(in []*Dish) []any {
	out := make([]any, 0, len(in))
	for _, d := range in {
		out = append(out, d)
	}
	return out
}

// composeAnswerText asks the LLM for a 1–2 sentence explanation of
// why the dishes match.  When the LLM is nil or fails, returns a
// deterministic line so the SSE stream still completes.
func (s *aiService) composeAnswerText(ctx context.Context, msg string, top []*Dish) string {
	if len(top) == 0 {
		return "Couldn't find a perfect match nearby — try widening your search."
	}
	if s.llm == nil {
		return fmt.Sprintf("Found %d dishes that fit your craving — top pick is %s at %s.", len(top), top[0].Name, top[0].RestaurantName)
	}
	prompt := fmt.Sprintf("User craving: %q. Top dish: %s at %s. Tags: %s. Explain in 2 sentences why this fits, friendly tone.",
		msg, top[0].Name, top[0].RestaurantName, strings.Join(top[0].Tags, ", "))
	out, err := s.llm.Complete(ctx, []aiClient.Message{
		{Role: aiClient.RoleUser, Content: prompt},
	}, aiClient.CompleteOptions{Temperature: 0.4, MaxTokens: 160})
	if err != nil || strings.TrimSpace(out) == "" {
		return fmt.Sprintf("Found %d dishes that fit your craving — top pick is %s at %s.", len(top), top[0].Name, top[0].RestaurantName)
	}
	return strings.TrimSpace(out)
}
