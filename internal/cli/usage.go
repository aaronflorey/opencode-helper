package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"opencode-cli/internal/store"
	"opencode-cli/internal/ui"
	"opencode-cli/internal/usage"

	"github.com/spf13/cobra"
)

type usageOptions struct {
	storagePath string
	dbPath      string
	groupType   string
	jsonOutput  bool
}

func NewUsageCommand() *cobra.Command {
	opts := &usageOptions{}

	cmd := &cobra.Command{
		Use:   "usage",
		Short: "Show token usage and pricing",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUsage(opts)
		},
	}

	cmd.Flags().StringVar(&opts.storagePath, "storage", "~/.local/share/opencode/storage", "OpenCode storage directory")
	cmd.Flags().StringVar(&opts.dbPath, "db", "", "Path to opencode.db (default: sibling of --storage)")
	cmd.Flags().StringVar(&opts.groupType, "type", string(usage.GroupDaily), "Grouping type: daily|weekly|monthly|session")
	cmd.Flags().BoolVar(&opts.jsonOutput, "json", false, "Output as JSON")

	return cmd
}

func runUsage(opts *usageOptions) error {
	groupType, err := parseGroupType(opts.groupType)
	if err != nil {
		return err
	}

	storagePath, err := store.ExpandPath(opts.storagePath)
	if err != nil {
		return err
	}

	resolvedDBPath, err := store.ResolveDBPath(storagePath, opts.dbPath)
	if err != nil {
		return err
	}

	var db *sql.DB
	if resolvedDBPath != "" {
		db, err = store.OpenDB(resolvedDBPath)
		if err != nil {
			return err
		}
		defer db.Close()
	}

	sessions, err := store.LoadUsageSessions(storagePath, db)
	if err != nil {
		return err
	}
	messages, err := store.LoadUsageMessages(storagePath, db)
	if err != nil {
		return err
	}

	priceCatalog, priceErr := usage.LoadPriceCatalog(context.Background())
	if priceErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: pricing data unavailable (%v). Costs will show as zero.\n", priceErr)
	}

	rows, totals := usage.BuildRows(messages, sessions, groupType, priceCatalog)
	if len(rows) == 0 {
		return fmt.Errorf("no usage data found")
	}

	if opts.jsonOutput {
		payload := struct {
			Type           usage.GroupType `json:"type"`
			Rows           []usage.Row     `json:"rows"`
			Totals         usage.Totals    `json:"totals"`
			PricingSources []string        `json:"pricingSources,omitempty"`
		}{
			Type:   groupType,
			Rows:   rows,
			Totals: totals,
		}
		if priceCatalog != nil {
			payload.PricingSources = priceCatalog.Sources()
		}

		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(payload)
	}

	_, err = fmt.Fprint(os.Stdout, ui.RenderUsageTable(rows, totals, groupType, priceCatalog))
	return err
}

func parseGroupType(raw string) (usage.GroupType, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	switch usage.GroupType(normalized) {
	case usage.GroupDaily, usage.GroupWeekly, usage.GroupMonthly, usage.GroupSession:
		return usage.GroupType(normalized), nil
	default:
		return "", fmt.Errorf("invalid --type %q (expected: daily, weekly, monthly, session)", raw)
	}
}
