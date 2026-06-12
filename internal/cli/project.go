package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"database/sql"

	"github.com/Ryong256/kanban/internal/db"
	"github.com/Ryong256/kanban/internal/project"
	"github.com/spf13/cobra"
)

// DetectProject resolves a project name from (in order):
//  1. explicit --project override
//  2. KB_PROJECT env var
//  3. project registry (longest matching path prefix of cwd)
//  4. .git root directory name — only auto-registers when a .git root is found
//
// Unlike the old behaviour, this function never auto-registers paths that have
// no .git root (e.g. /home/alfredo). Callers that already hold an open *sql.DB
// should use DetectProjectDB to avoid opening a second connection.
func DetectProject(override string) string {
	if override != "" {
		return override
	}
	d, err := db.Open()
	if err != nil {
		if env := os.Getenv("KB_PROJECT"); env != "" {
			return env
		}
		return detectFromFS()
	}
	defer d.Close()
	return DetectProjectDB(d, "")
}

// DetectProjectDB resolves the project name WITHOUT registering it.
// Resolution order: override → KB_PROJECT env → registry lookup → .git basename.
// Read commands (list, count, view, scope) use this so they never mutate the
// registry. Returns the .git basename even when the project is unregistered, so
// read commands still filter correctly for repos that have tasks but no registry
// row.
func DetectProjectDB(d *sql.DB, override string) string {
	if override != "" {
		return override
	}
	if env := os.Getenv("KB_PROJECT"); env != "" {
		return env
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	// Try registry lookup first (fastest path).
	if p, found, err := project.FindByPath(d, cwd); err == nil && found {
		return p.Name
	}
	// Walk up looking for a .git root — return basename without registering.
	gitRoot, found := findGitRoot(cwd)
	if !found {
		return ""
	}
	return filepath.Base(gitRoot)
}

// DetectAndRegisterProject resolves the project name and auto-registers the repo
// in the project registry when a .git root is found and the project is not yet
// registered. Write commands (add, event) use this so newly-seen repos appear in
// the TUI tab list after the first task is created.
func DetectAndRegisterProject(d *sql.DB, override string) string {
	if override != "" {
		return override
	}
	if env := os.Getenv("KB_PROJECT"); env != "" {
		return env
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	// Try registry lookup first (fastest path).
	if p, found, err := project.FindByPath(d, cwd); err == nil && found {
		return p.Name
	}
	// Walk up looking for a .git root.
	gitRoot, found := findGitRoot(cwd)
	if !found {
		return ""
	}
	name := filepath.Base(gitRoot)
	// Auto-register the project so it shows up in TUI tabs.
	project.Ensure(d, name, gitRoot)
	return name
}

// ResolveProjectReadOnly resolves a project for the given dir without
// auto-registering anything. Used by the detect-project subcommand.
func ResolveProjectReadOnly(d *sql.DB, dir string) string {
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return ""
		}
	}
	if p, found, err := project.FindByPath(d, dir); err == nil && found {
		return p.Name
	}
	gitRoot, found := findGitRoot(dir)
	if !found {
		return ""
	}
	return filepath.Base(gitRoot)
}

// detectFromFS is a last-resort fallback when no DB is available.
// It only returns a name when a .git root is found; never auto-registers.
func detectFromFS() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	gitRoot, found := findGitRoot(cwd)
	if !found {
		return ""
	}
	return filepath.Base(gitRoot)
}

// findGitRoot walks up from dir until it finds a directory containing .git.
// Returns (path, true) when found, ("", false) when none.
func findGitRoot(dir string) (string, bool) {
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "project",
		Aliases: []string{"p"},
		Short:   "Manage registered projects",
	}
	cmd.AddCommand(
		newProjectAddCmd(),
		newProjectListCmd(),
		newProjectRmCmd(),
	)
	return cmd
}

func newProjectAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name> [path]",
		Short: "Register a project (defaults to current directory)",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			dir := ""
			if len(args) > 1 {
				dir = args[1]
			} else {
				var err error
				dir, err = os.Getwd()
				if err != nil {
					return err
				}
			}
			abs, err := filepath.Abs(dir)
			if err != nil {
				return err
			}
			// Verify path exists
			info, err := os.Stat(abs)
			if err != nil {
				return fmt.Errorf("path %q does not exist", abs)
			}
			if !info.IsDir() {
				return fmt.Errorf("path %q is not a directory", abs)
			}
			d, err := db.Open()
			if err != nil {
				return err
			}
			defer d.Close()
			if err := project.Add(d, name, abs); err != nil {
				return err
			}
			fmt.Printf("registered project %q → %s\n", name, abs)
			return nil
		},
	}
}

func newProjectListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List registered projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := db.Open()
			if err != nil {
				return err
			}
			defer d.Close()
			projects, err := project.List(d)
			if err != nil {
				return err
			}
			if len(projects) == 0 {
				fmt.Println("no registered projects — use: kb project add <name> [path]")
				return nil
			}
			for _, p := range projects {
				when := time.Unix(p.CreatedTS, 0).Format("Jan 02")
				fmt.Printf("%-20s  %s  %s\n", p.Name, when, p.Path)
			}
			return nil
		},
	}
}

func newProjectRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "rm <name>",
		Aliases: []string{"remove"},
		Short:   "Unregister a project",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := db.Open()
			if err != nil {
				return err
			}
			defer d.Close()
			if err := project.Remove(d, args[0]); err != nil {
				return err
			}
			fmt.Printf("removed project %q\n", args[0])
			return nil
		},
	}
}

// newDetectProjectCmd adds the `kb detect-project [path]` subcommand.
// It resolves the project name read-only (no auto-registration) and prints it
// to stdout (name only, no decoration). Empty output + exit 0 when no project
// is found — this is intentional for script consumers.
func newDetectProjectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "detect-project [path]",
		Short: "Resolve the project name for a path (read-only, no auto-registration)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := ""
			if len(args) > 0 {
				abs, err := filepath.Abs(args[0])
				if err != nil {
					return err
				}
				dir = abs
			}
			d, err := db.Open()
			if err != nil {
				// DB unavailable — fall back to pure FS lookup.
				if dir == "" {
					var cwdErr error
					dir, cwdErr = os.Getwd()
					if cwdErr != nil {
						return nil // exit 0, empty output
					}
				}
				if root, found := findGitRoot(dir); found {
					fmt.Println(filepath.Base(root))
				}
				return nil
			}
			defer d.Close()
			name := ResolveProjectReadOnly(d, dir)
			if name != "" {
				fmt.Println(name)
			}
			return nil
		},
	}
}
