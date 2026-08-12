package reconcile

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Ryong256/kanban/internal/db"
	"github.com/Ryong256/kanban/internal/event"
)

func addTask(t *testing.T, d *sql.DB, in event.Insert) int64 {
	t.Helper()
	id, err := event.Add(d, in)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	return id
}

func TestReconcile_session_first_then_stale(t *testing.T) {
	d := db.OpenTest(t)
	now := time.Now().Unix()
	old := now - 10*24*60*60

	sessionID := "ses_abc"
	project := "proj"

	owned := addTask(t, d, event.Insert{
		Type: event.TaskNew, Project: project, Title: "owned", Source: "test",
		SessionID: sessionID, Status: event.StatusInProgress,
		MetaJSON: `{"schema_version":1,"closure_condition":"tests pass","evidence_type":"test-output"}`,
	})
	stale := addTask(t, d, event.Insert{
		Type: event.TaskNew, Project: project, Title: "stale", Source: "test",
		Status:   event.StatusInProgress,
		MetaJSON: `{"schema_version":1,"closure_condition":"review","evidence_type":"review-approved"}`,
	})
	recent := addTask(t, d, event.Insert{
		Type: event.TaskNew, Project: project, Title: "recent", Source: "test",
		Status:   event.StatusInProgress,
		MetaJSON: `{"schema_version":1,"closure_condition":"x","evidence_type":"test-output"}`,
	})

	if _, err := d.Exec(`UPDATE events SET ts = ? WHERE id = ? OR ref_id = ?`, old, stale, stale); err != nil {
		t.Fatalf("backdate stale: %v", err)
	}

	svc := NewService(d)
	res, err := svc.Reconcile(Config{Project: project, SessionID: sessionID, Now: now})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if len(res.SessionOwned) != 1 || res.SessionOwned[0].ID != owned {
		t.Errorf("session_owned = %+v, want task %d", res.SessionOwned, owned)
	}
	if len(res.StaleReview) != 1 || res.StaleReview[0].ID != stale {
		t.Errorf("stale_review = %+v, want task %d", res.StaleReview, stale)
	}
	for _, task := range res.StaleReview {
		if task.ID == recent {
			t.Error("recent task appeared in stale_review")
		}
	}
}

func TestReconcile_stale_bounded_and_sorted(t *testing.T) {
	d := db.OpenTest(t)
	now := time.Now().Unix()

	project := "proj"
	for i := 0; i < 55; i++ {
		id := addTask(t, d, event.Insert{
			Type: event.TaskNew, Project: project, Title: fmt.Sprintf("task %d", i), Source: "test",
			Status:   event.StatusInProgress,
			MetaJSON: `{"schema_version":1,"closure_condition":"c","evidence_type":"test-output"}`,
		})
		if _, err := d.Exec(`UPDATE events SET ts = ? WHERE id = ?`, now-int64(i+10)*24*60*60, id); err != nil {
			t.Fatalf("backdate: %v", err)
		}
	}

	svc := NewService(d)
	res, err := svc.Reconcile(Config{Project: project, Now: now})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(res.StaleReview) > 50 {
		t.Errorf("stale_review len = %d, want <= 50", len(res.StaleReview))
	}
	for i := 1; i < len(res.StaleReview); i++ {
		a, b := res.StaleReview[i-1], res.StaleReview[i]
		if a.LastTS > b.LastTS || (a.LastTS == b.LastTS && a.ID > b.ID) {
			t.Errorf("stale_review not sorted at %d: %+v then %+v", i, a, b)
		}
	}
}

