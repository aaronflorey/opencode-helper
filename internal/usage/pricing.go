package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	liteLLMModelPricesURL = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"
	openRouterModelsURL   = "https://openrouter.ai/api/v1/models"
)

type Price struct {
	InputPer1M      float64 `json:"inputPer1M"`
	OutputPer1M     float64 `json:"outputPer1M"`
	CacheReadPer1M  float64 `json:"cacheReadPer1M"`
	CacheWritePer1M float64 `json:"cacheWritePer1M"`
	Source          string  `json:"source"`
}

type PriceCatalog struct {
	pricesByKey map[string]Price
	sources     []string
}

func LoadPriceCatalog(ctx context.Context) (*PriceCatalog, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	prices := make(map[string]Price)
	sources := make([]string, 0, 2)

	if litellmPrices, err := loadLiteLLMPrices(ctx, client); err == nil {
		sources = append(sources, "litellm")
		for key, price := range litellmPrices {
			prices[key] = price
		}
	}

	if openRouterPrices, err := loadOpenRouterPrices(ctx, client); err == nil {
		sources = append(sources, "openrouter")
		for key, price := range openRouterPrices {
			if _, exists := prices[key]; !exists {
				prices[key] = price
			}
		}
	}

	if len(prices) == 0 {
		return nil, fmt.Errorf("failed to load pricing from LiteLLM and OpenRouter")
	}

	return &PriceCatalog{pricesByKey: prices, sources: sources}, nil
}

func (c *PriceCatalog) Sources() []string {
	if c == nil {
		return nil
	}
	out := make([]string, len(c.sources))
	copy(out, c.sources)
	return out
}

func (c *PriceCatalog) Resolve(providerID, modelID string) (Price, bool) {
	if c == nil {
		return Price{}, false
	}

	candidates := modelLookupCandidates(providerID, modelID)
	for _, key := range candidates {
		if p, ok := c.pricesByKey[key]; ok {
			return p, true
		}
	}

	return Price{}, false
}

func modelLookupCandidates(providerID, modelID string) []string {
	provider := strings.ToLower(strings.TrimSpace(providerID))
	model := strings.ToLower(strings.TrimSpace(modelID))
	normalized := NormalizeModelName(provider, model)

	candidates := make([]string, 0, 12)
	push := func(value string) {
		if value == "" {
			return
		}
		for _, existing := range candidates {
			if existing == value {
				return
			}
		}
		candidates = append(candidates, value)
	}

	push(model)
	push(normalized)
	if provider != "" {
		push(provider + "/" + model)
		push(provider + "/" + normalized)
	}

	if slash := strings.LastIndex(model, "/"); slash > -1 && slash < len(model)-1 {
		suffix := model[slash+1:]
		push(suffix)
		push(NormalizeModelName(provider, suffix))
	}

	push(strings.ReplaceAll(normalized, ".", "-"))
	push(strings.ReplaceAll(model, ".", "-"))

	return candidates
}

func loadLiteLLMPrices(ctx context.Context, client *http.Client) (map[string]Price, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, liteLLMModelPricesURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("litellm pricing request failed: %s", resp.Status)
	}

	var payload map[string]map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decoding litellm pricing response: %w", err)
	}

	prices := make(map[string]Price)
	for model, values := range payload {
		inputPerToken := numberFromAny(values["input_cost_per_token"])
		outputPerToken := numberFromAny(values["output_cost_per_token"])
		cacheReadPerToken := numberFromAny(values["cache_read_input_token_cost"])
		cacheWritePerToken := numberFromAny(values["cache_creation_input_token_cost"])

		if inputPerToken == 0 && outputPerToken == 0 && cacheReadPerToken == 0 && cacheWritePerToken == 0 {
			continue
		}

		price := Price{
			InputPer1M:      inputPerToken * 1_000_000,
			OutputPer1M:     outputPerToken * 1_000_000,
			CacheReadPer1M:  cacheReadPerToken * 1_000_000,
			CacheWritePer1M: cacheWritePerToken * 1_000_000,
			Source:          "litellm",
		}

		key := strings.ToLower(strings.TrimSpace(model))
		prices[key] = price
		if slash := strings.LastIndex(key, "/"); slash > -1 && slash < len(key)-1 {
			prices[key[slash+1:]] = price
		}
		prices[NormalizeModelName("", key)] = price
	}

	return prices, nil
}

func loadOpenRouterPrices(ctx context.Context, client *http.Client) (map[string]Price, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, openRouterModelsURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openrouter pricing request failed: %s", resp.Status)
	}

	var payload struct {
		Data []struct {
			ID      string `json:"id"`
			Pricing struct {
				Prompt     string `json:"prompt"`
				Completion string `json:"completion"`
				Cached     string `json:"cached"`
			} `json:"pricing"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decoding openrouter pricing response: %w", err)
	}

	prices := make(map[string]Price)
	for _, item := range payload.Data {
		inputPerToken := numberFromString(item.Pricing.Prompt)
		outputPerToken := numberFromString(item.Pricing.Completion)
		cacheReadPerToken := numberFromString(item.Pricing.Cached)

		if inputPerToken == 0 && outputPerToken == 0 && cacheReadPerToken == 0 {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(item.ID))
		price := Price{
			InputPer1M:      inputPerToken * 1_000_000,
			OutputPer1M:     outputPerToken * 1_000_000,
			CacheReadPer1M:  cacheReadPerToken * 1_000_000,
			CacheWritePer1M: 0,
			Source:          "openrouter",
		}

		prices[key] = price
		if slash := strings.LastIndex(key, "/"); slash > -1 && slash < len(key)-1 {
			suffix := key[slash+1:]
			prices[suffix] = price
			prices[NormalizeModelName("", suffix)] = price
		}
		prices[NormalizeModelName("", key)] = price
	}

	return prices, nil
}

func numberFromString(raw string) float64 {
	if raw == "" {
		return 0
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
	}
	return v
}

func numberFromAny(value interface{}) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		f, _ := v.Float64()
		return f
	case string:
		return numberFromString(v)
	default:
		return 0
	}
}
