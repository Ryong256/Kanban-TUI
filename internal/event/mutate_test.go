package event_test

import (
	"testing"

	"github.com/Ryong256/kanban/internal/db"
	"github.com/Ryong256/kanban/internal/event"
)

func TestDeleteTask_removes_task_and_its_events(t *testing.T) {
	d := db.OpenTest(t)

	id, err := event.Add(d, event.Insert{
		Type: event.TaskNew, Project: "proj", Title: "mistake", Source: "test",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := event.MoveTask(d, id, event.StatusInProgress, "test"); err != nil {
		t.Fatalf("MoveTask: %v", err)
	}

	if err := event.DeleteTask(d, id); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}

	var n int
	if err := d.QueryRow(`SELECT COUNT(*) FROM events WHERE id = ? OR ref_id = ?`, id, id).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Errorf("%d rows survived the delete, want 0", n)
	}
}

func TestDeleteTask_unknown_id_errors(t *testing.T) {
	d := db.OpenTest(t)
	if err := event.DeleteTask(d, 4242); err == nil {
		t.Error("DeleteTask on a missing id returned nil, want an error")
	}
}

func TestDeleteTask_refuses_non_task_events(t *testing.T) {
	d := db.OpenTest(t)

	id, err := event.Add(d, event.Insert{
		Type: event.ScopeShift, Project: "proj", Title: "pivot", Source: "test",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := event.DeleteTask(d, id); err == nil {
		t.Error("DeleteTask on a scope.shift returned nil, want an error")
	}
}

func TestConvertTaskToNote_keeps_content_clears_board(t *testing.T) {
	d := db.OpenTest(t)

	id, err := event.Add(d, event.Insert{
		Type:    event.TaskNew,
		Project: "proj",
		Scope:   "infra",
		Title:   "PR #829 shipped",
		Body:    "cross-tenant P0 closed",
		Source:  "test",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := event.MoveTask(d, id, event.StatusInProgress, "test"); err != nil {
		t.Fatalf("MoveTask: %v", err)
	}

	if err := event.ConvertTaskToNote(d, id); err != nil {
		t.Fatalf("ConvertTaskToNote: %v", err)
	}

	open, err := event.ListOpen(d, "proj", 0)
	if err != nil {
		t.Fatalf("ListOpen: %v", err)
	}
	if len(open) != 0 {
		t.Errorf("ListOpen = %+v, want the demoted task gone from the board", open)
	}

	notes, err := event.ListNotes(d, "proj", 0)
	if err != nil {
		t.Fatalf("ListNotes: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("ListNotes returned %d rows, want 1", len(notes))
	}
	if notes[0].Title != "PR #829 shipped" {
		t.Errorf("title = %q, want it preserved", notes[0].Title)
	}
	if !notes[0].Body.Valid || notes[0].Body.String != "cross-tenant P0 closed" {
		t.Errorf("body = %+v, want it preserved", notes[0].Body)
	}
	if !notes[0].Scope.Valid || notes[0].Scope.String != "infra" {
		t.Errorf("scope = %+v, want it preserved", notes[0].Scope)
	}

	// The stale status history must be gone, otherwise v_task_latest would
	// still resolve a column for a row that no longer has one.
	var updates int
	if err := d.QueryRow(`SELECT COUNT(*) FROM events WHERE ref_id = ?`, id).Scan(&updates); err != nil {
		t.Fatalf("count updates: %v", err)
	}
	if updates != 0 {
		t.Errorf("%d status events survived, want 0", updates)
	}

	var status any
	if err := d.QueryRow(`SELECT status FROM events WHERE id = ?`, id).Scan(&status); err != nil {
		t.Fatalf("select status: %v", err)
	}
	if status != nil {
		t.Errorf("status = %v, want NULL", status)
	}
}

func TestConvertTaskToNote_is_idempotent(t *testing.T) {
	d := db.OpenTest(t)

	id, err := event.Add(d, event.Insert{
		Type: event.Note, Project: "proj", Title: "already a note", Source: "test",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := event.ConvertTaskToNote(d, id); err != nil {
		t.Errorf("ConvertTaskToNote on an existing note = %v, want nil", err)
	}
}
