package reconcile

import (
	"strings"
	"unicode"
)

// TaskKey is the canonical identity used for strict duplicate detection.
type TaskKey struct {
	Project string
	Scope   string
	Title   string
	Closure string
}

// NormalizeKey returns a deterministic string key for duplicate detection.
// Project and scope preserve punctuation; title and closure collapse ASCII
// punctuation to spaces. All fields are lowercased and Unicode whitespace is
// collapsed.
func NormalizeKey(k TaskKey) string {
	return strings.Join([]string{
		normalizeProjectScope(k.Project),
		normalizeProjectScope(k.Scope),
		normalizeTitleClosure(k.Title),
		normalizeTitleClosure(k.Closure),
	}, "\x00")
}

func normalizeProjectScope(s string) string {
	return collapseSpace(strings.ToLower(s))
}

func normalizeTitleClosure(s string) string {
	var b strings.Builder
	for _, r := range s {
		if isAsciiPunct(r) {
			b.WriteRune(' ')
			continue
		}
		b.WriteRune(r)
	}
	return collapseSpace(strings.ToLower(b.String()))
}

func isAsciiPunct(r rune) bool {
	return strings.ContainsRune(".,!?;:'\"()[]{}-_", r)
}

func collapseSpace(s string) string {
	var b strings.Builder
	inSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			inSpace = true
			continue
		}
		if inSpace && b.Len() > 0 {
			b.WriteRune(' ')
		}
		inSpace = false
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}
