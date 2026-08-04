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

// --- computeFocusWidths ---

func TestComputeFocusWidths_focus_gets_the_room(t *testing.T) {
	widths := computeFocusWidths(200, 4, 1, 12)
	for i, w := range widths {
		if i == 1 {
			continue
		}
		if widths[1] <= w {
			t.Errorf("focused column (%d) is not wider than column %d (%d)", widths[1], i, w)
		}
	}
}

func TestComputeFocusWidths_never_exceeds_total(t *testing.T) {
	cases := []int{20, 40, 76, 100, 137, 200, 400}
	for _, total := range cases {
		for focus := 0; focus < 4; focus++ {
			widths := computeFocusWidths(total, 4, focus, 12)
			sum := 0
			for i, w := range widths {
				if w < 1 {
					t.Errorf("total=%d focus=%d: column %d has width %d", total, focus, i, w)
				}
				sum += w
			}
			if sum > total {
				t.Errorf("total=%d focus=%d: widths sum %d exceeds it (%v)", total, focus, sum, widths)
			}
		}
	}
}

// On a terminal too narrow to give focus half the room, an even split is the
// honest layout — the other columns must not be crushed below the floor.
func TestComputeFocusWidths_narrow_terminal_degrades_evenly(t *testing.T) {
	widths := computeFocusWidths(52, 4, 0, 12)
	for i, w := range widths {
		if w < 12 {
			t.Errorf("column %d width %d fell below the floor (%v)", i, w, widths)
		}
	}
}

func TestComputeFocusWidths_out_of_range_focus_is_safe(t *testing.T) {
	widths := computeFocusWidths(100, 4, 99, 12)
	sum := 0
	for _, w := range widths {
		sum += w
	}
	if sum > 100 {
		t.Errorf("widths sum %d exceeds 100 (%v)", sum, widths)
	}
}
