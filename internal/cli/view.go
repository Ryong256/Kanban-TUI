package cli

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/Ryong256/kanban/internal/db"
	"github.com/Ryong256/kanban/internal/tui"
	"github.com/spf13/cobra"
)

func newViewCmd() *cobra.Command {
	var (
		project string
		all     bool
	)
	cmd := &cobra.Command{
		Use:     "view",
		Aliases: []string{"v", "tui"},
		Short:   "Interactive kanban board TUI",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := db.Open()
			if err != nil {
				return err
			}
			defer d.Close()

			p := ""
			if !all {
				p = DetectProject(project)
			}

			m := tui.NewModel(d, p)
			program := tea.NewProgram(m, tea.WithAltScreen())
			_, err = program.Run()
			return err
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "project name (auto-detected)")
	cmd.Flags().BoolVarP(&all, "all", "a", false, "show tasks across all projects")
	return cmd
}
