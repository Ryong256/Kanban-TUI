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

	// LastTS is the timestamp of the most recent event touching this task.
	// Ordering by it surfaces what actually moved recently, which is more
	// useful than insertion order once a column holds a hundred rows.
	LastTS int64

	// StatusSince is when the task entered its current column. Age in a column
	// is the signal a personal kanban exists to give: a week in IN PROGRESS
	// means something the title never will.
	StatusSince int64

	// ScopeMoved reports that this task's scope has scope.shift/scope.expand
	// events. Scope events carry no ref_id — they attach to (project, scope) —
	// so this is the only honest way to surface them next to a task.
	ScopeMoved bool
}

// taskSelect is shared by the board queries. status_since only considers
// updates that actually carried a status: a title-only task.update must not
// reset a task's age in its column.
const taskSelect = `
        SELECT t.id, t.created_ts, t.project, t.scope, t.title, t.body, t.status,
               COALESCE((SELECT MAX(e.ts) FROM events e
                         WHERE e.id = t.id OR e.ref_id = t.id), t.created_ts) AS last_ts,
               COALESCE((SELECT MAX(u.ts) FROM events u
                         WHERE u.ref_id = t.id AND u.type = 'task.update'
                           AND u.status IS NOT NULL), t.created_ts) AS status_since
        FROM   v_task_latest t
`

