package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Ryong256/kanban/internal/event"
	"github.com/Ryong256/kanban/internal/reconcile"
	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	var (
		project   string
		scope     string
		body      string
		closure   string
		evidence  string
		sessionID string
		future    bool
	)
	cmd := &cobra.Command{
		Use:   "add <title...>",
		Short: "Add a new task (task.new event)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if closure == "" || evidence == "" {
				return fmt.Errorf("--closure and --evidence are required")
			}
			if !reconcile.ValidEvidenceType(evidence) {
				return fmt.Errorf("unknown evidence type: %q", evidence)
			}

			d, closeDB, err := openDB()
			if err != nil {
				return err
			}
			defer closeDB()

			proj := DetectAndRegisterProject(d, project)
			title := strings.Join(args, " ")

			existing, err := findDuplicate(d, proj, scope, title, closure)
			if err != nil {
				return err
			}
			if existing != 0 {
				return fmt.Errorf("duplicate of task #%d", existing)
			}

			meta, _ := json.Marshal(reconcile.Meta{
				SchemaVersion:    1,
				ClosureCondition: closure,
				EvidenceType:     evidence,
			})

			status := event.StatusInProgress
			if future {
				status = event.StatusBacklog
			}

			id, err := event.Add(d, event.Insert{
				Type:      event.TaskNew,
				Project:   proj,
				Scope:     scope,
				Title:     title,
				Body:      body,
				SessionID: sessionID,
				Source:    "manual",
				MetaJSON:  string(meta),
				Status:    status,
			})
			if err != nil {
				return err
			}
			outf(cmd, "added task #%d\n", id)
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "project name (auto-detected from cwd)")
	cmd.Flags().StringVarP(&scope, "scope", "s", "", "feature/change scope (optional)")
	cmd.Flags().StringVarP(&body, "body", "b", "", "longer description / reasoning")
	cmd.Flags().StringVar(&closure, "closure", "", "condition that closes this task")
	cmd.Flags().StringVar(&evidence, "evidence", "", "evidence type (test-output, review-approved)")
	cmd.Flags().StringVar(&sessionID, "session-id", "", "owning session id")
	cmd.Flags().BoolVar(&future, "future", false, "place in backlog instead of in_progress")
	return cmd
}

func findDuplicate(d dbQuerier, project, scope, title, closure string) (int64, error) {
	// meta_json is NULL on every task created before schema-v1 metadata existed.
	// Those rows still have to be scanned, so the COALESCE is not cosmetic: a
	// single legacy task would otherwise break `kb add` for the whole database.
	rows, err := d.Query(`
		SELECT t.id, t.project, COALESCE(t.scope, ''), t.title, COALESCE(base.meta_json, '')
		FROM   v_task_latest t
		JOIN   events base ON base.id = t.id
		WHERE  t.status != 'done' AND t.project = ?
	`, project)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	target := reconcile.NormalizeKey(reconcile.TaskKey{
		Project: project,
		Scope:   scope,
		Title:   title,
		Closure: closure,
	})

	for rows.Next() {
		var id int64
		var proj, sc, tl, meta string
		if err := rows.Scan(&id, &proj, &sc, &tl, &meta); err != nil {
			return 0, err
		}
		m, _ := reconcile.ParseMeta(meta)
		if reconcile.NormalizeKey(reconcile.TaskKey{
			Project: proj,
			Scope:   sc,
			Title:   tl,
			Closure: m.ClosureCondition,
		}) == target {
			return id, nil
		}
	}
	return 0, rows.Err()
}
