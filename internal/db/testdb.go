package db

import (
	"database/sql"
	"fmt"
	"testing"
)

// OpenTest opens a fresh in-memory SQLite database with all migrations applied.
// It registers a cleanup function on t so the database is closed after the test.
func OpenTest(t testing.TB) *sql.DB {
	t.Helper()
	// Use a unique per-test name so parallel tests don't share state.
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_pragma=foreign_keys(1)", t.Name())
	d, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("OpenTest sql.Open: %v", err)
	}
	if err := migrate(d); err != nil {
		d.Close()
		t.Fatalf("OpenTest migrate: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}
