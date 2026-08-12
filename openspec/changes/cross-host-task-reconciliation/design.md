# Design: Cross-Host Task Reconciliation

## Technical Approach

Keep policy in Go: Cobra commands call an `internal/reconcile` service over the existing append-only `events` table; OpenCode and Claude only detect host/session context, invoke `kb`, and return its versioned JSON. Reconciliation selects active tasks matching resolved project plus session first, then up to 50 other project tasks with `LastTS < now-168h`, ordered `(LastTS,id)` ascending. Notes remain outside task queries and lifecycle.

## Architecture Decisions

| Option | Tradeoff | Decision and rationale |
|---|---|---|
| New event types/columns vs `meta_json` | Columns are easier to query but require migration | Use versioned metadata on existing events; this preserves the proposal's no-migration, append-only boundary. |
| Semantic vs deterministic dedupe | Semantic matching catches more but creates false positives | Compare a canonical `(project, scope, title, closure)` key only. Lowercase all fields and collapse Unicode whitespace; for title/closure, map exactly ASCII `.,!?;:'"()[]{}-_` to spaces before collapsing. Scope/project preserve punctuation (especially paths). No Unicode punctuation removal, stemming, fuzzy matching, or NLP. |
| Adapter policy vs core service | Adapter policy duplicates behavior | Go owns selection, evidence validation, flags, idempotency, and schema; adapters only use literal argv. This also fixes a stable Codex boundary. |
| Free-form vs typed evidence | Free-form is flexible but unverifiable | A pure validator registry keyed by declared `evidence_type` (v1: `test-output`, `review-approved`) validates supplied text without executing it; unknown/invalid evidence appends one idempotent unverified update, never closes. |

## Data Flow

    Host close → thin adapter → kb reconcile → reconcile.Service → event queries
                              ← schema v1 JSON ← session first + stale(limit 50)
    kb add/done → normalize/validate → append task.new | task.update/task.done

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/reconcile/{service,model,normalize,evidence}.go` | Create | Host-neutral policy, canonical keys, validators, schema-v1 DTOs. |
| `internal/reconcile/*_test.go` | Create | Table-driven RED tests with `db.OpenTest`. |
| `internal/event/{event.go,queries.go}` | Modify | Read metadata; add `ListBySession`, bounded `ListStale`, atomic/idempotent evidence updates. |
| `internal/cli/{add,done,list,reconcile,root}.go` | Modify/Create | Add flags/output and register reconciliation. |
| `integrations/opencode/kanban.ts`, `kanban.test.ts` | Modify | Replace audit policy with bounded core invocation. |
| `integrations/claude/kanban.sh`, `integrations/shared/reconcile.schema.json`, `integrations/codex/CONTRACT.md` | Create | Quoted Claude hook, shared schema, future Codex contract. |
| `Makefile` | Modify | Explicit Claude/OpenCode installation targets. |

## Interfaces / Contracts

`kb add <title> --closure C --evidence TYPE [--session-id S] [--future]`; default status is `in_progress`, while `--future` selects `backlog`. `task.new.meta_json` is `{"schema_version":1,"closure_condition":C,"evidence_type":TYPE}` and ownership is immutable `session_id`.

`kb done ID --evidence VALUE` appends, in one transaction, `task.update` plus `task.done` carrying schema-v1 evidence. Invalid evidence appends at most one `task.update` with `flag:"completion-unverified"` and a SHA-256 attempt key; `kb list` prints that flag and `Nd stale`. Repeated valid completion or attempt is a no-op. Backlog tasks must first move to `in_progress`; `testing` is optional and may be skipped only when the declared evidence validates.

`kb reconcile [--project P] [--session-id S] [--stale-after 168h] [--stale-limit 50] [--dry-run] --json` returns `schema_version:1`, `project`, `session_id`, `dry_run`, `session_owned[]`, `stale_review[]`, and `errors[]`; task objects include identity, status, closure/evidence declarations, flags, and last-update/stale ages. V1 permits additive fields only; breaking changes require schema v2. Empty session IDs own nothing. `KB_RECONCILING=1` returns a versioned no-op without writes.

## Testing Strategy

Strict RED-GREEN-REFACTOR: table-driven normalization/evidence/idempotency tests; SQLite integration tests for session ownership, stale bounds, append-only history, notes excluded, JSON snapshots, and dry-run event counts; Bun adapter tests for argv, disabled hooks, recursion, failure isolation, and concurrent close. Run `go test ./...`, `bun test integrations`, then `go build ./...`.

## Threat Matrix

| Boundary | Applicability | Safe/failure behavior | Planned RED tests |
|---|---|---|---|
| Documentation-like paths | Applicable | Install/execute only allowlisted `kanban.ts` and `kanban.sh`; unknown or doc-like paths are ignored, never inferred executable. | `requirements.txt`, `CMakeLists.txt`, executable `.md/.mdx`, and `README.sh` each cannot be selected. |
| Git repository selection | N/A | No Git command or repository selector is introduced. | None. |
| Commit state | N/A | No commit/index operation. | None. |
| Push state | N/A | No push/ref resolution. | None. |
| PR commands | N/A | No PR command composition. | None. |

All subprocesses use argv arrays (no `eval`/composed shell), fixed commands, explicit cwd, ignored stdin where applicable, bounded output, and timeout. Claude variables are double-quoted. Failure returns a no-op/diagnostic and never blocks the host. Every callback checks `KB_HOOKS_DISABLED=1` and inherited `KB_RECONCILING=1` before spawning; the first invocation is unmarked, while any future core child process inherits the recursion marker.

## Migration / Rollout

No migration required. Ship core/schema first, then adapters; keep hooks opt-in and use `KB_HOOKS_DISABLED=1` as kill switch. Roll back adapters/CLI independently; old metadata remains inert and history is never rewritten.

## Open Questions

None.
