package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/Ryong256/kanban/internal/event"
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
		}

	case tabsLoadedMsg:
		m.tabs = []string{"All"}
		m.tabs = append(m.tabs, msg.names...)

	case boardLoadedMsg:
		m.board = msg.board
		m.clampCursors()

	case taskMovedMsg:
		return m, m.loadBoard()

	case taskAddedMsg:
		m.screen = ScreenBoard
		m.input = ""
		m.addProject = ""
		return m, m.loadBoard()

	case detailLoadedMsg:
		m.detailTask = &msg.task
		m.detailTimeline = msg.timeline
		m.screen = ScreenDetail
		return m, nil

	case errMsg:
		m.err = msg
	}

	return m, nil
}

func (m *Model) updateBoard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

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
		col := columns[m.colIdx]
		if m.rowIdx[m.colIdx] < len(m.board[col])-1 {
			m.rowIdx[m.colIdx]++
		}

	case "k", "up":
		if m.rowIdx[m.colIdx] > 0 {
			m.rowIdx[m.colIdx]--
		}

	// Move task to previous status (left)
	case "backspace":
		return m.moveTaskLeft()

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

	// Task detail
	case "i", "enter":
		if msg.String() == "i" {
			task, ok := m.selectedTask()
			if ok {
				return m, m.loadDetail(task)
			}
		} else {
			return m.moveTaskRight()
		}
	}

	return m, nil
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

func (m *Model) selectedTask() (event.OpenTask, bool) {
	col := columns[m.colIdx]
	tasks := m.board[col]
	if len(tasks) == 0 || m.rowIdx[m.colIdx] >= len(tasks) {
		return event.OpenTask{}, false
	}
	return tasks[m.rowIdx[m.colIdx]], true
}

func (m *Model) moveTask(taskID int64, newStatus string) tea.Cmd {
	return func() tea.Msg {
		_, err := event.MoveTask(m.db, taskID, newStatus, "tui")
		if err != nil {
			return errMsg{err}
		}
		return taskMovedMsg{}
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
		tasks := m.board[col]
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
	col := columns[m.colIdx]
	tasks := m.board[col]
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
		return m, nil
	}
	return m, nil
}
