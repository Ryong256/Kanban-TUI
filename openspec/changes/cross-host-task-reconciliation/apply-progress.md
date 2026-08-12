# Apply Progress: Cross-Host Task Reconciliation

## Current Remediation

**What**: Scoped remediation of the six admitted critical defects from the failed verify report (`verdict: fail`, `evidence_revision: sha256:dbbce48744baf4144038553c8730fa528d6f173b1e4615c290a9e3b6e282ecd4`).

**Why**: The native attempt authorized exactly this work unit with a 200 changed-line cap; warnings, refactors, and product-scope changes were explicitly out of scope.

**Where**:
- `internal/event/queries.go` — empty-session guard, configurable `staleAfter`, valid evidence payload.
- `internal/reconcile/service.go` — pass `StaleAfter` into `ListStale`.
- `integrations/opencode/kanban.ts` — do not set `KB_RECONCILING=1` on first spawn; return reconcile output to host.
- `integrations/claude/kanban.sh` — remove `export KB_RECONCILING=1` before first invocation.
- `integrations/opencode/kanban.test.ts` — production-spawn regression test for marker/output.
- `integrations/opencode/threat.test.ts` — assertion-bearing threat coverage.
- `integrations/claude/harness.sh` — executable runtime harness for argv safety and unmarked first invocation.

**Learned**:
- `bun.spawn` uses `cwd` literally; a missing directory makes the spawn fail silently in the adapter, so production-spawn tests must use a real temp dir.
- The existing OpenCode failure-isolation test reassigned `runKb` after plugin construction, so it never exercised the override; test-first remediation exposed the brittle mock wiring.

## Remediation TDD Cycle Evidence

| Defect | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 1. Adapter marker suppresses first reconcile | `integrations/opencode/kanban.test.ts` | Bun / process | ✅ 12/12 | ✅ production spawn saw `no_op:true` | ✅ passed | ✅ argv + no-no-op cases | ✅ compact fake kb |
| 2. OpenCode discards reconcile output | `integrations/opencode/kanban.test.ts` | Bun / process | ✅ 12/12 | ✅ event returned `undefined` | ✅ passed | ✅ same test asserts returned JSON | ✅ `return await ...` |
| 3. Valid evidence missing from `task.done` | `internal/event/queries_test.go` | Go integration | ✅ all prior pass | ✅ `meta_json` empty | ✅ passed | ✅ asserts `"value":"PASS ./..."` | ✅ `evidenceMetaJSON` helper |
| 4. `--stale-after` ignored | `internal/event/queries_test.go` | Go integration | ✅ all prior pass | ✅ 2-day task excluded with `stale-after=1d` | ✅ passed | ✅ default 7-day path still passes | ✅ `staleAfter` parameter |
| 5. Empty session owns all unowned tasks | `internal/event/queries_test.go` | Go integration | ✅ all prior pass | ✅ returned unowned task | ✅ passed | ✅ owned session path still passes | ✅ early return for `""` |
| 6. Adapter/threat tests bypass production spawn / ghost loop | `integrations/opencode/{kanban,threat}.test.ts` | Bun / static | ✅ 12/12 | ✅ marker test mocked env; threat loop could run 0 assertions | ✅ passed | ✅ production spawn + non-empty installLines + strict allowlist | ✅ `reconcileError` harness option |

## Remediation Work Unit Evidence

| Evidence | Value |
|---|---|
| Focused test command and exact result | `go test -count=1 ./internal/event/... ./internal/reconcile/...` → PASS; `go test -count=1 ./internal/cli/...` → PASS; `bun test integrations` → 12 pass, 0 fail |
| Runtime harness command/scenario and exact result | `bash integrations/claude/harness.sh` → `OK`; fake `kb` confirms `KB_RECONCILING=1` not set and argv preserves `ses; touch /tmp/pwned` |
| Rollback boundary | Revert `internal/event/queries.go`, `internal/reconcile/service.go`, `integrations/opencode/kanban.ts`, `integrations/opencode/kanban.test.ts`, `integrations/opencode/threat.test.ts`, `integrations/claude/kanban.sh`, and remove `integrations/claude/harness.sh` |

## Remediation Verification

