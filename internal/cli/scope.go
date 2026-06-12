package cli

import (
	"fmt"
	"time"

	"github.com/Ryong256/kanban/internal/db"
	"github.com/Ryong256/kanban/internal/event"
	"github.com/spf13/cobra"
)

func newScopeCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "scope <scope-name>",
		Short: "Show timeline of events for a scope (how it evolved)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := db.Open()
			if err != nil {
				return err
			}
			defer d.Close()
			entries, err := event.ScopeTimeline(d, DetectProjectDB(d, project), args[0])
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				fmt.Println("no events for this scope")
				return nil
			}
			for _, e := range entries {
				when := time.Unix(e.TS, 0).Format("Jan 02 15:04")
				fmt.Printf("[%s] %-13s #%-4d  %s\n", when, e.Type, e.ID, e.Title)
				if e.Body.Valid && e.Body.String != "" {
					fmt.Printf("                                  %s\n", e.Body.String)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "project (auto-detected)")
	return cmd
}
