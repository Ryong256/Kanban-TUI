package cli

import (
	"fmt"
	"strconv"

	"github.com/Ryong256/kanban/internal/event"
	"github.com/Ryong256/kanban/internal/reconcile"
	"github.com/spf13/cobra"
)

func newDoneCmd() *cobra.Command {
	var evidence string
	cmd := &cobra.Command{
		Use:   "done <task-id>",
		Short: "Mark a task as done with evidence",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid task id: %w", err)
			}
			d, closeDB, err := openDB()
			if err != nil {
				return err
			}
			defer closeDB()

			meta, err := event.ReadTaskMeta(d, id)
			if err != nil {
				return err
			}

			// A task that declared no evidence type has nothing to validate
			// against. Every task filed before declarations existed is in that
			// state, and refusing to close them would strand the whole board.
			if meta.EvidenceType == "" {
				if evidence != "" {
					return fmt.Errorf("task #%d declared no evidence type; re-file it with --closure and --evidence to require proof", id)
				}
				doneID, err := event.MarkDone(d, id)
				if err != nil {
					return err
				}
				outf(cmd, "task #%d marked done (event #%d, no evidence declared)\n", id, doneID)
				return nil
			}

			if evidence == "" {
				return fmt.Errorf("task #%d declares evidence_type %q: --evidence is required", id, meta.EvidenceType)
			}

			updateID, doneID, err := event.ApplyEvidence(d, id, meta.EvidenceType, evidence, "manual", reconcile.ValidateEvidence)
			if err != nil {
				return err
			}
			switch {
			case doneID != 0:
				outf(cmd, "task #%d marked done (event #%d)\n", id, doneID)
			case updateID != 0:
				outf(cmd, "task #%d stays open, flagged %s (event #%d)\n", id, event.FlagCompletionUnverified, updateID)
			default:
				outf(cmd, "task #%d unchanged (same evidence already recorded)\n", id)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&evidence, "evidence", "", "evidence satisfying the task's declared evidence_type")
	return cmd
}
