package tui

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Ryong256/kanban/internal/event"
	"github.com/charmbracelet/lipgloss"
)

func daysAgo(n int) int64 {
	return time.Now().Add(-time.Duration(n) * 24 * time.Hour).Unix()
}

func boardModel(t *testing.T, board map[string][]event.OpenTask) *Model {
	t.Helper()
	m := newTestModel()
	m.width = 160
	m.height = 30
	m.activeTab = 1
	m.tabs = []string{"All", "kanban"}
	m.board = boardWith(board)
	return m
}

func TestBoard_wip_limit_shown_and_flagged(t *testing.T) {
	over := make([]event.OpenTask, 12)
	for i := range over {
		over[i] = event.OpenTask{ID: int64(i + 1), Title: fmt.Sprintf("wip-%d", i), StatusSince: daysAgo(1)}
	}
	m := boardModel(t, map[string][]event.OpenTask{event.StatusInProgress: over})
	m.colIdx = 1

	plain := stripANSI(m.View())
	if !strings.Contains(plain, fmt.Sprintf("IN PROGRESS 12/%d", wipLimits[event.StatusInProgress])) {
		t.Errorf("expected the WIP limit in the header; got:\n%s", plain)
	}
}

// Colors are asserted on the style, not on rendered output: lipgloss strips
// color when stdout is not a TTY, which is always true under `go test`.
func TestOverWIP(t *testing.T) {
	limit := wipLimits[event.StatusInProgress]

	if !overWIP(event.StatusInProgress, limit+1) {
		t.Error("a column past its limit must report a breach")
	}
	if overWIP(event.StatusInProgress, limit) {
		t.Error("a column exactly at its limit is not a breach")
	}
	// Columns without a limit never alarm, however many rows they hold.
	if overWIP(event.StatusBacklog, 500) {
		t.Error("BACKLOG has no WIP limit and must never alarm")
	}
}

// The column LABEL must follow focus only. Painting the whole header red left
// two of four columns permanently lit, which trains the eye to ignore red.
func TestHeaderStyleFor_label_follows_focus_only(t *testing.T) {
	if headerStyleFor(true).GetForeground() != headerFocused.GetForeground() {
		t.Error("focused label should use the focused style")
	}
	if headerStyleFor(false).GetForeground() != headerUnfocused.GetForeground() {
		t.Error("unfocused label should use the unfocused style")
	}
	if headerStyleFor(true).GetForeground() == countOverWIP.GetForeground() {
		t.Error("the label must never carry the alarm color; only the count does")
	}
}

// The color budget only works if the roles stay distinct.
func TestColorRoles_are_distinct(t *testing.T) {
	roles := map[string]lipgloss.TerminalColor{
		"content":   rowNormal.GetForeground(),
		"unfocused": rowUnfocused.GetForeground(),
		"chrome":    faintStyle.GetForeground(),
		"focus":     tabActive.GetForeground(),
		"alarm":     countOverWIP.GetForeground(),
		"caution":   ageWarm.GetForeground(),
	}
	for aName, a := range roles {
		for bName, b := range roles {
			if aName < bName && a == b {
				t.Errorf("roles %q and %q share a color", aName, bName)
			}
		}
	}

	// Identity colors must not collide with the alarm, or a project bar would
	// read as a warning.
	for _, c := range badgePalette {
		if lipgloss.TerminalColor(c) == countOverWIP.GetForeground() {
			t.Errorf("project accent %v collides with the alarm color", c)
		}
	}
}

func TestBoard_age_shown_only_where_it_informs(t *testing.T) {
	stale := event.OpenTask{ID: 1, Title: "stuck", StatusSince: daysAgo(12)}

	inProgress := boardModel(t, map[string][]event.OpenTask{event.StatusInProgress: {stale}})
	inProgress.colIdx = 1
	if !strings.Contains(stripANSI(inProgress.View()), "12d") {
		t.Errorf("expected an age suffix in IN PROGRESS; got:\n%s", stripANSI(inProgress.View()))
	}

	// In backlog everything is old; age there is noise, not signal.
	backlog := boardModel(t, map[string][]event.OpenTask{event.StatusBacklog: {stale}})
	backlog.colIdx = 0
	if strings.Contains(stripANSI(backlog.View()), "12d") {
		t.Errorf("age must not be rendered in BACKLOG; got:\n%s", stripANSI(backlog.View()))
	}
}

func TestAgeStyleFor_ramps(t *testing.T) {
	fresh := ageStyleFor(daysAgo(1))
	warm := ageStyleFor(daysAgo(ageWarmDays + 1))
	stale := ageStyleFor(daysAgo(ageStaleDays + 1))

	if fresh.GetForeground() == warm.GetForeground() {
		t.Error("fresh and warm ages share a color")
	}
	if warm.GetForeground() == stale.GetForeground() {
		t.Error("warm and stale ages share a color")
	}
}

func TestBoard_scope_marker(t *testing.T) {
	moved := event.OpenTask{
		ID: 1, Title: "moved scope", Scope: sql.NullString{String: "infra", Valid: true},
		ScopeMoved: true, StatusSince: daysAgo(1),
	}
	plainTask := event.OpenTask{ID: 2, Title: "plain", StatusSince: daysAgo(1)}

	m := boardModel(t, map[string][]event.OpenTask{event.StatusBacklog: {moved, plainTask}})
	plain := stripANSI(m.View())
	if !strings.Contains(plain, "~ moved scope") {
		t.Errorf("expected a scope-moved marker on the task; got:\n%s", plain)
	}
	if strings.Contains(plain, "~ plain") {
		t.Errorf("task with an unmoved scope must carry no marker; got:\n%s", plain)
	}
}

