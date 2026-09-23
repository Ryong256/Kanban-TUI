package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Ryong256/kanban/internal/event"
	"github.com/charmbracelet/lipgloss"
)

// The board runs on a strict color budget, because a channel that means five
// things means none. In priority order:
//
//	structure  greys        borders, labels, unfocused columns
//	content    near-white   text in the focused column
//	focus      one accent   the selected row and the active tab
//	alarm      red          only things that are genuinely wrong, and rarely
//	identity   muted hues   which project a row belongs to (lowest priority)
//
// Anything permanently lit stops being a signal, so alarm colors are tuned to
// fire on outliers, not on the steady state.
var (
	// Structure and content.
	fg      = lipgloss.Color("252") // focused content
	dim     = lipgloss.Color("245") // unfocused content, labels
	faint   = lipgloss.Color("238") // rules, separators, chrome
	accent  = lipgloss.Color("39")  // focus: selection and active tab
	alarm   = lipgloss.Color("203") // wrong and worth interrupting for
	caution = lipgloss.Color("179") // worth noticing, not interrupting
	good    = lipgloss.Color("71")  // confirmations

	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(accent)
	tabActive  = lipgloss.NewStyle().Bold(true).Foreground(accent).Underline(true)

	tabInactive = lipgloss.NewStyle().Foreground(dim)
	dimStyle    = lipgloss.NewStyle().Foreground(dim)
	faintStyle  = lipgloss.NewStyle().Foreground(faint)
	helpStyle   = lipgloss.NewStyle().Foreground(faint)
	errStyle    = lipgloss.NewStyle().Foreground(alarm)
	flashStyle  = lipgloss.NewStyle().Foreground(good)
	warnStyle   = lipgloss.NewStyle().Bold(true).Foreground(alarm)
	labelStyle  = lipgloss.NewStyle().Foreground(dim)

	// Selection is a band, not a shout: a dark plate with bright text reads as
	// "you are here" without competing with the alarm color for attention.
	rowSelected = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.Color("231")).
			Background(lipgloss.Color("238"))
	rowNormal = lipgloss.NewStyle().Foreground(fg)
	// Rows in a column that does not have focus are muted so the eye lands on
	// the focused column first.
	rowUnfocused = lipgloss.NewStyle().Foreground(dim)

	// Column headers never carry the alarm themselves — only the count inside
	// them does. Painting a whole header red leaves two of four columns lit
	// permanently, which trains you to ignore red.
	headerFocused   = lipgloss.NewStyle().Bold(true).Foreground(fg)
	headerUnfocused = lipgloss.NewStyle().Foreground(dim)
	countOverWIP    = lipgloss.NewStyle().Bold(true).Foreground(alarm)

	// Age ramp. Thresholds are deliberately far out: on a personal board most
	// things are days old, so days must read as normal.
	ageFresh = lipgloss.NewStyle().Foreground(faint)
	ageWarm  = lipgloss.NewStyle().Foreground(caution)
	ageStale = lipgloss.NewStyle().Bold(true).Foreground(alarm)
)

var colHeaderLabel = map[string]string{
	event.StatusBacklog:    "BACKLOG",
	event.StatusInProgress: "IN PROGRESS",
	event.StatusTesting:    "TESTING",
	event.StatusDone:       "DONE",
}

// wipLimits caps the columns where work actually sits. Without a limit the
// board is a list that happens to have columns.
var wipLimits = map[string]int{
	event.StatusInProgress: 5,
	event.StatusTesting:    5,
}

// Age thresholds for the columns where age is a signal.
//
// A week in progress is ordinary on a personal board; a month is not. Warning
// at three days painted an entire column amber and said nothing.
const (
	ageWarmDays  = 7
	ageStaleDays = 30
)

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
	case ScreenConfirmDelete:
		return m.viewConfirmDelete()
	case ScreenConfirmDeleteProject:
		return m.viewConfirmDeleteProject()
	case ScreenHelp:
		return m.viewHelp()
	default:
		return "unknown screen"
	}
}

// visibleTasks applies the active filter to a column.
func (m *Model) visibleTasks(status string) []event.OpenTask {
	tasks := m.board[status]
	if m.filter == "" {
		return tasks
	}
	out := make([]event.OpenTask, 0, len(tasks))
	for _, t := range tasks {
		if fuzzyMatch(t.Title, m.filter) ||
			(t.Scope.Valid && fuzzyMatch(t.Scope.String, m.filter)) ||
			idMatch(t.ID, m.filter) ||
			(t.Body.Valid && containsFold(t.Body.String, m.filter)) {
			out = append(out, t)
		}
	}
	return out
}

