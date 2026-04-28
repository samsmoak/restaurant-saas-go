package service

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"

	aiClient "restaurantsaas/internal/apps/ai/client"
	"restaurantsaas/internal/apps/ai/parser"
	discoveryModel "restaurantsaas/internal/apps/discovery/model"
	discoverySvc "restaurantsaas/internal/apps/discovery/service"
)

// fallbackChatReply is the deterministic response returned when no LLM is
// configured. The Savorar Flutter app surfaces this string as-is.
const fallbackChatReply = "AI is unavailable right now — please try again later."

// Action is an inline payload the assistant can return alongside a textual
// reply. The mobile client renders Action.Type == "search" as a horizontal
// restaurant carousel inside the assistant bubble.
type Action struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
}

type ChatRequest struct {
	Messages []ChatMessage `json:"messages"`
	Context  *ChatContext  `json:"context,omitempty"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatContext struct {
	Lat *float64 `json:"lat,omitempty"`
	Lng *float64 `json:"lng,omitempty"`
}

type ChatResponse struct {
	Reply   string   `json:"reply"`
	Actions []Action `json:"actions,omitempty"`
}

type SearchRequest struct {
	Query string   `json:"query"`
	Lat   *float64 `json:"lat,omitempty"`
	Lng   *float64 `json:"lng,omitempty"`
}

type SearchResponse struct {
	Intent      parser.Intent                    `json:"intent"`
	Restaurants []*discoveryModel.RestaurantResult `json:"restaurants"`
}

type AIService interface {
	Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error)
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
}

type aiService struct {
	llm       aiClient.Client // may be nil when LLM_API_KEY unset
	discovery discoverySvc.DiscoveryService
}

func NewAIService(llm aiClient.Client, discovery discoverySvc.DiscoveryService) AIService {
	return &aiService{llm: llm, discovery: discovery}
}

// Search always parses the query rule-based first to produce a structured
// intent; then it asks the LLM for a refinement only if one is configured.
// If the LLM returns invalid JSON or errors, we fall back to the rule-based
// intent — the endpoint must not 5xx for a flaky LLM either.
func (s *aiService) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	if strings.TrimSpace(req.Query) == "" {
		return nil, errors.New("query is required")
	}
	intent := parser.Parse(req.Query)

	// LLM refinement (best-effort).
	if s.llm != nil {
		if refined, err := s.refineIntent(ctx, req.Query, intent); err == nil {
			intent = refined
		} else {
			log.Printf("aiService.Search: LLM refine failed (using rule-based): %v", err)
		}
	}

	// Translate intent → discovery.ListParams. We use the cuisine + free-text
	// query against the existing discovery search (Mongo $text + composite
	// ranking). If the intent says "near me" but no lat/lng was provided,
	// we simply skip the geo bias rather than guessing.
	listParams := discoveryModel.ListParams{
		Q:       req.Query,
		Cuisine: intent.Cuisine,
		Lat:     req.Lat,
		Lng:     req.Lng,
		Limit:   25,
	}
	results, _, err := s.discovery.Search(ctx, listParams)
	if err != nil {
		// Even if discovery fails, surface the structured intent so the
		// client can render *something*; this matches the spec's intent of
		// graceful degradation.
		return &SearchResponse{Intent: intent, Restaurants: []*discoveryModel.RestaurantResult{}}, nil
	}

	// Optional client-side post-filter for max_delivery_minutes — Mongo
	// aggregation already ranks on speed, so this only trims hard outliers.
	if intent.MaxDeliveryMinutes != nil {
		max := *intent.MaxDeliveryMinutes
		filtered := make([]*discoveryModel.RestaurantResult, 0, len(results))
		for _, r := range results {
			if r.PublicView == nil || r.PublicView.AveragePrepMinutes == 0 || r.PublicView.AveragePrepMinutes <= max {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	return &SearchResponse{Intent: intent, Restaurants: results}, nil
}

// refineIntent asks the LLM to extract structured fields and merges them
// over the rule-based intent. Errors are non-fatal.
func (s *aiService) refineIntent(ctx context.Context, query string, base parser.Intent) (parser.Intent, error) {
	system := `You extract structured intent from a single user query about food delivery.
Return ONLY a JSON object with these optional fields, no prose:
  cuisine: string (lowercase, e.g. "thai", "italian", "sushi")
  max_delivery_minutes: integer
  dietary_tags: array of strings (e.g. ["vegan", "gluten-free"])
  near_me: boolean
Omit fields you can't determine. Do not invent.`
	out, err := s.llm.Complete(ctx, []aiClient.Message{
		{Role: aiClient.RoleUser, Content: query},
	}, aiClient.CompleteOptions{System: system, Temperature: 0, MaxTokens: 256})
	if err != nil {
		return base, err
	}
	out = stripFenced(out)
	var parsed struct {
		Cuisine            string   `json:"cuisine"`
		MaxDeliveryMinutes *int     `json:"max_delivery_minutes"`
		DietaryTags        []string `json:"dietary_tags"`
		NearMe             *bool    `json:"near_me"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		return base, err
	}
	merged := base
	if parsed.Cuisine != "" {
		merged.Cuisine = parsed.Cuisine
	}
	if parsed.MaxDeliveryMinutes != nil {
		merged.MaxDeliveryMinutes = parsed.MaxDeliveryMinutes
	}
	if len(parsed.DietaryTags) > 0 {
		merged.DietaryTags = parsed.DietaryTags
	}
	if parsed.NearMe != nil {
		merged.NearMe = *parsed.NearMe
	}
	return merged, nil
}

// stripFenced removes ```json … ``` fences some models wrap JSON in.
func stripFenced(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		// drop first line
		if nl := strings.IndexByte(s, '\n'); nl >= 0 {
			s = s[nl+1:]
		}
		if i := strings.LastIndex(s, "```"); i >= 0 {
			s = s[:i]
		}
	}
	return strings.TrimSpace(s)
}

