package reconcile

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/Ryong256/kanban/internal/event"
)

// Service hosts the provider-neutral reconciliation policy.
type Service struct {
	db *sql.DB
}

// NewService creates a reconciliation service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// NoOpResult is the schema-v1 answer for a run that was suppressed before it
// began. Suppression is an environment decision, so the caller makes it; the
// service stays free of process state.
func NoOpResult(cfg Config) *Result {
	res := emptyResult(cfg)
	res.NoOp = true
	return res
}

func emptyResult(cfg Config) *Result {
	return &Result{
		SchemaVersion: 1,
		Project:       cfg.Project,
		SessionID:     cfg.SessionID,
		DryRun:        cfg.DryRun,
		SessionOwned:  []TaskSnapshot{},
		StaleReview:   []TaskSnapshot{},
		Errors:        []string{},
	}
}

// Reconcile returns session-owned tasks first, then bounded project-wide stale
// tasks. It never appends events; --dry-run is effectively always true at the
// core level because selection is read-only.
func (s *Service) Reconcile(cfg Config) (*Result, error) {
	if cfg.StaleAfter == 0 {
		cfg.StaleAfter = 7 * 24 * 60 * 60
	}
	if cfg.StaleLimit == 0 {
		cfg.StaleLimit = 50
	}

	res := emptyResult(cfg)

	if cfg.Project == "" {
		res.Errors = append(res.Errors, "project is required")
		return res, nil
	}

	owned, err := event.ListBySession(s.db, cfg.Project, cfg.SessionID)
	if err != nil {
		res.Errors = append(res.Errors, fmt.Sprintf("list session tasks: %v", err))
		return res, nil
	}
	res.SessionOwned = s.toSnapshots(owned, cfg.Now, cfg.StaleAfter)

	ownedIDs := make(map[int64]bool, len(owned))
	for _, t := range owned {
		ownedIDs[t.ID] = true
	}

	stale, err := event.ListStale(s.db, cfg.Project, cfg.Now, cfg.StaleAfter, cfg.StaleLimit)
	if err != nil {
		res.Errors = append(res.Errors, fmt.Sprintf("list stale tasks: %v", err))
		return res, nil
	}
	var filtered []event.ReconcileTask
	for _, t := range stale {
		if !ownedIDs[t.ID] {
			filtered = append(filtered, t)
		}
	}
	res.StaleReview = s.toSnapshots(filtered, cfg.Now, cfg.StaleAfter)

	return res, nil
}

func (s *Service) toSnapshots(tasks []event.ReconcileTask, now, staleAfter int64) []TaskSnapshot {
	out := make([]TaskSnapshot, 0, len(tasks))
	for _, t := range tasks {
		meta, _ := event.ReadTaskMeta(s.db, t.ID)
		var flags []string
		if t.Flag != "" {
			flags = append(flags, t.Flag)
		}
		snap := TaskSnapshot{
			ID:               t.ID,
			Project:          t.Project,
			Scope:            t.Scope,
			Title:            t.Title,
			Status:           t.Status,
			ClosureCondition: meta.ClosureCondition,
			EvidenceType:     meta.EvidenceType,
			EvidenceValue:    t.EvidenceValue,
			SessionID:        t.SessionID,
			Flags:            flags,
			LastTS:           t.LastTS,
		}
		if now-t.LastTS > staleAfter {
			snap.StaleAgeHours = (now - t.LastTS) / 3600
		}
		out = append(out, snap)
	}
	return out
}

// ParseMeta decodes schema-v1 metadata from JSON.
func ParseMeta(raw string) (Meta, error) {
	var m Meta
	if raw == "" {
		return m, nil
	}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return m, err
	}
	return m, nil
}