// idMatch reports whether filter is a task id ("2345" or "#2345"). It matches
// by prefix so the task stays in view while the id is still being typed.
func idMatch(id int64, filter string) bool {
	digits := strings.TrimPrefix(filter, "#")
	if digits == "" {
		return false
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return false
		}
	}
	return strings.HasPrefix(strconv.FormatInt(id, 10), digits)
}

// containsFold is a case-insensitive substring test. Bodies get it instead of
// fuzzyMatch: a short pattern is a subsequence of almost any paragraph, so a
// fuzzy body match would keep nearly every task.
func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

func (m *Model) viewBoard() string {
	var b strings.Builder

	// Per-column cursor/scroll slices must match the column count; a Model
	// built without them must not panic the whole TUI.
	if len(m.rowIdx) != len(columns) {
		m.rowIdx = make([]int, len(columns))
	}
	if len(m.scrollIdx) != len(columns) {
		m.scrollIdx = make([]int, len(columns))
	}

	// Budget: colHeight lines of board, then preview + tabs + status = 3,
	// plus a blank separator and one spare for the terminal's bottom line.
	colHeight := m.height - 5
	if colHeight < 5 {
		colHeight = 5
	}
	// Task rows = colHeight minus column header (1) and separator (1).
	maxRows := colHeight - 2
	if maxRows < 1 {
		maxRows = 1
	}

	showAccent := m.activeProject() == "" // All tab: mark rows by project

	totalWidth := m.width
	if totalWidth <= 0 {
		totalWidth = 80
	}
	numSeparators := len(columns) - 1
	colAreaWidth := totalWidth - numSeparators
	if colAreaWidth < len(columns) {
		colAreaWidth = len(columns)
	}
	colWidths := computeFocusWidths(colAreaWidth, len(columns), m.colIdx, 12)

	renderedCols := make([]string, len(columns))
	for ci, status := range columns {
		tasks := m.visibleTasks(status)
		focused := ci == m.colIdx
		colWidth := colWidths[ci]

		var col strings.Builder
		col.WriteString(m.renderColHeader(ci, status, tasks, maxRows, colWidth))
		col.WriteString(faintStyle.Render(strings.Repeat("─", colWidth)) + "\n")

		if len(tasks) == 0 {
			label := "  (empty)"
			if m.filter != "" {
				label = "  (no match)"
			}
			col.WriteString(faintStyle.Render(label) + "\n")
		} else {
			start, end := m.viewportFor(ci, len(tasks), maxRows)
			for ri := start; ri < end; ri++ {
				col.WriteString(m.renderTaskRow(tasks[ri], status, colWidth, showAccent,
					focused && ri == m.rowIdx[ci], focused))
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

	b.WriteString(joinColumnsAdaptive(renderedCols, colWidths, colHeight))
	b.WriteString("\n")
	b.WriteString(m.renderPreview())
	b.WriteString(m.renderTabs())
	b.WriteString(m.renderStatusLine())

	return b.String()
}

// viewportFor returns the [start,end) slice of a column to render, scrolled so
// the cursor is always on screen.
func (m *Model) viewportFor(ci, n, maxRows int) (int, int) {
	capacity := maxRows
	if capacity < 1 {
		capacity = 1
	}

	start := m.scrollIdx[ci]
	if max := n - capacity; start > max {
		start = max
	}
	if start < 0 {
		start = 0
	}
	if ci == m.colIdx {
		if m.rowIdx[ci] >= n {
			m.rowIdx[ci] = n - 1
		}
		if m.rowIdx[ci] < 0 {
			m.rowIdx[ci] = 0
		}
		if m.rowIdx[ci] < start {
			start = m.rowIdx[ci]
		}
		if m.rowIdx[ci] >= start+capacity {
			start = m.rowIdx[ci] - capacity + 1
		}
	}
	m.scrollIdx[ci] = start

	end := start + capacity
	if end > n {
		end = n
	}
	return start, end
}

// headerStyleFor picks the style for a column's label. The label follows focus
// only — a breached WIP limit colors the count, not the whole header.
func headerStyleFor(focused bool) lipgloss.Style {
	if focused {
		return headerFocused
	}
	return headerUnfocused
}

// overWIP reports whether a column has breached its limit.
func overWIP(status string, count int) bool {
	limit, ok := wipLimits[status]
	return ok && count > limit
}

// renderColHeader draws "LABEL n/limit  ↑a ↓b". Hidden-row counts live here
// rather than in a footer row: a dead row at the bottom of a column costs a
// task slot to say something that belongs with the count.
func (m *Model) renderColHeader(ci int, status string, tasks []event.OpenTask, maxRows, colWidth int) string {
	label := colHeaderLabel[status]
	count := len(tasks)

	style := headerStyleFor(ci == m.colIdx)

	// Two forms per header: the full one for the focused (wide) column, a
	// compact one for the narrow columns beside it. A count that does not fit
	// is a count the user never sees, so it degrades instead of vanishing.
	countText := fmt.Sprintf("%d", count)
	text := fmt.Sprintf("%s %d", label, count)
	short := text
	if limit, ok := wipLimits[status]; ok {
		countText = fmt.Sprintf("%d/%d", count, limit)
		text = fmt.Sprintf("%s %s", label, countText)
		short = text
	}
	if status == event.StatusDone {
		countText = fmt.Sprintf("%d", m.doneInWindow)
		text = fmt.Sprintf("%s %d this week", label, m.doneInWindow)
		short = fmt.Sprintf("%s %d/wk", label, m.doneInWindow)
	}

	var marks, shortMarks []string
	if len(tasks) > 0 {
		start, end := m.peekViewport(ci, len(tasks), maxRows)
		if start > 0 {
			marks = append(marks, fmt.Sprintf("↑%d", start))
			shortMarks = append(shortMarks, fmt.Sprintf("↑%d", start))
		}
		if status == event.StatusDone && m.doneTotal > m.doneInWindow {
			// Older done work is not "hidden below" — it is outside the window
			// on purpose. Say so instead of pretending it scrolls.
			older := m.doneTotal - m.doneInWindow
			marks = append(marks, fmt.Sprintf("%d older", older))
			shortMarks = append(shortMarks, fmt.Sprintf("+%d", older))
		}
		if below := len(tasks) - end; below > 0 {
			marks = append(marks, fmt.Sprintf("↓%d", below))
			shortMarks = append(shortMarks, fmt.Sprintf("↓%d", below))
		}
	}

	prefix := "  "
	if ci == m.colIdx {
		prefix = "▸ "
	}

	join := func(t string, mk []string) string {
		if len(mk) == 0 {
			return t
		}
		return t + " " + strings.Join(mk, " ")
	}

	// Only the count wears the alarm. A whole header in red leaves two of four
	// columns permanently lit, and a permanent alarm is wallpaper.
	render := func(s string) string {
		if !overWIP(status, count) {
			return style.Render(prefix + s)
		}
		i := strings.Index(s, countText)
		if i < 0 {
			return style.Render(prefix + s)
		}
		return style.Render(prefix+s[:i]) +
			countOverWIP.Render(countText) +
			style.Render(s[i+len(countText):])
	}

	for _, candidate := range []string{
		join(text, marks),
		join(short, shortMarks),
		join(short, nil),
		short,
	} {
		if len(prefix)+len([]rune(candidate)) <= colWidth {
			return render(candidate) + "\n"
		}
	}
	return style.Render(truncate(prefix+short, colWidth)) + "\n"
}

// peekViewport computes the viewport without mutating scroll state.
func (m *Model) peekViewport(ci, n, maxRows int) (int, int) {
	saved := m.scrollIdx[ci]
	start, end := m.viewportFor(ci, n, maxRows)
	m.scrollIdx[ci] = saved
	return start, end
}

func (m *Model) renderTaskRow(t event.OpenTask, status string, colWidth int, showAccent, selected, colFocused bool) string {
	// Accent bar: one character instead of an eight-character truncated tag.
	// The tab bar below already carries the project names, so the bar only has
	// to distinguish, not name.
	accent := ""
	accentW := 0
	if showAccent {
		accent = lipgloss.NewStyle().Foreground(badgeColor(t.Project)).Render("▌")
		accentW = 2 // bar + space
	}

	scopeMark := ""
	scopeW := 0
	if t.ScopeMoved {
		scopeMark = "~"
		scopeW = 2 // mark + space
	}

	age := ""
	ageW := 0
	if showAge(status) {
		if s := humanAge(t.StatusSince); s != "" {
			age = s
			ageW = len([]rune(s)) + 1
		}
	}

	titleWidth := colWidth - 2 - accentW - scopeW - ageW
	if titleWidth < 4 {
		titleWidth = 4
		ageW, age = 0, ""
	}
	title := truncate(t.Title, titleWidth)

	// Compose the plain text first so the selected band can span the full
	// column width regardless of the pieces inside it.
	var plain strings.Builder
	plain.WriteString("  ")
	if accent != "" {
		plain.WriteString("▌ ")
	}
	if scopeMark != "" {
		plain.WriteString(scopeMark + " ")
	}
	plain.WriteString(title)

	if selected {
		line := plain.String()
		if age != "" {
			pad := colWidth - visibleLen(line) - len([]rune(age))
			if pad < 1 {
				pad = 1
			}
			line += strings.Repeat(" ", pad) + age
		}
		if pad := colWidth - visibleLen(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		return rowSelected.Render(line) + "\n"
	}

	textStyle := rowNormal
	if !colFocused {
		textStyle = rowUnfocused
	}

	var b strings.Builder
	b.WriteString("  ")
	if accent != "" {
		b.WriteString(accent + " ")
	}
	if scopeMark != "" {
		b.WriteString(dimStyle.Render(scopeMark) + " ")
	}
	b.WriteString(textStyle.Render(title))
	if age != "" {
		pad := colWidth - visibleLen(b.String()) - len([]rune(age))
		if pad < 1 {
			pad = 1
		}
		b.WriteString(strings.Repeat(" ", pad) + ageStyleFor(t.StatusSince).Render(age))
	}
	b.WriteString("\n")
	return b.String()
}

// showAge reports whether age carries information for a column. In backlog it
// does not: everything there is old by definition.
func showAge(status string) bool {
	return status == event.StatusInProgress || status == event.StatusTesting
}

func humanAge(since int64) string {
	if since <= 0 {
		return ""
	}
	d := time.Since(time.Unix(since, 0))
	days := int(d.Hours() / 24)
	switch {
	case days >= 1:
		return fmt.Sprintf("%dd", days)
	case d.Hours() >= 1:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return ""
	}
}

func ageStyleFor(since int64) lipgloss.Style {
	days := int(time.Since(time.Unix(since, 0)).Hours() / 24)
	switch {
	case days >= ageStaleDays:
		return ageStale
	case days >= ageWarmDays:
		return ageWarm
	default:
		return ageFresh
	}
}

// renderPreview shows the selected task at full terminal width. Columns are
// narrow by construction; this is where the whole title is always readable.
func (m *Model) renderPreview() string {
	task, ok := m.selectedTask()
	if !ok {
		return faintStyle.Render("  (no task selected)") + "\n"
	}
	head := fmt.Sprintf("#%d ", task.ID)
	if task.Scope.Valid && task.Scope.String != "" {
		head += "[" + task.Scope.String + "] "
	}
	line := head + task.Title
	if m.width > 0 {
		line = truncate(line, m.width-2)
	}
	return "  " + lipgloss.NewStyle().Bold(true).Foreground(fg).Render(line) + "\n"
}

// renderTabs draws the project tab bar, windowed so it always fits the terminal
// and the active tab is never scrolled out of sight.
func (m *Model) renderTabs() string {
	label := func(i int) string {
		name := m.tabs[i]
		if i == 0 {
			return fmt.Sprintf(" All(%d) ", m.totalOpen())
		}
		if n, ok := m.tabCounts[name]; ok && n > 0 {
			return fmt.Sprintf(" %s(%d) ", name, n)
		}
		return " " + name + " "
	}

	// On the All tab every project tab also draws a one-glyph accent legend, so
	// its rendered width is one wider than its label.
	accentLegend := m.activeProject() == ""
	width := func(i int) int {
		w := visibleLen(label(i))
		if accentLegend && i > 0 {
			w++
		}
		return w
	}

	budget := m.width - 4 // room for the ‹ › markers
	if budget < 10 {
		budget = 10
	}

	lo, hi := m.activeTab, m.activeTab+1
	used := width(m.activeTab)
	for lo > 0 || hi < len(m.tabs) {
		grew := false
		if hi < len(m.tabs) {
			if w := width(hi); used+w <= budget {
				used += w
				hi++
				grew = true
			}
		}
		if lo > 0 {
			if w := width(lo - 1); used+w <= budget {
				used += w
				lo--
				grew = true
			}
		}
		if !grew {
			break
		}
	}

	var b strings.Builder
	if lo > 0 {
		b.WriteString(dimStyle.Render("‹"))
	}
	for i := lo; i < hi; i++ {
		style := tabInactive
		if i == m.activeTab {
			style = tabActive
		}
		// On the All tab each project also needs a legend for its accent bar,
		// but coloring the LABEL turned the bar into a row of competing text.
		// The color goes on a single glyph; the name stays neutral.
		if accentLegend && i > 0 {
			b.WriteString(lipgloss.NewStyle().Foreground(badgeColor(m.tabs[i])).Render(" ▌"))
			b.WriteString(style.Render(strings.TrimPrefix(label(i), " ")))
			continue
		}
		b.WriteString(style.Render(label(i)))
	}
	if hi < len(m.tabs) {
		b.WriteString(dimStyle.Render("›"))
	}
	b.WriteString("\n")
	return b.String()
}

// renderStatusLine is one line: filter input, error, flash, or a hint. The full
// keymap lives behind "?" so chrome does not eat two rows permanently.
func (m *Model) renderStatusLine() string {
	switch {
	case m.filtering:
		return "  /" + m.filter + "_\n"
	case m.err != nil:
		return errStyle.Render(truncate("Error: "+m.err.Error(), m.width)) + "\n"
	case m.flash != "":
		return flashStyle.Render(truncate("  "+m.flash, m.width)) + "\n"
	case m.filter != "":
		return dimStyle.Render(truncate(fmt.Sprintf("  filter: %s   (esc to clear)", m.filter), m.width)) + "\n"
	default:
		return helpStyle.Render(truncate("  ?: keys • /: filter • enter: detail • a: add • d: delete", m.width)) + "\n"
	}
}

func (m *Model) totalOpen() int {
	n := 0
	for _, c := range m.tabCounts {
		n += c
	}
	return n
}

func (m *Model) viewHelp() string {
	rows := [][2]string{
		{"h/l ←/→", "move between columns"},
		{"j/k ↑/↓", "move between tasks"},
		{"g/G", "first / last task in column"},
		{"H/L", "move the task left / right"},
		{"1-4", "send the task to a column"},
		{"enter/i", "open task detail"},
		{"/", "filter by text or #id (esc clears)"},
		{"a", "add a task"},
		{"d", "delete the task (asks first)"},
		{"n", "turn the task into a note"},
		{"tab/shift+tab", "switch project"},
		{"D", "delete the active project"},
		{"?", "close this help"},
		{"q", "quit"},
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("keys") + "\n")
	b.WriteString(faintStyle.Render(strings.Repeat("─", maxInt(1, m.width-2))) + "\n\n")
	for _, r := range rows {
		b.WriteString(fmt.Sprintf("  %s  %s\n",
			lipgloss.NewStyle().Bold(true).Foreground(fg).Width(14).Render(r[0]),
			dimStyle.Render(r[1])))
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  any key: back") + "\n")
	return b.String()
}

func (m *Model) viewAddTask() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("add task") + "\n")
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

	contentWidth := m.width - 4
	if contentWidth < 20 {
		contentWidth = 20
	}

	// Build the whole body as a line slice first, so scrolling is a slice
	// operation and nothing can spill past the terminal edge.
	var lines []string

	for _, l := range wrapText(fmt.Sprintf("#%d  %s", t.ID, t.Title), contentWidth) {
		lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(fg).Render(l))
	}
	lines = append(lines, "")

	statusLabel := colHeaderLabel[t.Status]
	if statusLabel == "" {
		statusLabel = t.Status
	}
	lines = append(lines, labelStyle.Render("project  ")+t.Project)
	if t.Scope.Valid && t.Scope.String != "" {
		scope := t.Scope.String
		if t.ScopeMoved {
			scope += dimStyle.Render("  (scope moved — see `kb scope " + t.Scope.String + "`)")
		}
		lines = append(lines, labelStyle.Render("scope    ")+scope)
	}
	lines = append(lines, labelStyle.Render("status   ")+statusLabel)
	lines = append(lines, labelStyle.Render("created  ")+formatTS(t.TS))
	if showAge(t.Status) {
		if age := humanAge(t.StatusSince); age != "" {
			lines = append(lines, labelStyle.Render("in column")+" "+ageStyleFor(t.StatusSince).Render(age))
		}
	}

	if t.Body.Valid && t.Body.String != "" {
		lines = append(lines, "")
		// Every paragraph is wrapped: bodies are pasted prose and long single
		// lines used to run straight off the right edge, unreadable.
		for _, para := range strings.Split(t.Body.String, "\n") {
			if strings.TrimSpace(para) == "" {
				lines = append(lines, "")
				continue
			}
			lines = append(lines, wrapText(para, contentWidth)...)
		}
	}

	if len(m.detailTimeline) > 0 {
		lines = append(lines, "")
		lines = append(lines, titleStyle.Render("timeline"))
		lines = append(lines, faintStyle.Render(strings.Repeat("─", minInt(40, contentWidth))))
		for _, e := range m.detailTimeline {
			lines = append(lines, dimStyle.Render(formatTS(e.TS))+"  "+e.Type)
		}
	}

	// Chrome: title (1) + rule (1) + blank (1) + blank (1) + help (1).
	viewport := m.height - 5
	if viewport < 3 {
		viewport = 3
	}
	maxScroll := len(lines) - viewport
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.detailScroll > maxScroll {
		m.detailScroll = maxScroll
	}
	end := m.detailScroll + viewport
	if end > len(lines) {
		end = len(lines)
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("task detail"))
	if maxScroll > 0 {
		b.WriteString(dimStyle.Render(fmt.Sprintf("   %d-%d/%d", m.detailScroll+1, end, len(lines))))
	}
	b.WriteString("\n")
	b.WriteString(faintStyle.Render(strings.Repeat("─", maxInt(1, m.width-2))) + "\n")
	b.WriteString("\n")
	for _, l := range lines[m.detailScroll:end] {
		b.WriteString("  " + l + "\n")
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render(truncate("esc: back • j/k: scroll • d: delete • q: quit", m.width)) + "\n")

	return b.String()
}

func (m *Model) viewConfirmDelete() string {
	if m.deleteTarget == nil {
		return "loading..."
	}
	t := m.deleteTarget
	contentWidth := m.width - 4
	if contentWidth < 20 {
		contentWidth = 20
	}

	var b strings.Builder
	b.WriteString(warnStyle.Render("delete task") + "\n")
	b.WriteString(faintStyle.Render(strings.Repeat("─", maxInt(1, m.width-2))) + "\n\n")

	for _, l := range wrapText(fmt.Sprintf("#%d  %s", t.ID, t.Title), contentWidth) {
		b.WriteString("  " + lipgloss.NewStyle().Bold(true).Foreground(fg).Render(l) + "\n")
	}
	b.WriteString("\n")
	b.WriteString("  " + dimStyle.Render("This deletes the task and its history. It cannot be undone.") + "\n\n")
	b.WriteString("  " + warnStyle.Render("y") + " delete permanently\n")
	b.WriteString("  " + flashStyle.Render("t") + " keep the content as a note instead\n")
	b.WriteString("  " + dimStyle.Render("n/esc") + " cancel\n")

	return b.String()
}

func (m *Model) viewConfirmDeleteProject() string {
	var b strings.Builder
	b.WriteString(warnStyle.Render("delete project") + "\n")
	b.WriteString(faintStyle.Render(strings.Repeat("─", maxInt(1, m.width-2))) + "\n\n")
	b.WriteString("  " + lipgloss.NewStyle().Bold(true).Foreground(fg).Render(m.deleteProject) + "\n\n")
	b.WriteString("  " + dimStyle.Render(fmt.Sprintf(
		"Unregisters the project and deletes all %d of its events — tasks, notes and history.",
		m.deleteProjectCount)) + "\n")
	b.WriteString("  " + dimStyle.Render("It cannot be undone.") + "\n\n")
	b.WriteString("  " + warnStyle.Render("y") + " delete everything\n")
	b.WriteString("  " + flashStyle.Render("u") + " only unregister (keep the events)\n")
	b.WriteString("  " + dimStyle.Render("n/esc") + " cancel\n")
	return b.String()
}

// computeFocusWidths splits totalWidth across numCols, giving the focused
// column roughly half.
//
// Equal columns mean every title is truncated everywhere. Titles here are
// semantic and long, so an even split guarantees the board cannot be read
// without opening each task — the exact cost a board is supposed to remove.
// Giving focus the room means the column you are reading is legible while the
// others stay as a preview.
//
// The sum NEVER exceeds totalWidth: fitting the terminal beats every other
// preference.
func computeFocusWidths(totalWidth, numCols, focusIdx, minWidth int) []int {
	if numCols <= 0 {
		return nil
	}
	widths := make([]int, numCols)
	if totalWidth < numCols {
		for i := range widths {
			widths[i] = 1
		}
		return widths
	}
	if focusIdx < 0 || focusIdx >= numCols {
		focusIdx = 0
	}

	even := totalWidth / numCols

	// Only widen focus when the others can still hold minWidth; on a narrow
	// terminal an even split is the honest layout.
	focusWidth := totalWidth / 2
	others := numCols - 1
	if others > 0 {
		rest := totalWidth - focusWidth
		if rest/others < minWidth {
			focusWidth = totalWidth - others*minWidth
		}
	}
	if focusWidth < even {
		focusWidth = even
	}
	if focusWidth > totalWidth-others {
		focusWidth = totalWidth - others
	}

	widths[focusIdx] = focusWidth
	remaining := totalWidth - focusWidth
	if others > 0 {
		base := remaining / others
		if base < 1 {
			base = 1
		}
		rem := remaining - base*others
		last := -1
		for i := 0; i < numCols; i++ {
			if i == focusIdx {
				continue
			}
			widths[i] = base
			last = i
		}
		if last >= 0 && rem > 0 {
			widths[last] += rem
		}
	}
	return widths
}

// joinColumnsAdaptive renders columns side by side with per-column widths.
func joinColumnsAdaptive(cols []string, widths []int, targetLines int) string {
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
			if visible > w {
				text = truncateStyled(text, w)
				visible = w
			}
			padding := w - visible
			if padding < 0 {
				padding = 0
			}
			b.WriteString(text)
			b.WriteString(strings.Repeat(" ", padding))

			if ci < len(splitCols)-1 {
				b.WriteString(faintStyle.Render("│"))
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

// truncateStyled cuts a string to max visible runes, keeping ANSI sequences
// intact and closing with a reset so styling never bleeds into the next column.
func truncateStyled(s string, max int) string {
	var b strings.Builder
	visible := 0
	inEsc := false
	sawEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			sawEsc = true
			b.WriteRune(r)
			continue
		}
		if inEsc {
			b.WriteRune(r)
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		if visible >= max {
			break
		}
		b.WriteRune(r)
		visible++
	}
	if sawEsc {
		b.WriteString("\x1b[0m")
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

// wrapText breaks s into lines of at most width visible characters, splitting on
// spaces and hard-breaking words that are longer than the width.
func wrapText(s string, width int) []string {
	if width < 1 {
		width = 1
	}
	var out []string
	for _, word := range strings.Fields(s) {
		runes := []rune(word)
		// Hard-break a word too long to ever fit (urls, paths, hashes).
		for len(runes) > width {
			out = append(out, string(runes[:width]))
			runes = runes[width:]
		}
		word = string(runes)
		if len(out) == 0 {
			out = append(out, word)
			continue
		}
		last := out[len(out)-1]
		if len([]rune(last))+1+len(runes) <= width {
			out[len(out)-1] = last + " " + word
		} else {
			out = append(out, word)
		}
	}
	if len(out) == 0 {
		return []string{""}
	}
	return out
}

// fuzzyMatch reports whether every rune of pattern appears in s in order.
// Case-insensitive, subsequence-based: "recq" matches "react-query".
func fuzzyMatch(s, pattern string) bool {
	if pattern == "" {
		return true
	}
	s = strings.ToLower(s)
	pattern = strings.ToLower(pattern)
	si := 0
	sr := []rune(s)
	for _, pr := range pattern {
		found := false
		for si < len(sr) {
			if sr[si] == pr {
				si++
				found = true
				break
			}
			si++
		}
		if !found {
			return false
		}
	}
	return true
}

func formatTS(ts int64) string {
	return time.Unix(ts, 0).Format("Jan 02 15:04")
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

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
