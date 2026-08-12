package cli

import (
	"database/sql"

	"github.com/Ryong256/kanban/internal/db"
)

// envReconciling marks a process that is already running a reconcile pass, so
// any kb it spawns short-circuits instead of recursing.
const envReconciling = "KB_RECONCILING"

// testDB is injected by tests to avoid hitting the real data directory.
var testDB *sql.DB

// openDB returns the application database together with the function that
// releases it. Tests inject a database whose lifetime the test owns, so the
// closer is a no-op there — callers always defer it and never have to ask
// which database they got.
var openDB = func() (*sql.DB, func(), error) {
	if testDB != nil {
		return testDB, func() {}, nil
	}
	d, err := db.Open()
	if err != nil {
		return nil, func() {}, err
	}
	return d, func() { _ = d.Close() }, nil
}

// dbQuerier is the small interface findDuplicate needs.
type dbQuerier interface {
	Query(string, ...any) (*sql.Rows, error)
}
