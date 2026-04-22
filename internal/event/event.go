package event

import (
	"database/sql"
	"fmt"
	"time"
)

type Type string

const (
	TaskNew     Type = "task.new"
	TaskDone    Type = "task.done"
	TaskUpdate  Type = "task.update"
	ScopeShift  Type = "scope.shift"
	ScopeExpand Type = "scope.expand"
)

func ValidType(t string) bool {
	switch Type(t) {
	case TaskNew, TaskDone, TaskUpdate, ScopeShift, ScopeExpand:
		return true
	}
	return false
}

type Event struct {
	ID        int64
	TS        int64
	Type      Type
	Project   string
	Scope     sql.NullString
	Title     string
	Body      sql.NullString
	RefID     sql.NullInt64
	SessionID sql.NullString
	Source    string
	MetaJSON  sql.NullString
}

type Insert struct {
	Type      Type
	Project   string
	Scope     string
	Title     string
	Body      string
	RefID     int64
	SessionID string
	Source    string
	MetaJSON  string
}

func Add(d *sql.DB, in Insert) (int64, error) {
	if !ValidType(string(in.Type)) {
		return 0, fmt.Errorf("invalid event type: %q", in.Type)
	}
	if in.Project == "" {
		return 0, fmt.Errorf("project is required")
	}
	if in.Title == "" {
		return 0, fmt.Errorf("title is required")
	}
	if in.Source == "" {
		in.Source = "manual"
	}
	res, err := d.Exec(`
        INSERT INTO events (ts, type, project, scope, title, body, ref_id, session_id, source, meta_json)
        VALUES (?,  ?,    ?,       ?,     ?,     ?,    ?,      ?,          ?,      ?)
    `,
		time.Now().Unix(),
		string(in.Type),
		in.Project,
		nullStr(in.Scope),
		in.Title,
		nullStr(in.Body),
		nullInt(in.RefID),
		nullStr(in.SessionID),
		in.Source,
		nullStr(in.MetaJSON),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullInt(i int64) any {
	if i == 0 {
		return nil
	}
	return i
}
