# Tasks: Cross-Host Task Reconciliation

## Review Workload Forecast

Actual: ~2164 changed lines — initial 700–950 estimate exceeded. Delivery: exception-ok; maintainer explicitly expanded `size:exception` to this candidate after the overrun. Single PR preserved; core → CLI → adapters work-unit boundaries remain for implementation/review organization.

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: size-exception
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|----|----------------------|-----------------|-------------------|
| 1 | Core: normalize, evidence, service, queries | 1 | `go test ./internal/reconcile/... ./internal/event/...` | `kb reconcile --dry-run --json` fixture DB | Revert `internal/reconcile/`+queries |
| 2 | CLI gates + JSON contract | 2 | `go test ./internal/cli/...` | `kb add/done/reconcile --json` `t.TempDir()` DB | Revert `internal/cli/*.go` |
| 3 | Adapters, schema, CONTRACT, Makefile | 3 | `bun test integrations` (planned; unit 3 sets up runner — none configured) | Session-close hook, `KB_HOOKS_DISABLED` toggle | Revert `integrations/`, `Makefile` |

## Phase 1: Core (RED)

- [x] 1.1 RED `reconcile/normalize_test.go`: case/whitespace/punctuation collapse; scope/project keep punctuation; distinct keys differ
- [x] 1.2 RED `reconcile/evidence_test.go`: `test-output`/`review-approved` validate; unknown rejected; pure
- [x] 1.3 RED `reconcile/service_test.go` (`db.OpenTest`): session-first; stale ≤50, `LastTS<now-168h`, `(LastTS,id)` asc; dry-run no appends; idempotent; `KB_RECONCILING=1` no-op; notes excluded; schema-v1 snapshot
- [x] 1.4 RED `event/queries_test.go`: `ListBySession`; bounded `ListStale`; metadata read; atomic/idempotent evidence update (SHA-256 attempt key)

## Phase 2: Core (GREEN + REFACTOR)

- [x] 2.1 GREEN `reconcile/normalize.go`: canonical `(project,scope,title,closure)` key
- [x] 2.2 GREEN `reconcile/evidence.go`: validator registry v1
- [x] 2.3 GREEN `reconcile/{service,model}.go`: selection, flags, idempotency, schema-v1 DTOs
- [x] 2.4 GREEN `event/{event.go,queries.go}`: metadata read, `ListBySession`/`ListStale`, transactional evidence update
- [x] 2.5 REFACTOR: dedupe helpers; `gofmt`; `go vet ./...`

## Phase 3: CLI Gates (RED → GREEN → REFACTOR)

- [x] 3.1 RED `cli/add_test.go`: missing `--closure`/`--evidence` rejected, no event; duplicate blocked, shows existing ID; `--session-id`/`--future` recorded; valid active, schema-v1 `meta_json`
- [x] 3.2 RED `cli/done_test.go`: missing `--evidence` rejected; valid appends `task.update`+`task.done` atomically; invalid→`completion-unverified`, stays open, repeat no-op; backlog→`in_progress` first; `testing` skippable if evidence valid
- [x] 3.3 RED `cli/list_test.go`: renders `completion-unverified` + `Nd stale`
- [x] 3.4 RED `cli/reconcile_test.go`: `--json` fields; `--dry-run` read-only; empty session owns nothing
- [x] 3.5 GREEN `cli/{add,done,list,reconcile,root}.go`; register `reconcile`
- [x] 3.6 REFACTOR: shared flag helpers, output parity

## Phase 4: Adapters + Contract (RED → GREEN → REFACTOR)

- [x] 4.1 RED `integrations/opencode/kanban.test.ts` (Bun): literal argv; `KB_HOOKS_DISABLED=1` skip; inherited `KB_RECONCILING=1` no-op; failure isolation; concurrent close
- [x] 4.2 RED threat-matrix doc-like paths: `requirements.txt`, `CMakeLists.txt`, executable `.md/.mdx`, `README.sh` rejected; only allowlisted `kanban.ts`/`kanban.sh` install/execute
- [x] 4.3 GREEN `opencode/kanban.ts`: thin invoker, core owns policy
- [x] 4.4 GREEN `claude/kanban.sh`: double-quoted vars, argv-only
- [x] 4.5 GREEN `shared/reconcile.schema.json`+`codex/CONTRACT.md`: versioned, no implementation
- [x] 4.6 GREEN `Makefile`: `install-opencode`/`install-claude` targets
- [x] 4.7 REFACTOR: share hook env-check; drop dead TS audit policy

## Phase 5: Rollout + Verification

- [x] 5.1 `go test ./...` → `bun test integrations` → `go build ./...` green
- [x] 5.2 Kill switch: `KB_HOOKS_DISABLED=1` disables both adapters; independent reverts; old metadata inert
- [x] 5.3 Confirm spec scenarios: creation gates, evidence closure, session-first+stale bound, thin adapters, recursion guard, idempotency, Codex contract
