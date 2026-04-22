package project

import (
	"database/sql"
	"fmt"
	"time"
)

type Project struct {
	Name      string
	Path      string
	CreatedTS int64
}

func Add(d *sql.DB, name, path string) error {
	if name == "" {
		return fmt.Errorf("project name is required")
	}
	if path == "" {
		return fmt.Errorf("project path is required")
	}
	_, err := d.Exec(
		`INSERT INTO projects (name, path, created_ts) VALUES (?, ?, ?)`,
		name, path, time.Now().Unix(),
	)
	return err
}

// Ensure registers a project if it doesn't exist yet.
// Uses INSERT OR IGNORE so it's safe to call repeatedly.
func Ensure(d *sql.DB, name, path string) {
	if name == "" || path == "" {
		return
	}
	_, _ = d.Exec(
		`INSERT OR IGNORE INTO projects (name, path, created_ts) VALUES (?, ?, ?)`,
		name, path, time.Now().Unix(),
	)
}

func Remove(d *sql.DB, name string) error {
	res, err := d.Exec(`DELETE FROM projects WHERE name = ?`, name)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("project %q not found", name)
	}
	return nil
}

func List(d *sql.DB) ([]Project, error) {
	rows, err := d.Query(`SELECT name, path, created_ts FROM projects ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.Name, &p.Path, &p.CreatedTS); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// FindByPath returns the project whose registered path is a prefix of (or equal to) dir.
// This lets DetectProject resolve cwd → project name from the registry.
func FindByPath(d *sql.DB, dir string) (Project, bool, error) {
	rows, err := d.Query(`SELECT name, path, created_ts FROM projects ORDER BY length(path) DESC`)
	if err != nil {
		return Project{}, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.Name, &p.Path, &p.CreatedTS); err != nil {
			return Project{}, false, err
		}
		// Longest-path-first: first match wins (most specific)
		if dir == p.Path || hasPathPrefix(dir, p.Path) {
			return p, true, nil
		}
	}
	return Project{}, false, rows.Err()
}

// hasPathPrefix checks if dir starts with prefix as a proper path prefix.
// e.g. /home/user/projects/foo is under /home/user/projects but not /home/user/proj.
func hasPathPrefix(dir, prefix string) bool {
	if len(dir) <= len(prefix) {
		return false
	}
	return dir[:len(prefix)] == prefix && dir[len(prefix)] == '/'
}

func Names(d *sql.DB) ([]string, error) {
	rows, err := d.Query(`SELECT name FROM projects ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
