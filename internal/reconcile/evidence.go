package reconcile

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidEvidenceType reports whether t is a known evidence type.
func ValidEvidenceType(t string) bool {
	switch t {
	case "test-output", "review-approved":
		return true
	}
	return false
}

// Evidence is closure proof, so a test-output value must carry a recognizable
// passing verdict from a test runner. Substring matching on "pass"/"ok"/"fail"
// is not enough: it accepts "password" and "the build is broken", and it treats
// a failing run as proof that the work is done. These patterns anchor on the
// shapes real runners emit (go test, jest, bun test, vitest).
var (
	// Counts are extracted before failure scanning so "0 fail" — a passing bun
	// or jest summary — is not mistaken for a failure marker.
	failureCountRe = regexp.MustCompile(`(?i)\b(\d+)\s+(?:tests?\s+)?fail(?:ed|ing|ures?|s)?\b`)
	failureRe      = regexp.MustCompile(`(?i)\bFAIL\b|\bpanic:|\bbuild failed\b`)
	successRe      = regexp.MustCompile(`(?im)^\s*ok\s+\S|^\s*(?:-{3}\s+)?PASS\b|\b\d+\s+(?:tests?\s+)?pass(?:ed|ing)?\b|\ball tests passed\b`)
)

// ValidateEvidence checks that value satisfies the rules for the declared
// evidence_type. It is pure: no execution, no side effects.
func ValidateEvidence(evidenceType, value string) error {
	switch evidenceType {
	case "test-output":
		return validateTestOutput(value)
	case "review-approved":
		if value == "" {
			return fmt.Errorf("review-approved evidence must not be empty")
		}
		if !strings.HasPrefix(strings.ToLower(value), "approved-by:") {
			return fmt.Errorf("review-approved evidence must start with approved-by:")
		}
		return nil
	default:
		return fmt.Errorf("unknown evidence_type: %q", evidenceType)
	}
}

func validateTestOutput(value string) error {
	if value == "" {
		return fmt.Errorf("test-output evidence must not be empty")
	}

	scanned := value
	for _, m := range failureCountRe.FindAllStringSubmatch(value, -1) {
		if strings.Trim(m[1], "0") != "" {
			return fmt.Errorf("test-output evidence reports %s failing test(s)", m[1])
		}
	}
	scanned = failureCountRe.ReplaceAllString(scanned, " ")

	if failureRe.MatchString(scanned) {
		return fmt.Errorf("test-output evidence reports a failing run")
	}
	if !successRe.MatchString(value) {
		return fmt.Errorf("test-output evidence carries no passing test verdict (expected output such as %q, %q or %q)",
			"PASS", "ok  pkg 0.02s", "12 passed")
	}
	return nil
}
