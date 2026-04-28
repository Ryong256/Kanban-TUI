package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/Ryong256/kanban/internal/event"
)

var (
	// Colors
	cyan    = lipgloss.Color("33")
	pink    = lipgloss.Color("212")
	dim     = lipgloss.Color("240")
	red     = lipgloss.Color("196")
	green   = lipgloss.Color("42")
	yellow  = lipgloss.Color("220")
	blue    = lipgloss.Color("75")
	magenta = lipgloss.Color("135")

	// Styles
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(cyan)
	tabActive     = lipgloss.NewStyle().Bold(true).Foreground(cyan).Underline(true)
	tabInactive   = lipgloss.NewStyle().Foreground(dim)
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(pink)
	dimStyle      = lipgloss.NewStyle().Foreground(dim)
	helpStyle     = lipgloss.NewStyle().Foreground(dim)
	errStyle      = lipgloss.NewStyle().Foreground(red)
)

// Column header colors per status
var colHeaderStyle = map[string]lipgloss.Style{
	event.StatusBacklog:    lipgloss.NewStyle().Bold(true).Foreground(blue),
	event.StatusInProgress: lipgloss.NewStyle().Bold(true).Foreground(yellow),
	event.StatusTesting:    lipgloss.NewStyle().Bold(true).Foreground(magenta),
	event.StatusComplete:   lipgloss.NewStyle().Bold(true).Foreground(green),
	event.StatusDone:       lipgloss.NewStyle().Bold(true).Foreground(dim),
}

var colHeaderLabel = map[string]string{
	event.StatusBacklog:    "BACKLOG",
	event.StatusInProgress: "IN PROGRESS",
	event.StatusTesting:    "TESTING",
	event.StatusComplete:   "COMPLETE",
	event.StatusDone:       "DONE",
}

func (m *Model) View() string {
	if m.width == 0 {
		return "loading..."
	}

	switch m.screen {
	case ScreenBoard:
		return m.viewBoard()
	case ScreenAddTask:
		return m.viewAddTask()
	case ScreenDetail:
		return m.viewDetail()
	default:
		return "unknown screen"
	}
}

func (m *Model) viewBoard() string {
	var b strings.Builder

	// No header here — project info goes in footer to avoid alt-screen clipping

	// Calculate column width
	colWidth := 0
	if m.width > 0 {
		colWidth = (m.width - 4) / len(columns) // 4 = margins
		if colWidth < 15 {
			colWidth = 15
		}
	} else {
		colWidth = 20
	}

	// Total budget: m.height lines exactly
	// joinColumns produces colHeight lines (each ending with \n)
	// footer: 1 blank + 1 tabs + 1 help = 3 lines
	// Extra 1 for safety (terminal bottom line)
	colHeight := m.height - 4
	if colHeight < 5 {
		colHeight = 5
	}
	// Task rows = colHeight minus column header (1) and separator (1)
	maxRows := colHeight - 2
	if maxRows < 1 {
		maxRows = 1
	}

	// Build each column
	renderedCols := make([]string, len(columns))
	for ci, status := range columns {
		tasks := m.board[status]
		headerLabel := colHeaderLabel[status]
		headerStyle := colHeaderStyle[status]

		var col strings.Builder

		// Column header with task count
		header := fmt.Sprintf("%s (%d)", headerLabel, len(tasks))
		if ci == m.colIdx {
			col.WriteString(headerStyle.Render("▸ " + header))
		} else {
			col.WriteString(headerStyle.Render("  " + header))
		}
		col.WriteString("\n")
		col.WriteString(dimStyle.Render(strings.Repeat("─", colWidth)) + "\n")

		// Tasks
		if len(tasks) == 0 {
			col.WriteString(dimStyle.Render("  (empty)") + "\n")
		} else {
			for ri, t := range tasks {
				if ri >= maxRows {
					remaining := len(tasks) - maxRows
					col.WriteString(dimStyle.Render(fmt.Sprintf("  +%d more", remaining)) + "\n")
					break
				}

				title := truncate(t.Title, colWidth-4)

				isSelected := ci == m.colIdx && ri == m.rowIdx[ci]
				if isSelected {
					col.WriteString(selectedStyle.Render("→ "+title) + "\n")
				} else {
					col.WriteString("  " + title + "\n")
				}
			}
		}

		// Pad to exact column height
		lines := strings.Count(col.String(), "\n")
		for lines < colHeight {
			col.WriteString("\n")
			lines++
		}

		renderedCols[ci] = col.String()
	}

	// Join columns side by side
	b.WriteString(joinColumns(renderedCols, colWidth, colHeight))

	// Footer: tabs + project + help
	b.WriteString("\n")

	// Tabs row
	proj := m.activeProject()
	if proj == "" {
		proj = "all projects"
	}
	b.WriteString(titleStyle.Render("📋 kanban") + "  ")
	for i, tab := range m.tabs {
		style := tabInactive
		if i == m.activeTab {
			style = tabActive
		}
		b.WriteString(style.Render(" "+tab+" "))
	}
	b.WriteString("  " + dimStyle.Render("("+proj+")"))
	b.WriteString("\n")

	if m.err != nil {
		b.WriteString(errStyle.Render("Error: "+m.err.Error()) + "\n")
	} else {
		shortcuts := []string{
			"h/l: nav",
			"j/k: tasks",
			"H/L: move",
			"1-5: jump",
			"enter/i: detail",
			"a: add",
			"tab: project",
			"q: quit",
		}
		b.WriteString(helpStyle.Render(strings.Join(shortcuts, " • ")) + "\n")
	}

	return b.String()
}