| Command | Exit | Output hash |
|---|---|---|
| `go test -count=1 ./...` | 0 | `sha256:ac00bfdaf18f08830b71b73573cca361e389d40b1196b427177d71a884e4f475` |
| `go build ./...` | 0 | `sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| `go vet ./...` | 0 | `sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| `bun test integrations` | 0 | `sha256:d6684989b8dd63b37d2f1954270826fbe1bd89a8debd12bcacf03ed1c6140ef6` |
| `bash integrations/claude/harness.sh` | 0 | `sha256:a12b7cb43c9d9134b5bb1b35e9096b66775d9e92e7611d1cc92b02edd6782a87` |

## Remediation Changed-Line Budget

**Total: 191 changed lines** (163 insertions, 28 deletions). Within the authorized 200-line cap.

Per-file delta vs. the pre-remediation candidate:
- `internal/event/queries.go`: +28 −3
- `internal/event/queries_test.go`: +50 −1
- `internal/reconcile/service.go`: +1 −1
- `integrations/opencode/kanban.ts`: +2 −4
- `integrations/opencode/kanban.test.ts`: +47 −16
- `integrations/opencode/threat.test.ts`: +7 −1
- `integrations/claude/kanban.sh`: +2 −2
- `integrations/claude/harness.sh`: +26 −0 (new file)

## Design Deviations

None for the remediation scope. The fixes restore the design intent: first adapter invocation unmarked, evidence carried in `task.done`, configurable stale threshold, and empty session IDs owning nothing.

## Issues Remaining

The warnings from the failed verify report (OpenCode timeout/bounded output, coverage thresholds, TDD safety-net entries, validator substring semantics) were explicitly out of scope for this remediation and remain unchanged.

---

## Prior Apply-Progress (obs 2398)

The following is the original apply-progress from the first implementation batch; it is preserved verbatim for cumulative context.

**What**: Implemented all 25 tasks for SDD change `cross-host-task-reconciliation` under strict TDD.

**Why**: Apply phase was ready; maintainer accepted `size:exception` with work-unit boundaries retained.

**Where**:
- Core: `internal/reconcile/{model,service,normalize,evidence}.go`, `internal/reconcile/*_test.go`, `internal/event/queries.go`, `internal/event/queries_test.go`
- CLI: `internal/cli/{add,done,list,reconcile,root,db}.go`, `internal/cli/*_test.go`, `internal/cli/helper_test.go`
- Adapters: `integrations/opencode/kanban.ts`, `integrations/opencode/kanban.test.ts`, `integrations/opencode/threat.test.ts`, `integrations/claude/kanban.sh`, `integrations/shared/reconcile.schema.json`, `integrations/codex/CONTRACT.md`, `package.json`, `Makefile`

**Learned**:
- `v_task_latest` does not expose `session_id`/`meta_json`; reconcile queries must join the base `events` row.
- `ApplyEvidence` needs an injected validator to avoid a circular `event <-> reconcile` import.
- Cobra subcommands do not inherit `SetOut`; tests need `NewTestRootWithOutput` to propagate the writer.
- Actual line count (modified +411/-219, new untracked ~1534 excluding openspec) exceeded the 950 attempt cap despite the accepted `size:exception`.

