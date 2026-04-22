package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Ryong256/kanban/internal/db"
	"github.com/Ryong256/kanban/internal/event"
	"github.com/spf13/cobra"
)

func newMoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "move <id> <status>",
		Short: "Move a task to a new status",
		Long: fmt.Sprintf("Move a task between kanban columns.\nValid statuses: %s",
			strings.Join(event.AllStatuses(), ", ")),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid task id: %q", args[0])
			}
			status := args[1]
			if !event.ValidStatus(status) {
				return fmt.Errorf("invalid status %q — valid: %s",
					status, strings.Join(event.AllStatuses(), ", "))
			}
			d, err := db.Open()
			if err != nil {
				return err
			}
			defer d.Close()
			eventID, err := event.MoveTask(d, id, status, "manual")
			if err != nil {
				return err
			}
			fmt.Printf("task #%d → %s (event #%d)\n", id, status, eventID)
			return nil
		},
	}
}
