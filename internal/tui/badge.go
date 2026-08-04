package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// doneLimit is the maximum number of done tasks shown in the DONE column.
// The column header always shows the true total; a dimmed "+N older" footer
// appears when the list is truncated.
const doneLimit = 15

// doneWindow bounds the DONE column to recent work. An all-time done column is
// a log: it fills a quarter of the screen with rows that inform no decision.
const doneWindow = 7 * 24 * time.Hour

// badgePalette is a fixed set of ANSI-256 colors that are visually distinct and
// read well on dark backgrounds.
//
// Deliberately mid-tone, not bright. These colors encode identity — which
// project a row belongs to — which is the lowest-priority thing on the board.
// Bright hues here compete with the alarm and focus colors, and when eight
// saturated bars run down a column nothing stands out at all.
var badgePalette = []lipgloss.Color{
	"67",  // steel blue
	"173", // muted orange
	"139", // dusty purple
	"108", // sage
	"168", // muted rose
	"73",  // teal
	"179", // tan
	"96",  // plum
}

// badgeColor returns a deterministic lipgloss color for a project name by
// hashing the name to an index in badgePalette.  Same name → same color always.
func badgeColor(name string) lipgloss.Color {
	h := uint32(0)
	for _, r := range name {
		h = h*31 + uint32(r)
	}
	return badgePalette[h%uint32(len(badgePalette))]
}

// badgeAbbrev produces a short label for a project name suitable for display in
// a narrow kanban column.  It uses the first path segment (split on "-" or "/")
// and truncates to at most 6 visible characters, appending "-" when truncated.
//
// Examples:
//
//	"kanban"                  → "kanban"
//	"laya-recruiting-poc"     → "laya-"
//	"my-very-long-project"    → "my-"
//	"shortname"               → "shortn"
func badgeAbbrev(name string) string {
	if name == "" {
		return ""
	}
	// Take first segment split on "-" or "/".
	seg := name
	if idx := strings.IndexAny(name, "-/"); idx > 0 {
		seg = name[:idx+1] // include the separator so it reads as "laya-"
	}
	// Cap at 6 visible chars.
	const maxBadgeLen = 6
	runes := []rune(seg)
	if len(runes) > maxBadgeLen {
		runes = runes[:maxBadgeLen]
		seg = string(runes)
	}
	return strings.ToLower(seg)
}

// projectBadge renders a colored, bracketed badge for project name, e.g. "[laya-]".
// Returns an empty string when name is empty (no badge needed).
func projectBadge(name string) string {
	if name == "" {
		return ""
	}
	abbrev := badgeAbbrev(name)
	style := lipgloss.NewStyle().Foreground(badgeColor(name))
	return style.Render("[" + abbrev + "]")
}
