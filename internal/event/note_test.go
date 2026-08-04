package event_test

import (
	"testing"

	"github.com/Ryong256/kanban/internal/db"
	"github.com/Ryong256/kanban/internal/event"
)

// A note must stay out of every task surface. This is the whole point of the
// type: log entries used to be filed as task.new and rotted in backlog forever.
func TestNote_never_reaches_task_surfaces(t *testing.T) {
	d := db.OpenTest(t)

	if _, err := event.Add(d, event.Insert{
		Type:    event.Note,
		Project: "proj",
		Title:   "shipped PR #829",
		Source:  "test",
	}); err != nil {
		t.Fatalf("Add note: %v", err)
	}
	if _, err := event.Add(d, event.Insert{
		Type:    event.TaskNew,
		Project: "proj",
		Title:   "real task",
		Source:  "test",
	}); err != nil {
		t.Fatalf("Add task: %v", err)
	}

	open, err := event.ListOpen(d, "proj", 0)
	if err != nil {
		t.Fatalf("ListOpen: %v", err)
	}
	if len(open) != 1 || open[0].Title != "real task" {
		t.Errorf("ListOpen = %+v, want only the task.new row", open)
	}

	n, err := event.CountOpen(d, "proj")
	if err != nil {
		t.Fatalf("CountOpen: %v", err)
	}
	if n != 1 {
		t.Errorf("CountOpen = %d, want 1", n)
	}

	board, err := event.ListByStatus(d, "proj", 0, 0)
	if err != nil {
		t.Fatalf("ListByStatus: %v", err)
	}
	total := 0
	for _, s := range event.AllStatuses() {
		total += len(board.Board[s])
	}
	if total != 1 {
		t.Errorf("board holds %d rows, want 1", total)
	}
}

// A note carries no status, so nothing downstream can misread it as a column.
func TestNote_has_no_status(t *testing.T) {
	d := db.OpenTest(t)

	if _, err := event.Add(d, event.Insert{
		Type:    event.Note,
		Project: "proj",
		Title:   "root cause confirmed",
		Source:  "test",
	}); err != nil {
		t.Fatalf("Add note: %v", err)
	}

	var status any
	if err := d.QueryRow(`SELECT status FROM events WHERE type = 'note'`).Scan(&status); err != nil {
		t.Fatalf("select status: %v", err)
	}
	if status != nil {
		t.Errorf("note status = %v, want NULL", status)
	}
}

func TestListNotes_returns_notes_only(t *testing.T) {
	d := db.OpenTest(t)

	for _, title := range []string{"first", "second"} {
		if _, err := event.Add(d, event.Insert{
			Type:    event.Note,
			Project: "proj",
			Title:   title,
			Source:  "test",
		}); err != nil {
			t.Fatalf("Add note %q: %v", title, err)
		}
	}
	if _, err := event.Add(d, event.Insert{
		Type:    event.TaskNew,
		Project: "proj",
		Title:   "a task",
		Source:  "test",
	}); err != nil {
		t.Fatalf("Add task: %v", err)
	}

	notes, err := event.ListNotes(d, "proj", 0)
	if err != nil {
		t.Fatalf("ListNotes: %v", err)
	}
	if len(notes) != 2 {
		t.Fatalf("ListNotes returned %d rows, want 2", len(notes))
	}
	// Newest first.
	if notes[0].Title != "second" {
		t.Errorf("notes[0].Title = %q, want %q", notes[0].Title, "second")
	}

	notes, err = event.ListNotes(d, "other-proj", 0)
	if err != nil {
		t.Fatalf("ListNotes other project: %v", err)
	}
	if len(notes) != 0 {
		t.Errorf("ListNotes for another project returned %d rows, want 0", len(notes))
	}
}

func TestNote_migration_preserves_existing_events(t *testing.T) {
	d := db.OpenTest(t)

	// The 0006 rebuild copies rows between tables; ids and ref_id links must
	// survive it, since task.done/task.update resolve tasks through ref_id.
	id, err := event.Add(d, event.Insert{
		Type:    event.TaskNew,
		Project: "proj",
		Title:   "task",
		Source:  "test",
	})
	if err != nil {
		t.Fatalf("Add task: %v", err)
	}
	if _, err := event.MoveTask(d, id, event.StatusDone, "test"); err != nil {
		t.Fatalf("MoveTask: %v", err)
	}

	open, err := event.ListOpen(d, "proj", 0)
	if err != nil {
		t.Fatalf("ListOpen: %v", err)
	}
	if len(open) != 0 {
		t.Errorf("ListOpen = %+v, want the closed task to be gone", open)
	}
}
