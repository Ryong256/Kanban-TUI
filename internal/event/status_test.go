package event_test

import (
	"testing"

	"github.com/Ryong256/kanban/internal/db"
	"github.com/Ryong256/kanban/internal/event"
)

// v_task_latest used to read title, body AND status from the single newest
// task.update row. A task.update written without a status (which `kb event
// --type=task.update` does by default) has status NULL, so once such a row
// became the newest, the status fell through to the task.new default and the
// task silently dropped back to backlog.
//
// This asserts against the view directly, since the view is what was wrong.
func TestTaskLatest_status_survives_a_title_only_update(t *testing.T) {
	d := db.OpenTest(t)

	id, err := event.Add(d, event.Insert{
		Type: event.TaskNew, Project: "proj", Title: "task", Source: "test",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := event.MoveTask(d, id, event.StatusInProgress, "test"); err != nil {
		t.Fatalf("MoveTask: %v", err)
	}

	// A later update that carries no status — a rename, not a move.
	if _, err := event.Add(d, event.Insert{
		Type: event.TaskUpdate, Project: "proj", Title: "renamed", RefID: id, Source: "test",
	}); err != nil {
		t.Fatalf("Add title update: %v", err)
	}

	var status, title string
	if err := d.QueryRow(
		`SELECT status, title FROM v_task_latest WHERE id = ?`, id,
	).Scan(&status, &title); err != nil {
		t.Fatalf("select: %v", err)
	}

	if status != event.StatusInProgress {
		t.Errorf("status = %q, want %q — a rename must not move the task",
			status, event.StatusInProgress)
	}
	// The rename must still take effect; only the status is sourced separately.
	if title != "renamed" {
		t.Errorf("title = %q, want %q", title, "renamed")
	}
}

// A task.done still forces 'done' regardless of update ordering.
func TestTaskLatest_done_still_wins(t *testing.T) {
	d := db.OpenTest(t)

	id, err := event.Add(d, event.Insert{
		Type: event.TaskNew, Project: "proj", Title: "task", Source: "test",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := event.MoveTask(d, id, event.StatusDone, "test"); err != nil {
		t.Fatalf("MoveTask: %v", err)
	}
	if _, err := event.Add(d, event.Insert{
		Type: event.TaskUpdate, Project: "proj", Title: "renamed", RefID: id, Source: "test",
	}); err != nil {
		t.Fatalf("Add title update: %v", err)
	}

	var status string
	if err := d.QueryRow(`SELECT status FROM v_task_latest WHERE id = ?`, id).Scan(&status); err != nil {
		t.Fatalf("select: %v", err)
	}
	if status != event.StatusDone {
		t.Errorf("status = %q, want %q", status, event.StatusDone)
	}
}
