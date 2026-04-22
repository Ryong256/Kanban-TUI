package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Ryong256/kanban/internal/db"
	"github.com/Ryong256/kanban/internal/project"
	"github.com/spf13/cobra"
)

// DetectProject resolves a project name from (in order):
// 1. explicit --project override
// 2. KB_PROJECT env var
// 3. project registry (longest matching path)
// 4. git root directory name
// 5. cwd basename
func DetectProject(override string) string {
	if override != "" {
		return override
	}
	if env := os.Getenv("KB_PROJECT"); env != "" {
		return env
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "unknown"
	}
	// Try registry lookup
	if d, err := db.Open(); err == nil {
		defer d.Close()
		if p, found, err := project.FindByPath(d, cwd); err == nil && found {
			return p.Name
		}
	}
	// Fallback: walk up to .git
	dir := cwd
	name := filepath.Base(cwd)
	rootDir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			name = filepath.Base(dir)
			rootDir = dir
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	// Auto-register the project so it shows up in tabs
	if d, err := db.Open(); err == nil {
		defer d.Close()
		project.Ensure(d, name, rootDir)
	}
	return name
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
