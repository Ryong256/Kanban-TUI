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

	// Total budget: m.height lines exactly
	// joinColumnsAdaptive produces colHeight lines (each ending with \n)
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

	showBadges := m.activeProject() == "" // All tab: show project badges

	// Determine per-column header widths for adaptive width computation.
	// Each header is "  LABEL (N)" or "▸ LABEL (N)" — the arrow/space prefix is
	// always 2 runes, the rest is label + space + count in parens.
	emptyMask := make([]bool, len(columns))
	headerWidths := make([]int, len(columns))
	for ci, status := range columns {
		tasks := m.board[status]
		emptyMask[ci] = len(tasks) == 0
		count := len(tasks)
		if status == event.StatusDone {
			count = m.doneTotal
		}
		label := colHeaderLabel[status]
		// "  LABEL (COUNT)" — 2 prefix + label + " (" + digits + ")"
		headerWidths[ci] = 2 + len(label) + 2 + len(fmt.Sprintf("%d", count)) + 1
	}

	totalWidth := m.width
	if totalWidth <= 0 {
		totalWidth = 80
	}
	// Subtract separator characters: (numCols-1) │ chars sit between columns.
	numSeparators := len(columns) - 1
	colAreaWidth := totalWidth - numSeparators
	if colAreaWidth < len(columns) {
		colAreaWidth = len(columns)
	}
	colWidths := computeColWidths(colAreaWidth, len(columns), 15, emptyMask, headerWidths)

	// Build each column
	renderedCols := make([]string, len(columns))
	for ci, status := range columns {
		tasks := m.board[status]
		headerLabel := colHeaderLabel[status]
		headerStyle := colHeaderStyle[status]
		colWidth := colWidths[ci]

		var col strings.Builder

		// Column header: done shows true total even when list is capped.
		count := len(tasks)
		if status == event.StatusDone {
			count = m.doneTotal
		}
		header := fmt.Sprintf("%s (%d)", headerLabel, count)
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
			// Reserve the last row for "+N more" or "+N older" footer if needed.
			visibleRows := maxRows
			hasOverflow := false
			overflowCount := 0
			if len(tasks) > maxRows {
				// Will need a footer line; shrink visible to maxRows-1 so the
				// footer fits within the column height budget.
				visibleRows = maxRows - 1
				hasOverflow = true
				overflowCount = len(tasks) - visibleRows
			}
			// For done column: also show "+N older" when total exceeds the cap.
			doneOlderCount := 0
			if status == event.StatusDone && m.doneTotal > len(tasks) {
				doneOlderCount = m.doneTotal - len(tasks)
			}

			for ri, t := range tasks {
				if ri >= visibleRows {
					break
				}

				// Badge prefix on All tab.
				prefix := "  "
				badgeStr := ""
				if showBadges {
					badgeStr = projectBadge(t.Project)
				}

				// Compute title width: colWidth minus prefix (2) minus badge visible len minus 1 space after badge.
				titleWidth := colWidth - 4
				if showBadges && badgeStr != "" {
					badgeVisible := visibleLen(badgeStr)
					titleWidth = colWidth - 2 - badgeVisible - 1
				}
				if titleWidth < 4 {
					titleWidth = 4
				}
				title := truncate(t.Title, titleWidth)

				isSelected := ci == m.colIdx && ri == m.rowIdx[ci]
				if isSelected {
					if showBadges && badgeStr != "" {
						col.WriteString(selectedStyle.Render("→ ") + badgeStr + " " + selectedStyle.Render(title) + "\n")
					} else {
						col.WriteString(selectedStyle.Render("→ "+title) + "\n")
					}
				} else {
					if showBadges && badgeStr != "" {
						col.WriteString(prefix + badgeStr + " " + title + "\n")
					} else {
						col.WriteString(prefix + title + "\n")
					}
				}
			}

			// Overflow footer. For the done column the cap overflow and the
			// viewport overflow fold into one honest hidden count, so the
			// footer never under-reports (e.g. "+6 more" hiding 100 tasks).
			if status == event.StatusDone {
				hidden := doneOlderCount
				if hasOverflow {
					hidden += overflowCount
				}
				if hidden > 0 {
					col.WriteString(dimStyle.Render(fmt.Sprintf("  +%d older", hidden)) + "\n")
				}
			} else if hasOverflow {
				col.WriteString(dimStyle.Render(fmt.Sprintf("  +%d more", overflowCount)) + "\n")
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

	// Join columns side by side with adaptive widths
	b.WriteString(joinColumnsAdaptive(renderedCols, colWidths, colHeight))

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
		help := strings.Join(shortcuts, " • ")
		if m.width > 0 {
			help = truncate(help, m.width)
		}
		b.WriteString(helpStyle.Render(help) + "\n")
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

// computeColWidths distributes totalWidth among numCols columns.
// emptyMask[i] == true means column i is empty and collapses to its header
// width (headerWidths[i]). Freed width is redistributed equally among
// non-empty columns.  All columns remain visible and the sum of returned
// widths NEVER exceeds totalWidth — fitting the terminal beats every other
// preference, so when space is short the layout degrades to an even split
// and content gets truncated.
//
// minWidth is a best-effort floor for non-empty columns: it is honored only
// while the budget allows, never by overflowing totalWidth.
func computeColWidths(totalWidth, numCols, minWidth int, emptyMask []bool, headerWidths []int) []int {
	if numCols == 0 {
		return nil
	}
	widths := make([]int, numCols)

	// evenSplit divides totalWidth equally among ALL columns — the degraded
	// layout for terminals too narrow for collapsed headers + content floors.
	evenSplit := func() []int {
		base := totalWidth / numCols
		if base < 1 {
			base = 1
		}
		rem := totalWidth - base*numCols
		if rem < 0 {
			rem = 0
		}
		for i := range widths {
			widths[i] = base
		}
		widths[numCols-1] += rem
		return widths
	}

	// Determine collapsed and freed width.
	collapsedTotal := 0
	nonEmptyCols := 0
	for i := 0; i < numCols; i++ {
		if emptyMask[i] {
			hw := headerWidths[i]
			if hw < 1 {
				hw = 1
			}
			widths[i] = hw
			collapsedTotal += hw
		} else {
			nonEmptyCols++
		}
	}

	if nonEmptyCols == 0 {
		return evenSplit()
	}

	available := totalWidth - collapsedTotal
	if available < nonEmptyCols {
		// Collapsed headers alone (almost) exhaust the budget — adaptive
		// layout cannot fit, so fall back to an even split.
		return evenSplit()
	}

	base := available / nonEmptyCols
	if base < minWidth {
		// The equal share fell below the content floor — the collapsed
		// headers are squeezing content out. Sacrifice them (they will be
		// truncated) and split the full budget evenly instead, which keeps
		// the sum within totalWidth.
		return evenSplit()
	}
	rem := available - base*nonEmptyCols

	// Assign base width to non-empty columns; add remainder to the last one.
	lastNonEmpty := -1
	for i := 0; i < numCols; i++ {
		if !emptyMask[i] {
			widths[i] = base
			lastNonEmpty = i
		}
	}
	if lastNonEmpty >= 0 {
		widths[lastNonEmpty] += rem
	}
	return widths
}

// joinColumnsAdaptive renders columns side by side with per-column widths.
func joinColumnsAdaptive(cols []string, widths []int, targetLines int) string {
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
			w := widths[ci]
			visible := visibleLen(text)
			padding := w - visible
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

// joinColumns renders columns side by side with exactly targetLines rows.
// Kept for backward compatibility; delegates to joinColumnsAdaptive with uniform widths.
func joinColumns(cols []string, colWidth, targetLines int) string {
	widths := make([]int, len(cols))
	for i := range widths {
		widths[i] = colWidth
	}
	return joinColumnsAdaptive(cols, widths, targetLines)
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
