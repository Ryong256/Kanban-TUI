package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Ryong256/kanban/internal/event"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var (
		project string
		all     bool
		limit   int
		search  string
	)
	cmd := &cobra.Command{
		Use:     "list [id]",
		Aliases: []string{"today", "ls"},
		Short:   "List open tasks (defaults to current project)",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var id int64
			if len(args) == 1 {
				var err error
				id, err = strconv.ParseInt(strings.TrimPrefix(args[0], "#"), 10, 64)
				if err != nil {
					return fmt.Errorf("invalid task id %q", args[0])
				}
			}
			d, closeDB, err := openDB()
			if err != nil {
				return err
			}
			defer closeDB()
			p := ""
			if !all {
				p = DetectProjectDB(d, project)
			}
			narrowed := id != 0 || search != ""
			// A lookup must see every open row, or the SQL limit could cut the
			// match off before the filter runs; the limit applies afterwards.
			fetch := limit
			if narrowed {
				fetch = 0
			}
			tasks, err := event.ListOpen(d, p, fetch)
			if err != nil {
				return err
			}
			if narrowed {
				tasks = findTasks(tasks, id, search)
				if id != 0 && len(tasks) == 0 {
					return fmt.Errorf("no open task #%d%s", id, projectSuffix(p))
				}
				if limit > 0 && len(tasks) > limit {
					tasks = tasks[:limit]
				}
			}
			if len(tasks) == 0 {
				if narrowed {
					outln(cmd, "no open tasks match")
				} else {
					outln(cmd, "no open tasks")
				}
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
	cmd.Flags().StringVarP(&search, "search", "s", "", "only tasks whose title or body contains this text")
	return cmd
}

// findTasks keeps the tasks matching id (when non-zero) and whose title or
// body contains search, case-insensitively (when non-empty).
func findTasks(tasks []event.OpenTask, id int64, search string) []event.OpenTask {
	needle := strings.ToLower(search)
	out := make([]event.OpenTask, 0, len(tasks))
	for _, t := range tasks {
		if id != 0 && t.ID != id {
			continue
		}
		if needle != "" &&
			!strings.Contains(strings.ToLower(t.Title), needle) &&
			!(t.Body.Valid && strings.Contains(strings.ToLower(t.Body.String), needle)) {
			continue
		}
		out = append(out, t)
	}
	return out
}

func projectSuffix(project string) string {
	if project == "" {
		return ""
	}
	return " in project " + project
}
