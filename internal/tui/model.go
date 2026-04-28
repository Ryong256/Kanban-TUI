package tui

import (
	"database/sql"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/Ryong256/kanban/internal/event"
	"github.com/Ryong256/kanban/internal/project"
)

type Screen int

const (
	ScreenBoard Screen = iota
	ScreenAddTask
	ScreenDetail
)

// columns maps to event.AllStatuses() indices.
var columns = event.AllStatuses() // backlog, in_progress, testing, complete, done

type Model struct {
	db *sql.DB

	// Tab state
	tabs      []string // "All" + registered project names
	activeTab int

	// Board state
	screen    Screen
	board     map[string][]event.OpenTask // status → tasks
	colIdx    int                         // active column index (0-4)
	rowIdx    []int                       // cursor row per column

	// Add task state
	input      string
	addProject string

	// Detail view state
	detailTask     *event.OpenTask
	detailTimeline []event.TimelineEntry

	// Cursor follows the task across moves: set when a move is dispatched,
	// applied (and cleared) on the next boardLoadedMsg.
	pendingFocusTaskID int64

	// Layout
	width  int
	height int
	err    error
}

func NewModel(db *sql.DB, initialProject string) *Model {
	m := &Model{
		db:     db,
		screen: ScreenBoard,
		board:  make(map[string][]event.OpenTask),
		rowIdx: make([]int, len(columns)),
		tabs:   []string{"All"},
	}
	return m
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.loadTabs(), m.loadBoard())
}

func (m *Model) activeProject() string {
	if m.activeTab == 0 {
		return "" // "All" tab
	}
	if m.activeTab < len(m.tabs) {
		return m.tabs[m.activeTab]
	}
	return ""
}

func (m *Model) loadTabs() tea.Cmd {
	return func() tea.Msg {
		names, err := project.Names(m.db)
		if err != nil {
			return errMsg{err}
		}
		return tabsLoadedMsg{names}
	}
}

func (m *Model) loadBoard() tea.Cmd {
	return func() tea.Msg {
		proj := m.activeProject()
		grouped, err := event.ListByStatus(m.db, proj)
		if err != nil {
			return errMsg{err}
		}
		return boardLoadedMsg{grouped}
	}
}

func (m *Model) loadDetail(task event.OpenTask) tea.Cmd {
	return func() tea.Msg {
		timeline, err := event.TaskTimeline(m.db, task.ID)
		if err != nil {
			return errMsg{err}
		}
		return detailLoadedMsg{task: task, timeline: timeline}
	}
}

type tabsLoadedMsg struct {
	names []string
}

type boardLoadedMsg struct {
	board map[string][]event.OpenTask
}

type taskMovedMsg struct {
	taskID int64
}
type taskAddedMsg struct{}

type detailLoadedMsg struct {
	task     event.OpenTask
	timeline []event.TimelineEntry
}

type errMsg struct {
	err error
}

func (e errMsg) Error() string {
	return e.err.Error()
}

var errNoProjects = fmt.Errorf("no registered projects — use: kb project add <name> [path]")
