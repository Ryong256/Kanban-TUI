package tui

import (
	"fmt"
	"strings"

	"github.com/Ryong256/kanban/internal/event"
	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch m.screen {
		case ScreenBoard:
			return m.updateBoard(msg)
		case ScreenAddTask:
			return m.updateAddTask(msg)
		case ScreenDetail:
			return m.updateDetail(msg)
		case ScreenConfirmDelete:
			return m.updateConfirmDelete(msg)
		case ScreenConfirmDeleteProject:
			return m.updateConfirmDeleteProject(msg)
		case ScreenHelp:
			m.screen = m.prevScreen
			return m, nil
		}

	case tabsLoadedMsg:
		m.tabs = []string{"All"}
		m.tabs = append(m.tabs, msg.names...)
		m.tabCounts = msg.counts
		// If an initial project was requested, switch to its tab.
		if m.initialProject != "" {
			for i, name := range m.tabs {
				if name == m.initialProject {
					m.activeTab = i
					m.initialProject = "" // consume so we don't re-apply on subsequent reloads
					return m, m.loadBoard()
				}
			}
			// Requested project not in registry — stay on "All".
			m.initialProject = ""
		}

	case boardLoadedMsg:
		m.board = msg.board
		m.doneTotal = msg.doneTotal
		m.doneInWindow = msg.doneInWindow
		if m.pendingFocusTaskID != 0 {
			m.focusTask(m.pendingFocusTaskID)
			m.pendingFocusTaskID = 0
		}
		m.clampCursors()

	case taskMovedMsg:
		m.pendingFocusTaskID = msg.taskID
		return m, m.loadBoard()

	case taskAddedMsg:
		m.screen = ScreenBoard
		m.input = ""
		m.addProject = ""
		return m, tea.Batch(m.loadBoard(), m.loadTabs())

	case taskDeletedMsg:
		m.screen = ScreenBoard
		m.deleteTarget = nil
		m.flash = fmt.Sprintf("deleted #%d", msg.taskID)
		return m, tea.Batch(m.loadBoard(), m.loadTabs())

	case taskDemotedMsg:
		m.screen = ScreenBoard
		m.deleteTarget = nil
		m.flash = fmt.Sprintf("#%d is now a note", msg.taskID)
		return m, tea.Batch(m.loadBoard(), m.loadTabs())

	case projectDeletedMsg:
		m.screen = ScreenBoard
		m.deleteProject = ""
		if msg.purged {
			m.flash = fmt.Sprintf("deleted project %q and its events", msg.name)
		} else {
			m.flash = fmt.Sprintf("unregistered project %q (events kept)", msg.name)
		}
		// The active tab index is stale once a tab disappears.
		m.activeTab = 0
		return m, tea.Batch(m.loadTabs(), m.loadBoard())

	case detailLoadedMsg:
		m.detailTask = &msg.task
		m.detailTimeline = msg.timeline
		m.detailScroll = 0
		m.screen = ScreenDetail
		return m, nil

	case errMsg:
		m.err = msg
	}

	return m, nil
}

