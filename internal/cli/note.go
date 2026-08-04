package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/Ryong256/kanban/internal/db"
	"github.com/Ryong256/kanban/internal/event"
	"github.com/spf13/cobra"
)

func newNoteCmd() *cobra.Command {
	var (
		project string
		scope   string
		body    string
		source  string
	)
	cmd := &cobra.Command{
		Use:   "note <title...>",
		Short: "Record something that happened (note event, never hits the board)",
		Long: `Record a standalone log entry.

Use a note when the thing is already finished and nobody has to act on it:
a PR that shipped, a root cause you confirmed, a decision you made. Notes have
no status and never appear in ` + "`kb list`" + ` or the board, so they cannot pile up
as stale backlog.

Use ` + "`kb add`" + ` instead when there is future work with a done condition.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := db.Open()
			if err != nil {
				return err
			}
			defer d.Close()
			id, err := event.Add(d, event.Insert{
				Type:    event.Note,
				Project: DetectAndRegisterProject(d, project),
				Scope:   scope,
				Title:   strings.Join(args, " "),
				Body:    body,
				Source:  source,
			})
			if err != nil {
				return err
			}
			fmt.Printf("note #%d\n", id)
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "project name (auto-detected from cwd)")
	cmd.Flags().StringVarP(&scope, "scope", "s", "", "feature/change scope (optional)")
	cmd.Flags().StringVarP(&body, "body", "b", "", "longer description / reasoning")
	cmd.Flags().StringVar(&source, "source", "manual", "source: manual | hook-stop | hook-post | agent")
	return cmd
}

func newNotesCmd() *cobra.Command {
	var (
		project string
		all     bool
		limit   int
	)
	cmd := &cobra.Command{
		Use:   "notes",
		Short: "List recent notes (defaults to current project)",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := db.Open()
			if err != nil {
				return err
			}
			defer d.Close()
			p := ""
			if !all {
				p = DetectProjectDB(d, project)
			}
			notes, err := event.ListNotes(d, p, limit)
			if err != nil {
				return err
			}
			if len(notes) == 0 {
				fmt.Println("no notes")
				return nil
			}
			for _, n := range notes {
				when := time.Unix(n.TS, 0).Format("Jan 02 15:04")
				scope := ""
				if n.Scope.Valid && n.Scope.String != "" {
					scope = " [" + n.Scope.String + "]"
				}
				proj := ""
				if all {
					proj = " (" + n.Project + ")"
				}
				fmt.Printf("#%-4d  %s%s%s  %s\n", n.ID, when, proj, scope, n.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "project name (auto-detected)")
	cmd.Flags().BoolVarP(&all, "all", "a", false, "list across all projects")
	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "max rows (0 = no limit)")
	return cmd
}
