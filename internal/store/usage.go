package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/aaronflorey/opencode-helper/internal/model"
)

func LoadUsageSessions(storage string, db *sql.DB) ([]model.SessionRecord, error) {
	byID := make(map[string]model.SessionRecord)

	if db != nil {
		fromDB, err := loadAllSessionsFromDB(db)
		if err != nil {
			return nil, err
		}
		for _, s := range fromDB {
			byID[s.ID] = s
		}
	}

	fromStorage, err := loadAllSessionsFromStorage(storage)
	if err != nil {
		return nil, err
	}
	for _, s := range fromStorage {
		if existing, ok := byID[s.ID]; ok {
			if existing.Time.Created == 0 && s.Time.Created != 0 {
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

	return out, nil
}

func LoadUsageMessages(storage string, db *sql.DB) ([]model.MessageRecord, error) {
	byID := make(map[string]model.MessageRecord)

	if db != nil {
		fromDB, err := loadAllMessagesFromDB(db)
		if err != nil {
			return nil, err
		}
		for _, m := range fromDB {
			byID[m.ID] = m
		}
	}

	fromStorage, err := loadAllMessagesFromStorage(storage)
	if err != nil {
		return nil, err
	}
	for _, m := range fromStorage {
		if existing, ok := byID[m.ID]; ok {
			if existing.TimeCreated == 0 && m.TimeCreated != 0 {
				byID[m.ID] = m
			}
			continue
		}
		byID[m.ID] = m
	}

	out := make([]model.MessageRecord, 0, len(byID))
	for _, m := range byID {
		out = append(out, m)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].TimeCreated == out[j].TimeCreated {
			return out[i].ID < out[j].ID
		}
		return out[i].TimeCreated < out[j].TimeCreated
	})

	return out, nil
}

func loadAllSessionsFromDB(db *sql.DB) ([]model.SessionRecord, error) {
	rows, err := db.Query(`
		SELECT id, project_id, directory, title, time_created, time_updated
		FROM session
	`)
	if err != nil {
		return nil, fmt.Errorf("querying sessions from database: %w", err)
	}
	defer rows.Close()

	out := make([]model.SessionRecord, 0)
	for rows.Next() {
		var s model.SessionRecord
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.Directory, &s.Title, &s.Time.Created, &s.Time.Updated); err != nil {
			return nil, fmt.Errorf("scanning session row: %w", err)
		}
		s.Time.Created = normalizeEpochMillis(s.Time.Created)
		s.Time.Updated = normalizeEpochMillis(s.Time.Updated)
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating session rows: %w", err)
	}

	return out, nil
}

func loadAllMessagesFromDB(db *sql.DB) ([]model.MessageRecord, error) {
	rows, err := db.Query(`
		SELECT id, session_id, time_created, data
		FROM message
		ORDER BY time_created ASC, id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("querying messages from database: %w", err)
	}
	defer rows.Close()

	out := make([]model.MessageRecord, 0)
	for rows.Next() {
		var id string
		var sessionID string
		var created int64
		var raw string
		if err := rows.Scan(&id, &sessionID, &created, &raw); err != nil {
			return nil, fmt.Errorf("scanning message row: %w", err)
		}

		record, ok := parseMessageRecordJSON([]byte(raw))
		if !ok {
			continue
		}
		if record.ID == "" {
			record.ID = id
		}
		if record.SessionID == "" {
			record.SessionID = sessionID
		}
		if record.TimeCreated == 0 {
			record.TimeCreated = normalizeEpochMillis(created)
		}
		out = append(out, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating message rows: %w", err)
	}

	return out, nil
}

func loadAllSessionsFromStorage(storage string) ([]model.SessionRecord, error) {
	sessionRoot := filepath.Join(storage, "session")
	if _, err := os.Stat(sessionRoot); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("checking session directory: %w", err)
	}

	out := make([]model.SessionRecord, 0)
	err := filepath.WalkDir(sessionRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading session file %s: %w", path, err)
		}

		var s model.SessionRecord
		if err := json.Unmarshal(data, &s); err != nil {
			return fmt.Errorf("parsing session file %s: %w", path, err)
		}
		s.Time.Created = normalizeEpochMillis(s.Time.Created)
		s.Time.Updated = normalizeEpochMillis(s.Time.Updated)
		out = append(out, s)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking session directory: %w", err)
	}

	return out, nil
}

func loadAllMessagesFromStorage(storage string) ([]model.MessageRecord, error) {
	messageRoot := filepath.Join(storage, "message")
	if _, err := os.Stat(messageRoot); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("checking message directory: %w", err)
	}

	out := make([]model.MessageRecord, 0)
	err := filepath.WalkDir(messageRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading message file %s: %w", path, err)
		}

		record, ok := parseMessageRecordJSON(data)
		if !ok {
			return nil
		}
		out = append(out, record)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking message directory: %w", err)
	}

	return out, nil
}

func parseMessageRecordJSON(data []byte) (model.MessageRecord, bool) {
	var payload struct {
		ID         string  `json:"id"`
		SessionID  string  `json:"sessionID"`
		Role       string  `json:"role"`
		ModelID    string  `json:"modelID"`
		ProviderID string  `json:"providerID"`
		Cost       float64 `json:"cost"`
		Time       struct {
			Created int64 `json:"created"`
		} `json:"time"`
		Model struct {
			ModelID    string `json:"modelID"`
			ProviderID string `json:"providerID"`
		} `json:"model"`
		Tokens struct {
			Input     int64 `json:"input"`
			Output    int64 `json:"output"`
			Reasoning int64 `json:"reasoning"`
			Cache     struct {
				Read  int64 `json:"read"`
				Write int64 `json:"write"`
			} `json:"cache"`
		} `json:"tokens"`
	}

	if err := json.Unmarshal(data, &payload); err != nil {
		return model.MessageRecord{}, false
	}

	modelID := payload.ModelID
	providerID := payload.ProviderID
	if modelID == "" {
		modelID = payload.Model.ModelID
	}
	if providerID == "" {
		providerID = payload.Model.ProviderID
	}

	record := model.MessageRecord{
		ID:          payload.ID,
		SessionID:   payload.SessionID,
		Role:        payload.Role,
		TimeCreated: normalizeEpochMillis(payload.Time.Created),
		ModelID:     modelID,
		ProviderID:  providerID,
		Cost:        payload.Cost,
		Tokens: model.TokenUsage{
			Input:      payload.Tokens.Input,
			Output:     payload.Tokens.Output,
			Reasoning:  payload.Tokens.Reasoning,
			CacheRead:  payload.Tokens.Cache.Read,
			CacheWrite: payload.Tokens.Cache.Write,
		},
	}

	return record, true
}
