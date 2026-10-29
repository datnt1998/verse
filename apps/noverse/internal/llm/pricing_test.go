package llm

import "testing"

func TestModelPricesKnownRegistryModel(t *testing.T) {
	usage := &Usage{Model: "anthropic/claude-sonnet-4.6", Input: 1000, Output: 100, CacheRead: 200}
	pricing := pricingForUsage(nil, usage)
	if pricing == nil {
		t.Fatal("known model has no core pricing")
	}
	cost := CalculateCost(pricing, usage)
	if cost == nil || cost.Input <= 0 || cost.Output <= 0 || cost.CacheRead <= 0 {
		t.Fatalf("missing per-category cost: %+v", cost)
	}
}
func TestModelDoesNotPriceUnknownAlias(t *testing.T) {
	if got := pricingForUsage(nil, &Usage{Model: "m"}); got != nil {
		t.Fatalf("fuzzy registry match fabricated pricing: %+v", got)
	}
}
