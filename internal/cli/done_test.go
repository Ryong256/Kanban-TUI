package cli_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Ryong256/kanban/internal/cli"
	"github.com/Ryong256/kanban/internal/db"
	"github.com/Ryong256/kanban/internal/event"
)

func TestDoneCmd_missing_evidence_rejected(t *testing.T) {
	d := db.OpenTest(t)
	id, _ := event.Add(d, event.Insert{
		Type: event.TaskNew, Project: "proj", Title: "x", Source: "test",
		Status:   event.StatusInProgress,
		MetaJSON: `{"schema_version":1,"closure_condition":"tests pass","evidence_type":"test-output"}`,
	})

	root := cli.NewTestRoot(d)
	root.SetArgs([]string{"done", fmt.Sprintf("%d", id)})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing evidence")
	}
}

func TestDoneCmd_valid_closes(t *testing.T) {
	d := db.OpenTest(t)
	id, _ := event.Add(d, event.Insert{
		Type: event.TaskNew, Project: "proj", Title: "x", Source: "test",
		Status:   event.StatusInProgress,
		MetaJSON: `{"schema_version":1,"closure_condition":"tests pass","evidence_type":"test-output"}`,
	})

	root := cli.NewTestRoot(d)
	root.SetArgs([]string{"done", fmt.Sprintf("%d", id), "--evidence", "PASS ./..."})
	if err := root.Execute(); err != nil {
		t.Fatalf("done: %v", err)
	}

	var updateCount, doneCount int
	if err := d.QueryRow(`SELECT COUNT(*) FROM events WHERE type = 'task.update' AND ref_id = ?`, id).Scan(&updateCount); err != nil {
		t.Fatalf("count update: %v", err)
	}
	if err := d.QueryRow(`SELECT COUNT(*) FROM events WHERE type = 'task.done' AND ref_id = ?`, id).Scan(&doneCount); err != nil {
		t.Fatalf("count done: %v", err)
	}
	if updateCount != 1 {
		t.Errorf("update events = %d, want 1", updateCount)
	}
	if doneCount != 1 {
		t.Errorf("done events = %d, want 1", doneCount)
	}
}

func TestDoneCmd_invalid_flags_and_idempotent(t *testing.T) {
	d := db.OpenTest(t)
	id, _ := event.Add(d, event.Insert{
		Type: event.TaskNew, Project: "proj", Title: "x", Source: "test",
		Status:   event.StatusInProgress,
		MetaJSON: `{"schema_version":1,"closure_condition":"tests pass","evidence_type":"test-output"}`,
	})

	root := cli.NewTestRoot(d)
	root.SetArgs([]string{"done", fmt.Sprintf("%d", id), "--evidence", "manual check"})
	if err := root.Execute(); err != nil {
		t.Fatalf("done: %v", err)
	}

	// The task keeps a real column; the unproved completion rides as a flag.
	var status string
	if err := d.QueryRow(`SELECT status FROM v_task_latest WHERE id = ?`, id).Scan(&status); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if status != event.StatusInProgress {
		t.Errorf("status = %q, want %q", status, event.StatusInProgress)
	}

	var metaRaw string
	if err := d.QueryRow(`
		SELECT COALESCE(meta_json, '') FROM events
		WHERE type = 'task.update' AND ref_id = ? ORDER BY id DESC LIMIT 1`, id).Scan(&metaRaw); err != nil {
		t.Fatalf("scan meta: %v", err)
	}
	if !strings.Contains(metaRaw, event.FlagCompletionUnverified) {
		t.Errorf("update meta = %q, want the %s flag", metaRaw, event.FlagCompletionUnverified)
	}

	root = cli.NewTestRoot(d)
	root.SetArgs([]string{"done", fmt.Sprintf("%d", id), "--evidence", "manual check"})
	if err := root.Execute(); err != nil {
		t.Fatalf("done repeat: %v", err)
	}

	var n int
	if err := d.QueryRow(`SELECT COUNT(*) FROM events WHERE type = 'task.update' AND ref_id = ?`, id).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 update event, got %d", n)
	}
}

// Tasks created before evidence declarations existed have nothing to validate
// against. Demanding evidence from them would make every task already on the
// board permanently uncloseable.
func TestDoneCmd_legacy_task_without_declaration_still_closes(t *testing.T) {
	d := db.OpenTest(t)
	id, _ := event.Add(d, event.Insert{
		Type: event.TaskNew, Project: "proj", Title: "legacy", Source: "test",
		Status: event.StatusInProgress,
	})

	root := cli.NewTestRoot(d)
	root.SetArgs([]string{"done", fmt.Sprintf("%d", id)})
	if err := root.Execute(); err != nil {
		t.Fatalf("closing a legacy task: %v", err)
	}

	var status string
	if err := d.QueryRow(`SELECT status FROM v_task_latest WHERE id = ?`, id).Scan(&status); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if status != event.StatusDone {
		t.Errorf("status = %q, want done", status)
	}
}

// A declared evidence type is still enforced, so the legacy path cannot be used
// to skip evidence on a task that asked for it.
func TestDoneCmd_declared_task_still_requires_evidence(t *testing.T) {
	d := db.OpenTest(t)
	id, _ := event.Add(d, event.Insert{
		Type: event.TaskNew, Project: "proj", Title: "declared", Source: "test",
		Status:   event.StatusInProgress,
		MetaJSON: `{"schema_version":1,"closure_condition":"tests pass","evidence_type":"test-output"}`,
	})

	root := cli.NewTestRoot(d)
	root.SetArgs([]string{"done", fmt.Sprintf("%d", id)})
	if err := root.Execute(); err == nil {
		t.Fatal("a task that declared an evidence type must not close without evidence")
	}
}

func TestDoneCmd_backlog_moves_first(t *testing.T) {
	d := db.OpenTest(t)
	id, _ := event.Add(d, event.Insert{
		Type: event.TaskNew, Project: "proj", Title: "x", Source: "test",
		Status:   event.StatusBacklog,
		MetaJSON: `{"schema_version":1,"closure_condition":"tests pass","evidence_type":"test-output"}`,
	})

	root := cli.NewTestRoot(d)
	root.SetArgs([]string{"done", fmt.Sprintf("%d", id), "--evidence", "PASS ./..."})
	if err := root.Execute(); err != nil {
		t.Fatalf("done: %v", err)
	}

	var status string
	if err := d.QueryRow(`SELECT status FROM v_task_latest WHERE id = ?`, id).Scan(&status); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if status != event.StatusDone {
		t.Errorf("status = %q, want done", status)
	}
}
