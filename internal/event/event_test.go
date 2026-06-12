package event_test

import (
	"testing"
	"time"

	"github.com/Ryong256/kanban/internal/db"
	"github.com/Ryong256/kanban/internal/event"
)

func TestListByStatus_empty(t *testing.T) {
	d := db.OpenTest(t)
	result, err := event.ListByStatus(d, "proj", 0)
	if err != nil {
		t.Fatalf("ListByStatus: %v", err)
	}
	for _, s := range event.AllStatuses() {
		if tasks := result.Board[s]; tasks == nil {
			t.Errorf("expected empty slice for status %q, got nil", s)
		}
	}
}

func TestListByStatus_groups_correctly(t *testing.T) {
	d := db.OpenTest(t)

	id1, err := event.Add(d, event.Insert{
		Type:    event.TaskNew,
		Project: "proj",
		Title:   "task one",
		Source:  "test",
		Status:  event.StatusBacklog,
	})
	if err != nil {
		t.Fatalf("Add task1: %v", err)
	}

	id2, err := event.Add(d, event.Insert{
		Type:    event.TaskNew,
		Project: "proj",
		Title:   "task two",
		Source:  "test",
		Status:  event.StatusBacklog,
	})
	if err != nil {
		t.Fatalf("Add task2: %v", err)
	}

	// Move task2 to in_progress
	if _, err := event.MoveTask(d, id2, event.StatusInProgress, "test"); err != nil {
		t.Fatalf("MoveTask: %v", err)
	}

	result, err := event.ListByStatus(d, "proj", 0)
	if err != nil {
		t.Fatalf("ListByStatus: %v", err)
	}

	if len(result.Board[event.StatusBacklog]) != 1 {
		t.Fatalf("expected 1 backlog task, got %d", len(result.Board[event.StatusBacklog]))
	}
	if result.Board[event.StatusBacklog][0].ID != id1 {
		t.Fatalf("expected task %d in backlog, got %d", id1, result.Board[event.StatusBacklog][0].ID)
	}
	if len(result.Board[event.StatusInProgress]) != 1 {
		t.Fatalf("expected 1 in_progress task, got %d", len(result.Board[event.StatusInProgress]))
	}
	if result.Board[event.StatusInProgress][0].ID != id2 {
		t.Fatalf("expected task %d in in_progress, got %d", id2, result.Board[event.StatusInProgress][0].ID)
	}
}

func TestMoveTask_invalid_status(t *testing.T) {
	d := db.OpenTest(t)

	id, err := event.Add(d, event.Insert{
		Type:    event.TaskNew,
		Project: "proj",
		Title:   "task",
		Source:  "test",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	if _, err := event.MoveTask(d, id, "not_a_status", "test"); err == nil {
		t.Fatal("expected error for invalid status, got nil")
	}
}

func TestMoveTask_not_found(t *testing.T) {
	d := db.OpenTest(t)
	if _, err := event.MoveTask(d, 9999, event.StatusDone, "test"); err == nil {
		t.Fatal("expected error for nonexistent task, got nil")
	}
}

func TestMoveTask_to_done_inserts_task_done_event(t *testing.T) {
	d := db.OpenTest(t)

	id, err := event.Add(d, event.Insert{
		Type:    event.TaskNew,
		Project: "proj",
		Title:   "finish me",
		Source:  "test",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	if _, err := event.MoveTask(d, id, event.StatusDone, "test"); err != nil {
		t.Fatalf("MoveTask: %v", err)
	}

	// task.done event must exist for this task
	var count int
	err = d.QueryRow(
		`SELECT COUNT(*) FROM events WHERE type='task.done' AND ref_id=?`, id,
	).Scan(&count)
	if err != nil {
		t.Fatalf("query task.done: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 task.done event, got %d", count)
	}

	// v_task_latest must show status=done
	result, err := event.ListByStatus(d, "proj", 0)
	if err != nil {
		t.Fatalf("ListByStatus: %v", err)
	}
	if len(result.Board[event.StatusDone]) != 1 {
		t.Fatalf("expected 1 done task, got %d", len(result.Board[event.StatusDone]))
	}
}

// TestDoneStatusMigration_phantom_task simulates the pre-migration scenario:
// a task was closed via task.done BEFORE 0003_status.sql, so it has no
// task.update with status='done'. After 0004_done_status.sql the view must
// report it as 'done', not 'backlog'.
func TestDoneStatusMigration_phantom_task(t *testing.T) {
	d := db.OpenTest(t)

	// Insert a task.new with backlog status (as if pre-0003)
	res, err := d.Exec(
		`INSERT INTO events (ts, type, project, title, source, status)
         VALUES (?, 'task.new', 'proj', 'phantom task', 'test', 'backlog')`,
		time.Now().Unix()-100,
	)
	if err != nil {
		t.Fatalf("insert task.new: %v", err)
	}
	taskID, _ := res.LastInsertId()

	// Insert a task.done event (no task.update status row)
	_, err = d.Exec(
		`INSERT INTO events (ts, type, project, title, ref_id, source)
         VALUES (?, 'task.done', 'proj', 'phantom task', ?, 'test')`,
		time.Now().Unix()-50,
		taskID,
	)
	if err != nil {
		t.Fatalf("insert task.done: %v", err)
	}

	result, err := event.ListByStatus(d, "proj", 0)
	if err != nil {
		t.Fatalf("ListByStatus: %v", err)
	}

	// Must be in done, not backlog
	for _, task := range result.Board[event.StatusBacklog] {
		if task.ID == taskID {
			t.Fatalf("phantom task %d still in backlog after migration fix", taskID)
		}
	}
	found := false
	for _, task := range result.Board[event.StatusDone] {
		if task.ID == taskID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected phantom task %d in done column, not found there", taskID)
	}
}

func TestMarkDone(t *testing.T) {
	d := db.OpenTest(t)

	id, err := event.Add(d, event.Insert{
		Type:    event.TaskNew,
		Project: "proj",
		Title:   "mark me done",
		Source:  "test",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	if _, err := event.MarkDone(d, id); err != nil {
		t.Fatalf("MarkDone: %v", err)
	}

	result, _ := event.ListByStatus(d, "proj", 0)
	if len(result.Board[event.StatusDone]) != 1 {
		t.Fatalf("expected task in done after MarkDone, got %d", len(result.Board[event.StatusDone]))
	}
}

// TestListByStatus_done_cap verifies the doneLimit is applied correctly and
// DoneTotal reflects the true total while the board slice is capped.
func TestListByStatus_done_cap(t *testing.T) {
	d := db.OpenTest(t)

	const total = 20
	const limit = 5
	for i := 0; i < total; i++ {
		id, err := event.Add(d, event.Insert{
			Type:    event.TaskNew,
			Project: "proj",
			Title:   "done task",
			Source:  "test",
		})
		if err != nil {
			t.Fatalf("Add: %v", err)
		}
		if _, err := event.MoveTask(d, id, event.StatusDone, "test"); err != nil {
			t.Fatalf("MoveTask: %v", err)
		}
	}

	result, err := event.ListByStatus(d, "proj", limit)
	if err != nil {
		t.Fatalf("ListByStatus: %v", err)
	}
	if result.DoneTotal != total {
		t.Fatalf("expected DoneTotal=%d, got %d", total, result.DoneTotal)
	}
	if len(result.Board[event.StatusDone]) != limit {
		t.Fatalf("expected %d done tasks (capped), got %d", limit, len(result.Board[event.StatusDone]))
	}
}
