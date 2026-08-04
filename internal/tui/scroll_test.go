package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Ryong256/kanban/internal/event"
)

// A cursor past the viewport used to render nothing: the selected task was
// invisible while still being the target of moves and deletes, and rows below
// the fold were unreachable.
func TestViewBoard_scrolls_to_keep_cursor_visible(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 20
	m.activeTab = 1
	m.tabs = []string{"All", "kanban"}

	tasks := make([]event.OpenTask, 60)
	for i := range tasks {
		tasks[i] = event.OpenTask{ID: int64(i + 1), Title: fmt.Sprintf("task-%02d", i)}
	}
	m.board = boardWith(map[string][]event.OpenTask{event.StatusBacklog: tasks})

	m.colIdx = 0
	m.rowIdx[0] = 55

	plain := stripANSI(m.View())
	if !strings.Contains(plain, "task-55") {
		t.Errorf("selected row task-55 is not rendered; got:\n%s", plain)
	}
	// And the viewport really moved rather than growing.
	if strings.Contains(plain, "task-00") {
		t.Errorf("expected the top of the list to scroll out of view; got:\n%s", plain)
	}
	if !strings.Contains(plain, "↑") {
		t.Errorf("expected a hidden-rows-above indicator; got:\n%s", plain)
	}
}

func TestViewBoard_scroll_reports_hidden_below(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 20
	m.activeTab = 1
	m.tabs = []string{"All", "kanban"}

	tasks := make([]event.OpenTask, 60)
	for i := range tasks {
		tasks[i] = event.OpenTask{ID: int64(i + 1), Title: fmt.Sprintf("task-%02d", i)}
	}
	m.board = boardWith(map[string][]event.OpenTask{event.StatusBacklog: tasks})

	// Hidden-row counts live in the column header, not in a footer row that
	// would cost a task slot to say so.
	plain := stripANSI(m.View())
	header := strings.SplitN(plain, "\n", 2)[0]
	if !strings.Contains(header, "↓") {
		t.Errorf("expected a '↓N' hidden-below mark in the column header; got header:\n%s", header)
	}
}

// The board never exceeds the terminal height, scrolled or not.
func TestViewBoard_height_fit_with_scroll(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 24
	m.activeTab = 1
	m.tabs = []string{"All", "kanban"}

	tasks := make([]event.OpenTask, 80)
	for i := range tasks {
		tasks[i] = event.OpenTask{ID: int64(i + 1), Title: fmt.Sprintf("task-%02d", i)}
	}
	m.board = boardWith(map[string][]event.OpenTask{event.StatusBacklog: tasks})
	m.rowIdx[0] = 79

	lines := strings.Split(strings.TrimRight(m.View(), "\n"), "\n")
	if len(lines) > m.height {
		t.Errorf("rendered %d lines, terminal height is %d", len(lines), m.height)
	}
}

func TestViewBoard_preview_shows_untruncated_title(t *testing.T) {
	m := newTestModel()
	m.width = 160
	m.height = 20
	m.activeTab = 1
	m.tabs = []string{"All", "kanban"}

	long := "next dev OOM'd compacting an 8GB Turbopack SST cache accumulated over two months"
	m.board = boardWith(map[string][]event.OpenTask{
		event.StatusBacklog: {{ID: 7, Title: long}},
	})

	plain := stripANSI(m.View())
	if !strings.Contains(plain, long) {
		t.Errorf("expected the full title in the preview line; got:\n%s", plain)
	}
}

func TestRenderTabs_fits_width_and_keeps_active_visible(t *testing.T) {
	m := newTestModel()
	m.width = 60
	m.height = 20
	m.tabs = []string{
		"All", "Cesar-s-junk-removal", "HDP115-Grupo4", "LAYA-IA-SDK",
		"SCC-v2", "SellConvertClean", "engram-memory", "infra", "kanban",
	}
	m.tabCounts = map[string]int{"kanban": 3}
	m.activeTab = len(m.tabs) - 1

	out := m.renderTabs()
	if vl := visibleLen(strings.TrimRight(out, "\n")); vl > m.width {
		t.Errorf("tab bar is %d visible chars, terminal is %d: %q", vl, m.width, stripANSI(out))
	}
	if !strings.Contains(stripANSI(out), "kanban(3)") {
		t.Errorf("active tab must stay visible with its count; got %q", stripANSI(out))
	}
}

func TestWrapText(t *testing.T) {
	got := wrapText("the quick brown fox jumps", 10)
	for _, line := range got {
		if len([]rune(line)) > 10 {
			t.Errorf("line %q exceeds width 10", line)
		}
	}
	if strings.Join(got, " ") != "the quick brown fox jumps" {
		t.Errorf("wrap lost or reordered words: %q", got)
	}
}

func TestWrapText_hard_breaks_long_words(t *testing.T) {
	got := wrapText(strings.Repeat("x", 25), 10)
	if len(got) != 3 {
		t.Fatalf("expected 3 lines for a 25-char word at width 10, got %d: %q", len(got), got)
	}
	for _, line := range got {
		if len([]rune(line)) > 10 {
			t.Errorf("line %q exceeds width 10", line)
		}
	}
}

func TestViewDetail_wraps_body_within_width(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 30
	m.screen = ScreenDetail

	body := strings.Repeat("next dev OOM'd compacting an 8GB Turbopack SST cache. ", 8)
	m.detailTask = &event.OpenTask{
		ID: 5, Title: "OOM on next dev", Project: "laya", Status: event.StatusBacklog,
		Body: nullString(body),
	}

	for _, line := range strings.Split(m.View(), "\n") {
		if vl := visibleLen(line); vl > m.width {
			t.Errorf("detail line exceeds width %d (visible=%d): %q", m.width, vl, line)
		}
	}
}

func TestViewDetail_scrolls_long_content(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 15
	m.screen = ScreenDetail

	body := strings.Repeat("paragraph line\n", 60)
	m.detailTask = &event.OpenTask{
		ID: 5, Title: "long", Project: "p", Status: event.StatusBacklog,
		Body: nullString(body),
	}

	lines := strings.Split(strings.TrimRight(m.View(), "\n"), "\n")
	if len(lines) > m.height {
		t.Errorf("detail rendered %d lines, terminal height is %d", len(lines), m.height)
	}

	m.detailScroll = 40
	scrolled := stripANSI(m.View())
	if !strings.Contains(scrolled, "/") {
		t.Errorf("expected a scroll position indicator; got:\n%s", scrolled)
	}
}

// The accent legend on the All tab adds a glyph per project tab; the bar must
// still fit the terminal.
func TestRenderTabs_fits_width_with_accent_legend(t *testing.T) {
	m := newTestModel()
	m.width = 60
	m.height = 20
	m.tabs = []string{
		"All", "Cesar-s-junk-removal", "HDP115-Grupo4", "LAYA-IA-SDK",
		"SCC-v2", "SellConvertClean", "engram-memory", "infra", "kanban",
	}
	m.tabCounts = map[string]int{"kanban": 3, "LAYA-IA-SDK": 74}

	for _, active := range []int{0, 3, len(m.tabs) - 1} {
		m.activeTab = active
		out := strings.TrimRight(m.renderTabs(), "\n")
		if vl := visibleLen(out); vl > m.width {
			t.Errorf("activeTab=%d: tab bar is %d visible chars, terminal is %d: %q",
				active, vl, m.width, stripANSI(out))
		}
	}
}