### Original TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 1.1 | `internal/reconcile/normalize_test.go` | Unit | N/A (new) | ✅ Written | ✅ 11 subcases pass | ✅ 8 cases | ✅ gofmt |
| 1.2 | `internal/reconcile/evidence_test.go` | Unit | N/A (new) | ✅ Written | ✅ 9 cases pass | ✅ 9 cases | ✅ gofmt |
| 1.3 | `internal/reconcile/service_test.go` | Integration | ✅ existing pass | ✅ Written | ✅ 7 cases pass | ✅ 7 cases | ✅ gofmt |
| 1.4 | `internal/event/queries_test.go` | Integration | ✅ existing pass | ✅ Written | ✅ 5 cases pass | ✅ 5 cases | ✅ gofmt |
| 2.1 | `internal/reconcile/normalize.go` | Unit | N/A | N/A (impl) | ✅ tests pass | N/A | ✅ gofmt |
| 2.2 | `internal/reconcile/evidence.go` | Unit | N/A | N/A (impl) | ✅ tests pass | N/A | ✅ gofmt |
| 2.3 | `internal/reconcile/{service,model}.go` | Integration | N/A | N/A (impl) | ✅ tests pass | N/A | ✅ gofmt |
| 2.4 | `internal/event/queries.go` | Integration | N/A | N/A (impl) | ✅ tests pass | N/A | ✅ gofmt |
| 2.5 | refactor | all | ✅ | N/A | N/A | N/A | ✅ gofmt, go vet |
| 3.1 | `internal/cli/add_test.go` | Integration | ✅ existing pass | ✅ Written | ✅ 4 cases pass | ✅ 4 cases | ✅ cmd output |
| 3.2 | `internal/cli/done_test.go` | Integration | ✅ existing pass | ✅ Written | ✅ 4 cases pass | ✅ 4 cases | ✅ cmd output |
| 3.3 | `internal/cli/list_test.go` | Integration | ✅ existing pass | ✅ Written | ✅ 1 case pass | ✅ 1 case | ✅ cmd output |
| 3.4 | `internal/cli/reconcile_test.go` | Integration | ✅ existing pass | ✅ Written | ✅ 3 cases pass | ✅ 3 cases | ✅ cmd output |
| 3.5 | `internal/cli/*.go` | Integration | N/A | N/A (impl) | ✅ tests pass | N/A | ✅ shared helpers |
| 3.6 | refactor | all | ✅ | N/A | N/A | N/A | ✅ gofmt, go vet |
| 4.1 | `integrations/opencode/kanban.test.ts` | Unit (Bun) | N/A (new) | ✅ Written | ✅ 10 pass | ✅ 10 cases | ✅ audit removed |
| 4.2 | `integrations/opencode/threat.test.ts` | Unit (Bun) | N/A (new) | ✅ Written | ✅ 2 pass | ✅ 2 cases | ✅ Makefile checked |
| 4.3 | `integrations/opencode/kanban.ts` | Unit (Bun) | N/A | N/A (impl) | ✅ tests pass | N/A | ✅ thin invoker |
| 4.4 | `integrations/claude/kanban.sh` | Shell | N/A | N/A (impl) | ✅ shellcheck-style | N/A | ✅ quoted vars |
| 4.5 | `integrations/shared/reconcile.schema.json` + `codex/CONTRACT.md` | Docs | N/A | N/A (impl) | ✅ schema valid | N/A | ✅ versioned |
| 4.6 | `Makefile` | Build | N/A | N/A (impl) | ✅ targets exist | N/A | ✅ install paths |
| 4.7 | refactor | adapters | ✅ | N/A | N/A | N/A | ✅ shared env-check |
| 5.1 | verification | all | ✅ | N/A | N/A | N/A | ✅ go test + bun + build |
| 5.2 | verification | adapters | ✅ | N/A | N/A | N/A | ✅ env checks in both adapters |
| 5.3 | verification | all | ✅ | N/A | N/A | N/A | ✅ spec scenarios covered |

### Original Work Unit Evidence

#### Unit 1: Core
- Focused test: `go test ./internal/reconcile/... ./internal/event/...` → PASS (20+ tests)
- Runtime harness: `go test ./internal/reconcile/... -run TestReconcile_session_first_then_stale` → PASS
- Rollback boundary: revert `internal/reconcile/` and `internal/event/queries.go` changes

#### Unit 2: CLI
- Focused test: `go test ./internal/cli/...` → PASS (existing + 11 new tests)
- Runtime harness: `go test ./internal/cli/... -run TestReconcileCmd_json_fields` → PASS
- Rollback boundary: revert `internal/cli/{add,done,list,reconcile,root,db}.go` and test files

#### Unit 3: Adapters
- Focused test: `bun test integrations` → 12 pass, 0 fail
- Runtime harness: `bun test integrations -t "invokes reconcile on top-level session idle"` → PASS
- Rollback boundary: revert `integrations/`, `Makefile`, `package.json`

### Original Design Deviations
- Evidence validator for `test-output` requires `PASS`, `FAIL`, or `ok` substring (or `approved-by:` for reviews); spec only said "validates supplied text" — the stricter shape prevents false positives.
- `ApplyEvidence` accepts a `validate func(string, string) error` parameter to avoid circular imports between `event` and `reconcile`.

### Original Issues
- Changed-line count exceeded the 950-line attempt cap: modified +411/-219 plus ~1534 new untracked lines (excluding openspec/) = ~2164 total. Maintainer had already accepted `size:exception`.
