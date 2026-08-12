package event

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Ryong256/kanban/internal/db"
)

func fakeValidator(evidenceType, value string) error {
	if evidenceType == "test-output" && (value == "PASS ./..." || value == "FAIL ./..." || value == "ok") {
		return nil
	}
	if evidenceType == "review-approved" && strings.HasPrefix(strings.ToLower(value), "approved-by:") {
		return nil
	}
	return fmt.Errorf("invalid evidence")
}

func TestListBySession_returns_owned_tasks(t *testing.T) {
	d := db.OpenTest(t)
	sessionID := "ses_abc"
	project := "proj"
	owned, _ := Add(d, Insert{
		Type: TaskNew, Project: project, Title: "owned", Source: "test",
		SessionID: sessionID, Status: StatusInProgress,
		MetaJSON: `{"schema_version":1,"closure_condition":"c","evidence_type":"test-output"}`,
	})
	_, _ = Add(d, Insert{
		Type: TaskNew, Project: project, Title: "other", Source: "test",
		Status:   StatusInProgress,
		MetaJSON: `{"schema_version":1,"closure_condition":"c","evidence_type":"test-output"}`,
	})

	tasks, err := ListBySession(d, project, sessionID)
	if err != nil {
		t.Fatalf("ListBySession: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].ID != owned {
		t.Errorf("got task %d, want %d", tasks[0].ID, owned)
	}
}

func TestListBySession_empty_session_owns_nothing(t *testing.T) {
	d := db.OpenTest(t)
	project := "proj"
	_, _ = Add(d, Insert{
		Type: TaskNew, Project: project, Title: "unowned", Source: "test",
		Status:   StatusInProgress,
		MetaJSON: `{"schema_version":1,"closure_condition":"c","evidence_type":"test-output"}`,
	})

	tasks, err := ListBySession(d, project, "")
	if err != nil {
		t.Fatalf("ListBySession: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks for empty session, got %+v", tasks)
	}
}

func TestListStale_bounded_and_ordered(t *testing.T) {
	d := db.OpenTest(t)
	now := time.Now().Unix()
	project := "proj"

	stale, _ := Add(d, Insert{
		Type: TaskNew, Project: project, Title: "stale", Source: "test",
		Status: StatusInProgress,
	})
	_, _ = Add(d, Insert{
		Type: TaskNew, Project: project, Title: "recent", Source: "test",
		Status: StatusInProgress,
	})

	old := now - 10*24*60*60
	if _, err := d.Exec(`UPDATE events SET ts = ? WHERE id = ? OR ref_id = ?`, old, stale, stale); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	tasks, err := ListStale(d, project, now, 7*24*60*60, 50)
	if err != nil {
		t.Fatalf("ListStale: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != stale {
		t.Errorf("got %+v, want stale task", tasks)
	}
}

func TestListStale_respects_stale_after(t *testing.T) {
	d := db.OpenTest(t)
	now := time.Now().Unix()
	project := "proj"

	twoDaysAgo := now - 2*24*60*60
	id, _ := Add(d, Insert{
		Type: TaskNew, Project: project, Title: "two-day-old", Source: "test",
		Status: StatusInProgress,
	})
	if _, err := d.Exec(`UPDATE events SET ts = ? WHERE id = ? OR ref_id = ?`, twoDaysAgo, id, id); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	tasks, err := ListStale(d, project, now, 24*60*60, 50)
	if err != nil {
		t.Fatalf("ListStale: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != id {
		t.Errorf("expected 2-day-old task with stale-after=1d, got %+v", tasks)
	}
}

func TestReadTaskMeta_reads_metadata(t *testing.T) {
	d := db.OpenTest(t)
	id, _ := Add(d, Insert{
		Type: TaskNew, Project: "proj", Title: "x", Source: "test",
		MetaJSON: `{"schema_version":1,"closure_condition":"tests pass","evidence_type":"test-output"}`,
	})
	meta, err := ReadTaskMeta(d, id)
	if err != nil {
		t.Fatalf("ReadTaskMeta: %v", err)
	}
	if meta.SchemaVersion != 1 {
		t.Errorf("schema_version = %d, want 1", meta.SchemaVersion)
	}
	if meta.ClosureCondition != "tests pass" {
		t.Errorf("closure_condition = %q, want tests pass", meta.ClosureCondition)
	}
	if meta.EvidenceType != "test-output" {
		t.Errorf("evidence_type = %q, want test-output", meta.EvidenceType)
	}
}

func TestApplyEvidence_valid_closes_task(t *testing.T) {
	d := db.OpenTest(t)
	id, _ := Add(d, Insert{
		Type: TaskNew, Project: "proj", Title: "x", Source: "test",
		Status:   StatusInProgress,
		MetaJSON: `{"schema_version":1,"closure_condition":"tests pass","evidence_type":"test-output"}`,
	})

	updateID, doneID, err := ApplyEvidence(d, id, "test-output", "PASS ./...", "manual", fakeValidator)
	if err != nil {
		t.Fatalf("ApplyEvidence: %v", err)
	}
	if updateID == 0 {
		t.Error("expected task.update event")
	}
	if doneID == 0 {
		t.Error("expected task.done event")
	}

	var status string
	if err := d.QueryRow(`SELECT status FROM v_task_latest WHERE id = ?`, id).Scan(&status); err != nil {
		t.Fatalf("scan status: %v", err)
	}
	if status != StatusDone {
		t.Errorf("status = %q, want done", status)
	}

	var doneMeta string
	if err := d.QueryRow(`SELECT COALESCE(meta_json, '') FROM events WHERE id = ?`, doneID).Scan(&doneMeta); err != nil {
		t.Fatalf("scan done meta: %v", err)
	}
	if !strings.Contains(doneMeta, `"value":"PASS ./..."`) {
		t.Errorf("task.done meta_json = %q, want evidence payload", doneMeta)
	}
}

func TestApplyEvidence_valid_idempotent_after_done(t *testing.T) {
	d := db.OpenTest(t)
	id, _ := Add(d, Insert{
		Type: TaskNew, Project: "proj", Title: "x", Source: "test",
		Status:   StatusInProgress,
		MetaJSON: `{"schema_version":1,"closure_condition":"tests pass","evidence_type":"test-output"}`,
	})

	// First valid closure must append task.update + task.done.
	updateID1, doneID1, err := ApplyEvidence(d, id, "test-output", "PASS ./...", "manual", fakeValidator)
	if err != nil {
		t.Fatalf("ApplyEvidence first: %v", err)
	}
	if updateID1 == 0 {
		t.Error("first valid closure expected task.update event")
	}
	if doneID1 == 0 {
		t.Error("first valid closure expected task.done event")
	}

	// Re-applying identical valid evidence to the already-done task must no-op.
	updateID2, doneID2, err := ApplyEvidence(d, id, "test-output", "PASS ./...", "manual", fakeValidator)
	if err != nil {
		t.Fatalf("ApplyEvidence second: %v", err)
	}
	if updateID2 != 0 {
		t.Errorf("repeat valid evidence on done task: expected no update, got %d", updateID2)
	}
	if doneID2 != 0 {
		t.Errorf("repeat valid evidence on done task: expected no done, got %d", doneID2)
	}

	// Exactly two lifecycle events should follow the original task.new.
	var count int
	if err := d.QueryRow(`SELECT COUNT(*) FROM events WHERE ref_id = ?`, id).Scan(&count); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 lifecycle events (update+done), got %d", count)
	}
}

func TestApplyEvidence_invalid_flags_and_idempotent(t *testing.T) {
	d := db.OpenTest(t)
	id, _ := Add(d, Insert{
		Type: TaskNew, Project: "proj", Title: "x", Source: "test",
		Status:   StatusInProgress,
		MetaJSON: `{"schema_version":1,"closure_condition":"tests pass","evidence_type":"test-output"}`,
	})

	updateID1, doneID1, err := ApplyEvidence(d, id, "test-output", "manual check", "manual", fakeValidator)
	if err != nil {
		t.Fatalf("ApplyEvidence: %v", err)
	}
	if doneID1 != 0 {
		t.Error("invalid evidence must not produce task.done")
	}

	// The task stays open in its real lifecycle column. completion-unverified is
	// a flag, not a status: writing it to the status column would put the task
	// outside AllStatuses() and drop it from the board entirely.
	var status string
	if err := d.QueryRow(`SELECT status FROM v_task_latest WHERE id = ?`, id).Scan(&status); err != nil {
		t.Fatalf("scan status: %v", err)
	}
	if status != StatusInProgress {
		t.Errorf("status = %q, want %q", status, StatusInProgress)
	}
	if !ValidStatus(status) {
		t.Errorf("status %q is outside the status enum", status)
	}

	// Repeat must be a no-op.
	updateID2, doneID2, err := ApplyEvidence(d, id, "test-output", "manual check", "manual", fakeValidator)
	if err != nil {
		t.Fatalf("ApplyEvidence second: %v", err)
	}
	if updateID2 != 0 {
		t.Error("repeat invalid attempt must produce no new event")
	}
	if doneID2 != 0 {
		t.Error("repeat invalid attempt must not produce task.done")
	}

	// The flag rides in meta_json on the update event.
	var metaRaw string
	if err := d.QueryRow(`SELECT COALESCE(meta_json, '') FROM events WHERE id = ?`, updateID1).Scan(&metaRaw); err != nil {
		t.Fatalf("scan update meta: %v", err)
	}
	if flag := flagFrom(metaRaw); flag != FlagCompletionUnverified {
		t.Errorf("update flag = %q, want %q", flag, FlagCompletionUnverified)
	}

	// The task must still be reachable on the board.
	board, err := ListByStatus(d, "proj", 10, 0)
	if err != nil {
		t.Fatalf("ListByStatus: %v", err)
	}
	found := false
	for _, task := range board.Board[StatusInProgress] {
		if task.ID == id {
			found = true
			if task.Flag != FlagCompletionUnverified {
				t.Errorf("board task flag = %q, want %q", task.Flag, FlagCompletionUnverified)
			}
		}
	}
	if !found {
		t.Errorf("task #%d vanished from the board after an unverified completion", id)
	}
}

func TestApplyEvidence_valid_attempt_clears_the_flag(t *testing.T) {
	d := db.OpenTest(t)
	id, _ := Add(d, Insert{
		Type: TaskNew, Project: "proj", Title: "x", Source: "test",
		Status:   StatusInProgress,
		MetaJSON: `{"schema_version":1,"closure_condition":"tests pass","evidence_type":"test-output"}`,
	})

	if _, _, err := ApplyEvidence(d, id, "test-output", "manual check", "manual", fakeValidator); err != nil {
		t.Fatalf("ApplyEvidence invalid: %v", err)
	}
	if _, doneID, err := ApplyEvidence(d, id, "test-output", "PASS ./...", "manual", fakeValidator); err != nil {
		t.Fatalf("ApplyEvidence valid: %v", err)
	} else if doneID == 0 {
		t.Fatal("valid evidence after a failed attempt must close the task")
	}

	tasks, err := ListStale(d, "proj", 0, 1, 10)
	if err != nil {
		t.Fatalf("ListStale: %v", err)
	}
	for _, task := range tasks {
		if task.ID == id {
			t.Errorf("closed task #%d still listed as open", id)
		}
	}
}
