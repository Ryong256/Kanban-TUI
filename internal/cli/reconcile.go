package cli

import (
	"encoding/json"
	"os"
	"time"

	"github.com/Ryong256/kanban/internal/reconcile"
	"github.com/spf13/cobra"
)

func newReconcileCmd() *cobra.Command {
	var (
		project    string
		sessionID  string
		staleAfter int64
		staleLimit int
		dryRun     bool
		jsonOut    bool
	)
	cmd := &cobra.Command{
		Use:   "reconcile",
		Short: "Session-first reconciliation with bounded stale review",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, closeDB, err := openDB()
			if err != nil {
				return err
			}
			defer closeDB()

			cfg := reconcile.Config{
				Project:    project,
				SessionID:  sessionID,
				StaleAfter: staleAfter,
				StaleLimit: staleLimit,
				DryRun:     dryRun,
				Now:        time.Now().Unix(),
			}

			// An adapter invokes kb; kb must not invoke an adapter back. The marker
			// goes on *this* process so anything kb spawns inherits it — marking the
			// spawned process from the adapter would suppress the first run instead
			// of the recursive one.
			if os.Getenv(envReconciling) == "1" {
				return emit(cmd, reconcile.NoOpResult(cfg), jsonOut)
			}
			if err := os.Setenv(envReconciling, "1"); err != nil {
				return err
			}

			cfg.Project = DetectProjectDB(d, project)
			res, err := reconcile.NewService(d).Reconcile(cfg)
			if err != nil {
				return err
			}

			return emit(cmd, res, jsonOut)
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "project name (auto-detected)")
	cmd.Flags().StringVar(&sessionID, "session-id", "", "session to prioritize")
	cmd.Flags().Int64Var(&staleAfter, "stale-after", 0, "stale threshold in seconds (default 7 days)")
	cmd.Flags().IntVar(&staleLimit, "stale-limit", 0, "max stale tasks (default 50)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report only")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output schema-v1 JSON")
	return cmd
}

func emit(cmd *cobra.Command, res *reconcile.Result, jsonOut bool) error {
	if jsonOut {
		b, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			return err
		}
		outln(cmd, string(b))
		return nil
	}

	if res.NoOp {
		outln(cmd, "no-op: already reconciling")
		return nil
	}
	outf(cmd, "project: %s\nsession: %s\ndry_run: %v\n", res.Project, res.SessionID, res.DryRun)
	outf(cmd, "session_owned: %d\n", len(res.SessionOwned))
	for _, t := range res.SessionOwned {
		outf(cmd, "  #%d %s [%s]\n", t.ID, t.Title, t.Status)
	}
	outf(cmd, "stale_review: %d\n", len(res.StaleReview))
	for _, t := range res.StaleReview {
		outf(cmd, "  #%d %s [%s] %dh stale\n", t.ID, t.Title, t.Status, t.StaleAgeHours)
	}
	for _, e := range res.Errors {
		outf(cmd, "error: %s\n", e)
	}
	return nil
}
