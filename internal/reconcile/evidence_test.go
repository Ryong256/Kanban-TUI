package reconcile

import "testing"

func TestValidateEvidence_known_types(t *testing.T) {
	tests := []struct {
		name    string
		typ     string
		value   string
		wantErr bool
	}{
		{"go summary pass", "test-output", "PASS ./...", false},
		{"go per-package ok", "test-output", "ok  \tgithub.com/x/y\t0.02s", false},
		{"go verbose pass", "test-output", "--- PASS: TestThing (0.00s)", false},
		{"jest style counts", "test-output", "Tests: 12 passed, 12 total", false},
		{"bun style counts", "test-output", "12 pass, 0 fail", false},

		// A failing run is not proof of closure. It is the opposite.
		{"go summary fail", "test-output", "FAIL ./...", true},
		{"go verbose fail", "test-output", "--- FAIL: TestThing (0.00s)", true},
		{"nonzero failure count", "test-output", "9 pass, 3 fail", true},
		{"panic", "test-output", "panic: runtime error: index out of range", true},

		// Prose that merely contains pass/ok/fail as substrings proves nothing.
		{"substring pass", "test-output", "password", true},
		{"substring ok", "test-output", "the build is broken", true},
		{"substring ok in prose", "test-output", "looks bad", true},
		{"bare ok", "test-output", "ok", true},
		{"manual check", "test-output", "manual check", true},
		{"empty value for test-output", "test-output", "", true},

		{"review-approved", "review-approved", "approved-by:alice", false},
		{"review-approved missing prefix", "review-approved", "alice", true},
		{"empty value for review-approved", "review-approved", "", true},

		{"unknown type", "screenshot", "x", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEvidence(tt.typ, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEvidence(%q, %q) error = %v, wantErr %v", tt.typ, tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateEvidence_failure_anywhere_in_multiline_output(t *testing.T) {
	// A run that passes one package and fails another is a failing run.
	out := "ok  \tgithub.com/x/y\t0.02s\nFAIL\tgithub.com/x/z\t0.10s\n"
	if err := ValidateEvidence("test-output", out); err == nil {
		t.Fatal("mixed pass/fail output accepted as closure evidence")
	}
}

func TestValidateEvidence_is_pure(t *testing.T) {
	// Pure: same input -> same output, no side effects.
	if ValidateEvidence("test-output", "PASS ./...") != nil {
		t.Fatal("first call failed")
	}
	if ValidateEvidence("test-output", "PASS ./...") != nil {
		t.Fatal("second call failed")
	}
}