func (m *Model) updateBoard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Any keypress clears stale feedback from the previous action.
	m.flash = ""

	// While the filter input is open it owns the keyboard, otherwise typing a
	// query would move tasks around the board.
	if m.filtering {
		return m.updateFilterInput(msg)
	}

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "?":
		m.prevScreen = ScreenBoard
		m.screen = ScreenHelp
		return m, nil

	case "/":
		m.filtering = true
		m.filter = ""
		return m, nil

	case "esc":
		if m.filter != "" {
			m.filter = ""
			m.resetScroll()
		}
		return m, nil

	// Delete the active project. Capital letter on purpose: this is the most
	// destructive key on the board and must not sit next to task navigation.
	case "D":
		proj := m.activeProject()
		if proj == "" {
			m.flash = "select a project tab first"
			return m, nil
		}
		m.deleteProject = proj
		m.deleteProjectCount = m.projectEventCount(proj)
		m.screen = ScreenConfirmDeleteProject
		return m, nil

	// Tab navigation
	case "tab":
		m.activeTab = (m.activeTab + 1) % len(m.tabs)
		return m, m.loadBoard()

	case "shift+tab":
		m.activeTab = (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
		return m, m.loadBoard()

	// Column navigation
	case "h", "left":
		if m.colIdx > 0 {
			m.colIdx--
			m.clampCurrentRow()
		}

	case "l", "right":
		if m.colIdx < len(columns)-1 {
			m.colIdx++
			m.clampCurrentRow()
		}

	// Row navigation within column
	case "j", "down":
		if m.rowIdx[m.colIdx] < len(m.visibleTasks(columns[m.colIdx]))-1 {
			m.rowIdx[m.colIdx]++
		}

	case "k", "up":
		if m.rowIdx[m.colIdx] > 0 {
			m.rowIdx[m.colIdx]--
		}

	case "g", "home":
		m.rowIdx[m.colIdx] = 0

	case "G", "end":
		if n := len(m.visibleTasks(columns[m.colIdx])); n > 0 {
			m.rowIdx[m.colIdx] = n - 1
		}

	// Move task across columns (cursor follows)
	case "H", "shift+left":
		return m.moveTaskLeft()

	case "L", "shift+right":
		return m.moveTaskRight()

	// Jump task directly to a column by number (1=backlog … 4=done)
	case "1", "2", "3", "4":
		return m.moveTaskToCol(int(msg.String()[0] - '1'))

	// Add task
	case "a":
		proj := m.activeProject()
		if proj != "" {
			m.addProject = proj
			m.screen = ScreenAddTask
			m.input = ""
			return m, nil
		}
		// All tab — need project picker, use first registered if only one
		if len(m.tabs) == 2 {
			m.addProject = m.tabs[1]
			m.screen = ScreenAddTask
			m.input = ""
			return m, nil
		}
		if len(m.tabs) <= 1 {
			m.err = errMsg{errNoProjects}
			return m, nil
		}
		// Multiple projects — switch to first project tab then add
		m.activeTab = 1
		m.addProject = m.tabs[1]
		m.screen = ScreenAddTask
		m.input = ""
		return m, nil

	// Delete the selected task (confirmation first — this is irreversible).
	case "d", "delete":
		task, ok := m.selectedTask()
		if ok {
			t := task
			m.deleteTarget = &t
			m.screen = ScreenConfirmDelete
		}
		return m, nil

	// Reclassify the selected task as a note. Non-destructive: the content
	// survives, so this needs no confirmation.
	case "n":
		task, ok := m.selectedTask()
		if ok {
			return m, m.demoteTaskCmd(task.ID)
		}
		return m, nil

	// Task detail
	case "i", "enter":
		task, ok := m.selectedTask()
		if ok {
			return m, m.loadDetail(task)
		}
	}

	return m, nil
}

func (m *Model) updateConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "y", "Y":
		if m.deleteTarget != nil {
			return m, m.deleteTaskCmd(m.deleteTarget.ID)
		}
		m.screen = ScreenBoard
		return m, nil

	// Offer the non-destructive escape hatch right where the user is already
	// deciding: most rows they want gone are notes, not mistakes. This is NOT
	// bound to "n" — in a yes/no prompt "n" means no, and stealing that key to
	// perform an action would be a trap.
	case "t":
		if m.deleteTarget != nil {
			return m, m.demoteTaskCmd(m.deleteTarget.ID)
		}
		m.screen = ScreenBoard
		return m, nil

	case "n", "N", "esc", "q":
		m.screen = ScreenBoard
		m.deleteTarget = nil
		return m, nil
	}
	return m, nil
}

func (m *Model) moveTaskToCol(targetIdx int) (tea.Model, tea.Cmd) {
	if targetIdx < 0 || targetIdx >= len(columns) || targetIdx == m.colIdx {
		return m, nil
	}
	task, ok := m.selectedTask()
	if !ok {
		return m, nil
	}
	return m, m.moveTask(task.ID, columns[targetIdx])
}

// focusTask locates a task by ID across all columns and points the cursor at it.
// Called after a move to keep the cursor on the task the user just moved.
func (m *Model) focusTask(taskID int64) {
	for i, col := range columns {
		for j, t := range m.visibleTasks(col) {
			if t.ID == taskID {
				m.colIdx = i
				m.rowIdx[i] = j
				return
			}
		}
	}
}

func (m *Model) moveTaskRight() (tea.Model, tea.Cmd) {
	if m.colIdx >= len(columns)-1 {
		return m, nil // already at done
	}
	task, ok := m.selectedTask()
	if !ok {
		return m, nil
	}
	nextStatus := columns[m.colIdx+1]
	return m, m.moveTask(task.ID, nextStatus)
}

func (m *Model) moveTaskLeft() (tea.Model, tea.Cmd) {
	if m.colIdx <= 0 {
		return m, nil // already at backlog
	}
	task, ok := m.selectedTask()
	if !ok {
		return m, nil
	}
	prevStatus := columns[m.colIdx-1]
	return m, m.moveTask(task.ID, prevStatus)
}

