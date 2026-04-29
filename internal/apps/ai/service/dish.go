package service

import (
	"context"
	"strings"
	"unicode"

	"go.mongodb.org/mongo-driver/bson/primitive"

	discoveryModel "restaurantsaas/internal/apps/discovery/model"
	menuModel "restaurantsaas/internal/apps/menu/model"
	"restaurantsaas/internal/pkg/money"
)

// Dish is the customer-facing shape used by every Savor-AI surface
// (BACKEND_REQUIREMENTS.md §7).  It joins a MenuItem to its parent
// Restaurant and adds a few derived fields (`match`, `flavor`,
// `emoji`) that drive the chat / recommend / search UI.
type Dish struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	RestaurantID   string         `json:"restaurant_id"`
	RestaurantName string         `json:"restaurant_name"`
	Match          int            `json:"match"`
	PriceCents     int64          `json:"price_cents"`
	ImageURL       string         `json:"image_url,omitempty"`
	Emoji          string         `json:"emoji,omitempty"`
	Tags           []string       `json:"tags"`
	Spice          int            `json:"spice,omitempty"`
	PrepMinutes    int            `json:"prep_minutes,omitempty"`
	Dietary        string         `json:"dietary,omitempty"`
	Description    string         `json:"description,omitempty"`
	Flavor         *FlavorProfile `json:"flavor,omitempty"`
}

// FlavorProfile is the radar-chart payload required by the dish detail
// (`A7`) screen.  Each axis is 0–10.
type FlavorProfile struct {
	Umami int `json:"umami"`
	Sweet int `json:"sweet"`
	Sour  int `json:"sour"`
	Salty int `json:"salty"`
	Spicy int `json:"spicy"`
	Rich  int `json:"rich"`
}

// DishFilter captures the chip filters from
// BACKEND_REQUIREMENTS.md §7 GET /api/ai/search.
type DishFilter struct {
	Spicy        bool
	Citrus       bool
	Under15      bool // price under $15
	Under30Min   bool
	Rating45Plus bool
}

// dishFromMenuItem turns a MenuItem (+restaurant) into a Dish DTO.
func dishFromMenuItem(item *menuModel.MenuItem, restaurantName string) *Dish {
	if item == nil {
		return nil
	}
	tags := item.Tags
	if tags == nil {
		tags = []string{}
	}
	d := &Dish{
		ID:             item.ID.Hex(),
		Name:           item.Name,
		RestaurantID:   item.RestaurantID.Hex(),
		RestaurantName: restaurantName,
		PriceCents:     money.ToCents(item.BasePrice),
		ImageURL:       item.ImageURL,
		Tags:           tags,
		Description:    item.Description,
		Emoji:          emojiForTags(tags, item.Name),
		Flavor:         flavorFromTags(tags),
	}
	d.Spice = d.Flavor.Spicy
	d.Dietary = dietaryFromTags(tags)
	return d
}

// emojiForTags maps a coarse cuisine / flavour tag onto a single
// emoji.  Deterministic, no LLM required — the Savorar cards render
// gracefully with an empty string when no match is found.
func emojiForTags(tags []string, name string) string {
	bag := strings.ToLower(strings.Join(tags, " ") + " " + name)
	switch {
	case strings.Contains(bag, "noodle"), strings.Contains(bag, "ramen"), strings.Contains(bag, "pho"), strings.Contains(bag, "soup"):
		return "🍜"
	case strings.Contains(bag, "sushi"), strings.Contains(bag, "sashimi"):
		return "🍣"
	case strings.Contains(bag, "burger"):
		return "🍔"
	case strings.Contains(bag, "pizza"):
		return "🍕"
	case strings.Contains(bag, "taco"):
		return "🌮"
	case strings.Contains(bag, "biryani"), strings.Contains(bag, "rice"):
		return "🍛"
	case strings.Contains(bag, "salad"):
		return "🥗"
	case strings.Contains(bag, "steak"):
		return "🥩"
	case strings.Contains(bag, "chicken"):
		return "🍗"
	case strings.Contains(bag, "spicy"):
		return "🌶"
	case strings.Contains(bag, "dessert"), strings.Contains(bag, "cake"):
		return "🍰"
	}
	return ""
}

func flavorFromTags(tags []string) *FlavorProfile {
	bag := strings.ToLower(strings.Join(tags, " "))
	out := &FlavorProfile{Umami: 5, Sweet: 4, Sour: 3, Salty: 4, Spicy: 3, Rich: 4}
	if strings.Contains(bag, "spicy") {
		out.Spicy = 8
	}
	if strings.Contains(bag, "mild") {
		out.Spicy = 2
	}
	if strings.Contains(bag, "citrus") || strings.Contains(bag, "sour") {
		out.Sour = 7
	}
	if strings.Contains(bag, "umami") {
		out.Umami = 8
	}
	if strings.Contains(bag, "sweet") {
		out.Sweet = 7
	}
	if strings.Contains(bag, "rich") || strings.Contains(bag, "creamy") {
		out.Rich = 7
	}
	return out
}

func dietaryFromTags(tags []string) string {
	for _, t := range tags {
		l := strings.ToLower(t)
		switch l {
		case "vegan":
			return "Vegan"
		case "vegetarian":
			return "Vegetarian"
		case "halal":
			return "Halal"
		case "kosher":
			return "Kosher"
		case "gluten-free", "gluten free":
			return "Gluten-Free"
		case "pescatarian":
			return "Pescatarian"
		}
	}
	return ""
}

