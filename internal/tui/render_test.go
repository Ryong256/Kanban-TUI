package tui

import (
	"strings"
	"testing"

	"github.com/Ryong256/kanban/internal/event"
)

// makeTaskWithProject creates a test task with a given project name.
func makeTaskWithProject(id int64, title, project string) event.OpenTask {
	return event.OpenTask{ID: id, Title: title, Project: project}
}

// TestViewBoard_badges_on_all_tab verifies that project badges appear in the
// rendered board when the All tab is active (activeProject == "").
func TestViewBoard_badges_on_all_tab(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 30
	m.activeTab = 0 // All tab → badges visible
	m.tabs = []string{"All", "kanban", "laya-", "opencode", "dotfiles", "infra", "homelab", "scripts"}

	m.board = boardWith(map[string][]event.OpenTask{
		event.StatusBacklog: {
			makeTaskWithProject(1, "Fix the parser", "kanban"),
			makeTaskWithProject(2, "Update readme", "laya-recruiting-poc"),
			makeTaskWithProject(3, "Deploy infra", "infra"),
		},
		event.StatusInProgress: {
			makeTaskWithProject(4, "Review PR", "opencode"),
		},
	})
	m.doneTotal = 0

	rendered := m.View()

	// Strip ANSI and look for badge abbreviations.
	// badgeAbbrev("kanban") = "kanban"
	// badgeAbbrev("laya-recruiting-poc") = "laya-"
	// badgeAbbrev("infra") = "infra"
	plainText := stripANSI(rendered)
	for _, badge := range []string{"[kanban]", "[laya-]", "[infra]", "[openc"} {
		if !strings.Contains(plainText, badge) {
			t.Errorf("expected badge %q in All-tab rendered output; got:\n%s", badge, plainText)
		}
	}
}

// TestViewBoard_no_badges_on_project_tab verifies that project badges do NOT
// appear when a specific project tab is selected.
func TestViewBoard_no_badges_on_project_tab(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 30
	m.activeTab = 1 // specific project tab → no badges
	m.tabs = []string{"All", "kanban"}

	m.board = boardWith(map[string][]event.OpenTask{
		event.StatusBacklog: {
			makeTaskWithProject(1, "Fix the parser", "kanban"),
		},
	})

	rendered := m.View()
	plain := stripANSI(rendered)

	// No badge brackets should appear in task rows.
	if strings.Contains(plain, "[kanban]") {
		t.Errorf("expected no project badge on specific project tab, but found [kanban] in:\n%s", plain)
	}
}

// TestViewBoard_done_cap_footer verifies that when doneTotal > len(done tasks),
// a "+N older" footer line appears in the done column.
func TestViewBoard_done_cap_footer(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 30
	m.activeTab = 0

	doneTasks := make([]event.OpenTask, doneLimit)
	for i := range doneTasks {
		doneTasks[i] = makeTask(int64(100 + i))
		doneTasks[i].Title = "done task"
	}

	m.board = boardWith(map[string][]event.OpenTask{
		event.StatusDone: doneTasks,
	})
	m.doneTotal = 109 // simulates 109 real done tasks; board has 15

	rendered := m.View()
	plain := stripANSI(rendered)

	// Must show true total in header.
	if !strings.Contains(plain, "DONE (109)") {
		t.Errorf("expected 'DONE (109)' header in rendered output; plain:\n%s", plain)
	}

	// Must show "+N older" footer.
	if !strings.Contains(plain, "+94 older") {
		t.Errorf("expected '+94 older' footer in rendered output; plain:\n%s", plain)
	}
}

// TestViewBoard_adaptive_empty_column verifies that an empty column does not
// take the same width as non-empty columns (adaptive widths in effect).
func TestViewBoard_adaptive_empty_column(t *testing.T) {
	const totalWidth = 100
	m := newTestModel()
	m.width = totalWidth
	m.height = 30

	// Only backlog has tasks; testing column is empty.
	m.board = boardWith(map[string][]event.OpenTask{
		event.StatusBacklog: {makeTask(1), makeTask(2)},
		event.StatusDone:    {makeTask(3)},
	})
	m.doneTotal = 1

	rendered := m.View()

	// Each line of the board section (up to colHeight) should not exceed totalWidth visible chars.
	for _, line := range strings.Split(rendered, "\n") {
		vl := visibleLen(line)
		if vl > totalWidth {
			t.Errorf("line exceeds totalWidth=%d (visible=%d): %q", totalWidth, vl, line)
		}
	}

	// Testing column header must still appear (discoverability).
	plain := stripANSI(rendered)
	if !strings.Contains(plain, "TESTING") {
		t.Errorf("expected TESTING column header in rendered output (discoverability)")
	}
}

// TestViewBoard_width_fit verifies the board renders within the given width
// with multiple projects (All tab, badges active) and the done cap footer.
func TestViewBoard_width_fit(t *testing.T) {
	const totalWidth = 140
	m := newTestModel()
	m.width = totalWidth
	m.height = 40
	m.activeTab = 0 // All tab
	m.tabs = []string{"All", "p1", "p2", "p3", "p4", "p5", "p6", "p7"}

	projects := []string{"kanban", "laya-recruiting-poc", "opencode", "dotfiles", "infra", "homelab", "scripts"}

	backlog := make([]event.OpenTask, 7)
	for i, p := range projects {
		backlog[i] = makeTaskWithProject(int64(i+1), "some task title that is moderately long", p)
	}

	doneTasks := make([]event.OpenTask, doneLimit)
	for i := range doneTasks {
		doneTasks[i] = makeTaskWithProject(int64(100+i), "old done task", "kanban")
	}

	m.board = boardWith(map[string][]event.OpenTask{
		event.StatusBacklog: backlog,
		event.StatusDone:    doneTasks,
	})
	m.doneTotal = 109

	rendered := m.View()

	// Board lines must fit in width.
	lines := strings.Split(rendered, "\n")
	for i, line := range lines {
		vl := visibleLen(line)
		if vl > totalWidth {
			t.Errorf("line %d exceeds totalWidth=%d (visible=%d): %q", i, totalWidth, vl, line)
		}
	}

	plain := stripANSI(rendered)

	// Done total header.
	if !strings.Contains(plain, "DONE (109)") {
		t.Errorf("expected 'DONE (109)' in rendered output")
	}

	// "+N older" footer.
	if !strings.Contains(plain, "+94 older") {
		t.Errorf("expected '+94 older' footer in rendered output")
	}

	// TESTING column still visible (it is empty in this scenario).
	if !strings.Contains(plain, "TESTING") {
		t.Errorf("expected TESTING column (empty) to be visible")
	}

	t.Logf("Sample rendered board (width=%d):\n%s", totalWidth, plain)
}

// stripANSI removes ANSI escape sequences for plain-text assertion.
func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
