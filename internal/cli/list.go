package cli

import (
	"fmt"
	"time"

	"github.com/Ryong256/kanban/internal/event"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var (
		project string
		all     bool
		limit   int
	)
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"today", "ls"},
		Short:   "List open tasks (defaults to current project)",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, closeDB, err := openDB()
			if err != nil {
				return err
			}
			defer closeDB()
			p := ""
			if !all {
				p = DetectProjectDB(d, project)
			}
			tasks, err := event.ListOpen(d, p, limit)
			if err != nil {
				return err
			}
			if len(tasks) == 0 {
				outln(cmd, "no open tasks")
				return nil
			}
			now := time.Now().Unix()
			threshold := 7 * 24 * 60 * 60
			for _, t := range tasks {
				when := time.Unix(t.TS, 0).Format("Jan 02 15:04")
				scope := ""
				if t.Scope.Valid && t.Scope.String != "" {
					scope = " [" + t.Scope.String + "]"
				}
				proj := ""
				if all {
					proj = " (" + t.Project + ")"
				}
				status := ""
				if t.Status != "" && t.Status != event.StatusBacklog {
					status = " <" + t.Status + ">"
				}
				stale := ""
				if now-t.LastTS > int64(threshold) {
					stale = fmt.Sprintf(" %dd stale", (now-t.LastTS)/(24*60*60))
				}
				flag := ""
				if t.Flag != "" {
					flag = " !" + t.Flag
				}
				outf(cmd, "#%-4d  %s%s%s%s%s%s  %s\n", t.ID, when, proj, scope, status, flag, stale, t.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "project name (auto-detected)")
	cmd.Flags().BoolVarP(&all, "all", "a", false, "list across all projects")
	cmd.Flags().IntVarP(&limit, "limit", "n", 0, "max rows (0 = no limit)")
	return cmd
}
