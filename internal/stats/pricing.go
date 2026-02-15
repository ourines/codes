package stats

import "strings"

// ModelPricing holds per-million-token pricing for a model.
type ModelPricing struct {
	InputPerMillion  float64 // USD per 1M input tokens
	OutputPerMillion float64 // USD per 1M output tokens
}

// modelPricing maps model name prefixes to their pricing.
// Cache write = 25% of input price, cache read = 10% of input price.
var modelPricing = map[string]ModelPricing{
	"claude-opus-4":      {InputPerMillion: 15.0, OutputPerMillion: 75.0},
	"claude-sonnet-4-5":  {InputPerMillion: 3.0, OutputPerMillion: 15.0},
	"claude-sonnet-4":    {InputPerMillion: 3.0, OutputPerMillion: 15.0},
	"claude-haiku-3-5":   {InputPerMillion: 0.80, OutputPerMillion: 4.0},
	"claude-haiku-3":     {InputPerMillion: 0.25, OutputPerMillion: 1.25},
}

// defaultPricing is used for unrecognized models (sonnet tier).
var defaultPricing = ModelPricing{InputPerMillion: 3.0, OutputPerMillion: 15.0}

// lookupPricing finds the pricing for a model string.
// It matches by prefix so "claude-sonnet-4-5-20250929" matches "claude-sonnet-4".
func lookupPricing(model string) ModelPricing {
	// Try exact match first
	if p, ok := modelPricing[model]; ok {
		return p
	}
	// Try prefix match (longest prefix wins)
	bestKey := ""
	for key := range modelPricing {
		if strings.HasPrefix(model, key) && len(key) > len(bestKey) {
			bestKey = key
		}
	}
	if bestKey != "" {
		return modelPricing[bestKey]
	}
	return defaultPricing
}

// CalculateCost computes the USD cost for a single API call given model and usage.
func CalculateCost(model string, usage Usage) float64 {
	p := lookupPricing(model)

	inputCost := float64(usage.InputTokens) * p.InputPerMillion / 1_000_000
	outputCost := float64(usage.OutputTokens) * p.OutputPerMillion / 1_000_000

	// Cache write costs 25% of input price
	cacheWriteCost := float64(usage.CacheCreateTokens) * (p.InputPerMillion * 0.25) / 1_000_000
	// Cache read costs 10% of input price
	cacheReadCost := float64(usage.CacheReadTokens) * (p.InputPerMillion * 0.10) / 1_000_000

	return inputCost + outputCost + cacheWriteCost + cacheReadCost
}
