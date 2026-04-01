package model

type ProjectRecord struct {
	ID       string `json:"id"`
	Worktree string `json:"worktree"`
	Time     struct {
		Created int64 `json:"created"`
		Updated int64 `json:"updated"`
	} `json:"time"`
}

type SessionRecord struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectID"`
	Directory string `json:"directory"`
	Title     string `json:"title"`
	Time      struct {
		Created int64 `json:"created"`
		Updated int64 `json:"updated"`
	} `json:"time"`
}

type DiffRecord struct {
	File      string `json:"file"`
	Before    string `json:"before"`
	After     string `json:"after"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Status    string `json:"status"`
}

type FileEvent struct {
	Session SessionRecord
	Change  DiffRecord
}

type ContentSnapshot struct {
	File      string
	Content   string
	Timestamp int64
	Source    string
	SessionID string
}

type ToolUsageEvent struct {
	SessionID string
	Command   string
	Output    string
}
