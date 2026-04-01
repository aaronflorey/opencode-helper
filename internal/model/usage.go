package model

type MessageRecord struct {
	ID          string
	SessionID   string
	Role        string
	TimeCreated int64
	ModelID     string
	ProviderID  string
	Cost        float64
	Tokens      TokenUsage
}

type TokenUsage struct {
	Input      int64
	Output     int64
	Reasoning  int64
	CacheRead  int64
	CacheWrite int64
}
