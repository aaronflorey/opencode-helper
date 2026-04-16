package cli

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/aaronflorey/opencode-helper/internal/gitstore"
	"github.com/aaronflorey/opencode-helper/internal/restore"
	"github.com/aaronflorey/opencode-helper/internal/store"
	"github.com/aaronflorey/opencode-helper/internal/ui"

	"github.com/spf13/cobra"
)

type restoreOptions struct {
	storagePath  string
	dbPath       string
	projectQuery string
	fileQuery    string
	outputPath   string
	noGit        bool
}

func NewRestoreCommand() *cobra.Command {
	opts := &restoreOptions{}

	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore files from session history",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRestore(opts)
		},
	}

	cmd.Flags().StringVar(&opts.storagePath, "storage", "~/.local/share/opencode/storage", "OpenCode storage directory")
	cmd.Flags().StringVar(&opts.dbPath, "db", "", "Path to opencode.db (default: sibling of --storage)")
	cmd.Flags().StringVar(&opts.projectQuery, "project", "", "Project id or worktree substring (skip project menu)")
	cmd.Flags().StringVar(&opts.fileQuery, "file", "", "File path filter (substring, or /prefix for root-anchored match)")
	cmd.Flags().StringVarP(&opts.outputPath, "output", "o", "", "Write reconstructed file to this path; pass --output without value to write to inferred original path")
	cmd.Flags().BoolVar(&opts.noGit, "no-git", false, "Disable git history lookup source")
	if outputFlag := cmd.Flags().Lookup("output"); outputFlag != nil {
		outputFlag.NoOptDefVal = restore.OutputInferSentinel
	}

	return cmd
}

func runRestore(opts *restoreOptions) error {
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

	projects, err := store.LoadProjects(storagePath, db)
	if err != nil {
		return err
	}
	if len(projects) == 0 {
		return fmt.Errorf("no projects found in %s", filepath.Join(storagePath, "project"))
	}

	project, ok, err := restore.InferProjectFromCWD(projects)
	if err != nil {
		return err
	}
	if opts.projectQuery != "" || !ok {
		project, err = ui.PickProject(projects, opts.projectQuery)
		if err != nil {
			return err
		}
	} else {
		fmt.Fprintf(os.Stderr, "Auto-matched project from current directory: %s (%s)\n", project.Worktree, project.ID)
	}

	sessions, err := store.LoadProjectSessions(storagePath, db, project)
	if err != nil {
		return err
	}

	files, history, snapshots, err := store.BuildFileHistory(storagePath, db, project, sessions)
	if err != nil {
		return err
	}

	useGit := !opts.noGit && gitstore.IsRepository(project.Worktree)
	if useGit {
		gitFiles, err := gitstore.ListFiles(project.Worktree)
		if err == nil {
			files = mergeFileLists(files, gitFiles)
		}
	}

	if len(files) == 0 {
		return fmt.Errorf("no file diffs found for project %s", project.ID)
	}

	selectedFiles, err := ui.PickFiles(files, history, snapshots, opts.fileQuery)
	if err != nil {
		return err
	}

	if len(selectedFiles) > 1 && opts.outputPath != "" && opts.outputPath != restore.OutputInferSentinel {
		return fmt.Errorf("multiple files selected with explicit --output path; use --output with no value to infer original paths")
	}

	for i, selectedFile := range selectedFiles {
		allSnapshots := snapshots[selectedFile]
		if useGit {
			gitSnapshots, err := gitstore.LoadSnapshots(project.Worktree, selectedFile)
			if err == nil && len(gitSnapshots) > 0 {
				allSnapshots = append(allSnapshots, gitSnapshots...)
			}
		}

		result := restore.ReconstructLatest(history[selectedFile], allSnapshots)

		resolvedOutputPath, err := restore.ResolveOutputPath(opts.outputPath, project, selectedFile)
		if err != nil {
			return err
		}

		if resolvedOutputPath != "" {
			if err := os.MkdirAll(filepath.Dir(resolvedOutputPath), 0o755); err != nil {
				return fmt.Errorf("creating output directory: %w", err)
			}
			if err := os.WriteFile(resolvedOutputPath, []byte(result.Content), 0o644); err != nil {
				return fmt.Errorf("writing output file: %w", err)
			}
			fmt.Fprintf(os.Stderr, "Reconstructed %s using %s into %s\n", selectedFile, result.Source, resolvedOutputPath)
		} else {
			if len(selectedFiles) > 1 {
				if i > 0 {
					fmt.Print("\n")
				}
				fmt.Printf("===== %s (%s) =====\n", selectedFile, result.Source)
			}
			fmt.Print(result.Content)
		}

		if result.BeforeMismatchSeen {
			fmt.Fprintln(os.Stderr, "Warning: encountered at least one discontinuity while replaying diffs (before content did not match current state).")
		}
	}

	return nil
}

func mergeFileLists(existing []string, incoming []string) []string {
	seen := make(map[string]bool, len(existing)+len(incoming))
	merged := make([]string, 0, len(existing)+len(incoming))

	for _, file := range existing {
		if seen[file] {
			continue
		}
		seen[file] = true
		merged = append(merged, file)
	}

	for _, file := range incoming {
		if seen[file] {
			continue
		}
		seen[file] = true
		merged = append(merged, file)
	}

	sort.Strings(merged)
	return merged
}
