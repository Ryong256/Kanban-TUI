package cli

import (
	"fmt"
	"strconv"

	"github.com/Ryong256/kanban/internal/db"
	"github.com/Ryong256/kanban/internal/event"
	"github.com/spf13/cobra"
)

func newDoneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "done <task-id>",
		Short: "Mark a task as done",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid task id: %w", err)
			}
			d, err := db.Open()
			if err != nil {
				return err
			}
			defer d.Close()
			doneID, err := event.MarkDone(d, id)
			if err != nil {
				return err
			}
			fmt.Printf("task #%d marked done (event #%d)\n", id, doneID)
			return nil
		},
	}
}
