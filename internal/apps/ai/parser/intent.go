// Package parser holds the rule-based fallback intent extractor used when no
// LLM is configured. It's intentionally narrow: cuisine word, delivery-time
// ceiling, dietary tags, and a "near me" hint. Anything richer (price band,
// vibe, party size) belongs in the LLM path.
package parser

import (
	"regexp"
	"strings"
)

// Intent is the structured representation of a free-text query.
// Field shapes match what the Flutter app expects to round-trip back from
// /api/ai/search.
type Intent struct {
	Cuisine            string   `json:"cuisine,omitempty"`
	MaxDeliveryMinutes *int     `json:"max_delivery_minutes,omitempty"`
	DietaryTags        []string `json:"dietary_tags,omitempty"`
	NearMe             bool     `json:"near_me,omitempty"`
	OriginalQuery      string   `json:"original_query"`
}

// known cuisines kept short; this is the fallback path, not the final taxonomy.
var knownCuisines = []string{
	"italian", "thai", "chinese", "japanese", "sushi", "mexican", "indian",
	"vietnamese", "korean", "mediterranean", "greek", "french", "spanish",
	"american", "burger", "burgers", "pizza", "ramen", "bbq", "barbecue",
	"vegan", "vegetarian", "seafood", "steak", "breakfast", "brunch", "salad",
	"sandwich", "sandwiches", "tacos", "noodles",
}

var knownDietary = []string{
	"vegan", "vegetarian", "gluten-free", "gluten free", "halal", "kosher",
	"dairy-free", "dairy free", "nut-free", "nut free",
}

var underMinutesRx = regexp.MustCompile(`(?i)(?:under|less than|<\s*)\s*(\d{1,3})\s*(?:min|minute|minutes)`)
var withinMinutesRx = regexp.MustCompile(`(?i)(?:within|in)\s*(\d{1,3})\s*(?:min|minute|minutes)`)

// Parse extracts an Intent from free text. It never fails — the worst case is
// an empty Intent with the original query echoed back.
func Parse(query string) Intent {
	intent := Intent{OriginalQuery: query}
	q := strings.ToLower(query)

	for _, c := range knownCuisines {
		// match as a whole token so "thai" doesn't fire on "thailand" etc.
		if containsWord(q, c) {
			// Normalize a few synonyms.
			switch c {
			case "burger":
				intent.Cuisine = "burgers"
			case "sandwich":
				intent.Cuisine = "sandwiches"
			case "barbecue":
				intent.Cuisine = "bbq"
			default:
				intent.Cuisine = c
			}
			break
		}
	}

	if m := underMinutesRx.FindStringSubmatch(q); len(m) == 2 {
		if n := atoi(m[1]); n > 0 {
			intent.MaxDeliveryMinutes = &n
		}
	} else if m := withinMinutesRx.FindStringSubmatch(q); len(m) == 2 {
		if n := atoi(m[1]); n > 0 {
			intent.MaxDeliveryMinutes = &n
		}
	}

	seen := make(map[string]struct{})
	for _, d := range knownDietary {
		if strings.Contains(q, d) {
			tag := strings.ReplaceAll(d, " ", "-")
			if _, dup := seen[tag]; dup {
				continue
			}
			seen[tag] = struct{}{}
			intent.DietaryTags = append(intent.DietaryTags, tag)
		}
	}

	if strings.Contains(q, "near me") || strings.Contains(q, "nearby") || strings.Contains(q, "close by") {
		intent.NearMe = true
	}

	return intent
}

func containsWord(s, word string) bool {
	idx := strings.Index(s, word)
	for idx >= 0 {
		left := idx == 0 || !isWordChar(s[idx-1])
		end := idx + len(word)
		right := end >= len(s) || !isWordChar(s[end])
		if left && right {
			return true
		}
		next := strings.Index(s[idx+1:], word)
		if next < 0 {
			return false
		}
		idx = idx + 1 + next
	}
	return false
}

func isWordChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

func atoi(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
