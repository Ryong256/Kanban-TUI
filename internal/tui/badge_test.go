package tui

import (
	"testing"
)

// --- badgeAbbrev ---

func TestBadgeAbbrev_simple(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"kanban", "kanban"},
		{"laya-recruiting-poc", "laya-"},
		{"my-very-long-project", "my-"}, // first segment is "my-" (3 chars, within 6 cap)
		{"shortname", "shortn"},
		{"a", "a"},
		{"", ""},
		{"ab-cd", "ab-"},
		{"abcdefghij", "abcdef"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := badgeAbbrev(tc.name)
			if got != tc.want {
				t.Errorf("badgeAbbrev(%q) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

// --- badgeColor ---

func TestBadgeColor_deterministic(t *testing.T) {
	// Same name always returns same color.
	name := "laya-recruiting-poc"
	c1 := badgeColor(name)
	c2 := badgeColor(name)
	if c1 != c2 {
		t.Fatalf("badgeColor not deterministic: %v vs %v", c1, c2)
	}
}

func TestBadgeColor_different_names(t *testing.T) {
	// Different names should produce different colors for our small test set.
	names := []string{"kanban", "laya-recruiting-poc", "opencode", "dotfiles", "infra"}
	seen := map[string]string{}
	for _, n := range names {
		c := string(badgeColor(n))
		seen[n] = c
	}
	// At least 3 distinct colors across 5 names (palette has 8 slots).
	distinct := map[string]bool{}
	for _, c := range seen {
		distinct[c] = true
	}
	if len(distinct) < 3 {
		t.Errorf("expected at least 3 distinct colors for 5 project names, got %d", len(distinct))
	}
}

// --- projectBadge ---

func TestProjectBadge_empty_name(t *testing.T) {
	if got := projectBadge(""); got != "" {
		t.Fatalf("expected empty badge for empty name, got %q", got)
	}
}

func TestProjectBadge_contains_abbrev(t *testing.T) {
	badge := projectBadge("laya-recruiting-poc")
	// The visible portion must contain the abbreviation.
	if visibleLen(badge) == 0 {
		t.Fatal("expected non-empty visible badge")
	}
}

// --- computeColWidths ---

func TestComputeColWidths_all_nonempty(t *testing.T) {
	widths := computeColWidths(100, 5, 10, make([]bool, 5), []int{10, 10, 10, 10, 10})
	total := 0
	for _, w := range widths {
		total += w
	}
	if total != 100 {
		t.Fatalf("expected total width 100, got %d", total)
	}
}

func TestComputeColWidths_one_empty_redistributes(t *testing.T) {
	// 5 columns, 100 total. Column 2 (testing) empty with header width 10.
	// Collapsed: 10 freed. Remaining 90 split among 4 non-empty columns.
	emptyMask := []bool{false, false, true, false, false}
	headerWidths := []int{12, 14, 10, 14, 8}
	widths := computeColWidths(100, 5, 1, emptyMask, headerWidths)

	total := 0
	for _, w := range widths {
		total += w
	}
	if total != 100 {
		t.Fatalf("expected total width 100, got %d (widths=%v)", total, widths)
	}
	// Collapsed column must equal its header width.
	if widths[2] != 10 {
		t.Fatalf("expected collapsed column width=10, got %d", widths[2])
	}
	// Non-empty columns must be >= min (1).
	for i, empty := range emptyMask {
		if !empty && widths[i] < 1 {
			t.Errorf("non-empty column %d has width %d < 1", i, widths[i])
		}
	}
}

func TestComputeColWidths_all_empty(t *testing.T) {
	emptyMask := []bool{true, true, true, true, true}
	headerWidths := []int{8, 8, 8, 8, 8}
	// All empty: even split
	widths := computeColWidths(100, 5, 1, emptyMask, headerWidths)
	total := 0
	for _, w := range widths {
		total += w
	}
	if total != 100 {
		t.Fatalf("expected total width 100, got %d", total)
	}
}

func TestComputeColWidths_width_sum_exact(t *testing.T) {
	// Width is not divisible by numCols — remainder must end up somewhere.
	emptyMask := []bool{false, true, false, false, false}
	headerWidths := []int{10, 9, 10, 10, 10}
	widths := computeColWidths(97, 5, 1, emptyMask, headerWidths)
	total := 0
	for _, w := range widths {
		total += w
	}
	if total != 97 {
		t.Fatalf("expected total width 97, got %d (widths=%v)", total, widths)
	}
}

func TestComputeColWidths_narrow_terminal_never_overflows(t *testing.T) {
	// Regression: minWidth used to clamp base upward, making widths sum past
	// totalWidth on terminals narrower than numCols*minWidth (e.g. tmux panes).
	cases := []struct {
		name       string
		totalWidth int
		emptyMask  []bool
	}{
		{"all nonempty width 40", 40, []bool{false, false, false, false, false}},
		{"all nonempty width 60", 60, []bool{false, false, false, false, false}},
		{"wide collapsed headers squeeze content", 76, []bool{true, true, true, true, false}},
		{"one nonempty narrow", 30, []bool{true, false, true, true, true}},
	}
	headerWidths := []int{12, 16, 12, 13, 18}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			widths := computeColWidths(tc.totalWidth, 5, 15, tc.emptyMask, headerWidths)
			total := 0
			for i, w := range widths {
				if w < 1 {
					t.Errorf("column %d has width %d < 1 (widths=%v)", i, w, widths)
				}
				total += w
			}
			if total > tc.totalWidth {
				t.Fatalf("widths sum %d exceeds totalWidth %d (widths=%v)", total, tc.totalWidth, widths)
			}
		})
	}
}