// scoreDish blends keyword overlap with taste-profile alignment.
// Returns a 0..100 integer that the UI shows as a "match" percentage.
func scoreDish(d *Dish, query string, taste *TasteFingerprint) int {
	q := strings.ToLower(strings.TrimSpace(query))
	score := 60.0 // baseline
	if q != "" {
		bag := strings.ToLower(d.Name + " " + d.Description + " " + strings.Join(d.Tags, " "))
		for _, w := range tokenise(q) {
			if w == "" {
				continue
			}
			if strings.Contains(bag, w) {
				score += 6
			}
		}
	}
	if taste != nil && d.Flavor != nil {
		// Spice alignment: closer is better, bonus up to 12 points.
		diff := abs(taste.Spice - d.Flavor.Spicy)
		score += float64(12 - diff)
		// Richness / acidity / carbs bonuses, smaller weights.
		score += float64(6 - abs(taste.Richness-d.Flavor.Rich))
		score += float64(6 - abs(taste.Acidity-d.Flavor.Sour))
	}
	if score > 99 {
		score = 99
	}
	if score < 50 {
		score = 50
	}
	return int(score + 0.5)
}

// hydrateDishes turns a list of menu_item ObjectIDs into Dish DTOs by
// looking each item up. Used by favorites and the chat answer event.
func (s *aiService) hydrateDishes(ctx context.Context, ids []primitive.ObjectID) []any {
	if len(ids) == 0 || s.menuSvc == nil {
		return []any{}
	}
	// Group ids by restaurant (we don't know it ahead of time so do a
	// single pass over all menu collections via the unscoped lookup).
	out := make([]any, 0, len(ids))
	for _, id := range ids {
		// We don't have the restaurant id, so we can't call
		// menuSvc.GetItemByID (which is restaurant-scoped). Fall back
		// to the underlying menu repo via the menu service interface
		// extension below. If that lookup fails, skip the dish — it's
		// likely been deleted.
		if s.menuLookup == nil {
			break
		}
		item, _ := s.menuLookup(ctx, id)
		if item == nil {
			continue
		}
		var name string
		if s.restSvc != nil {
			if r, _ := s.restSvc.GetByID(ctx, item.RestaurantID); r != nil {
				name = r.Name
			}
		}
		out = append(out, dishFromMenuItem(item, name))
	}
	return out
}

// listDishes runs the discovery + menu-fetch pipeline and returns the
// top-N dishes ranked against the optional query / taste profile.
func (s *aiService) listDishes(ctx context.Context, query string, lat, lng *float64, taste *TasteFingerprint, filter DishFilter, limit int) []*Dish {
	if limit <= 0 {
		limit = 25
	}
	// 1) Discovery: pick candidate restaurants.
	listParams := discoveryModel.ListParams{Q: query, Lat: lat, Lng: lng, Limit: 15}
	rests, _, err := s.discovery.Search(ctx, listParams)
	if err != nil || len(rests) == 0 {
		// Fall back to the un-queried list so the chat still has
		// something to render in seed environments.
		rests, _, _ = s.discovery.List(ctx, discoveryModel.ListParams{Lat: lat, Lng: lng, Limit: 15})
	}
	if filter.Rating45Plus {
		filtered := rests[:0]
		for _, r := range rests {
			if r.PublicView != nil && r.PublicView.AverageRating >= 4.5 {
				filtered = append(filtered, r)
			}
		}
		rests = filtered
	}
	if filter.Under30Min {
		filtered := rests[:0]
		for _, r := range rests {
			if r.PublicView != nil && r.PublicView.EstimatedDeliveryTime > 0 && r.PublicView.EstimatedDeliveryTime <= 30 {
				filtered = append(filtered, r)
			}
		}
		rests = filtered
	}
	// 2) Menu fetch: pull all menu items from the candidate set.
	out := make([]*Dish, 0, limit*2)
	for _, r := range rests {
		if r == nil || r.PublicView == nil {
			continue
		}
		menus, err := s.menuSvc.PublicMenu(ctx, r.PublicView.ID)
		if err != nil {
			continue
		}
		for _, group := range menus {
			for _, it := range group.Items {
				if !it.IsAvailable {
					continue
				}
				d := dishFromMenuItem(it, r.PublicView.Name)
				if d == nil {
					continue
				}
				if filter.Under15 && d.PriceCents > 1500 {
					continue
				}
				if filter.Spicy && d.Flavor != nil && d.Flavor.Spicy < 6 {
					continue
				}
				if filter.Citrus && d.Flavor != nil && d.Flavor.Sour < 6 {
					continue
				}
				d.Match = scoreDish(d, query, taste)
				out = append(out, d)
			}
		}
	}
	// 3) Rank.
	sortDishesByMatchDesc(out)
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func sortDishesByMatchDesc(d []*Dish) {
	for i := 1; i < len(d); i++ {
		for j := i; j > 0 && d[j].Match > d[j-1].Match; j-- {
			d[j], d[j-1] = d[j-1], d[j]
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func tokenise(s string) []string {
	out := []string{}
	cur := strings.Builder{}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur.WriteRune(r)
		} else {
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}
