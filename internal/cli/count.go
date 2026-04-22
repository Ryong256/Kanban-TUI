package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Ryong256/kanban/internal/db"
	"github.com/Ryong256/kanban/internal/event"
	"github.com/spf13/cobra"
)

func newCountCmd() *cobra.Command {
	var (
		project string
		all     bool
		jsonOut bool
	)
	cmd := &cobra.Command{
		Use:   "count",
		Short: "Print open task count (waybar-friendly)",
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
			n, err := event.CountOpen(d, p)
			if err != nil {
				return err
			}
			if !jsonOut {
				fmt.Println(n)
				return nil
			}
			top, err := event.ListOpen(d, p, 3)
			if err != nil {
				return err
			}
			lines := make([]string, 0, len(top))
			for _, t := range top {
				lines = append(lines, "• "+t.Title)
			}
			tooltip := "no open tasks"
			if len(lines) > 0 {
				tooltip = strings.Join(lines, "\n")
			}
			out := map[string]any{
				"text":    fmt.Sprintf("%d", n),
				"alt":     fmt.Sprintf("%d", n),
				"tooltip": tooltip,
				"class":   classFor(n),
			}
			b, err := json.Marshal(out)
			if err != nil {
				return err
			}
			fmt.Println(string(b))
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "project (auto-detected)")
	cmd.Flags().BoolVarP(&all, "all", "a", false, "count across all projects")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "waybar JSON output")
	return cmd
}

func classFor(n int) string {
	switch {
	case n == 0:
		return "empty"
	case n < 5:
		return "low"
	case n < 10:
		return "medium"
	default:
		return "high"
	}
}
