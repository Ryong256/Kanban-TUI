# Proposal: Cross-Host Task Reconciliation

## Intent

AI hosts create duplicate tasks and leave finished work open: `kb add` has no duplicate gate; closure is a one-shot prompt without evidence or session-end passes. Make creation and closure verifiable, evidence-gated, and provider-neutral.

## Scope

### In Scope
- `kb reconcile` (`--project/--session-id/--dry-run/--json`): session-owned first, then bounded project-wide stale review at close.
- Creation gates: verifiable tasks declaring closure condition + evidence, in a real lifecycle.
- Strict duplicate blocking on identical title/scope/closure condition; no semantic dedupe.
- Current work → active; future work MAY stay backlog.
- Closure only with task-declared evidence; unproved stays open, flagged; required transitions honored, inapplicable states MAY be skipped with evidence.
- Thin OpenCode (refactor) + Claude Code (new) adapters; Codex contract documented.
- `kb add --session-id`; `ListStale`/`ListBySession`; staleness in `kb list`.

### Out of Scope
- Codex adapter implementation (prepared contract only).
- Broad semantic/NLP duplicate detection.
- TUI rendering changes; automatic backlog pruning.
- New event types (existing suffice; new ones require numbered append-only migrations).

## Capabilities

### New Capabilities
- `task-creation-control`: gates, strict duplicate blocking, placement.
- `task-closure-evidence`: evidence-proved closure, flagged unproved completion, lifecycle rules.
- `session-reconciliation`: session-priority pass, bounded project-wide review, staleness.
- `host-integration-protocol`: provider-neutral core, thin adapters, Codex contract.

### Modified Capabilities
None (no existing specs).

## Approach

Exploration Approach 3: reconciliation logic in the Go core; adapters become ~30-line invokers. Strict-equivalence dedupe is a simple lookup (not the deferred NLP similarity), so creation control ships here per approved decisions. Host-neutral core; note-vs-task separation unchanged; recursion blocked via env markers.

## Affected Areas

- `internal/cli/reconcile.go` — New: reconcile command
- `internal/cli/{add,list,root}.go` — Modified: session-id, gates, duplicate block, staleness
- `internal/event/queries.go` — Modified: stale/session queries
- `integrations/opencode/kanban.ts` (+test) — Modified: thin-adapter refactor
- `integrations/{claude,shared}/`, `Makefile` — New/Modified: Claude adapter, contract docs, install

## Risks

- JSON schema instability (Med) — versioned schema
- Duplicate false positives (Med) — strict equivalence only
- Hook shell quoting (Low) — quoted templates, tested
- Session-ID spoofing (Low) — advisory metadata, not auth
- Review budget exceed (Med) — chained PRs: core then adapters

## Rollback Plan

`KB_HOOKS_DISABLED=1` disables adapters instantly. `kb reconcile` is additive: remove it and reinstall prior adapters. No migrations, no DB rollback.

## Dependencies

- None beyond the existing stack.

## Success Criteria

- [ ] `kb reconcile --json`: stable schema, session-owned priority, bounded stale set.
- [ ] `kb add` blocks strict duplicates and tasks without closure condition/evidence.
- [ ] Closure only with declared evidence; unproved stays open, flagged.
- [ ] Both adapters reconcile at session close; note/task separation intact.
- [ ] `go test ./...` + `go build ./...` green.