func TestReconcile_dry_run_no_appends(t *testing.T) {
	d := db.OpenTest(t)
	now := time.Now().Unix()
	project := "proj"
	addTask(t, d, event.Insert{
		Type: event.TaskNew, Project: project, Title: "x", Source: "test",
		SessionID: "ses", Status: event.StatusInProgress,
		MetaJSON: `{"schema_version":1,"closure_condition":"c","evidence_type":"test-output"}`,
	})

	var before int64
	if err := d.QueryRow(`SELECT COUNT(*) FROM events`).Scan(&before); err != nil {
		t.Fatalf("count before: %v", err)
	}

	svc := NewService(d)
	if _, err := svc.Reconcile(Config{Project: project, SessionID: "ses", DryRun: true, Now: now}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	var after int64
	if err := d.QueryRow(`SELECT COUNT(*) FROM events`).Scan(&after); err != nil {
		t.Fatalf("count after: %v", err)
	}
	if after != before {
		t.Errorf("event count changed: %d -> %d", before, after)
	}
}

func TestReconcile_idempotent(t *testing.T) {
	d := db.OpenTest(t)
	now := time.Now().Unix()
	project := "proj"
	addTask(t, d, event.Insert{
		Type: event.TaskNew, Project: project, Title: "x", Source: "test",
		SessionID: "ses", Status: event.StatusInProgress,
		MetaJSON: `{"schema_version":1,"closure_condition":"c","evidence_type":"test-output"}`,
	})

	svc := NewService(d)
	cfg := Config{Project: project, SessionID: "ses", Now: now}
	r1, err := svc.Reconcile(cfg)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	r2, err := svc.Reconcile(cfg)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if !snapshotsEqual(r1.SessionOwned, r2.SessionOwned) {
		t.Error("session_owned differ between runs")
	}
	if !snapshotsEqual(r1.StaleReview, r2.StaleReview) {
		t.Error("stale_review differ between runs")
	}
}

func TestNoOpResult_is_a_complete_schema_v1_answer(t *testing.T) {
	res := NoOpResult(Config{Project: "proj", SessionID: "ses", DryRun: true})
	if !res.NoOp {
		t.Error("NoOpResult must set no_op")
	}
	if res.SchemaVersion != 1 || res.Project != "proj" || res.SessionID != "ses" || !res.DryRun {
		t.Errorf("NoOpResult dropped caller context: %+v", res)
	}
	if res.SessionOwned == nil || res.StaleReview == nil || res.Errors == nil {
		t.Error("NoOpResult must emit empty arrays, not null")
	}
}

// Suppression is a process-environment decision and belongs to the caller. The
// service must select the same tasks no matter what the environment says.
func TestReconcile_ignores_process_environment(t *testing.T) {
	d := db.OpenTest(t)
	t.Setenv("KB_RECONCILING", "1")
	addTask(t, d, event.Insert{
		Type: event.TaskNew, Project: "proj", Title: "x", Source: "test",
		SessionID: "ses", Status: event.StatusInProgress,
		MetaJSON: `{"schema_version":1,"closure_condition":"c","evidence_type":"test-output"}`,
	})

	res, err := NewService(d).Reconcile(Config{Project: "proj", SessionID: "ses", Now: time.Now().Unix()})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if res.NoOp {
		t.Error("the service must not read the environment; suppression is the caller's call")
	}
	if len(res.SessionOwned) != 1 {
		t.Errorf("session_owned = %d, want 1", len(res.SessionOwned))
	}
}

func TestReconcile_notes_excluded(t *testing.T) {
	d := db.OpenTest(t)
	now := time.Now().Unix()
	project := "proj"
	addTask(t, d, event.Insert{
		Type: event.Note, Project: project, Title: "note", Source: "test",
	})
	addTask(t, d, event.Insert{
		Type: event.TaskNew, Project: project, Title: "task", Source: "test",
		SessionID: "ses", Status: event.StatusInProgress,
		MetaJSON: `{"schema_version":1,"closure_condition":"c","evidence_type":"test-output"}`,
	})

	svc := NewService(d)
	res, err := svc.Reconcile(Config{Project: project, SessionID: "ses", Now: now})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(res.SessionOwned) != 1 || res.SessionOwned[0].Title != "task" {
		t.Errorf("session_owned = %+v, want only task", res.SessionOwned)
	}
}

func TestReconcile_schema_v1_snapshot(t *testing.T) {
	d := db.OpenTest(t)
	now := time.Now().Unix()
	project := "proj"
	addTask(t, d, event.Insert{
		Type: event.TaskNew, Project: project, Title: "x", Source: "test",
		SessionID: "ses", Status: event.StatusInProgress,
		MetaJSON: `{"schema_version":1,"closure_condition":"c","evidence_type":"test-output"}`,
	})

	svc := NewService(d)
	res, err := svc.Reconcile(Config{Project: project, SessionID: "ses", DryRun: true, Now: now})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	res.SessionOwned = nil
	res.StaleReview = nil
	res.Errors = nil

	b, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if raw["schema_version"] != float64(1) {
		t.Errorf("schema_version = %v, want 1", raw["schema_version"])
	}
	if raw["project"] != project {
		t.Errorf("project = %v, want %q", raw["project"], project)
	}
	if raw["session_id"] != "ses" {
		t.Errorf("session_id = %v, want ses", raw["session_id"])
	}
	if raw["dry_run"] != true {
		t.Errorf("dry_run = %v, want true", raw["dry_run"])
	}
}

func snapshotsEqual(a, b []TaskSnapshot) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID {
			return false
		}
	}
	return true
}
