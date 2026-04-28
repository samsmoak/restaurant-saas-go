package parser_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"restaurantsaas/internal/apps/ai/parser"
)

func TestParse_PromptExample(t *testing.T) {
	// The canonical example from the Savorar prompt.
	in := parser.Parse("spicy thai under 30 min near me")
	assert.Equal(t, "thai", in.Cuisine)
	if assert.NotNil(t, in.MaxDeliveryMinutes) {
		assert.Equal(t, 30, *in.MaxDeliveryMinutes)
	}
	assert.True(t, in.NearMe)
	assert.Equal(t, "spicy thai under 30 min near me", in.OriginalQuery)
}

func TestParse_DietaryDetection(t *testing.T) {
	in := parser.Parse("vegan gluten-free bowls")
	// Vegan is also a cuisine in our list, so it lands as the cuisine here;
	// dietary tags still pick up the gluten-free token.
	assert.Contains(t, in.DietaryTags, "gluten-free")
}

func TestParse_DietaryWithSpace(t *testing.T) {
	in := parser.Parse("anything dairy free near me")
	assert.Contains(t, in.DietaryTags, "dairy-free")
	assert.True(t, in.NearMe)
}

func TestParse_SynonymNormalisation(t *testing.T) {
	cases := map[string]string{
		"the best burger in town":     "burgers",
		"slow-smoked barbecue please":  "bbq",
		"a quick sandwich":            "sandwiches",
		"date-night sushi":            "sushi",
		"italian for two":             "italian",
	}
	for q, want := range cases {
		t.Run(q, func(t *testing.T) {
			assert.Equal(t, want, parser.Parse(q).Cuisine)
		})
	}
}

func TestParse_WithinMinutes(t *testing.T) {
	in := parser.Parse("noodles within 20 minutes")
	if assert.NotNil(t, in.MaxDeliveryMinutes) {
		assert.Equal(t, 20, *in.MaxDeliveryMinutes)
	}
}

func TestParse_NoMatches(t *testing.T) {
	in := parser.Parse("hello world")
	assert.Empty(t, in.Cuisine)
	assert.Nil(t, in.MaxDeliveryMinutes)
	assert.Empty(t, in.DietaryTags)
	assert.False(t, in.NearMe)
}

func TestParse_WordBoundary(t *testing.T) {
	// "thailand" must not match the cuisine "thai".
	in := parser.Parse("travel guide for thailand")
	assert.Empty(t, in.Cuisine)
}
