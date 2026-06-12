package cli

import (
	"fmt"

	"github.com/Ryong256/kanban/internal/db"
	"github.com/Ryong256/kanban/internal/event"
	"github.com/spf13/cobra"
)

func newEventCmd() *cobra.Command {
	var (
		typ       string
		project   string
		scope     string
		title     string
		body      string
		refID     int64
		sessionID string
		source    string
		metaJSON  string
		status    string
	)
	cmd := &cobra.Command{
		Use:   "event",
		Short: "Append a generic event (used by hooks)",
		Long: `Append a generic event to the log. Used by Claude Code hooks
(Stop, PostToolUse) for automated capture. Supported types:
  task.new, task.done, task.update, scope.shift, scope.expand`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !event.ValidType(typ) {
				return fmt.Errorf("invalid --type: %q", typ)
			}
			if title == "" {
				return fmt.Errorf("--title is required")
			}
			d, err := db.Open()
			if err != nil {
				return err
			}
			defer d.Close()
			id, err := event.Add(d, event.Insert{
				Type:      event.Type(typ),
				Project:   DetectAndRegisterProject(d, project),
				Scope:     scope,
				Title:     title,
				Body:      body,
				RefID:     refID,
				SessionID: sessionID,
				Source:    source,
				MetaJSON:  metaJSON,
				Status:    status,
			})
			if err != nil {
				return err
			}
			fmt.Printf("event #%d (%s)\n", id, typ)
			return nil
		},
	}
	cmd.Flags().StringVar(&typ, "type", "", "event type (required)")
	cmd.Flags().StringVarP(&project, "project", "p", "", "project (auto-detected)")
	cmd.Flags().StringVarP(&scope, "scope", "s", "", "scope/feature name")
	cmd.Flags().StringVarP(&title, "title", "t", "", "short title (required)")
	cmd.Flags().StringVarP(&body, "body", "b", "", "full body / reasoning")
	cmd.Flags().Int64Var(&refID, "ref", 0, "id of related event (for done/update)")
	cmd.Flags().StringVar(&sessionID, "session", "", "Claude Code session id")
	cmd.Flags().StringVar(&source, "source", "agent", "source: manual | hook-stop | hook-post | agent")
	cmd.Flags().StringVar(&metaJSON, "meta", "", "extra JSON metadata")
	cmd.Flags().StringVar(&status, "status", "", "task status: backlog|in_progress|testing|complete|done")
	_ = cmd.MarkFlagRequired("type")
	_ = cmd.MarkFlagRequired("title")
	return cmd
}
