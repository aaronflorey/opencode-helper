package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"opencode-cli/internal/model"

	_ "modernc.org/sqlite"
)

func ExpandPath(path string) (string, error) {
	if path == "" {
		return "", errors.New("empty path")
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("finding home directory: %w", err)
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
	}
	return path, nil
}

func ResolveDBPath(storage, dbPathFlag string) (string, error) {
	if dbPathFlag != "" {
		return ExpandPath(dbPathFlag)
	}
	storageAbs, err := filepath.Abs(storage)
	if err != nil {
		return "", fmt.Errorf("resolving storage path: %w", err)
	}
	candidate := filepath.Join(filepath.Dir(storageAbs), "opencode.db")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	return "", nil
}

func OpenDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database %s: %w", dbPath, err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("connecting to database %s: %w", dbPath, err)
	}
	return db, nil
}

func LoadProjects(storage string, db *sql.DB) ([]model.ProjectRecord, error) {
	if db != nil {
		projects, err := loadProjectsFromDB(db)
		if err == nil && len(projects) > 0 {
			return projects, nil
		}
	}

	projectDir := filepath.Join(storage, "project")
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil, fmt.Errorf("reading project directory: %w", err)
	}

	projects := make([]model.ProjectRecord, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		path := filepath.Join(projectDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading project file %s: %w", path, err)
		}

		var p model.ProjectRecord
		if err := json.Unmarshal(data, &p); err != nil {
			return nil, fmt.Errorf("parsing project file %s: %w", path, err)
		}
		projects = append(projects, p)
	}

	sort.Slice(projects, func(i, j int) bool {
		if projects[i].Worktree == projects[j].Worktree {
			return projects[i].ID < projects[j].ID
		}
		return projects[i].Worktree < projects[j].Worktree
	})

	return projects, nil
}

func LoadProjectSessions(storage string, db *sql.DB, project model.ProjectRecord) ([]model.SessionRecord, error) {
	projectID := project.ID

	if db != nil {
		sessions, err := loadProjectSessionsFromDB(db, projectID)
		if err == nil && len(sessions) > 0 {
			return sessions, nil
		}
	}

	sessionDir := filepath.Join(storage, "session", projectID)
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading session directory: %w", err)
	}

	sessions := make([]model.SessionRecord, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		path := filepath.Join(sessionDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading session file %s: %w", path, err)
		}

		var s model.SessionRecord
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, fmt.Errorf("parsing session file %s: %w", path, err)
		}
		sessions = append(sessions, s)
	}

	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].Time.Created == sessions[j].Time.Created {
			return sessions[i].ID < sessions[j].ID
		}
		return sessions[i].Time.Created < sessions[j].Time.Created
	})

	return sessions, nil
}

func BuildFileHistory(storage string, db *sql.DB, project model.ProjectRecord, sessions []model.SessionRecord) ([]string, map[string][]model.FileEvent, map[string][]model.ContentSnapshot, error) {
	if db != nil && project.Worktree != "" {
		dirSessions, err := loadSessionsByDirectoryFromDB(db, project.Worktree)
		if err == nil && len(dirSessions) > 0 {
			sessions = mergeUniqueSessions(sessions, dirSessions)
		}
	}

	history := make(map[string][]model.FileEvent)
	snapshots := make(map[string][]model.ContentSnapshot)
	seenDiffSession := make(map[string]bool)

	for _, s := range sessions {
		diffPath := filepath.Join(storage, "session_diff", s.ID+".json")
		data, err := os.ReadFile(diffPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, nil, nil, fmt.Errorf("reading session diff %s: %w", diffPath, err)
		}

		var changes []model.DiffRecord
		if err := json.Unmarshal(data, &changes); err != nil {
			return nil, nil, nil, fmt.Errorf("parsing session diff %s: %w", diffPath, err)
		}

		for _, change := range changes {
			history[change.File] = append(history[change.File], model.FileEvent{Session: s, Change: change})
		}
		seenDiffSession[s.ID] = true
	}

	if db != nil {
		fromMessages, err := loadFileHistoryFromMessageSummaries(db, sessions, seenDiffSession)
		if err != nil {
			return nil, nil, nil, err
		}
		for file, events := range fromMessages {
			history[file] = append(history[file], events...)
		}

		fromParts, err := loadSnapshotsFromPartToolOutputs(db, project, sessions)
		if err != nil {
			return nil, nil, nil, err
		}
		for file, versions := range fromParts {
			snapshots[file] = append(snapshots[file], versions...)
		}
	}

	files := make([]string, 0, len(history)+len(snapshots))
	seenFile := make(map[string]bool)
	for file := range history {
		seenFile[file] = true
		files = append(files, file)
		sort.Slice(history[file], func(i, j int) bool {
			a := history[file][i]
			b := history[file][j]
			if a.Session.Time.Created == b.Session.Time.Created {
				return a.Session.ID < b.Session.ID
			}
			return a.Session.Time.Created < b.Session.Time.Created
		})
	}

	for file := range snapshots {
		sort.Slice(snapshots[file], func(i, j int) bool {
			if snapshots[file][i].Timestamp == snapshots[file][j].Timestamp {
				return snapshots[file][i].Source < snapshots[file][j].Source
			}
			return snapshots[file][i].Timestamp < snapshots[file][j].Timestamp
		})
		if !seenFile[file] {
			files = append(files, file)
		}
	}
	sort.Strings(files)

	return files, history, snapshots, nil
}

