package cli

import (
	"fmt"

	"github.com/Ryong256/kanban/internal/db"
	"github.com/Ryong256/kanban/internal/store"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create the database and apply migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := db.Open()
			if err != nil {
				return err
			}
			defer d.Close()
			fmt.Printf("kb initialized at %s\n", store.DBPath())
			return nil
		},
	}
}
