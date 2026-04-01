package cli

import (
	"database/sql"
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"opencode-cli/internal/model"
	"opencode-cli/internal/restore"
	"opencode-cli/internal/store"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"mvdan.cc/sh/v3/syntax"
)

var (
	uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
	hexPattern  = regexp.MustCompile(`^[0-9a-fA-F]{7,64}$`)
	numPattern  = regexp.MustCompile(`^[0-9]+$`)
)

type toolUsageOptions struct {
	storagePath    string
	dbPath         string
	currentProject bool
	fullCommand    bool
	limit          int
}

type usageStat struct {
	Command      string
	Runs         int
	OutputTokens int
}

func NewToolUsageCommand() *cobra.Command {
	opts := &toolUsageOptions{}

	cmd := &cobra.Command{
		Use:   "tool-usage",
		Short: "Show bash tool usage stats from OpenCode history",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runToolUsage(opts)
		},
	}

	cmd.Flags().StringVar(&opts.storagePath, "storage", "~/.local/share/opencode/storage", "OpenCode storage directory")
	cmd.Flags().StringVar(&opts.dbPath, "db", "", "Path to opencode.db (default: sibling of --storage)")
	cmd.Flags().BoolVar(&opts.currentProject, "current-project", false, "Only include usage from the project matching current working directory")
	cmd.Flags().BoolVar(&opts.fullCommand, "full-command", false, "Group by full raw command string (disables normalization)")
	cmd.Flags().IntVar(&opts.limit, "limit", 25, "Max number of rows to show; use 0 for all")

	return cmd
}

func runToolUsage(opts *toolUsageOptions) error {
	storagePath, err := store.ExpandPath(opts.storagePath)
	if err != nil {
		return err
	}

	resolvedDBPath, err := store.ResolveDBPath(storagePath, opts.dbPath)
	if err != nil {
		return err
	}
	if resolvedDBPath == "" {
		return fmt.Errorf("opencode.db not found; pass --db explicitly or ensure it exists next to --storage")
	}

	db, err := store.OpenDB(resolvedDBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	projectID, err := resolveToolUsageProjectID(db, storagePath, opts.currentProject)
	if err != nil {
		return err
	}

	events, err := store.LoadBashToolUsageEvents(db, projectID)
	if err != nil {
		return err
	}

	stats := aggregateToolUsage(events, opts.fullCommand)
	if len(stats) == 0 {
		fmt.Println("No bash tool usage found.")
		return nil
	}

	if opts.limit > 0 && opts.limit < len(stats) {
		stats = stats[:opts.limit]
	}

	printToolUsageTable(stats)
	return nil
}

func resolveToolUsageProjectID(db *sql.DB, storagePath string, currentProjectOnly bool) (string, error) {
	if !currentProjectOnly {
		return "", nil
	}

	projects, err := store.LoadProjects(storagePath, db)
	if err != nil {
		return "", err
	}
	if len(projects) == 0 {
		return "", fmt.Errorf("no projects found")
	}

	project, ok, err := restore.InferProjectFromCWD(projects)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("could not infer current project from working directory; run without --current-project to include all projects")
	}

	fmt.Fprintf(os.Stderr, "Using current project: %s (%s)\n", project.Worktree, project.ID)
	return project.ID, nil
}

func aggregateToolUsage(events []model.ToolUsageEvent, fullCommand bool) []usageStat {
	byCommand := make(map[string]*usageStat)

	for _, event := range events {
		tokens := estimateOutputTokens(event.Output)
		keys := make([]string, 0, 1)
		if fullCommand {
			key := strings.TrimSpace(event.Command)
			if key != "" {
				keys = append(keys, key)
			}
		} else {
			keys = extractNormalizedCommands(event.Command)
		}
		if len(keys) == 0 {
			continue
		}

		base := 0
		extra := 0
		if tokens > 0 {
			base = tokens / len(keys)
			extra = tokens % len(keys)
		}

		for i, key := range keys {
			stat, ok := byCommand[key]
			if !ok {
				stat = &usageStat{Command: key}
				byCommand[key] = stat
			}
			stat.Runs++
			stat.OutputTokens += base
			if i < extra {
				stat.OutputTokens++
			}
		}
	}

	stats := make([]usageStat, 0, len(byCommand))
	for _, stat := range byCommand {
		stats = append(stats, *stat)
	}

	sort.Slice(stats, func(i, j int) bool {
		if stats[i].OutputTokens == stats[j].OutputTokens {
			if stats[i].Runs == stats[j].Runs {
				return stats[i].Command < stats[j].Command
			}
			return stats[i].Runs > stats[j].Runs
		}
		return stats[i].OutputTokens > stats[j].OutputTokens
	})

	return stats
}

func estimateOutputTokens(output string) int {
	if output == "" {
		return 0
	}
	runes := utf8.RuneCountInString(output)
	if runes <= 0 {
		return 0
	}
	return int(math.Ceil(float64(runes) / 4.0))
}

