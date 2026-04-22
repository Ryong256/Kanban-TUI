package db

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"

	"github.com/Ryong256/kanban/internal/store"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func Open() (*sql.DB, error) {
	if err := store.EnsureDataDir(); err != nil {
		return nil, fmt.Errorf("ensure data dir: %w", err)
	}
	dsn := "file:" + store.DBPath() + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	d, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := d.Ping(); err != nil {
		return nil, err
	}
	if err := migrate(d); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return d, nil
}

func migrate(d *sql.DB) error {
	// Create migration tracking table
	if _, err := d.Exec(`CREATE TABLE IF NOT EXISTS _migrations (name TEXT PRIMARY KEY)`); err != nil {
		return fmt.Errorf("create _migrations table: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, n := range names {
		// Skip already-applied migrations
		var count int
		if err := d.QueryRow(`SELECT COUNT(*) FROM _migrations WHERE name = ?`, n).Scan(&count); err != nil {
			return fmt.Errorf("check migration %s: %w", n, err)
		}
		if count > 0 {
			continue
		}

		b, err := migrationsFS.ReadFile("migrations/" + n)
		if err != nil {
			return err
		}
		if _, err := d.Exec(string(b)); err != nil {
			return fmt.Errorf("%s: %w", n, err)
		}

		// Mark as applied
		if _, err := d.Exec(`INSERT INTO _migrations (name) VALUES (?)`, n); err != nil {
			return fmt.Errorf("record migration %s: %w", n, err)
		}
	}
	return nil
}
