package cli

import (
	"fmt"
	"strings"

	"github.com/Ryong256/kanban/internal/db"
	"github.com/Ryong256/kanban/internal/event"
	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	var (
		project string
		scope   string
		body    string
	)
	cmd := &cobra.Command{
		Use:   "add <title...>",
		Short: "Add a new task (task.new event)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := db.Open()
			if err != nil {
				return err
			}
			defer d.Close()
			id, err := event.Add(d, event.Insert{
				Type:    event.TaskNew,
				Project: DetectAndRegisterProject(d, project),
				Scope:   scope,
				Title:   strings.Join(args, " "),
				Body:    body,
				Source:  "manual",
			})
			if err != nil {
				return err
			}
			fmt.Printf("added task #%d\n", id)
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "project name (auto-detected from cwd)")
	cmd.Flags().StringVarP(&scope, "scope", "s", "", "feature/change scope (optional)")
	cmd.Flags().StringVarP(&body, "body", "b", "", "longer description / reasoning")
	return cmd
}
