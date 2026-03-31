package gitstore

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"opencode-cli/internal/model"
)

func IsRepository(worktree string) bool {
	cmd := exec.Command("git", "-C", worktree, "rev-parse", "--is-inside-work-tree")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

func ListFiles(worktree string) ([]string, error) {
	cmd := exec.Command("git", "-C", worktree, "log", "--name-only", "--pretty=format:", "--all", "--")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing git history files: %w", err)
	}

	seen := make(map[string]bool)
	files := make([]string, 0)
	for _, line := range bytes.Split(out, []byte("\n")) {
		f := strings.TrimSpace(string(line))
		if f == "" || seen[f] {
			continue
		}
		seen[f] = true
		files = append(files, f)
	}

	return files, nil
}

func LoadSnapshots(worktree, file string) ([]model.ContentSnapshot, error) {
	cmd := exec.Command("git", "-C", worktree, "log", "--follow", "--format=%H|%ct", "--", file)
	out, err := cmd.Output()
	if err != nil {
		return nil, nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, nil
	}

	snapshots := make([]model.ContentSnapshot, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		commit := parts[0]
		sec, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			continue
		}
		timestamp := time.Unix(sec, 0).UTC().UnixMilli()

		show := exec.Command("git", "-C", worktree, "show", fmt.Sprintf("%s:%s", commit, file))
		blob, err := show.Output()
		if err != nil {
			continue
		}

		snapshots = append(snapshots, model.ContentSnapshot{
			File:      file,
			Content:   string(blob),
			Timestamp: timestamp,
			Source:    "git",
		})
	}

	return snapshots, nil
}
