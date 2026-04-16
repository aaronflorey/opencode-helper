package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aaronflorey/opencode-helper/internal/model"
)

func TestBuildFileHistorySkipsUnsafePaths(t *testing.T) {
	t.Parallel()

	storage := t.TempDir()
	if err := os.MkdirAll(filepath.Join(storage, "session_diff"), 0o755); err != nil {
		t.Fatalf("creating session_diff dir: %v", err)
	}

	data := `[
		{"file":"safe/file.txt","before":"old","after":"new","status":"modified"},
		{"file":"../secret.txt","before":"bad","after":"worse","status":"modified"}
	]`
	if err := os.WriteFile(filepath.Join(storage, "session_diff", "session-1.json"), []byte(data), 0o644); err != nil {
		t.Fatalf("writing session diff: %v", err)
	}

	session := model.SessionRecord{ID: "session-1"}
	session.Time.Created = 1

	files, history, snapshots, err := BuildFileHistory(storage, nil, model.ProjectRecord{}, []model.SessionRecord{session})
	if err != nil {
		t.Fatalf("BuildFileHistory returned error: %v", err)
	}

	if len(files) != 1 || files[0] != filepath.Clean("safe/file.txt") {
		t.Fatalf("expected only safe file, got %v", files)
	}
	if len(history[filepath.Clean("safe/file.txt")]) != 1 {
		t.Fatalf("expected safe file history to be kept, got %v", history)
	}
	if len(snapshots) != 0 {
		t.Fatalf("expected no snapshots, got %v", snapshots)
	}
	if _, ok := history[filepath.Clean("../secret.txt")]; ok {
		t.Fatal("expected unsafe file history to be dropped")
	}
}

func TestParseReadToolOutputDetectsPartialContent(t *testing.T) {
	t.Parallel()

	completeRaw := "<content>\n1: first\n2: second\n(End of file - total 2 lines)\n</content>"
	content, complete, ok := parseReadToolOutput(completeRaw)
	if !ok {
		t.Fatal("expected complete read output to parse")
	}
	if !complete {
		t.Fatal("expected complete read output to be marked complete")
	}
	if content != "first\nsecond" {
		t.Fatalf("unexpected complete content %q", content)
	}

	partialRaw := "<content>\n1: first\n2: second\n</content>"
	content, complete, ok = parseReadToolOutput(partialRaw)
	if !ok {
		t.Fatal("expected partial read output to parse")
	}
	if complete {
		t.Fatal("expected partial read output to be marked incomplete")
	}
	if content != "first\nsecond" {
		t.Fatalf("unexpected partial content %q", content)
	}
}

func TestNormalizeProjectPathRejectsTraversal(t *testing.T) {
	t.Parallel()

	worktree := filepath.Join(string(filepath.Separator), "tmp", "project")

	if got := normalizeProjectPath(worktree, filepath.Join(worktree, "dir", "file.txt")); got != filepath.Join("dir", "file.txt") {
		t.Fatalf("expected path inside worktree to normalize, got %q", got)
	}
	if got := normalizeProjectPath(worktree, "../secret.txt"); got != "" {
		t.Fatalf("expected traversal path to be rejected, got %q", got)
	}
	if got := normalizeProjectPath(worktree, filepath.Join(worktree, "..", "secret.txt")); got != "" {
		t.Fatalf("expected absolute path outside worktree to be rejected, got %q", got)
	}
}