func scanTasks(rows *sql.Rows) ([]OpenTask, error) {
	defer rows.Close()
	var out []OpenTask
	for rows.Next() {
		var t OpenTask
		if err := rows.Scan(&t.ID, &t.TS, &t.Project, &t.Scope, &t.Title, &t.Body,
			&t.Status, &t.LastTS, &t.StatusSince); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// MovedScopes returns the set of scopes that have scope.shift/scope.expand
// events, keyed "project\x00scope".
func MovedScopes(d *sql.DB) (map[string]bool, error) {
	rows, err := d.Query(`
        SELECT DISTINCT project, scope
        FROM   events
        WHERE  type IN ('scope.shift', 'scope.expand') AND scope IS NOT NULL
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]bool)
	for rows.Next() {
		var p, s string
		if err := rows.Scan(&p, &s); err != nil {
			return nil, err
		}
		out[ScopeKey(p, s)] = true
	}
	return out, rows.Err()
}

// ScopeKey builds the lookup key used by MovedScopes.
func ScopeKey(project, scope string) string {
	return project + "\x00" + scope
}

// DeleteProjectEvents removes every event belonging to a project and returns how
// many rows went. Used by `kb project rm --purge`.
func DeleteProjectEvents(d *sql.DB, project string) (int64, error) {
	if project == "" {
		return 0, fmt.Errorf("project is required")
	}
	tx, err := d.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Count before deleting anything: the two DELETEs below split the project's
	// rows between them, so neither statement's RowsAffected is the number the
	// caller wants to report.
	var n int64
	if err := tx.QueryRow(`SELECT COUNT(*) FROM events WHERE project = ?`, project).Scan(&n); err != nil {
		return 0, err
	}

	// Children first: ref_id is a foreign key onto events(id).
	if _, err := tx.Exec(`
        DELETE FROM events
        WHERE  ref_id IN (SELECT id FROM events WHERE project = ?)
    `, project); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`DELETE FROM events WHERE project = ?`, project); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return n, nil
}

func ListOpen(d *sql.DB, project string, limit int) ([]OpenTask, error) {
	q := taskSelect + `
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
	return scanTasks(rows)
}

// ListByStatus returns tasks grouped by their current status.
//
// Open columns are ordered by most-recent activity, not insertion: with a
// hundred rows in a column, "what moved lately" is the useful order.
//
// The done column is windowed to doneWindow (a duration back from now). An
// all-time done column is a log, not a decision surface — 437 rows in a panel
// that fits 15 helps nobody. DoneTotal still carries the all-time count.
func ListByStatus(d *sql.DB, project string, doneLimit int, doneWindow time.Duration) (*BoardResult, error) {
	q := taskSelect + ` WHERE t.status != 'done' `
	args := []any{}
	if project != "" {
		q += " AND t.project = ? "
		args = append(args, project)
	}
	q += " ORDER BY last_ts DESC "
	rows, err := d.Query(q, args...)
	if err != nil {
		return nil, err
	}
	open, err := scanTasks(rows)
	if err != nil {
		return nil, err
	}

	moved, err := MovedScopes(d)
	if err != nil {
		// Scope markers are decoration; losing them must not cost the board.
		moved = map[string]bool{}
	}
	markScope := func(t *OpenTask) {
		if t.Scope.Valid && t.Scope.String != "" {
			t.ScopeMoved = moved[ScopeKey(t.Project, t.Scope.String)]
		}
	}

	result := &BoardResult{Board: make(map[string][]OpenTask)}
	for _, s := range AllStatuses() {
		result.Board[s] = []OpenTask{} // initialize all columns even if empty
	}
	for _, t := range open {
		status := t.Status
		if status == "" {
			status = StatusBacklog
		}
		if _, ok := result.Board[status]; ok {
			markScope(&t)
			result.Board[status] = append(result.Board[status], t)
		}
	}

	doneRows, err := listDone(d, project, doneLimit, doneWindow)
	if err != nil {
		return nil, err
	}
	if doneRows.Tasks == nil {
		doneRows.Tasks = []OpenTask{}
	}
	for i := range doneRows.Tasks {
		markScope(&doneRows.Tasks[i])
	}
	result.Board[StatusDone] = doneRows.Tasks
	result.DoneTotal = doneRows.Total
	result.DoneInWindow = doneRows.InWindow
	return result, nil
}

// BoardResult is returned by ListByStatus.
type BoardResult struct {
	Board map[string][]OpenTask
	// DoneTotal is the all-time done count; DoneInWindow counts only those
	// inside the requested window, which is what the column shows.
	DoneTotal    int
	DoneInWindow int
}

// doneQueryResult holds the windowed task slice, the in-window count and the
// all-time total.
type doneQueryResult struct {
	Tasks    []OpenTask
	Total    int
	InWindow int
}

// listDone fetches done tasks finished within window, ordered by most-recent
// activity and capped at limit. A zero window means all time.
func listDone(d *sql.DB, proj string, limit int, window time.Duration) (doneQueryResult, error) {
	cutoff := int64(0)
	if window > 0 {
		cutoff = time.Now().Add(-window).Unix()
	}

	lastTS := `(SELECT MAX(e.ts) FROM events e WHERE e.id = t.id OR e.ref_id = t.id)`

	countQ := func(withCutoff bool) (string, []any) {
		q := `SELECT COUNT(*) FROM v_task_latest t WHERE t.status = 'done' `
		args := []any{}
		if proj != "" {
			q += " AND t.project = ? "
			args = append(args, proj)
		}
		if withCutoff && cutoff > 0 {
			q += " AND COALESCE(" + lastTS + ", t.created_ts) >= ? "
			args = append(args, cutoff)
		}
		return q, args
	}

	var total, inWindow int
	q, args := countQ(false)
	if err := d.QueryRow(q, args...).Scan(&total); err != nil {
		return doneQueryResult{}, err
	}
	q, args = countQ(true)
	if err := d.QueryRow(q, args...).Scan(&inWindow); err != nil {
		return doneQueryResult{}, err
	}

	listQ := taskSelect + ` WHERE t.status = 'done' `
	listArgs := []any{}
	if proj != "" {
		listQ += " AND t.project = ? "
		listArgs = append(listArgs, proj)
	}
	if cutoff > 0 {
		listQ += " AND COALESCE(" + lastTS + ", t.created_ts) >= ? "
		listArgs = append(listArgs, cutoff)
	}
	listQ += " ORDER BY last_ts DESC "
	if limit > 0 {
		listQ += " LIMIT ? "
		listArgs = append(listArgs, limit)
	}

	rows, err := d.Query(listQ, listArgs...)
	if err != nil {
		return doneQueryResult{}, err
	}
	tasks, err := scanTasks(rows)
	if err != nil {
		return doneQueryResult{}, err
	}
	return doneQueryResult{Tasks: tasks, Total: total, InWindow: inWindow}, nil
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

// CountOpenByProject returns the open-task count per project, for the tab bar.
func CountOpenByProject(d *sql.DB) (map[string]int, error) {
	rows, err := d.Query(`SELECT project, COUNT(*) FROM v_open_tasks GROUP BY project`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int)
	for rows.Next() {
		var p string
		var n int
		if err := rows.Scan(&p, &n); err != nil {
			return nil, err
		}
		out[p] = n
	}
	return out, rows.Err()
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

// DeleteTask removes a task and every event referencing it.
//
// The log is append-only for history that means something. A task filed by
// mistake has no history worth keeping, and closing it as done would be a lie:
// it was never work. Deleting is the honest operation, and leaving no way to do
// it is what let the board fill with rows nobody could ever clear.
func DeleteTask(d *sql.DB, taskID int64) error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var typ string
	if err := tx.QueryRow(`SELECT type FROM events WHERE id = ?`, taskID).Scan(&typ); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("event #%d not found", taskID)
		}
		return err
	}
	if typ != string(TaskNew) && typ != string(Note) {
		return fmt.Errorf("event #%d is a %s, not a task or note", taskID, typ)
	}

	// Children first: ref_id has a foreign key onto events(id).
	if _, err := tx.Exec(`DELETE FROM events WHERE ref_id = ?`, taskID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM events WHERE id = ?`, taskID); err != nil {
		return err
	}
	return tx.Commit()
}

// ConvertTaskToNote reclassifies a task as a note: it keeps the title, body and
// scope, drops the status, and discards the task's status history.
//
// This is the non-destructive way to clear a backlog row that was never work —
// a status report filed as a task. The content survives, the board is freed.
func ConvertTaskToNote(d *sql.DB, taskID int64) error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var typ string
	if err := tx.QueryRow(`SELECT type FROM events WHERE id = ?`, taskID).Scan(&typ); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("task #%d not found", taskID)
		}
		return err
	}
	if typ == string(Note) {
		return nil // already a note
	}
	if typ != string(TaskNew) {
		return fmt.Errorf("event #%d is a %s, not a task", taskID, typ)
	}

	// A note has no lifecycle, so the task.update/task.done rows pointing at it
	// become meaningless. Keeping them would leave v_task_latest resolving a
	// status for a row that no longer has one.
	if _, err := tx.Exec(`DELETE FROM events WHERE ref_id = ?`, taskID); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`UPDATE events SET type = 'note', status = NULL WHERE id = ?`, taskID,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// NoteEntry is a standalone log entry. Notes carry no status and never appear
// on the board; they exist so that "this happened" has somewhere to go other
// than the backlog.
type NoteEntry struct {
	ID      int64
	TS      int64
	Project string
	Scope   sql.NullString
	Title   string
	Body    sql.NullString
}

// ListNotes returns the most recent notes, newest first.
func ListNotes(d *sql.DB, project string, limit int) ([]NoteEntry, error) {
	q := `
        SELECT id, ts, project, scope, title, body
        FROM   events
        WHERE  type = 'note'
    `
	args := []any{}
	if project != "" {
		q += " AND project = ? "
		args = append(args, project)
	}
	q += " ORDER BY ts DESC, id DESC "
	if limit > 0 {
		q += " LIMIT ? "
		args = append(args, limit)
	}
	rows, err := d.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NoteEntry
	for rows.Next() {
		var n NoteEntry
		if err := rows.Scan(&n.ID, &n.TS, &n.Project, &n.Scope, &n.Title, &n.Body); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
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
func MarkDone(d *sql.DB, refID int64) (int64, error) {
	return MoveTask(d, refID, StatusDone, "manual")
}
