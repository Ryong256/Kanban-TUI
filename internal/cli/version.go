package cli

import (
	"fmt"

	"github.com/Ryong256/kanban/internal/buildinfo"
	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the installed kb version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("kb %s  built %s\n", buildinfo.Version, buildinfo.Date)
			if buildinfo.SourceDir != "" {
				fmt.Printf("source %s\n", buildinfo.SourceDir)
			}
		},
	}
}