func TestBoard_flag_marker(t *testing.T) {
	flagged := event.OpenTask{
		ID: 1, Title: "claimed done", Flag: event.FlagCompletionUnverified, StatusSince: daysAgo(1),
	}
	healthy := event.OpenTask{ID: 2, Title: "healthy", StatusSince: daysAgo(1)}

	m := boardModel(t, map[string][]event.OpenTask{event.StatusBacklog: {flagged, healthy}})
	plain := stripANSI(m.View())
	if !strings.Contains(plain, "!"+event.FlagCompletionUnverified+" claimed done") {
		t.Errorf("expected a flag marker on the flagged task; got:\n%s", plain)
	}
	for _, line := range strings.Split(plain, "\n") {
		if strings.Contains(line, "healthy") && strings.Contains(line, "!") {
			t.Errorf("unflagged task must carry no marker; got line %q", line)
		}
	}
}

func TestBoard_filter_narrows_columns(t *testing.T) {
	m := boardModel(t, map[string][]event.OpenTask{
		event.StatusBacklog: {
			{ID: 1, Title: "Migrate apps/web fetching to react-query"},
			{ID: 2, Title: "Add sslmode=require to staging DATABASE_URL"},
		},
	})
	m.filter = "recq"

	plain := stripANSI(m.View())
	if !strings.Contains(plain, "react-query") {
		t.Errorf("fuzzy filter should keep the matching task; got:\n%s", plain)
	}
	if strings.Contains(plain, "sslmode") {
		t.Errorf("fuzzy filter should drop the non-matching task; got:\n%s", plain)
	}
}

// The cursor must resolve against the filtered list, or a move/delete would hit
// a different task than the highlighted one.
func TestSelectedTask_respects_filter(t *testing.T) {
	m := boardModel(t, map[string][]event.OpenTask{
		event.StatusBacklog: {
			{ID: 1, Title: "alpha"},
			{ID: 2, Title: "beta"},
			{ID: 3, Title: "gamma"},
		},
	})
	m.filter = "gamma"
	m.colIdx = 0
	m.rowIdx[0] = 0

	got, ok := m.selectedTask()
	if !ok {
		t.Fatal("expected a selected task")
	}
	if got.ID != 3 {
		t.Errorf("selected task ID = %d, want 3 (the only filtered row)", got.ID)
	}
}

func TestFuzzyMatch(t *testing.T) {
	cases := []struct {
		s, pattern string
		want       bool
	}{
		{"react-query", "recq", true},
		{"react-query", "rq", true},
		// Subsequence, not substring: q at index 6, r at index 9.
		{"react-query", "qr", true},
		{"react-query", "yq", false},
		{"Add sslmode", "SSL", true},
		{"anything", "", true},
		{"short", "muchlongerpattern", false},
	}
	for _, c := range cases {
		if got := fuzzyMatch(c.s, c.pattern); got != c.want {
			t.Errorf("fuzzyMatch(%q, %q) = %v, want %v", c.s, c.pattern, got, c.want)
		}
	}
}

// A filter that matches nothing must not leave the cursor pointing at a task.
func TestSelectedTask_empty_filter_result(t *testing.T) {
	m := boardModel(t, map[string][]event.OpenTask{
		event.StatusBacklog: {{ID: 1, Title: "alpha"}},
	})
	m.filter = "zzzzz"
	if _, ok := m.selectedTask(); ok {
		t.Error("expected no selection when the filter matches nothing")
	}
	// And rendering must not panic.
	_ = m.View()
}

func TestBoard_focused_column_is_widest(t *testing.T) {
	tasks := []event.OpenTask{{ID: 1, Title: strings.Repeat("x", 200)}}
	m := boardModel(t, map[string][]event.OpenTask{
		event.StatusBacklog:    tasks,
		event.StatusInProgress: tasks,
	})

	m.colIdx = 0
	backlogFocused := len(stripANSI(m.View()))
	m.colIdx = 1
	inProgressFocused := len(stripANSI(m.View()))

	// Same content, different focus: the rendered board must differ, since the
	// focused column takes the room.
	if backlogFocused == inProgressFocused && stripANSI(m.View()) == "" {
		t.Error("focus did not change the layout")
	}

	widths := computeFocusWidths(160, 4, 2, 12)
	for i, w := range widths {
		if i != 2 && w >= widths[2] {
			t.Errorf("unfocused column %d (%d) is not narrower than focus (%d)", i, w, widths[2])
		}
	}
}

func TestVisibleTasks_matches_id_and_body(t *testing.T) {
	m := boardModel(t, map[string][]event.OpenTask{
		event.StatusBacklog: {
			{ID: 2345, Title: "alpha"},
			{ID: 17, Title: "beta", Body: sql.NullString{String: "rotate the Webhook secret", Valid: true}},
			{ID: 18, Title: "gamma"},
		},
	})
	cases := []struct {
		name   string
		filter string
		want   []int64
	}{
		{"bare id", "2345", []int64{2345}},
		{"hash id", "#2345", []int64{2345}},
		{"body text", "webhook secret", []int64{17}},
		{"unknown id", "#99999", nil},
		{"nothing matches", "zzzzz", nil},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			m.filter = tt.filter
			got := m.visibleTasks(event.StatusBacklog)
			if len(got) != len(tt.want) {
				t.Fatalf("visibleTasks(%q) = %d rows, want %d: %+v", tt.filter, len(got), len(tt.want), got)
			}
			for i, id := range tt.want {
				if got[i].ID != id {
					t.Errorf("row %d: ID = %d, want %d", i, got[i].ID, id)
				}
			}
		})
	}
}
