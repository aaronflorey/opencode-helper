package usage

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"opencode-cli/internal/model"
)

type GroupType string

const (
	GroupDaily   GroupType = "daily"
	GroupWeekly  GroupType = "weekly"
	GroupMonthly GroupType = "monthly"
	GroupSession GroupType = "session"
)

type Row struct {
	Group          string  `json:"group"`
	SessionID      string  `json:"sessionID,omitempty"`
	SessionTitle   string  `json:"sessionTitle,omitempty"`
	Model          string  `json:"model"`
	Messages       int     `json:"messages"`
	Input          int64   `json:"input"`
	Output         int64   `json:"output"`
	Reasoning      int64   `json:"reasoning"`
	CacheRead      int64   `json:"cacheRead"`
	CacheWrite     int64   `json:"cacheWrite"`
	TotalTokens    int64   `json:"totalTokens"`
	InputCost      float64 `json:"inputCost"`
	OutputCost     float64 `json:"outputCost"`
	CacheReadCost  float64 `json:"cacheReadCost"`
	CacheWriteCost float64 `json:"cacheWriteCost"`
	TotalCost      float64 `json:"totalCost"`
	PriceSource    string  `json:"priceSource,omitempty"`
	PriceFound     bool    `json:"priceFound"`
}

type Totals struct {
	Messages    int     `json:"messages"`
	Input       int64   `json:"input"`
	Output      int64   `json:"output"`
	Reasoning   int64   `json:"reasoning"`
	CacheRead   int64   `json:"cacheRead"`
	CacheWrite  int64   `json:"cacheWrite"`
	TotalTokens int64   `json:"totalTokens"`
	TotalCost   float64 `json:"totalCost"`
}

func BuildRows(messages []model.MessageRecord, sessions []model.SessionRecord, groupType GroupType, prices *PriceCatalog) ([]Row, Totals) {
	sessionByID := make(map[string]model.SessionRecord, len(sessions))
	for _, s := range sessions {
		sessionByID[s.ID] = s
	}

	type key struct {
		group string
		model string
	}

	agg := make(map[key]Row)
	totals := Totals{}

	for _, msg := range messages {
		if !isUsageMessage(msg) {
			continue
		}

		group, sessionID, sessionTitle := groupValue(groupType, msg, sessionByID[msg.SessionID])
		modelName := NormalizeModelName(msg.ProviderID, msg.ModelID)

		k := key{group: group, model: modelName}
		row := agg[k]
		if row.Group == "" {
			row.Group = group
			row.SessionID = sessionID
			row.SessionTitle = sessionTitle
			row.Model = modelName
		}

		row.Messages++
		row.Input += msg.Tokens.Input
		row.Output += msg.Tokens.Output
		row.Reasoning += msg.Tokens.Reasoning
		row.CacheRead += msg.Tokens.CacheRead
		row.CacheWrite += msg.Tokens.CacheWrite
		row.TotalTokens += tokenTotal(msg.Tokens)

		if price, ok := prices.Resolve(msg.ProviderID, msg.ModelID); ok {
			row.InputCost += tokenCost(msg.Tokens.Input, price.InputPer1M)
			row.OutputCost += tokenCost(msg.Tokens.Output, price.OutputPer1M)
			row.CacheReadCost += tokenCost(msg.Tokens.CacheRead, price.CacheReadPer1M)
			row.CacheWriteCost += tokenCost(msg.Tokens.CacheWrite, price.CacheWritePer1M)
			row.TotalCost = row.InputCost + row.OutputCost + row.CacheReadCost + row.CacheWriteCost
			row.PriceSource = price.Source
			row.PriceFound = true
		}

		agg[k] = row

		totals.Messages++
		totals.Input += msg.Tokens.Input
		totals.Output += msg.Tokens.Output
		totals.Reasoning += msg.Tokens.Reasoning
		totals.CacheRead += msg.Tokens.CacheRead
		totals.CacheWrite += msg.Tokens.CacheWrite
		totals.TotalTokens += tokenTotal(msg.Tokens)
	}

	rows := make([]Row, 0, len(agg))
	for _, row := range agg {
		rows = append(rows, row)
		totals.TotalCost += row.TotalCost
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Group == rows[j].Group {
			if rows[i].TotalCost == rows[j].TotalCost {
				return rows[i].Model < rows[j].Model
			}
			return rows[i].TotalCost > rows[j].TotalCost
		}
		return rows[i].Group < rows[j].Group
	})

	return rows, totals
}

func NormalizeModelName(providerID, modelID string) string {
	model := strings.ToLower(strings.TrimSpace(modelID))
	provider := strings.ToLower(strings.TrimSpace(providerID))
	if model == "" {
		return "unknown"
	}

	replacements := []struct{ old, new string }{
		{"claude-sonnet-4-5", "claude-sonnet-4.5"},
		{"claude-opus-4-5", "claude-opus-4.5"},
		{"claude-haiku-4-5", "claude-haiku-4.5"},
	}
	for _, pair := range replacements {
		model = strings.ReplaceAll(model, pair.old, pair.new)
	}

	if strings.HasPrefix(model, provider+"/") {
		model = strings.TrimPrefix(model, provider+"/")
	}

	if idx := strings.LastIndex(model, "-"); idx > -1 {
		suffix := model[idx+1:]
		if len(suffix) == 8 && isAllDigits(suffix) {
			model = model[:idx]
		}
	}

	return model
}

func tokenTotal(tokens model.TokenUsage) int64 {
	return tokens.Input + tokens.Output + tokens.Reasoning + tokens.CacheRead + tokens.CacheWrite
}

func tokenCost(tokens int64, per1M float64) float64 {
	if tokens == 0 || per1M == 0 {
		return 0
	}
	return (float64(tokens) / 1_000_000) * per1M
}

func isUsageMessage(msg model.MessageRecord) bool {
	if strings.ToLower(msg.Role) != "assistant" {
		return false
	}
	return tokenTotal(msg.Tokens) > 0
}

func groupValue(groupType GroupType, msg model.MessageRecord, session model.SessionRecord) (group string, sessionID string, sessionTitle string) {
	ts := msg.TimeCreated
	if ts == 0 {
		ts = session.Time.Created
	}
	t := time.UnixMilli(ts).UTC()

	switch groupType {
	case GroupWeekly:
		year, week := t.ISOWeek()
		return fmt.Sprintf("%04d-W%02d", year, week), "", ""
	case GroupMonthly:
		return t.Format("2006-01"), "", ""
	case GroupSession:
		title := session.Title
		if strings.TrimSpace(title) == "" {
			title = msg.SessionID
		}
		return msg.SessionID, msg.SessionID, title
	default:
		return t.Format("2006-01-02"), "", ""
	}
}

func isAllDigits(value string) bool {
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return value != ""
}