func (s *aiService) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if s.llm == nil {
		return &ChatResponse{Reply: fallbackChatReply}, nil
	}
	msgs := make([]aiClient.Message, 0, len(req.Messages))
	for _, m := range req.Messages {
		role := aiClient.Role(strings.ToLower(strings.TrimSpace(m.Role)))
		if role != aiClient.RoleUser && role != aiClient.RoleAssistant && role != aiClient.RoleSystem {
			role = aiClient.RoleUser
		}
		msgs = append(msgs, aiClient.Message{Role: role, Content: m.Content})
	}
	if len(msgs) == 0 {
		return nil, errors.New("at least one message is required")
	}

	system := `You are Savorar's helpful food-delivery concierge. Recommend restaurants concisely. Keep replies under 80 words.`

	reply, err := s.llm.Complete(ctx, msgs, aiClient.CompleteOptions{
		System: system, Temperature: 0.4, MaxTokens: 512,
	})
	if err != nil {
		// LLM call failed — degrade gracefully rather than 5xx.
		log.Printf("aiService.Chat: LLM error (using fallback reply): %v", err)
		return &ChatResponse{Reply: fallbackChatReply}, nil
	}

	resp := &ChatResponse{Reply: strings.TrimSpace(reply)}

	// If the most recent user message looks like a search request, attach a
	// search action carrying the discovery results so the client can render
	// an inline restaurant carousel without a second round-trip.
	if last := lastUserMessage(req.Messages); last != "" && looksLikeRestaurantQuery(last) {
		var lat, lng *float64
		if req.Context != nil {
			lat, lng = req.Context.Lat, req.Context.Lng
		}
		searchResp, err := s.Search(ctx, &SearchRequest{Query: last, Lat: lat, Lng: lng})
		if err == nil && searchResp != nil && len(searchResp.Restaurants) > 0 {
			resp.Actions = append(resp.Actions, Action{
				Type: "search",
				Payload: map[string]any{
					"query":       last,
					"intent":      searchResp.Intent,
					"restaurants": searchResp.Restaurants,
				},
			})
		}
	}
	return resp, nil
}

func lastUserMessage(msgs []ChatMessage) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if strings.ToLower(msgs[i].Role) == "user" {
			return msgs[i].Content
		}
	}
	return ""
}

// looksLikeRestaurantQuery is a deliberately-loose heuristic that fires when
// the user is plainly asking for food. False positives are cheap (the carousel
// just stays empty); false negatives only mean "no inline carousel," not a
// broken response.
func looksLikeRestaurantQuery(q string) bool {
	q = strings.ToLower(q)
	for _, kw := range []string{"food", "restaurant", "eat", "dinner", "lunch", "delivery", "takeout", "take-out", "hungry", "cuisine", "near me", "menu"} {
		if strings.Contains(q, kw) {
			return true
		}
	}
	intent := parser.Parse(q)
	return intent.Cuisine != "" || intent.MaxDeliveryMinutes != nil
}