// selectedTask resolves the cursor against the FILTERED column, which is what
// the user is looking at. Resolving against the raw column would act on a
// different task than the highlighted one whenever a filter is active.
func (m *Model) selectedTask() (event.OpenTask, bool) {
	tasks := m.visibleTasks(columns[m.colIdx])
	idx := m.rowIdx[m.colIdx]
	if len(tasks) == 0 || idx < 0 || idx >= len(tasks) {
		return event.OpenTask{}, false
	}
	return tasks[idx], true
}

func (m *Model) resetScroll() {
	for i := range m.scrollIdx {
		m.scrollIdx[i] = 0
	}
	for i := range m.rowIdx {
		m.rowIdx[i] = 0
	}
}

// projectEventCount reports how many events a project owns, so the delete
// confirmation can state the real cost instead of a vague warning.
func (m *Model) projectEventCount(name string) int {
	if m.db == nil {
		return 0
	}
	var n int
	if err := m.db.QueryRow(`SELECT COUNT(*) FROM events WHERE project = ?`, name).Scan(&n); err != nil {
		return 0
	}
	return n
}

func (m *Model) updateFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		m.filtering = false
		m.filter = ""
		m.resetScroll()
		return m, nil

	case "enter":
		// Keep the filter, hand the keyboard back to the board.
		m.filtering = false
		m.resetScroll()
		return m, nil

	case "backspace":
		if r := []rune(m.filter); len(r) > 0 {
			m.filter = string(r[:len(r)-1])
			m.resetScroll()
		}
		return m, nil

	default:
		if s := msg.String(); len([]rune(s)) == 1 {
			m.filter += s
			m.resetScroll()
		}
		return m, nil
	}
}

func (m *Model) updateConfirmDeleteProject(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "y", "Y":
		if m.deleteProject != "" {
			return m, m.deleteProjectCmd(m.deleteProject, true)
		}
		m.screen = ScreenBoard
		return m, nil

	case "u":
		if m.deleteProject != "" {
			return m, m.deleteProjectCmd(m.deleteProject, false)
		}
		m.screen = ScreenBoard
		return m, nil

	case "n", "N", "esc", "q":
		m.screen = ScreenBoard
		m.deleteProject = ""
		return m, nil
	}
	return m, nil
}

func (m *Model) moveTask(taskID int64, newStatus string) tea.Cmd {
	return func() tea.Msg {
		_, err := event.MoveTask(m.db, taskID, newStatus, "tui")
		if err != nil {
			return errMsg{err}
		}
		return taskMovedMsg{taskID: taskID}
	}
}

func (m *Model) updateAddTask(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		m.screen = ScreenBoard
		m.input = ""
		m.addProject = ""
		return m, nil

	case "enter":
		if strings.TrimSpace(m.input) != "" {
			return m, m.addTaskCmd(m.input)
		}

	case "backspace":
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}

	default:
		m.input += msg.String()
	}
	return m, nil
}

func (m *Model) addTaskCmd(title string) tea.Cmd {
	return func() tea.Msg {
		_, err := event.Add(m.db, event.Insert{
			Type:    event.TaskNew,
			Project: m.addProject,
			Title:   title,
			Source:  "tui",
			Status:  event.StatusBacklog,
		})
		if err != nil {
			return errMsg{err}
		}
		return taskAddedMsg{}
	}
}

func (m *Model) clampCursors() {
	for i, col := range columns {
		tasks := m.visibleTasks(col)
		if m.rowIdx[i] >= len(tasks) {
			if len(tasks) > 0 {
				m.rowIdx[i] = len(tasks) - 1
			} else {
				m.rowIdx[i] = 0
			}
		}
	}
}

func (m *Model) clampCurrentRow() {
	tasks := m.visibleTasks(columns[m.colIdx])
	if m.rowIdx[m.colIdx] >= len(tasks) {
		if len(tasks) > 0 {
			m.rowIdx[m.colIdx] = len(tasks) - 1
		} else {
			m.rowIdx[m.colIdx] = 0
		}
	}
}

func (m *Model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "esc", "backspace":
		m.screen = ScreenBoard
		m.detailTask = nil
		m.detailTimeline = nil
		m.detailScroll = 0
		return m, nil

	case "j", "down":
		m.detailScroll++

	case "k", "up":
		if m.detailScroll > 0 {
			m.detailScroll--
		}

	case "g", "home":
		m.detailScroll = 0

	case "d", "delete":
		if m.detailTask != nil {
			t := *m.detailTask
			m.deleteTarget = &t
			m.screen = ScreenConfirmDelete
		}
		return m, nil
	}
	return m, nil
}
