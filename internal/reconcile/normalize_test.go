package reconcile

import (
	"testing"
)

func TestNormalizeKey_cases_and_whitespace(t *testing.T) {
	tests := []struct {
		name string
		a, b TaskKey
		want bool
	}{
		{
			name: "exact match",
			a:    TaskKey{Project: "proj", Scope: "pkg/a", Title: "Fix Auth", Closure: "tests pass"},
			b:    TaskKey{Project: "proj", Scope: "pkg/a", Title: "Fix Auth", Closure: "tests pass"},
			want: true,
		},
		{
			name: "case variant",
			a:    TaskKey{Project: "proj", Scope: "pkg/a", Title: "Fix Auth", Closure: "tests pass"},
			b:    TaskKey{Project: "proj", Scope: "pkg/a", Title: "fix auth", Closure: "Tests Pass"},
			want: true,
		},
		{
			name: "whitespace variant",
			a:    TaskKey{Project: "proj", Scope: "pkg/a", Title: "Fix Auth", Closure: "tests pass"},
			b:    TaskKey{Project: "proj", Scope: "pkg/a", Title: "  Fix   Auth ", Closure: "tests  pass"},
			want: true,
		},
		{
			name: "punctuation variant",
			a:    TaskKey{Project: "proj", Scope: "pkg/a", Title: "Fix auth", Closure: "tests pass"},
			b:    TaskKey{Project: "proj", Scope: "pkg/a", Title: "Fix, auth.", Closure: "tests pass!"},
			want: true,
		},
		{
			name: "different title",
			a:    TaskKey{Project: "proj", Scope: "pkg/a", Title: "Fix Auth", Closure: "tests pass"},
			b:    TaskKey{Project: "proj", Scope: "pkg/a", Title: "Add Auth", Closure: "tests pass"},
			want: false,
		},
		{
			name: "different scope preserves punctuation",
			a:    TaskKey{Project: "proj", Scope: "pkg/a", Title: "Fix Auth", Closure: "tests pass"},
			b:    TaskKey{Project: "proj", Scope: "pkg/b", Title: "Fix Auth", Closure: "tests pass"},
			want: false,
		},
		{
			name: "different project preserves punctuation",
			a:    TaskKey{Project: "proj-a", Scope: "pkg/a", Title: "Fix Auth", Closure: "tests pass"},
			b:    TaskKey{Project: "proj.a", Scope: "pkg/a", Title: "Fix Auth", Closure: "tests pass"},
			want: false,
		},
		{
			name: "different closure",
			a:    TaskKey{Project: "proj", Scope: "pkg/a", Title: "Fix Auth", Closure: "tests pass"},
			b:    TaskKey{Project: "proj", Scope: "pkg/a", Title: "Fix Auth", Closure: "coverage > 80%"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeKey(tt.a) == NormalizeKey(tt.b)
			if got != tt.want {
				t.Errorf("NormalizeKey(%+v) == NormalizeKey(%+v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestNormalizeKey_scope_and_project_keep_punctuation(t *testing.T) {
	// Scope and project are compared verbatim except for case/whitespace.
	a := TaskKey{Project: "my.proj", Scope: "pkg/a-b", Title: "x", Closure: "y"}
	b := TaskKey{Project: "my proj", Scope: "pkg/a b", Title: "x", Closure: "y"}
	if NormalizeKey(a) == NormalizeKey(b) {
		t.Error("project/scope punctuation must be preserved")
	}
}

func TestNormalizeKey_unicode_whitespace(t *testing.T) {
	a := TaskKey{Project: "p", Scope: "s", Title: "fix\u00A0auth", Closure: "tests pass"}
	b := TaskKey{Project: "p", Scope: "s", Title: "fix auth", Closure: "tests pass"}
	if NormalizeKey(a) != NormalizeKey(b) {
		t.Error("unicode whitespace should collapse")
	}
}
