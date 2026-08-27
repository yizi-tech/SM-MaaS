package handler

import (
	"strings"

	"github.com/mass-platform/backend/internal/billing"
	"github.com/mass-platform/backend/internal/model"
	"github.com/mass-platform/backend/internal/repository"
	"github.com/shopspring/decimal"
)

// buildModelMarketplace computes the invocable model list under a single set
// of rules shared by the public marketplace (GET /api/v1/models) and the
// OpenAI-compatible gateway (GET /api/v1/llm/models): a model is exposed only
// when an admin price is configured in model_prices (a model price IS the
// allowlist) AND an enabled channel serves it. Keeping both callers on one
// implementation guarantees they can never drift.
func buildModelMarketplace(channelRepo *repository.ChannelRepository, billingService *billing.BillingService) []model.ModelCatalogEntry {
	catalog := model.GetModelCatalog()

	var enabled []model.LLMChannel
	if channelRepo != nil {
		list, err := channelRepo.ListEnabled()
		if err != nil || len(list) == 0 {
			list = nil
		}
		enabled = list
	}

	priceByModel := make(map[string]model.ModelPrice)
	if billingService != nil {
		for _, p := range billingService.ListEnabledPrices() {
			if _, dup := priceByModel[p.Model]; !dup {
				priceByModel[p.Model] = p
			}
		}
	}

	perMillion := decimal.NewFromInt(1_000_000)
	formatPrice := func(v decimal.Decimal) string {
		return "¥" + v.Mul(perMillion).Round(4).String()
	}

	// cachePrices resolves the effective cache read/write per-token prices.
	// Without an explicit admin price the same fallbacks as billing apply:
	// cache read = input×10%, cache write = input×125%.
	cachePrices := func(p model.ModelPrice) (decimal.Decimal, decimal.Decimal) {
		read := p.InputPrice.Mul(decimal.RequireFromString(billing.CachedInputDiscountFactor))
		write := p.InputPrice.Mul(decimal.RequireFromString(billing.CacheWriteMarkupFactor))
		if p.CacheReadPrice.Valid {
			read = p.CacheReadPrice.Decimal
		}
		if p.CacheWritePrice.Valid {
			write = p.CacheWritePrice.Decimal
		}
		return read, write
	}

	filtered := make([]model.ModelCatalogEntry, 0, len(catalog))
	served := make(map[string]bool) // model already exposed
	for _, m := range catalog {
		price, ok := priceByModel[m.ID]
		if !ok {
			continue // no admin price configured -> not invocable -> hidden
		}
		if len(enabled) > 0 {
			matched := false
			for i := range enabled {
				if enabled[i].MatchesModel(m.ID) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		cacheRead, cacheWrite := cachePrices(price)
		m.InputPrice = formatPrice(price.InputPrice)
		m.OutputPrice = formatPrice(price.OutputPrice)
		m.CacheReadPrice = formatPrice(cacheRead)
		m.CacheWritePrice = formatPrice(cacheWrite)
		m.SupportUnlimited = price.SupportUnlimited
		m.UnlimitedEnabled = price.UnlimitedEnabled
		served[m.ID] = true
		filtered = append(filtered, m)
	}

	// Add concrete channel models that are missing from the built-in catalog,
	// so models served by real upstream channels appear in the marketplace —
	// but only when an admin price has been configured for them. Wildcard
	// patterns (e.g. "gpt-*") cannot be enumerated and are skipped.
	for i := range enabled {
		ch := &enabled[i]
		for _, mm := range ch.Models {
			name := strings.TrimSpace(mm)
			if name == "" || strings.HasSuffix(name, "*") || served[name] {
				continue
			}
			price, ok := priceByModel[name]
			if !ok {
				continue
			}
			cacheRead, cacheWrite := cachePrices(price)
			served[name] = true
			filtered = append(filtered, model.ModelCatalogEntry{
				ID:              name,
				Provider:        ch.Type,
				Name:            name,
				Context:         "-",
				InputPrice:      formatPrice(price.InputPrice),
				OutputPrice:     formatPrice(price.OutputPrice),
				CacheReadPrice:  formatPrice(cacheRead),
				CacheWritePrice: formatPrice(cacheWrite),
				SupportUnlimited: price.SupportUnlimited,
				UnlimitedEnabled: price.UnlimitedEnabled,
				Features:        []string{"按量计费"},
				Status:          "available",
			})
		}
	}

	return filtered
}
