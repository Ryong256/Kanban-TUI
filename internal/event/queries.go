package event

import (
	"database/sql"
	"fmt"
	"time"
)

type OpenTask struct {
	ID      int64
	TS      int64
	Project string
	Scope   sql.NullString
	Title   string
	Body    sql.NullString
	Status  string
}

func ListOpen(d *sql.DB, project string, limit int) ([]OpenTask, error) {
	q := `
        SELECT t.id, t.created_ts, t.project, t.scope, t.title, t.body, t.status
        FROM   v_task_latest t
        WHERE  NOT EXISTS (
                   SELECT 1 FROM events done
                   WHERE  done.type   = 'task.done'
                     AND  done.ref_id = t.id
               )
    `
	args := []any{}
	if project != "" {
		q += " AND t.project = ? "
		args = append(args, project)
	}
	q += " ORDER BY t.created_ts DESC "
	if limit > 0 {
		q += " LIMIT ? "
		args = append(args, limit)
	}
	rows, err := d.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OpenTask
	for rows.Next() {
		var t OpenTask
		if err := rows.Scan(&t.ID, &t.TS, &t.Project, &t.Scope, &t.Title, &t.Body, &t.Status); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ListAllByStatus returns ALL tasks (including done) for a project grouped by their current status.
// Unlike ListOpen, this does NOT filter out tasks that have a task.done event.
func ListByStatus(d *sql.DB, project string) (map[string][]OpenTask, error) {
	q := `
        SELECT t.id, t.created_ts, t.project, t.scope, t.title, t.body, t.status
        FROM   v_task_latest t
        WHERE  1 = 1
    `
	args := []any{}
	if project != "" {
		q += " AND t.project = ? "
		args = append(args, project)
	}
	q += " ORDER BY t.created_ts DESC "
	rows, err := d.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	grouped := make(map[string][]OpenTask)
	for _, s := range AllStatuses() {
		grouped[s] = []OpenTask{} // initialize all columns even if empty
	}
	for rows.Next() {
		var t OpenTask
		if err := rows.Scan(&t.ID, &t.TS, &t.Project, &t.Scope, &t.Title, &t.Body, &t.Status); err != nil {
			return nil, err
		}
		status := t.Status
		if status == "" {
			status = StatusBacklog
		}
		// Only add to known columns; ignore unknown statuses
		if _, ok := grouped[status]; ok {
			grouped[status] = append(grouped[status], t)
		}
	}
	return grouped, rows.Err()
}

// MoveTask transitions a task to a new status by appending a task.update event.
// If the new status is "done", it also appends a task.done event.
func MoveTask(d *sql.DB, taskID int64, newStatus, source string) (int64, error) {
	if !ValidStatus(newStatus) {
		return 0, fmt.Errorf("invalid status: %q", newStatus)
	}

	// Fetch original task info
	var origProject, origTitle string
	err := d.QueryRow(
		`SELECT project, title FROM events WHERE id = ? AND type = 'task.new'`,
		taskID,
	).Scan(&origProject, &origTitle)
	if err != nil {
		return 0, fmt.Errorf("task #%d not found: %w", taskID, err)
	}

	if source == "" {
		source = "manual"
	}

	// Insert status update event
	res, err := d.Exec(`
        INSERT INTO events (ts, type, project, title, ref_id, source, status)
        VALUES (?, 'task.update', ?, ?, ?, ?, ?)
    `, time.Now().Unix(), origProject, origTitle, taskID, source, newStatus)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()

	// If moved to "done", also insert the task.done event for backward compat
	if newStatus == StatusDone {
		_, err = d.Exec(`
            INSERT INTO events (ts, type, project, title, ref_id, source, status)
            VALUES (?, 'task.done', ?, ?, ?, ?, 'done')
        `, time.Now().Unix(), origProject, origTitle, taskID, source)
		if err != nil {
			return id, err
		}
	}

	return id, nil
}

func CountOpen(d *sql.DB, project string) (int, error) {
	q := `
        SELECT COUNT(*)
        FROM   v_open_tasks
        WHERE  1 = 1
    `
	args := []any{}
	if project != "" {
		q += " AND project = ?"
		args = append(args, project)
	}
	var n int
	err := d.QueryRow(q, args...).Scan(&n)
	return n, err
}

type TimelineEntry struct {
	ID    int64
	TS    int64
	Type  string
	Title string
	Body  sql.NullString
}

func ScopeTimeline(d *sql.DB, project, scope string) ([]TimelineEntry, error) {
	rows, err := d.Query(`
        SELECT id, ts, type, title, body
        FROM   events
        WHERE  project = ? AND scope = ?
        ORDER  BY ts ASC, id ASC
    `, project, scope)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TimelineEntry
	for rows.Next() {
		var e TimelineEntry
		if err := rows.Scan(&e.ID, &e.TS, &e.Type, &e.Title, &e.Body); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// TaskTimeline returns the full event history for a task (creation + all updates).
func TaskTimeline(d *sql.DB, taskID int64) ([]TimelineEntry, error) {
	rows, err := d.Query(`
        SELECT id, ts, type, title, body, COALESCE(status, '')
        FROM   events
        WHERE  id = ? OR ref_id = ?
        ORDER  BY ts ASC, id ASC
    `, taskID, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TimelineEntry
	for rows.Next() {
		var e TimelineEntry
		var status string
		if err := rows.Scan(&e.ID, &e.TS, &e.Type, &e.Title, &e.Body, &status); err != nil {
			return nil, err
		}
		if status != "" {
			e.Type = e.Type + " → " + status
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// MarkDone is kept for backward compat with CLI `kb done <id>`.
func MarkDone(d *sql.DB, refID int64, project string) (int64, error) {
	return MoveTask(d, refID, StatusDone, "manual")
}
