package cli_test

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/Ryong256/kanban/internal/cli"
	"github.com/Ryong256/kanban/internal/db"
	"github.com/Ryong256/kanban/internal/event"
)

func TestListCmd_renders_flag_and_staleness(t *testing.T) {
	d := db.OpenTest(t)
	now := time.Now().Unix()
	project := "proj"

	flagged, _ := event.Add(d, event.Insert{
		Type: event.TaskNew, Project: project, Title: "flagged", Source: "test",
		Status:   event.StatusInProgress,
		MetaJSON: `{"schema_version":1,"closure_condition":"tests pass","evidence_type":"test-output"}`,
	})
	_, _, _ = event.ApplyEvidence(d, flagged, "test-output", "manual check", "manual", fakeValidator)

	stale, _ := event.Add(d, event.Insert{
		Type: event.TaskNew, Project: project, Title: "stale", Source: "test",
		Status:   event.StatusInProgress,
		MetaJSON: `{"schema_version":1,"closure_condition":"c","evidence_type":"test-output"}`,
	})
	if _, err := d.Exec(`UPDATE events SET ts = ? WHERE id = ? OR ref_id = ?`, now-10*24*60*60, stale, stale); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	var buf bytes.Buffer
	root := cli.NewTestRootWithOutput(d, &buf, &buf)
	root.SetArgs([]string{"list", "--project", project})
	if err := root.Execute(); err != nil {
		t.Fatalf("list: %v", err)
	}

	out := buf.String()
	if !bytes.Contains([]byte(out), []byte("completion-unverified")) {
		t.Errorf("output missing flag: %s", out)
	}
	if !bytes.Contains([]byte(out), []byte("stale")) {
		t.Errorf("output missing stale indicator: %s", out)
	}
}

// fakeValidator is a stub: it accepts one fixed passing value so the lifecycle
// tests exercise ApplyEvidence without depending on the real evidence rules.
func fakeValidator(evidenceType, value string) error {
	if evidenceType == "test-output" && value == "PASS ./..." {
		return nil
	}
	if evidenceType == "review-approved" && value != "" {
		return nil
	}
	return fmt.Errorf("invalid evidence")
}
