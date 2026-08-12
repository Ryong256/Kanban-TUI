package cli_test

import (
	"testing"

	"github.com/Ryong256/kanban/internal/cli"
	"github.com/Ryong256/kanban/internal/db"
	"github.com/Ryong256/kanban/internal/event"
)

func TestAddCmd_missing_closure_rejected(t *testing.T) {
	d := db.OpenTest(t)
	root := cli.NewTestRoot(d)
	root.SetArgs([]string{"add", "title"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing closure/evidence")
	}
	var n int
	if err := d.QueryRow(`SELECT COUNT(*) FROM events`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Errorf("expected no events, got %d", n)
	}
}

func TestAddCmd_duplicate_blocked(t *testing.T) {
	d := db.OpenTest(t)
	root := cli.NewTestRoot(d)
	root.SetArgs([]string{"add", "Fix Auth", "--closure", "tests pass", "--evidence", "test-output", "--project", "proj"})
	if err := root.Execute(); err != nil {
		t.Fatalf("first add: %v", err)
	}

	root = cli.NewTestRoot(d)
	root.SetArgs([]string{"add", "fix, auth.", "--closure", "Tests Pass!", "--evidence", "test-output", "--project", "proj"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected duplicate blocked")
	}
	var n int
	if err := d.QueryRow(`SELECT COUNT(*) FROM events`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 event, got %d", n)
	}
}

// Every task created before schema-v1 metadata existed has a NULL meta_json.
// Duplicate detection scans them all, so one legacy row must not break `kb add`
// for the whole database.
func TestAddCmd_tolerates_tasks_without_metadata(t *testing.T) {
	d := db.OpenTest(t)
	if _, err := event.Add(d, event.Insert{
		Type: event.TaskNew, Project: "proj", Title: "legacy task", Source: "test",
	}); err != nil {
		t.Fatalf("seed legacy task: %v", err)
	}

	root := cli.NewTestRoot(d)
	root.SetArgs([]string{"add", "New Task", "--closure", "c", "--evidence", "test-output", "--project", "proj"})
	if err := root.Execute(); err != nil {
		t.Fatalf("add alongside a legacy task: %v", err)
	}
}

// The closure condition is part of a task's identity, so the same title filed
// under a different closure is a different task. That is the design, and it is
// what lets a legacy task (no closure at all) coexist with its restated version.
func TestAddCmd_closure_participates_in_identity(t *testing.T) {
	d := db.OpenTest(t)
	if _, err := event.Add(d, event.Insert{
		Type: event.TaskNew, Project: "proj", Title: "Fix Auth", Source: "test",
	}); err != nil {
		t.Fatalf("seed legacy task: %v", err)
	}

	root := cli.NewTestRoot(d)
	root.SetArgs([]string{"add", "Fix Auth", "--closure", "auth tests pass", "--evidence", "test-output", "--project", "proj"})
	if err := root.Execute(); err != nil {
		t.Fatalf("restating a legacy task with a closure condition: %v", err)
	}
}

// Duplicate detection is scoped to the project: the same title in another
// project is a different task.
func TestAddCmd_duplicates_are_scoped_to_the_project(t *testing.T) {
	d := db.OpenTest(t)
	root := cli.NewTestRoot(d)
	root.SetArgs([]string{"add", "Fix Auth", "--closure", "c", "--evidence", "test-output", "--project", "one"})
	if err := root.Execute(); err != nil {
		t.Fatalf("first add: %v", err)
	}

	root = cli.NewTestRoot(d)
	root.SetArgs([]string{"add", "Fix Auth", "--closure", "c", "--evidence", "test-output", "--project", "two"})
	if err := root.Execute(); err != nil {
		t.Fatalf("same title in another project must be allowed: %v", err)
	}
}

func TestAddCmd_session_and_future_recorded(t *testing.T) {
	d := db.OpenTest(t)
	root := cli.NewTestRoot(d)
	root.SetArgs([]string{"add", "Future", "--closure", "c", "--evidence", "test-output", "--project", "proj", "--session-id", "ses_1", "--future"})
	if err := root.Execute(); err != nil {
		t.Fatalf("add: %v", err)
	}

	var sessionID, status, meta string
	if err := d.QueryRow(`SELECT session_id, status, meta_json FROM events WHERE type = 'task.new'`).Scan(&sessionID, &status, &meta); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if sessionID != "ses_1" {
		t.Errorf("session_id = %q, want ses_1", sessionID)
	}
	if status != event.StatusBacklog {
		t.Errorf("status = %q, want backlog", status)
	}
	if meta == "" {
		t.Error("expected meta_json")
	}
}

func TestAddCmd_valid_active(t *testing.T) {
	d := db.OpenTest(t)
	root := cli.NewTestRoot(d)
	root.SetArgs([]string{"add", "Active", "--closure", "c", "--evidence", "test-output", "--project", "proj"})
	if err := root.Execute(); err != nil {
		t.Fatalf("add: %v", err)
	}
	var status string
	if err := d.QueryRow(`SELECT status FROM events WHERE type = 'task.new'`).Scan(&status); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if status != event.StatusInProgress {
		t.Errorf("status = %q, want in_progress", status)
	}
}