func extractNormalizedCommands(command string) []string {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return nil
	}

	parser := syntax.NewParser()
	file, err := parser.Parse(strings.NewReader(trimmed), "")
	if err != nil {
		fallback := normalizeFallbackCommand(trimmed)
		if fallback == "" {
			return nil
		}
		return []string{fallback}
	}

	commands := make([]string, 0)
	syntax.Walk(file, func(node syntax.Node) bool {
		call, ok := node.(*syntax.CallExpr)
		if !ok {
			return true
		}

		parts := make([]string, 0, len(call.Args))
		for _, arg := range call.Args {
			parts = append(parts, normalizeWord(arg))
		}
		if len(parts) == 0 {
			return true
		}

		commands = append(commands, strings.Join(parts, " "))
		return true
	})

	if len(commands) == 0 {
		fallback := normalizeFallbackCommand(trimmed)
		if fallback == "" {
			return nil
		}
		return []string{fallback}
	}

	return commands
}

func normalizeWord(word *syntax.Word) string {
	raw, ok := wordToText(word)
	if !ok {
		return "<expr>"
	}

	return normalizeToken(raw)
}

func wordToText(word *syntax.Word) (string, bool) {
	if word == nil || len(word.Parts) == 0 {
		return "", false
	}

	b := strings.Builder{}
	for _, part := range word.Parts {
		switch v := part.(type) {
		case *syntax.Lit:
			b.WriteString(v.Value)
		case *syntax.SglQuoted:
			b.WriteString(v.Value)
		case *syntax.DblQuoted:
			text, ok := wordPartsToText(v.Parts)
			if !ok {
				return "", false
			}
			b.WriteString(text)
		default:
			return "", false
		}
	}

	return b.String(), true
}

func wordPartsToText(parts []syntax.WordPart) (string, bool) {
	b := strings.Builder{}
	for _, part := range parts {
		switch v := part.(type) {
		case *syntax.Lit:
			b.WriteString(v.Value)
		case *syntax.SglQuoted:
			b.WriteString(v.Value)
		default:
			return "", false
		}
	}
	return b.String(), true
}

func normalizeFallbackCommand(command string) string {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return ""
	}

	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		normalized = append(normalized, normalizeToken(part))
	}
	return strings.Join(normalized, " ")
}

func normalizeToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}

	if strings.Contains(token, "`") || strings.Contains(token, "$(") || strings.Contains(token, "${") || strings.HasPrefix(token, "$") {
		return "<expr>"
	}

	if strings.HasPrefix(token, "-") {
		if key, value, ok := strings.Cut(token, "="); ok {
			return key + "=" + normalizeToken(value)
		}
		return token
	}

	if looksLikeUUID(token) || looksLikeHexID(token) {
		return "<id>"
	}
	if looksLikeNumber(token) {
		return "<num>"
	}
	if looksLikePath(token) {
		return "<path>"
	}

	return token
}

func looksLikeUUID(value string) bool {
	return uuidPattern.MatchString(value)
}

func looksLikeHexID(value string) bool {
	return hexPattern.MatchString(value)
}

func looksLikeNumber(value string) bool {
	return numPattern.MatchString(value)
}

func looksLikePath(value string) bool {
	if strings.HasPrefix(value, "/") || strings.HasPrefix(value, "./") || strings.HasPrefix(value, "../") || strings.HasPrefix(value, "~/") {
		return true
	}
	if strings.Contains(value, "/") || strings.Contains(value, "\\") {
		return true
	}
	if strings.HasSuffix(value, ".go") || strings.HasSuffix(value, ".json") || strings.HasSuffix(value, ".yaml") || strings.HasSuffix(value, ".yml") || strings.HasSuffix(value, ".md") || strings.HasSuffix(value, ".txt") || strings.HasSuffix(value, ".log") {
		return true
	}
	return false
}

func printToolUsageTable(stats []usageStat) {
	width := terminalWidth()
	commandWidth := width - 30
	if commandWidth < 24 {
		commandWidth = 24
	}

	tw := table.NewWriter()
	tw.SetStyle(table.StyleRounded)
	tw.SetAllowedRowLength(width)
	tw.AppendHeader(table.Row{"COMMAND", "RUNS", "OUTPUT TOKENS"})
	tw.SetColumnConfigs([]table.ColumnConfig{
		{Name: "COMMAND", WidthMax: commandWidth},
		{Name: "RUNS", Align: text.AlignRight},
		{Name: "OUTPUT TOKENS", Align: text.AlignRight},
	})

	for _, row := range stats {
		tw.AppendRow(table.Row{truncateWithEllipsis(row.Command, commandWidth), row.Runs, row.OutputTokens})
	}

	fmt.Println(tw.Render())
}

func terminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return 100
	}
	return width
}

func truncateWithEllipsis(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	if limit <= 1 {
		return "…"
	}
	runes := []rune(value)
	return string(runes[:limit-1]) + "…"
}