func (m *Model) viewAddTask() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("📋 kanban") + " → " + titleStyle.Render("add task") + "\n")
	if m.addProject != "" {
		b.WriteString(dimStyle.Render("project: "+m.addProject) + "\n")
	}
	b.WriteString("\n")
	b.WriteString("title: ")
	b.WriteString(m.input)
	b.WriteString("_\n")
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("enter: create • esc: cancel") + "\n")

	return b.String()
}

func (m *Model) viewDetail() string {
	if m.detailTask == nil {
		return "loading..."
	}
	t := m.detailTask
	var b strings.Builder

	b.WriteString(titleStyle.Render("📋 kanban") + " → " + titleStyle.Render("task detail") + "\n")
	b.WriteString(dimStyle.Render(strings.Repeat("─", m.width-2)) + "\n")
	b.WriteString("\n")

	// Task header
	b.WriteString(selectedStyle.Render(fmt.Sprintf("#%d  %s", t.ID, t.Title)) + "\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("project: %s", t.Project)) + "\n")

	statusStyle := colHeaderStyle[t.Status]
	statusLabel := colHeaderLabel[t.Status]
	if statusLabel == "" {
		statusLabel = t.Status
	}
	b.WriteString("status: " + statusStyle.Render(statusLabel) + "\n")

	if t.Body.Valid && t.Body.String != "" {
		b.WriteString("\n")
		b.WriteString(t.Body.String + "\n")
	}

	// Timeline
	if len(m.detailTimeline) > 0 {
		b.WriteString("\n")
		b.WriteString(titleStyle.Render("timeline") + "\n")
		b.WriteString(dimStyle.Render(strings.Repeat("─", 40)) + "\n")
		for _, e := range m.detailTimeline {
			ts := fmt.Sprintf("%d", e.TS)
			b.WriteString(dimStyle.Render(ts) + "  " + e.Type + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("esc/bksp: back • q: quit") + "\n")

	return b.String()
}

// joinColumns renders columns side by side with exactly targetLines rows.
func joinColumns(cols []string, colWidth, targetLines int) string {
	// Split each column into lines
	splitCols := make([][]string, len(cols))
	for i, col := range cols {
		splitCols[i] = strings.Split(strings.TrimRight(col, "\n"), "\n")
	}

	var b strings.Builder
	for line := 0; line < targetLines; line++ {
		for ci, colLines := range splitCols {
			text := ""
			if line < len(colLines) {
				text = colLines[line]
			}
			// Pad to column width (using visible length)
			visible := visibleLen(text)
			padding := colWidth - visible
			if padding < 0 {
				padding = 0
			}
			b.WriteString(text)
			b.WriteString(strings.Repeat(" ", padding))

			if ci < len(splitCols)-1 {
				b.WriteString(dimStyle.Render("│"))
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

// truncate cuts a string to max visible length, adding ellipsis.
func truncate(s string, max int) string {
	if max <= 3 {
		max = 3
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}

// visibleLen estimates the visible length of a string (strips ANSI codes).
func visibleLen(s string) int {
	inEsc := false
	n := 0
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
		n++
	}
	return n
}