func loadProjectsFromDB(db *sql.DB) ([]model.ProjectRecord, error) {
	rows, err := db.Query(`
		SELECT id, worktree, time_created, time_updated
		FROM project
	`)
	if err != nil {
		return nil, fmt.Errorf("querying projects from database: %w", err)
	}
	defer rows.Close()

	projects := make([]model.ProjectRecord, 0)
	for rows.Next() {
		var p model.ProjectRecord
		if err := rows.Scan(&p.ID, &p.Worktree, &p.Time.Created, &p.Time.Updated); err != nil {
			return nil, fmt.Errorf("scanning project row: %w", err)
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating project rows: %w", err)
	}

	sort.Slice(projects, func(i, j int) bool {
		if projects[i].Worktree == projects[j].Worktree {
			return projects[i].ID < projects[j].ID
		}
		return projects[i].Worktree < projects[j].Worktree
	})

	return projects, nil
}

func loadProjectSessionsFromDB(db *sql.DB, projectID string) ([]model.SessionRecord, error) {
	rows, err := db.Query(`
		SELECT id, project_id, directory, title, time_created, time_updated
		FROM session
		WHERE project_id = ?
		ORDER BY time_created ASC, id ASC
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("querying sessions from database: %w", err)
	}
	defer rows.Close()

	sessions := make([]model.SessionRecord, 0)
	for rows.Next() {
		var s model.SessionRecord
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.Directory, &s.Title, &s.Time.Created, &s.Time.Updated); err != nil {
			return nil, fmt.Errorf("scanning session row: %w", err)
		}
		s.Time.Created = normalizeEpochMillis(s.Time.Created)
		s.Time.Updated = normalizeEpochMillis(s.Time.Updated)
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating session rows: %w", err)
	}

	return sessions, nil
}

func loadSessionsByDirectoryFromDB(db *sql.DB, directory string) ([]model.SessionRecord, error) {
	rows, err := db.Query(`
		SELECT id, project_id, directory, title, time_created, time_updated
		FROM session
		WHERE directory = ?
		ORDER BY time_created ASC, id ASC
	`, directory)
	if err != nil {
		return nil, fmt.Errorf("querying sessions by directory from database: %w", err)
	}
	defer rows.Close()

	sessions := make([]model.SessionRecord, 0)
	for rows.Next() {
		var s model.SessionRecord
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.Directory, &s.Title, &s.Time.Created, &s.Time.Updated); err != nil {
			return nil, fmt.Errorf("scanning session row by directory: %w", err)
		}
		s.Time.Created = normalizeEpochMillis(s.Time.Created)
		s.Time.Updated = normalizeEpochMillis(s.Time.Updated)
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating session rows by directory: %w", err)
	}

	return sessions, nil
}

func mergeUniqueSessions(existing []model.SessionRecord, incoming []model.SessionRecord) []model.SessionRecord {
	byID := make(map[string]model.SessionRecord, len(existing)+len(incoming))
	for _, s := range existing {
		byID[s.ID] = s
	}
	for _, s := range incoming {
		if cur, ok := byID[s.ID]; ok {
			if cur.Time.Created == 0 && s.Time.Created != 0 {
				byID[s.ID] = s
			}
			continue
		}
		byID[s.ID] = s
	}

	out := make([]model.SessionRecord, 0, len(byID))
	for _, s := range byID {
		out = append(out, s)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Time.Created == out[j].Time.Created {
			return out[i].ID < out[j].ID
		}
		return out[i].Time.Created < out[j].Time.Created
	})

	return out
}

func loadFileHistoryFromMessageSummaries(db *sql.DB, sessions []model.SessionRecord, seenDiffSession map[string]bool) (map[string][]model.FileEvent, error) {
	history := make(map[string][]model.FileEvent)

	for _, s := range sessions {
		if seenDiffSession[s.ID] {
			continue
		}

		rows, err := db.Query(`
			SELECT time_created, data
			FROM message
			WHERE session_id = ?
			ORDER BY time_created ASC, id ASC
		`, s.ID)
		if err != nil {
			return nil, fmt.Errorf("querying messages for session %s: %w", s.ID, err)
		}

		for rows.Next() {
			var msgCreated int64
			var raw string
			if err := rows.Scan(&msgCreated, &raw); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scanning message row for session %s: %w", s.ID, err)
			}

			var payload struct {
				Summary struct {
					Diffs []model.DiffRecord `json:"diffs"`
				} `json:"summary"`
			}
			if err := json.Unmarshal([]byte(raw), &payload); err != nil {
				continue
			}
			if len(payload.Summary.Diffs) == 0 {
				continue
			}

			eventSession := s
			if msgCreated > 0 {
				eventSession.Time.Created = normalizeEpochMillis(msgCreated)
			}

			for _, change := range payload.Summary.Diffs {
				history[change.File] = append(history[change.File], model.FileEvent{
					Session: eventSession,
					Change:  change,
				})
			}
		}

		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("iterating messages for session %s: %w", s.ID, err)
		}
		rows.Close()
	}

	return history, nil
}

func loadSnapshotsFromPartToolOutputs(db *sql.DB, project model.ProjectRecord, sessions []model.SessionRecord) (map[string][]model.ContentSnapshot, error) {
	out := make(map[string][]model.ContentSnapshot)
	if len(sessions) == 0 {
		return out, nil
	}

	for _, s := range sessions {
		rows, err := db.Query(`
			SELECT time_created, data
			FROM part
			WHERE session_id = ?
			ORDER BY time_created ASC, id ASC
		`, s.ID)
		if err != nil {
			return nil, fmt.Errorf("querying parts for session %s: %w", s.ID, err)
		}

		for rows.Next() {
			var created int64
			var raw string
			if err := rows.Scan(&created, &raw); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scanning part row for session %s: %w", s.ID, err)
			}

			created = normalizeEpochMillis(created)

			var payload struct {
				Type  string `json:"type"`
				Tool  string `json:"tool"`
				State struct {
					Input  map[string]interface{} `json:"input"`
					Output interface{}            `json:"output"`
				} `json:"state"`
			}
			if err := json.Unmarshal([]byte(raw), &payload); err != nil {
				continue
			}
			if payload.Type != "tool" {
				continue
			}

			switch payload.Tool {
			case "read":
				rawPath, _ := payload.State.Input["filePath"].(string)
				normalized := normalizeProjectPath(project.Worktree, rawPath)
				if normalized == "" {
					continue
				}
				outputStr, ok := payload.State.Output.(string)
				if !ok {
					continue
				}
				content, ok := parseReadToolOutput(outputStr)
				if !ok {
					continue
				}
				out[normalized] = append(out[normalized], model.ContentSnapshot{
					File:      normalized,
					Content:   content,
					Timestamp: created,
					Source:    "tool-read",
					SessionID: s.ID,
				})

			case "write":
				rawPath, _ := payload.State.Input["filePath"].(string)
				normalized := normalizeProjectPath(project.Worktree, rawPath)
				if normalized == "" {
					continue
				}
				content, _ := payload.State.Input["content"].(string)
				if content == "" {
					continue
				}
				out[normalized] = append(out[normalized], model.ContentSnapshot{
					File:      normalized,
					Content:   content,
					Timestamp: created,
					Source:    "tool-write",
					SessionID: s.ID,
				})
			}
		}

		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("iterating parts for session %s: %w", s.ID, err)
		}
		rows.Close()
	}

	return out, nil
}

func normalizeProjectPath(worktree, raw string) string {
	if raw == "" {
		return ""
	}
	raw = filepath.Clean(raw)
	if filepath.IsAbs(raw) {
		wt := filepath.Clean(worktree)
		prefix := wt + string(os.PathSeparator)
		if strings.HasPrefix(raw, prefix) {
			return strings.TrimPrefix(raw, prefix)
		}
		return ""
	}
	return raw
}

func parseReadToolOutput(raw string) (string, bool) {
	start := strings.Index(raw, "<content>")
	end := strings.Index(raw, "</content>")
	if start == -1 || end == -1 || end <= start {
		return "", false
	}
	body := raw[start+len("<content>") : end]
	body = strings.TrimPrefix(body, "\n")

	lines := strings.Split(body, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(line, "(End of file") {
			continue
		}
		idx := strings.Index(line, ":")
		if idx == -1 {
			continue
		}
		prefix := strings.TrimSpace(line[:idx])
		if prefix == "" {
			continue
		}
		numeric := true
		for _, r := range prefix {
			if r < '0' || r > '9' {
				numeric = false
				break
			}
		}
		if !numeric {
			continue
		}
		value := line[idx+1:]
		if strings.HasPrefix(value, " ") {
			value = value[1:]
		}
		out = append(out, value)
	}

	return strings.Join(out, "\n"), true
}

func normalizeEpochMillis(ms int64) int64 {
	if ms <= 0 {
		return ms
	}
	return time.UnixMilli(ms).UTC().UnixMilli()
}
