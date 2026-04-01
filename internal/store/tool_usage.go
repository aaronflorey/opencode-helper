package store

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"opencode-cli/internal/model"
)

func LoadBashToolUsageEvents(db *sql.DB, projectID string) ([]model.ToolUsageEvent, error) {
	if db == nil {
		return nil, fmt.Errorf("database is required")
	}

	var (
		rows *sql.Rows
		err  error
	)

	if projectID != "" {
		rows, err = db.Query(`
			SELECT p.session_id, p.data
			FROM part p
			JOIN session s ON s.id = p.session_id
			WHERE s.project_id = ?
				AND json_extract(p.data, '$.type') = 'tool'
				AND json_extract(p.data, '$.tool') = 'bash'
			ORDER BY p.time_created ASC, p.id ASC
		`, projectID)
	} else {
		rows, err = db.Query(`
			SELECT p.session_id, p.data
			FROM part p
			WHERE json_extract(p.data, '$.type') = 'tool'
				AND json_extract(p.data, '$.tool') = 'bash'
			ORDER BY p.time_created ASC, p.id ASC
		`)
	}
	if err != nil {
		return nil, fmt.Errorf("querying bash tool parts: %w", err)
	}
	defer rows.Close()

	result := make([]model.ToolUsageEvent, 0)
	for rows.Next() {
		var (
			sessionID string
			raw       string
		)
		if err := rows.Scan(&sessionID, &raw); err != nil {
			return nil, fmt.Errorf("scanning bash tool part row: %w", err)
		}

		var payload struct {
			State struct {
				Input struct {
					Command string `json:"command"`
				} `json:"input"`
				Output interface{} `json:"output"`
			} `json:"state"`
		}
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			continue
		}
		if payload.State.Input.Command == "" {
			continue
		}

		result = append(result, model.ToolUsageEvent{
			SessionID: sessionID,
			Command:   payload.State.Input.Command,
			Output:    stringifyToolOutput(payload.State.Output),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating bash tool part rows: %w", err)
	}

	return result, nil
}

func stringifyToolOutput(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(data)
	}
}
