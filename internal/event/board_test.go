package event_test

import (
	"testing"
	"time"

	"github.com/Ryong256/kanban/internal/db"
	"github.com/Ryong256/kanban/internal/event"
)

func TestListByStatus_orders_by_recent_activity(t *testing.T) {
	d := db.OpenTest(t)

	older, err := event.Add(d, event.Insert{
		Type: event.TaskNew, Project: "proj", Title: "older", Source: "test",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := event.Add(d, event.Insert{
		Type: event.TaskNew, Project: "proj", Title: "newer", Source: "test",
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Touch the older task so it becomes the most recently active one.
	if _, err := d.Exec(
		`UPDATE events SET ts = ts + 1000 WHERE id = ?`, older,
	); err != nil {
		t.Fatalf("touch: %v", err)
	}

	result, err := event.ListByStatus(d, "proj", 0, 0)
	if err != nil {
		t.Fatalf("ListByStatus: %v", err)
	}
	backlog := result.Board[event.StatusBacklog]
	if len(backlog) != 2 {
		t.Fatalf("expected 2 backlog tasks, got %d", len(backlog))
	}
	if backlog[0].Title != "older" {
		t.Errorf("column head = %q, want the most recently touched task (%q)", backlog[0].Title, "older")
	}
}

func TestListByStatus_done_window_excludes_old_work(t *testing.T) {
	d := db.OpenTest(t)

	recent, err := event.Add(d, event.Insert{
		Type: event.TaskNew, Project: "proj", Title: "recent", Source: "test",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	ancient, err := event.Add(d, event.Insert{
		Type: event.TaskNew, Project: "proj", Title: "ancient", Source: "test",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	for _, id := range []int64{recent, ancient} {
		if _, err := event.MoveTask(d, id, event.StatusDone, "test"); err != nil {
			t.Fatalf("MoveTask: %v", err)
		}
	}

	// Push every event of the ancient task a month back.
	old := time.Now().Add(-30 * 24 * time.Hour).Unix()
	if _, err := d.Exec(
		`UPDATE events SET ts = ? WHERE id = ? OR ref_id = ?`, old, ancient, ancient,
	); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	result, err := event.ListByStatus(d, "proj", 0, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("ListByStatus: %v", err)
	}

	if result.DoneTotal != 2 {
		t.Errorf("DoneTotal = %d, want 2 (all time)", result.DoneTotal)
	}
	if result.DoneInWindow != 1 {
		t.Errorf("DoneInWindow = %d, want 1", result.DoneInWindow)
	}
	done := result.Board[event.StatusDone]
	if len(done) != 1 || done[0].Title != "recent" {
		t.Errorf("done column = %+v, want only the recent task", done)
	}
}

func TestListByStatus_zero_window_means_all_time(t *testing.T) {
	d := db.OpenTest(t)

	id, err := event.Add(d, event.Insert{
		Type: event.TaskNew, Project: "proj", Title: "ancient", Source: "test",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := event.MoveTask(d, id, event.StatusDone, "test"); err != nil {
		t.Fatalf("MoveTask: %v", err)
	}
	old := time.Now().Add(-365 * 24 * time.Hour).Unix()
	if _, err := d.Exec(`UPDATE events SET ts = ? WHERE id = ? OR ref_id = ?`, old, id, id); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	result, err := event.ListByStatus(d, "proj", 0, 0)
	if err != nil {
		t.Fatalf("ListByStatus: %v", err)
	}
	if len(result.Board[event.StatusDone]) != 1 {
		t.Errorf("a zero window must not filter anything out; got %+v", result.Board[event.StatusDone])
	}
}

func TestScopeMoved_marks_tasks_in_a_moved_scope(t *testing.T) {
	d := db.OpenTest(t)

	if _, err := event.Add(d, event.Insert{
		Type: event.TaskNew, Project: "proj", Scope: "infra", Title: "in moved scope", Source: "test",
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := event.Add(d, event.Insert{
		Type: event.TaskNew, Project: "proj", Scope: "ui", Title: "in stable scope", Source: "test",
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	// Scope events carry no ref_id — they attach to (project, scope).
	if _, err := event.Add(d, event.Insert{
		Type: event.ScopeShift, Project: "proj", Scope: "infra", Title: "from A to B", Source: "test",
	}); err != nil {
		t.Fatalf("Add scope shift: %v", err)
	}

	result, err := event.ListByStatus(d, "proj", 0, 0)
	if err != nil {
		t.Fatalf("ListByStatus: %v", err)
	}
	for _, task := range result.Board[event.StatusBacklog] {
		want := task.Scope.String == "infra"
		if task.ScopeMoved != want {
			t.Errorf("task %q ScopeMoved = %v, want %v", task.Title, task.ScopeMoved, want)
		}
	}

	// A scope event in another project must not bleed across.
	if _, err := event.Add(d, event.Insert{
		Type: event.TaskNew, Project: "other", Scope: "infra", Title: "other project", Source: "test",
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	other, err := event.ListByStatus(d, "other", 0, 0)
	if err != nil {
		t.Fatalf("ListByStatus: %v", err)
	}
	for _, task := range other.Board[event.StatusBacklog] {
		if task.ScopeMoved {
			t.Errorf("scope events leaked across projects for %q", task.Title)
		}
	}
}

func TestDeleteProjectEvents(t *testing.T) {
	d := db.OpenTest(t)

	id, err := event.Add(d, event.Insert{
		Type: event.TaskNew, Project: "doomed", Title: "task", Source: "test",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := event.MoveTask(d, id, event.StatusInProgress, "test"); err != nil {
		t.Fatalf("MoveTask: %v", err)
	}
	if _, err := event.Add(d, event.Insert{
		Type: event.Note, Project: "doomed", Title: "note", Source: "test",
	}); err != nil {
		t.Fatalf("Add note: %v", err)
	}
	keep, err := event.Add(d, event.Insert{
		Type: event.TaskNew, Project: "keeper", Title: "survivor", Source: "test",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	n, err := event.DeleteProjectEvents(d, "doomed")
	if err != nil {
		t.Fatalf("DeleteProjectEvents: %v", err)
	}
	if n != 3 { // task.new + task.update + note
		t.Errorf("deleted %d rows, want 3", n)
	}

	var remaining int
	if err := d.QueryRow(`SELECT COUNT(*) FROM events WHERE project = 'doomed'`).Scan(&remaining); err != nil {
		t.Fatalf("count: %v", err)
	}
	if remaining != 0 {
		t.Errorf("%d rows of the deleted project survived", remaining)
	}

	var survivor int
	if err := d.QueryRow(`SELECT COUNT(*) FROM events WHERE id = ?`, keep).Scan(&survivor); err != nil {
		t.Fatalf("count survivor: %v", err)
	}
	if survivor != 1 {
		t.Error("deleting one project took another project's events with it")
	}
}

func TestDeleteProjectEvents_requires_a_name(t *testing.T) {
	d := db.OpenTest(t)
	if _, err := event.DeleteProjectEvents(d, ""); err == nil {
		t.Error("expected an error for an empty project name")
	}
}

func TestCountOpenByProject(t *testing.T) {
	d := db.OpenTest(t)

	for _, p := range []string{"a", "a", "b"} {
		if _, err := event.Add(d, event.Insert{
			Type: event.TaskNew, Project: p, Title: "t", Source: "test",
		}); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	// A note must not inflate any project's count.
	if _, err := event.Add(d, event.Insert{
		Type: event.Note, Project: "b", Title: "n", Source: "test",
	}); err != nil {
		t.Fatalf("Add note: %v", err)
	}

	counts, err := event.CountOpenByProject(d)
	if err != nil {
		t.Fatalf("CountOpenByProject: %v", err)
	}
	if counts["a"] != 2 {
		t.Errorf("counts[a] = %d, want 2", counts["a"])
	}
	if counts["b"] != 1 {
		t.Errorf("counts[b] = %d, want 1", counts["b"])
	}
}
