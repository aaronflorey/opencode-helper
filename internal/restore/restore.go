package restore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"opencode-cli/internal/model"
	"opencode-cli/internal/store"
)

const OutputInferSentinel = "__INFER_OUTPUT_PATH__"

type ReconstructionResult struct {
	Content            string
	EventsApplied      int
	BeforeMismatchSeen bool
	Source             string
	Timestamp          int64
}

func InferProjectFromCWD(projects []model.ProjectRecord) (model.ProjectRecord, bool, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return model.ProjectRecord{}, false, fmt.Errorf("reading current directory: %w", err)
	}
	cwd = filepath.Clean(cwd)

	best := model.ProjectRecord{}
	bestLen := -1
	for _, p := range projects {
		wt := filepath.Clean(p.Worktree)
		if wt == "" {
			continue
		}
		if pathIsWithin(wt, cwd) && len(wt) > bestLen {
			best = p
			bestLen = len(wt)
		}
	}

	if bestLen == -1 {
		return model.ProjectRecord{}, false, nil
	}

	return best, true, nil
}

func ResolveOutputPath(outputPath string, project model.ProjectRecord, file string) (string, error) {
	if outputPath == "" {
		return "", nil
	}
	if outputPath == OutputInferSentinel {
		if project.Worktree == "" {
			return "", fmt.Errorf("cannot infer output path without project worktree")
		}
		return filepath.Join(project.Worktree, file), nil
	}
	if outputPath == "~" || strings.HasPrefix(outputPath, "~/") {
		return store.ExpandPath(outputPath)
	}
	return outputPath, nil
}

func ReconstructLatest(events []model.FileEvent, snapshots []model.ContentSnapshot) ReconstructionResult {
	best := ReconstructionResult{}
	haveBest := false

	if len(events) > 0 {
		best = reconstructFromEvents(events)
		haveBest = true
	}

	for _, snap := range snapshots {
		if !haveBest || snap.Timestamp >= best.Timestamp {
			best = ReconstructionResult{Content: snap.Content, Source: snap.Source, Timestamp: snap.Timestamp}
			haveBest = true
		}
	}

	if !haveBest {
		return ReconstructionResult{Content: "", Source: "none"}
	}
	if best.Source == "" {
		best.Source = "diff-replay"
	}
	return best
}

func reconstructFromEvents(events []model.FileEvent) ReconstructionResult {
	res := ReconstructionResult{}
	if len(events) == 0 {
		return res
	}

	current := ""
	initialized := false

	for _, ev := range events {
		if !initialized {
			current = ev.Change.Before
			initialized = true
		}

		if ev.Change.Before != "" && ev.Change.Before != current {
			res.BeforeMismatchSeen = true
		}

		if ev.Change.Status == "deleted" {
			current = ""
		} else {
			current = ev.Change.After
		}

		res.EventsApplied++
	}

	res.Content = current
	res.Source = "diff-replay"
	res.Timestamp = events[len(events)-1].Session.Time.Created
	return res
}

func pathIsWithin(root, p string) bool {
	root = filepath.Clean(root)
	p = filepath.Clean(p)
	if root == p {
		return true
	}
	return strings.HasPrefix(p, root+string(os.PathSeparator))
}
