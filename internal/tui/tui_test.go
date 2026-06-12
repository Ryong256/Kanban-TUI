package tui

import (
	"testing"

	"github.com/Ryong256/kanban/internal/event"
)

// boardWith creates a board map populated with tasks for testing pure helpers.
func boardWith(statuses map[string][]event.OpenTask) map[string][]event.OpenTask {
	board := make(map[string][]event.OpenTask)
	for _, s := range event.AllStatuses() {
		board[s] = []event.OpenTask{}
	}
	for s, tasks := range statuses {
		board[s] = tasks
	}
	return board
}

func makeTask(id int64) event.OpenTask {
	return event.OpenTask{ID: id, Title: "task"}
}

// newTestModel builds a Model with a nil db — safe for pure-logic tests that
// never call DB methods.
func newTestModel() *Model {
	return &Model{
		db:     nil,
		screen: ScreenBoard,
		board:  make(map[string][]event.OpenTask),
		rowIdx: make([]int, len(columns)),
		tabs:   []string{"All"},
	}
}

// --- clampCursors ---

func TestClampCursors_empty_columns(t *testing.T) {
	m := newTestModel()
	m.board = boardWith(nil)
	m.rowIdx = []int{5, 3, 2, 1, 0}
	m.clampCursors()
	for i, v := range m.rowIdx {
		if v != 0 {
			t.Errorf("column %d: expected rowIdx=0 for empty column, got %d", i, v)
		}
	}
}

func TestClampCursors_nonempty(t *testing.T) {
	m := newTestModel()
	m.board = boardWith(map[string][]event.OpenTask{
		event.StatusBacklog: {makeTask(1), makeTask(2)},
	})
	m.rowIdx = []int{5, 0, 0, 0, 0} // cursor beyond end of backlog
	m.clampCursors()
	if m.rowIdx[0] != 1 {
		t.Errorf("expected rowIdx[0]=1 (last task), got %d", m.rowIdx[0])
	}
}

// --- selectedTask ---

func TestSelectedTask_empty(t *testing.T) {
	m := newTestModel()
	m.board = boardWith(nil)
	_, ok := m.selectedTask()
	if ok {
		t.Fatal("expected false for empty board")
	}
}

func TestSelectedTask_returns_correct_task(t *testing.T) {
	m := newTestModel()
	m.board = boardWith(map[string][]event.OpenTask{
		event.StatusBacklog: {makeTask(10), makeTask(20)},
	})
	m.colIdx = 0
	m.rowIdx[0] = 1

	task, ok := m.selectedTask()
	if !ok {
		t.Fatal("expected ok=true")
	}
	if task.ID != 20 {
		t.Fatalf("expected task id 20, got %d", task.ID)
	}
}

// --- focusTask ---

func TestFocusTask_finds_task(t *testing.T) {
	m := newTestModel()
	m.board = boardWith(map[string][]event.OpenTask{
		event.StatusInProgress: {makeTask(5), makeTask(7)},
	})
	m.focusTask(7)

	expectedCol := 1 // in_progress is index 1
	if m.colIdx != expectedCol {
		t.Errorf("expected colIdx=%d, got %d", expectedCol, m.colIdx)
	}
	if m.rowIdx[expectedCol] != 1 {
		t.Errorf("expected rowIdx[%d]=1, got %d", expectedCol, m.rowIdx[expectedCol])
	}
}

func TestFocusTask_not_found_noop(t *testing.T) {
	m := newTestModel()
	m.board = boardWith(nil)
	m.colIdx = 2
	m.rowIdx[2] = 3
	m.focusTask(999) // task not in board — must not panic or change cursor
	if m.colIdx != 2 {
		t.Errorf("expected colIdx unchanged at 2, got %d", m.colIdx)
	}
}

// --- activeProject ---

func TestActiveProject_all_tab(t *testing.T) {
	m := newTestModel()
	m.tabs = []string{"All", "kanban", "other"}
	m.activeTab = 0
	if got := m.activeProject(); got != "" {
		t.Fatalf("expected empty string for All tab, got %q", got)
	}
}

func TestActiveProject_project_tab(t *testing.T) {
	m := newTestModel()
	m.tabs = []string{"All", "kanban", "other"}
	m.activeTab = 1
	if got := m.activeProject(); got != "kanban" {
		t.Fatalf("expected %q, got %q", "kanban", got)
	}
}

// --- initialProject tab selection (Task 479) ---

func TestTabsLoaded_initialProject_switches_tab(t *testing.T) {
	m := newTestModel()
	m.initialProject = "kanban"

	// The returned tea.Cmd (loadBoard) is not executed, so the nil db is safe.
	updated, cmd := m.Update(tabsLoadedMsg{names: []string{"kanban", "other"}})
	m = updated.(*Model)

	if m.activeTab != 1 {
		t.Fatalf("expected activeTab=1 (kanban), got %d", m.activeTab)
	}
	if m.initialProject != "" {
		t.Fatal("expected initialProject to be consumed")
	}
	if cmd == nil {
		t.Fatal("expected a board reload command after switching tab")
	}
}

func TestTabsLoaded_initialProject_not_found_stays_all(t *testing.T) {
	m := newTestModel()
	m.initialProject = "nonexistent"

	updated, _ := m.Update(tabsLoadedMsg{names: []string{"kanban", "other"}})
	m = updated.(*Model)

	if m.activeTab != 0 {
		t.Fatalf("expected activeTab=0 (All), got %d", m.activeTab)
	}
	if m.initialProject != "" {
		t.Fatal("expected initialProject to be consumed even when not found")
	}
}

func TestTabsLoaded_empty_initialProject_stays_all(t *testing.T) {
	m := newTestModel()

	updated, _ := m.Update(tabsLoadedMsg{names: []string{"kanban"}})
	m = updated.(*Model)

	if m.activeTab != 0 {
		t.Fatalf("expected activeTab=0, got %d", m.activeTab)
	}
}
