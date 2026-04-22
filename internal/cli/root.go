package cli

import (
	"github.com/spf13/cobra"
)

func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "kb",
		Short:         "Personal kanban — event-sourced task and scope tracker",
		SilenceUsage:  true,
		SilenceErrors: false,
	}
	root.AddCommand(
		newInitCmd(),
		newAddCmd(),
		newListCmd(),
		newDoneCmd(),
		newEventCmd(),
		newCountCmd(),
		newScopeCmd(),
	)
	return root
}
