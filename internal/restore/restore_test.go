package restore

import (
	"path/filepath"
	"testing"

	"github.com/aaronflorey/opencode-helper/internal/model"
)

func TestResolveOutputPathRejectsTraversal(t *testing.T) {
	t.Parallel()

	project := model.ProjectRecord{Worktree: filepath.Join(string(filepath.Separator), "tmp", "project")}

	_, err := ResolveOutputPath(OutputInferSentinel, project, "../secret.txt")
	if err == nil {
		t.Fatal("expected traversal path to be rejected")
	}
}

func TestReconstructLatestKeepsDiffOverPartialReadSnapshot(t *testing.T) {
	t.Parallel()

	events := []model.FileEvent{{
		Session: model.SessionRecord{Time: struct {
			Created int64 `json:"created"`
			Updated int64 `json:"updated"`
		}{Created: 100}},
		Change: model.DiffRecord{Before: "before", After: "full content"},
	}}

	snapshots := []model.ContentSnapshot{{
		File:      "file.txt",
		Content:   "partial content",
		Timestamp: 200,
		Source:    "tool-read-partial",
	}}

	result := ReconstructLatest(events, snapshots)
	if result.Source != "diff-replay" {
		t.Fatalf("expected diff-replay to win, got %q", result.Source)
	}
	if result.Content != "full content" {
		t.Fatalf("expected full diff content, got %q", result.Content)
	}
}
