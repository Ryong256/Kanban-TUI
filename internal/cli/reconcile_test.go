package cli_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"os"
	"testing"

	"github.com/Ryong256/kanban/internal/cli"
	"github.com/Ryong256/kanban/internal/db"
	"github.com/Ryong256/kanban/internal/event"
)

func TestReconcileCmd_json_fields(t *testing.T) {
	d := db.OpenTest(t)
	project := "proj"
	addTask(d, t, event.Insert{
		Type: event.TaskNew, Project: project, Title: "owned", Source: "test",
		SessionID: "ses_1", Status: event.StatusInProgress,
		MetaJSON: `{"schema_version":1,"closure_condition":"c","evidence_type":"test-output"}`,
	})

	var buf bytes.Buffer
	root := cli.NewTestRootWithOutput(d, &buf, &buf)
	root.SetArgs([]string{"reconcile", "--project", project, "--session-id", "ses_1", "--json", "--dry-run"})
	if err := root.Execute(); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, buf.String())
	}
	if raw["schema_version"] != float64(1) {
		t.Errorf("schema_version = %v", raw["schema_version"])
	}
	if raw["project"] != project {
		t.Errorf("project = %v", raw["project"])
	}
	if raw["session_id"] != "ses_1" {
		t.Errorf("session_id = %v", raw["session_id"])
	}
	if raw["dry_run"] != true {
		t.Errorf("dry_run = %v", raw["dry_run"])
	}
}

func TestReconcileCmd_dry_run_read_only(t *testing.T) {
	d := db.OpenTest(t)
	project := "proj"
	addTask(d, t, event.Insert{
		Type: event.TaskNew, Project: project, Title: "x", Source: "test",
		SessionID: "ses", Status: event.StatusInProgress,
		MetaJSON: `{"schema_version":1,"closure_condition":"c","evidence_type":"test-output"}`,
	})

	var before int64
	if err := d.QueryRow(`SELECT COUNT(*) FROM events`).Scan(&before); err != nil {
		t.Fatalf("count: %v", err)
	}

	root := cli.NewTestRootWithOutput(d, &bytes.Buffer{}, &bytes.Buffer{})
	root.SetArgs([]string{"reconcile", "--project", project, "--session-id", "ses", "--json", "--dry-run"})
	if err := root.Execute(); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	var after int64
	if err := d.QueryRow(`SELECT COUNT(*) FROM events`).Scan(&after); err != nil {
		t.Fatalf("count: %v", err)
	}
	if after != before {
		t.Errorf("event count changed: %d -> %d", before, after)
	}
}

func TestReconcileCmd_empty_session_owns_nothing(t *testing.T) {
	d := db.OpenTest(t)
	project := "proj"
	addTask(d, t, event.Insert{
		Type: event.TaskNew, Project: project, Title: "x", Source: "test",
		SessionID: "ses", Status: event.StatusInProgress,
		MetaJSON: `{"schema_version":1,"closure_condition":"c","evidence_type":"test-output"}`,
	})

	var buf bytes.Buffer
	root := cli.NewTestRootWithOutput(d, &buf, &buf)
	root.SetArgs([]string{"reconcile", "--project", project, "--session-id", "", "--json", "--dry-run"})
	if err := root.Execute(); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	owned, _ := raw["session_owned"].([]any)
	if len(owned) != 0 {
		t.Errorf("session_owned = %v, want empty", owned)
	}
}

// Adapters capture stdout and pipe it to jq. cobra's cmd.Print* family writes to
// stderr, so a command that reports through it produces nothing for a caller —
// and a test that passes one buffer as both streams cannot tell the difference.
func TestReconcileCmd_json_goes_to_stdout(t *testing.T) {
	t.Setenv("KB_RECONCILING", "")
	d := db.OpenTest(t)
	addTask(d, t, event.Insert{
		Type: event.TaskNew, Project: "proj", Title: "x", Source: "test",
		SessionID: "ses", Status: event.StatusInProgress,
		MetaJSON: `{"schema_version":1,"closure_condition":"c","evidence_type":"test-output"}`,
	})

	var out, errOut bytes.Buffer
	root := cli.NewTestRootWithOutput(d, &out, &errOut)
	root.SetArgs([]string{"reconcile", "--project", "proj", "--session-id", "ses", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if out.Len() == 0 {
		t.Fatalf("nothing on stdout; stderr got: %q", errOut.String())
	}
	var raw map[string]any
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
		t.Fatalf("stdout is not the schema-v1 document: %v\n%s", err, out.String())
	}
}

func TestListCmd_writes_to_stdout(t *testing.T) {
	d := db.OpenTest(t)
	addTask(d, t, event.Insert{
		Type: event.TaskNew, Project: "proj", Title: "visible", Source: "test",
		Status: event.StatusInProgress,
	})

	var out, errOut bytes.Buffer
	root := cli.NewTestRootWithOutput(d, &out, &errOut)
	root.SetArgs([]string{"list", "--project", "proj"})
	if err := root.Execute(); err != nil {
		t.Fatalf("list: %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("visible")) {
		t.Errorf("task not on stdout; stdout=%q stderr=%q", out.String(), errOut.String())
	}
}

// The recursion guard only works if the core marks its own process, so that a
// kb invoked *by* a reconcile run inherits the marker and short-circuits. An
// adapter marking the process it spawns would suppress the very first run.
func TestReconcileCmd_marks_its_process_for_children(t *testing.T) {
	t.Setenv("KB_RECONCILING", "")
	d := db.OpenTest(t)

	root := cli.NewTestRootWithOutput(d, &bytes.Buffer{}, &bytes.Buffer{})
	root.SetArgs([]string{"reconcile", "--project", "proj", "--session-id", "ses", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if got := os.Getenv("KB_RECONCILING"); got != "1" {
		t.Errorf("KB_RECONCILING = %q after a reconcile run, want \"1\" so nested kb calls short-circuit", got)
	}
}

func TestReconcileCmd_already_reconciling_is_a_no_op(t *testing.T) {
	t.Setenv("KB_RECONCILING", "1")
	d := db.OpenTest(t)
	addTask(d, t, event.Insert{
		Type: event.TaskNew, Project: "proj", Title: "x", Source: "test",
		SessionID: "ses", Status: event.StatusInProgress,
		MetaJSON: `{"schema_version":1,"closure_condition":"c","evidence_type":"test-output"}`,
	})

	var buf bytes.Buffer
	root := cli.NewTestRootWithOutput(d, &buf, &buf)
	root.SetArgs([]string{"reconcile", "--project", "proj", "--session-id", "ses", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, buf.String())
	}
	if raw["no_op"] != true {
		t.Errorf("no_op = %v, want true", raw["no_op"])
	}
	if owned, _ := raw["session_owned"].([]any); len(owned) != 0 {
		t.Errorf("session_owned = %v, want empty on a suppressed run", owned)
	}
}

func addTask(d *sql.DB, t *testing.T, in event.Insert) int64 {
	t.Helper()
	id, err := event.Add(d, in)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	return id
}
