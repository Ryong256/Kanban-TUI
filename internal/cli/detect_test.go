package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Ryong256/kanban/internal/cli"
	"github.com/Ryong256/kanban/internal/db"
	"github.com/Ryong256/kanban/internal/project"
)

func TestDetectProjectDB_override(t *testing.T) {
	d := db.OpenTest(t)
	got := cli.DetectProjectDB(d, "explicit")
	if got != "explicit" {
		t.Fatalf("expected %q, got %q", "explicit", got)
	}
}

func TestDetectProjectDB_envvar(t *testing.T) {
	d := db.OpenTest(t)
	t.Setenv("KB_PROJECT", "from-env")
	got := cli.DetectProjectDB(d, "")
	if got != "from-env" {
		t.Fatalf("expected %q, got %q", "from-env", got)
	}
}

func TestDetectProjectDB_registry_lookup(t *testing.T) {
	d := db.OpenTest(t)

	tmp := t.TempDir()
	repoDir := filepath.Join(tmp, "myrepo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	project.Ensure(d, "myrepo", repoDir)

	// Simulate cwd = repoDir/src by chdir-ing in the test.
	subDir := filepath.Join(repoDir, "src")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) }) //nolint
	if err := os.Chdir(subDir); err != nil {
		t.Fatal(err)
	}

	got := cli.DetectProjectDB(d, "")
	if got != "myrepo" {
		t.Fatalf("expected %q, got %q", "myrepo", got)
	}
}

func TestDetectProjectDB_git_root_found_no_register(t *testing.T) {
	d := db.OpenTest(t)

	tmp := t.TempDir()
	repoDir := filepath.Join(tmp, "gitrepo")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) }) //nolint
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}

	got := cli.DetectProjectDB(d, "")
	if got != "gitrepo" {
		t.Fatalf("expected %q, got %q", "gitrepo", got)
	}

	// DetectProjectDB is read-only — must NOT auto-register.
	list, _ := project.List(d)
	if len(list) != 0 {
		t.Fatalf("DetectProjectDB must not auto-register, got %v", list)
	}
}

func TestDetectAndRegisterProject_git_root_registers(t *testing.T) {
	d := db.OpenTest(t)

	tmp := t.TempDir()
	repoDir := filepath.Join(tmp, "gitrepo-rw")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) }) //nolint
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}

	got := cli.DetectAndRegisterProject(d, "")
	if got != "gitrepo-rw" {
		t.Fatalf("expected %q, got %q", "gitrepo-rw", got)
	}

	// Write path must auto-register.
	list, _ := project.List(d)
	if len(list) != 1 || list[0].Name != "gitrepo-rw" {
		t.Fatalf("expected project gitrepo-rw to be auto-registered, got %v", list)
	}
}

func TestDetectProjectDB_no_git_no_register(t *testing.T) {
	d := db.OpenTest(t)

	// A plain directory with no .git ancestor should produce "" and NOT
	// auto-register anything.
	tmp := t.TempDir()
	// TempDir is under /tmp which should not have a .git root.
	plainDir := filepath.Join(tmp, "nongit")
	if err := os.MkdirAll(plainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) }) //nolint
	if err := os.Chdir(plainDir); err != nil {
		t.Fatal(err)
	}

	got := cli.DetectProjectDB(d, "")
	if got != "" {
		t.Fatalf("expected empty string for dir without .git, got %q", got)
	}

	list, _ := project.List(d)
	if len(list) != 0 {
		t.Fatalf("expected no auto-registration, got %v", list)
	}
}

func TestDetectProjectDB_git_in_parent(t *testing.T) {
	d := db.OpenTest(t)

	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "parentrepo")
	subPkg := filepath.Join(repoRoot, "pkg", "sub")
	if err := os.MkdirAll(subPkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) }) //nolint
	if err := os.Chdir(subPkg); err != nil {
		t.Fatal(err)
	}

	got := cli.DetectProjectDB(d, "")
	if got != "parentrepo" {
		t.Fatalf("expected %q from parent .git, got %q", "parentrepo", got)
	}
}

func TestResolveProjectReadOnly_no_register(t *testing.T) {
	d := db.OpenTest(t)

	tmp := t.TempDir()
	repoDir := filepath.Join(tmp, "readonly-repo")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := cli.ResolveProjectReadOnly(d, repoDir)
	if got != "readonly-repo" {
		t.Fatalf("expected %q, got %q", "readonly-repo", got)
	}

	// Must NOT auto-register.
	list, _ := project.List(d)
	if len(list) != 0 {
		t.Fatalf("ResolveProjectReadOnly must not register, got %v", list)
	}
}
