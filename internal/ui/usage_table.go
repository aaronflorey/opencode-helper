package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aaronflorey/opencode-helper/internal/usage"

	"github.com/charmbracelet/lipgloss"
)

func RenderUsageTable(rows []usage.Row, totals usage.Totals, groupType usage.GroupType, prices *usage.PriceCatalog) string {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	cellStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	numberStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	costStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	headers := []string{"GROUP", "MODEL", "MSGS", "INPUT", "OUTPUT", "CACHE R", "CACHE W", "TOTAL TOKENS", "COST"}
	if groupType == usage.GroupSession {
		headers[0] = "SESSION"
	}

	widths := make([]int, len(headers))
	for i := range headers {
		widths[i] = len(headers[i])
	}

	values := make([][]string, 0, len(rows))
	for _, row := range rows {
		group := row.Group
		if groupType == usage.GroupSession && row.SessionTitle != "" {
			group = fmt.Sprintf("%s (%s)", row.SessionTitle, row.SessionID)
		}

		line := []string{
			group,
			row.Model,
			strconv.Itoa(row.Messages),
			formatInt(row.Input),
			formatInt(row.Output),
			formatInt(row.CacheRead),
			formatInt(row.CacheWrite),
			formatInt(row.TotalTokens),
			formatUSD(row.TotalCost),
		}
		values = append(values, line)
		for i := range line {
			if len(line[i]) > widths[i] {
				widths[i] = len(line[i])
			}
		}
	}

	b := strings.Builder{}
	b.WriteString(headerStyle.Render(strings.ToUpper(string(groupType)) + " Usage"))
	b.WriteString("\n")
	if prices != nil && len(prices.Sources()) > 0 {
		b.WriteString(mutedStyle.Render("Pricing sources: " + strings.Join(prices.Sources(), ", ")))
		b.WriteString("\n")
	}

	for i, h := range headers {
		b.WriteString(padRight(headerStyle.Render(h), widths[i]))
		if i < len(headers)-1 {
			b.WriteString("  ")
		}
	}
	b.WriteString("\n")

	for _, row := range values {
		for i, v := range row {
			styled := cellStyle.Render(v)
			if i >= 2 && i <= 7 {
				styled = numberStyle.Render(v)
			}
			if i == 8 {
				styled = costStyle.Render(v)
			}
			b.WriteString(padRight(styled, widths[i]))
			if i < len(row)-1 {
				b.WriteString("  ")
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(headerStyle.Render("Totals"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render(fmt.Sprintf(
		"messages=%d input=%s output=%s cache_read=%s cache_write=%s total_tokens=%s total_cost=%s",
		totals.Messages,
		formatInt(totals.Input),
		formatInt(totals.Output),
		formatInt(totals.CacheRead),
		formatInt(totals.CacheWrite),
		formatInt(totals.TotalTokens),
		formatUSD(totals.TotalCost),
	)))
	b.WriteString("\n")

	return b.String()
}

func formatInt(value int64) string {
	if value == 0 {
		return "0"
	}

	negative := value < 0
	if negative {
		value = -value
	}

	raw := strconv.FormatInt(value, 10)
	parts := make([]string, 0, (len(raw)+2)/3)
	for len(raw) > 3 {
		parts = append([]string{raw[len(raw)-3:]}, parts...)
		raw = raw[:len(raw)-3]
	}
	parts = append([]string{raw}, parts...)
	out := strings.Join(parts, ",")
	if negative {
		out = "-" + out
	}
	return out
}

func formatUSD(value float64) string {
	return fmt.Sprintf("$%.4f", value)
}

func padRight(value string, width int) string {
	plainWidth := lipgloss.Width(value)
	if plainWidth >= width {
		return value
	}
	return value + strings.Repeat(" ", width-plainWidth)
}
