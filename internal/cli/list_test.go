package cli_test

import (
	"bytes"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
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

// seedFindable files two open tasks in one project and one in another, so the
// lookup tests can check that id and text filters compose with --project.
func seedFindable(t *testing.T) (d *sql.DB, cache, other int64) {
	t.Helper()
	d = db.OpenTest(t)
	cache, _ = event.Add(d, event.Insert{
		Type: event.TaskNew, Project: "proj", Title: "Cache the asset manifest", Source: "test",
	})
	_, _ = event.Add(d, event.Insert{
		Type: event.TaskNew, Project: "proj", Title: "Rotate staging keys", Source: "test",
		Body: "the Webhook secret expires soon",
	})
	other, _ = event.Add(d, event.Insert{
		Type: event.TaskNew, Project: "elsewhere", Title: "Cache warmup job", Source: "test",
	})
	return d, cache, other
}

func runList(t *testing.T, d *sql.DB, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	root := cli.NewTestRootWithOutput(d, &buf, &buf)
	root.SetArgs(append([]string{"list"}, args...))
	err := root.Execute()
	return buf.String(), err
}

func TestListCmd_by_id_shows_only_that_task(t *testing.T) {
	d, cache, _ := seedFindable(t)
	out, err := runList(t, d, "--project", "proj", strconv.FormatInt(cache, 10))
	if err != nil {
		t.Fatalf("list by id: %v", err)
	}
	if !strings.Contains(out, "Cache the asset manifest") {
		t.Errorf("output missing the requested task: %s", out)
	}
	if strings.Contains(out, "Rotate staging keys") {
		t.Errorf("output must hold only the requested task: %s", out)
	}
}

func TestListCmd_missing_id_errors(t *testing.T) {
	d, _, other := seedFindable(t)
	cases := []struct {
		name string
		args []string
	}{
		{"unknown id", []string{"--all", "999999"}},
		// The task exists, but not in the project being listed.
		{"id outside project", []string{"--project", "proj", strconv.FormatInt(other, 10)}},
		{"not a number", []string{"--all", "abc"}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runList(t, d, tt.args...)
			if err == nil {
				t.Fatal("expected an error for an id that is not listed")
			}
		})
	}
}

func TestListCmd_search_matches_title_and_body(t *testing.T) {
	d, _, _ := seedFindable(t)
	cases := []struct {
		name      string
		args      []string
		want      []string
		wantEmpty bool
	}{
		{"title, case-insensitive", []string{"--project", "proj", "--search", "CACHE"}, []string{"Cache the asset manifest"}, false},
		{"body", []string{"--project", "proj", "--search", "webhook"}, []string{"Rotate staging keys"}, false},
		{"across projects", []string{"--all", "--search", "cache"}, []string{"Cache the asset manifest", "Cache warmup job"}, false},
		{"nothing matches", []string{"--all", "--search", "no-such-text"}, nil, true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			out, err := runList(t, d, tt.args...)
			if err != nil {
				t.Fatalf("list --search: %v", err)
			}
			for _, w := range tt.want {
				if !strings.Contains(out, w) {
					t.Errorf("output missing %q: %s", w, out)
				}
			}
			if lines := strings.Count(strings.TrimSpace(out), "\n") + 1; !tt.wantEmpty && lines != len(tt.want) {
				t.Errorf("got %d rows, want %d: %s", lines, len(tt.want), out)
			}
			if tt.wantEmpty && !strings.Contains(out, "no open tasks match") {
				t.Errorf("expected an empty-result message: %s", out)
			}
		})
	}
}
