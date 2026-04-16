package usage

import (
	"testing"

	"opencode-cli/internal/model"
)

func TestBuildRowsUsesStoredCostAndPricingFallback(t *testing.T) {
	t.Parallel()

	priceCatalog := &PriceCatalog{
		pricesByKey: map[string]Price{
			"openai/gpt-4.1": {
				InputPer1M:  2,
				OutputPer1M: 4,
				Source:      "litellm",
			},
		},
	}

	messages := []model.MessageRecord{
		{
			ID:          "msg-1",
			SessionID:   "session-1",
			Role:        "assistant",
			TimeCreated: 1,
			ProviderID:  "openai",
			ModelID:     "gpt-4.1",
			Cost:        1.25,
			Tokens: model.TokenUsage{
				Input: 1000,
			},
		},
		{
			ID:          "msg-2",
			SessionID:   "session-1",
			Role:        "assistant",
			TimeCreated: 2,
			ProviderID:  "openai",
			ModelID:     "gpt-4.1",
			Tokens: model.TokenUsage{
				Input:  500000,
				Output: 250000,
			},
		},
	}

	rows, totals := BuildRows(messages, nil, GroupDaily, priceCatalog)
	if len(rows) != 1 {
		t.Fatalf("expected one row, got %d", len(rows))
	}

	row := rows[0]
	expectedCatalogCost := tokenCost(500000, 2) + tokenCost(250000, 4)
	expectedTotalCost := 1.25 + expectedCatalogCost

	if row.TotalCost != expectedTotalCost {
		t.Fatalf("expected total cost %.6f, got %.6f", expectedTotalCost, row.TotalCost)
	}
	if totals.TotalCost != expectedTotalCost {
		t.Fatalf("expected totals cost %.6f, got %.6f", expectedTotalCost, totals.TotalCost)
	}
	if row.InputCost != tokenCost(500000, 2) {
		t.Fatalf("expected only priced message input cost to be counted, got %.6f", row.InputCost)
	}
	if row.OutputCost != tokenCost(250000, 4) {
		t.Fatalf("expected only priced message output cost to be counted, got %.6f", row.OutputCost)
	}
	if !row.PriceFound {
		t.Fatal("expected row to report price information")
	}
}

func TestBuildRowsUsesStoredCostWithoutCatalog(t *testing.T) {
	t.Parallel()

	messages := []model.MessageRecord{{
		ID:          "msg-1",
		SessionID:   "session-1",
		Role:        "assistant",
		TimeCreated: 1,
		ProviderID:  "openai",
		ModelID:     "gpt-4.1",
		Cost:        0.75,
		Tokens: model.TokenUsage{
			Input: 100,
		},
	}}

	rows, totals := BuildRows(messages, nil, GroupDaily, nil)
	if len(rows) != 1 {
		t.Fatalf("expected one row, got %d", len(rows))
	}
	if rows[0].TotalCost != 0.75 {
		t.Fatalf("expected stored cost to be preserved, got %.6f", rows[0].TotalCost)
	}
	if totals.TotalCost != 0.75 {
		t.Fatalf("expected totals stored cost to be preserved, got %.6f", totals.TotalCost)
	}
}
