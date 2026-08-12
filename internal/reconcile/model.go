package reconcile

// Meta is the schema-v1 metadata carried by task.new events.
type Meta struct {
	SchemaVersion    int    `json:"schema_version"`
	ClosureCondition string `json:"closure_condition"`
	EvidenceType     string `json:"evidence_type"`
}

// Config controls a reconcile run.
type Config struct {
	Project    string
	SessionID  string
	StaleAfter int64
	StaleLimit int
	DryRun     bool
	Now        int64
}

// TaskSnapshot is the JSON representation of a task in reconcile output.
type TaskSnapshot struct {
	ID               int64    `json:"id"`
	Project          string   `json:"project"`
	Scope            string   `json:"scope,omitempty"`
	Title            string   `json:"title"`
	Status           string   `json:"status"`
	ClosureCondition string   `json:"closure_condition"`
	EvidenceType     string   `json:"evidence_type"`
	EvidenceValue    string   `json:"evidence_value,omitempty"`
	SessionID        string   `json:"session_id,omitempty"`
	Flags            []string `json:"flags,omitempty"`
	LastTS           int64    `json:"last_ts"`
	StaleAgeHours    int64    `json:"stale_age_hours,omitempty"`
}

// Result is the schema-v1 reconcile output.
type Result struct {
	SchemaVersion int            `json:"schema_version"`
	Project       string         `json:"project"`
	SessionID     string         `json:"session_id"`
	DryRun        bool           `json:"dry_run"`
	NoOp          bool           `json:"no_op,omitempty"`
	SessionOwned  []TaskSnapshot `json:"session_owned"`
	StaleReview   []TaskSnapshot `json:"stale_review"`
	Errors        []string       `json:"errors"`
}
