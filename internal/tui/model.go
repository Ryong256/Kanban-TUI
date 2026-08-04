package tui

import (
	"database/sql"
	"fmt"

	"github.com/Ryong256/kanban/internal/event"
	"github.com/Ryong256/kanban/internal/project"
	tea "github.com/charmbracelet/bubbletea"
)

type Screen int

const (
	ScreenBoard Screen = iota
	ScreenAddTask
	ScreenDetail
	ScreenConfirmDelete
	ScreenConfirmDeleteProject
	ScreenHelp
)

// columns maps to event.AllStatuses() indices.
var columns = event.AllStatuses() // backlog, in_progress, testing, done

type Model struct {
	db *sql.DB

	// Tab state
	tabs           []string // "All" + registered project names
	tabCounts      map[string]int
	activeTab      int
	initialProject string // project to select on first tab load

	// Board state
	screen       Screen
	board        map[string][]event.OpenTask // status → tasks (done capped at doneLimit)
	doneTotal    int                         // all-time done count
	doneInWindow int                         // done inside doneWindow — what the column shows
	colIdx       int                         // active column index (0-3)
	rowIdx       []int                       // cursor row per column
	scrollIdx    []int                       // first visible row per column

	// Add task state
	input      string
	addProject string

	// Detail view state
	detailTask     *event.OpenTask
	detailTimeline []event.TimelineEntry
	detailScroll   int

	// Delete confirmation state
	deleteTarget       *event.OpenTask
	deleteProject      string
	deleteProjectCount int

	// Incremental filter over task titles and scopes.
	filter    string
	filtering bool

	// Screen to return to when a modal closes.
	prevScreen Screen

	// Transient one-line feedback shown in the footer (e.g. "deleted #12").
	flash string

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
		db:             db,
		screen:         ScreenBoard,
		board:          make(map[string][]event.OpenTask),
		rowIdx:         make([]int, len(columns)),
		scrollIdx:      make([]int, len(columns)),
		tabs:           []string{"All"},
		tabCounts:      make(map[string]int),
		initialProject: initialProject,
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
		// Counts are best-effort: a failure here must not cost the user the
		// tab bar itself.
		counts, err := event.CountOpenByProject(m.db)
		if err != nil {
			counts = map[string]int{}
		}
		return tabsLoadedMsg{names: names, counts: counts}
	}
}

func (m *Model) deleteTaskCmd(taskID int64) tea.Cmd {
	return func() tea.Msg {
		if err := event.DeleteTask(m.db, taskID); err != nil {
			return errMsg{err}
		}
		return taskDeletedMsg{taskID: taskID}
	}
}

func (m *Model) demoteTaskCmd(taskID int64) tea.Cmd {
	return func() tea.Msg {
		if err := event.ConvertTaskToNote(m.db, taskID); err != nil {
			return errMsg{err}
		}
		return taskDemotedMsg{taskID: taskID}
	}
}

func (m *Model) loadBoard() tea.Cmd {
	return func() tea.Msg {
		proj := m.activeProject()
		result, err := event.ListByStatus(m.db, proj, doneLimit, doneWindow)
		if err != nil {
			return errMsg{err}
		}
		return boardLoadedMsg{
			board:        result.Board,
			doneTotal:    result.DoneTotal,
			doneInWindow: result.DoneInWindow,
		}
	}
}

func (m *Model) deleteProjectCmd(name string, purge bool) tea.Cmd {
	return func() tea.Msg {
		if purge {
			if _, err := event.DeleteProjectEvents(m.db, name); err != nil {
				return errMsg{err}
			}
		}
		// Unregistering a project that was never in the registry is not an
		// error worth surfacing — the events are what the user came for.
		_ = project.Remove(m.db, name)
		return projectDeletedMsg{name: name, purged: purge}
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
	names  []string
	counts map[string]int
}

type taskDeletedMsg struct {
	taskID int64
}

type taskDemotedMsg struct {
	taskID int64
}

type boardLoadedMsg struct {
	board        map[string][]event.OpenTask
	doneTotal    int
	doneInWindow int
}

type projectDeletedMsg struct {
	name   string
	purged bool
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
