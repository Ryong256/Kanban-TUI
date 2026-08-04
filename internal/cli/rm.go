package cli

import (
	"fmt"
	"strconv"

	"github.com/Ryong256/kanban/internal/db"
	"github.com/Ryong256/kanban/internal/event"
	"github.com/spf13/cobra"
)

func parseIDs(args []string) ([]int64, error) {
	ids := make([]int64, 0, len(args))
	for _, a := range args {
		id, err := strconv.ParseInt(a, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid id %q", a)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func newRmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "rm <id...>",
		Aliases: []string{"delete"},
		Short:   "Delete a task or note permanently",
		Long: `Delete a task or note and every event referencing it.

Use this for rows that were never real work. Closing them with ` + "`kb done`" + ` would
record a completion that never happened; deleting says what actually occurred.

To keep the content but get it off the board, use ` + "`kb demote`" + ` instead.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ids, err := parseIDs(args)
			if err != nil {
				return err
			}
			d, err := db.Open()
			if err != nil {
				return err
			}
			defer d.Close()
			for _, id := range ids {
				if err := event.DeleteTask(d, id); err != nil {
					return err
				}
				fmt.Printf("deleted #%d\n", id)
			}
			return nil
		},
	}
	return cmd
}

func newDemoteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "demote <id...>",
		Aliases: []string{"to-note"},
		Short:   "Reclassify a task as a note (keeps content, leaves the board)",
		Long: `Turn a task into a note.

The title, body and scope survive; the status and the task's status history are
dropped. Use this for backlog rows that are really log entries — "shipped X",
"X is archived" — which can never be closed because there is nothing to do.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ids, err := parseIDs(args)
			if err != nil {
				return err
			}
			d, err := db.Open()
			if err != nil {
				return err
			}
			defer d.Close()
			for _, id := range ids {
				if err := event.ConvertTaskToNote(d, id); err != nil {
					return err
				}
				fmt.Printf("#%d is now a note\n", id)
			}
			return nil
		},
	}
	return cmd
}
