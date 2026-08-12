```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:dbbce48744baf4144038553c8730fa528d6f173b1e4615c290a9e3b6e282ecd4
verdict: fail
blockers: 6
critical_findings: 6
requirements: 12/17
scenarios: 21/25
test_command: go test ./...
test_exit_code: 0
test_output_hash: sha256:f60eeb21d5af093a312acec509ed2924eefb9201a223d16b06906cca283ea67b
build_command: go build ./...
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

## Verification Report

**Change**: cross-host-task-reconciliation  
**Version**: N/A  
**Mode**: Strict TDD  
**Verdict**: **FAIL**

The Go and Bun suites pass, but independent runtime harnesses expose substantive spec failures. The candidate is not archive-ready.

### Completeness

| Metric | Value |
|---|---:|
| Requirements | 17 actual; 12 compliant |
| Scenarios | 25 actual; 21 compliant |
| Tasks total | 25 |
| Tasks complete | 25 |
| Tasks incomplete | 0 |
| Candidate files represented | 37 modified/untracked files |

The candidate manifest includes the untracked Go, TypeScript, shell, schema, contract, package, and OpenSpec files. `go test ./...`, `go build ./...`, and `bun test integrations` executed against the current worktree, so untracked implementation and test files were included by the runners.

### Build & Tests Execution

| Command | Exit | Output hash | Result |
|---|---:|---|---|
| `go test ./...` | 0 | `sha256:f60eeb21d5af093a312acec509ed2924eefb9201a223d16b06906cca283ea67b` | ✅ Passed; cached package results |
| `go test -count=1 ./...` | 0 | `sha256:7a1aa7779fd54449cdeb9383636571aeb97c18e2f64b98c623de14de800e5465` | ✅ Passed uncached |
| `go test ./internal/reconcile/... ./internal/event/...` | 0 | `sha256:226481a1feed2707bb6cbfe5ead07e4af92c59a7cb19394e6b8f80ab4ff1b1d3` | ✅ Passed |
| `go test ./internal/cli/...` | 0 | `sha256:207ec56c99df5034c8436a09a8eecb353c508cd1d0470db55f3543730236b0bc` | ✅ Passed |
| `bun test integrations` | 0 | `sha256:c74869624c342e844d2244355c6240f1fc94bab24f0518ef1316d425ef18a34a` | ✅ 12 passed, 0 failed |
| `bun test integrations -t "invokes reconcile on top-level session idle"` | 0 | `sha256:2765ca5d955843c1f0e548713c0e500cc2e2ffa932a4791b2c5f996430be9cdb` | ✅ 1 passed |
| `go build ./...` | 0 | `sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | ✅ Passed |
| `go vet ./...` | 0 | `sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | ✅ Clean |
| `bash -n integrations/claude/kanban.sh` | 0 | `sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | ✅ Syntax valid |
| `python /tmp/opencode/kanban-spec-harness.py` | 1 | `sha256:16796fb94cf6d24fe605bbcd7a8ddbbf0a97527ab1de941952165774d75c9ba8` | ❌ Evidence payload, empty-session ownership, and configurable threshold failed |
| `bun /tmp/opencode/opencode-runtime-harness.ts` | 0 | `sha256:69f143cbdb29f4a25a8ad4ec85e71ba734d15fcec237f51b6e71f48f4f37e0ae` | ❌ Harness ran; production spawn carried `KB_RECONCILING=1` and hook returned no output |
| Claude literal-argv runtime harness | 0 | `sha256:0d4234adf09a476af3dc71ee7ead2c92df9ad4720af334b822c8190603e0b87b` | ⚠️ Quoting safe; first reconcile carried `KB_RECONCILING=1` |
| Core recursion runtime harness | 0 | `sha256:bab9396e2756d11152bd159756da5ef477e5bddb8b04ecce4d9f2eed458c2b96` | ✅ Marker produces schema-v1 `no_op:true` |
| Codex contract executable check | 0 | `sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | ✅ Required sections/version found |
| Claude disabled-hook executable check | 0 | `sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | ✅ No invocation/output |

**Coverage command**: `go test -coverprofile=/tmp/opencode/kanban-cover.out ./...`  
**Coverage exit**: 0  
**Coverage output hash**: `sha256:5a7642777d815bbf44adb470ab9cab7a1408bbdee8133952a663151cb118b611`  
**Whole-project Go coverage**: 52.8% statements; configured threshold is 0%.

### Spec Compliance Matrix

| # | Requirement | Scenario | Runtime evidence | Result |
|---:|---|---|---|---|
| 1 | Closure Condition Declaration | Missing closure condition | Spec harness; `TestAddCmd_missing_closure_rejected` | ✅ COMPLIANT |
| 2 | Closure Condition Declaration | Valid declaration | Spec harness; `TestAddCmd_valid_active` | ✅ COMPLIANT |
| 3 | Normalized Duplicate Blocking | Exact duplicate blocked | Spec harness | ✅ COMPLIANT |
| 4 | Normalized Duplicate Blocking | Case-variant duplicate blocked | Spec harness; normalization table | ✅ COMPLIANT |
| 5 | Normalized Duplicate Blocking | Whitespace-variant duplicate blocked | Spec harness; normalization table | ✅ COMPLIANT |
| 6 | Normalized Duplicate Blocking | Punctuation-variant duplicate blocked | Spec harness; exact punctuation table | ✅ COMPLIANT |
| 7 | Normalized Duplicate Blocking | Materially different task allowed | Spec harness | ✅ COMPLIANT |
| 8 | Normalized Duplicate Blocking | Different scope allowed | Spec harness | ✅ COMPLIANT |
| 9 | Session-Scoped Placement | Session-owned task | Spec harness; `TestAddCmd_session_and_future_recorded` | ✅ COMPLIANT |
| 10 | Evidence-Proved Closure | Evidence satisfies declaration | Spec harness | ❌ FAILING — `task.done.meta_json` is empty; evidence payload is absent |
| 11 | Evidence-Proved Closure | Missing evidence rejected | Spec harness; `TestDoneCmd_missing_evidence_rejected` | ✅ COMPLIANT |
| 12 | Flagged Unproved Completion | Unverifiable evidence | Spec harness; done/list tests | ✅ COMPLIANT |
| 13 | Lifecycle Transition Rules | Skip intermediate state | Spec harness with `approved-by:alice` | ✅ COMPLIANT |
| 14 | Session-First Pass | Session-owned priority | Spec harness with plain + flagged owned tasks and non-owned exclusions | ✅ COMPLIANT |
| 15 | Bounded Stale Review | Stale tasks surfaced | `TestReconcile_session_first_then_stale` | ✅ COMPLIANT |
| 16 | Bounded Stale Review | Recent tasks excluded | `TestReconcile_session_first_then_stale` | ✅ COMPLIANT |
| 17 | Staleness in List | Staleness shown | `TestListCmd_renders_flag_and_staleness` | ✅ COMPLIANT |
| 18 | Dry-Run Mode | Dry-run is read-only | Core + CLI dry-run tests | ✅ COMPLIANT |
| 19 | Provider-Neutral Core | Adapter is a thin invoker | Production OpenCode/Claude harnesses | ❌ FAILING — adapters mark the first core call as recursive, so core performs no reconciliation |
| 20 | OpenCode Adapter | OpenCode session close | Production OpenCode harness | ❌ FAILING — spawned core receives recursion marker and hook returns `undefined` rather than core output |
| 21 | Claude Adapter | Claude session close | Claude + core recursion harnesses | ❌ FAILING — shell exports recursion marker before the first reconcile call |
| 22 | Codex Contract | Contract document exists | Executable contract check | ✅ COMPLIANT |
| 23 | Recursion Guard | Recursion blocked | `TestReconcile_recursion_guard`; core runtime harness | ✅ COMPLIANT |
| 24 | Idempotency | Double reconcile | Spec harness compares full JSON and event counts for flagged task | ✅ COMPLIANT |
| 25 | Hooks Disabled | Hooks disabled | Bun test + Claude executable check | ✅ COMPLIANT |

**Compliance summary**: 21/25 scenarios compliant.

### Correctness (Static Evidence)

| Requirement | Status | Notes |
|---|---|---|
| Closure Condition Declaration | ✅ Implemented | Rejects before DB open and appends nothing. |
| Normalized Duplicate Blocking | ✅ Implemented | Exact design punctuation set `.,!?;:'"()[]{}-_`; project/scope punctuation preserved; Unicode whitespace collapsed; no fuzzy/NLP path. |
| Session-Scoped Placement | ✅ Implemented | `task.new.session_id` is persisted. |
| Evidence-Proved Closure | ❌ Incomplete | Validation runs, but valid evidence is not persisted on `task.done`. |
| Flagged Unproved Completion | ✅ Implemented | Invalid attempts remain open, visible, and hash-idempotent. |
| Lifecycle Transition Rules | ✅ Implemented | Backlog activation and review-approved closure execute. |
| Session-First Pass | ✅ Implemented for non-empty IDs | Empty session IDs incorrectly select all unowned tasks. |
| Bounded Stale Review | ❌ Incomplete | `Config.StaleAfter` is calculated but never passed to `ListStale`; query is hard-coded to 7 days. |
| Staleness in List | ✅ Implemented | Active rows older than 7 days render `Nd stale`. |
| Dry-Run Mode | ✅ Implemented | Reconcile is read-only. |
| Provider-Neutral Core | ❌ Broken at adapter boundary | Policy is in Go, but first adapter invocation is forced into core no-op. |
| OpenCode Adapter | ❌ Broken | Literal argv is safe, but marker/output behavior violates the scenario. |
| Claude Adapter | ❌ Broken | Quoting is safe, but marker behavior violates the scenario. |
| Codex Contract | ✅ Implemented | Documentation only; no Codex implementation exists. |
| Recursion Guard | ✅ Implemented | Core returns versioned no-op. |
| Idempotency | ✅ Implemented | Full output/event-count runtime comparison passed. |
| Hooks Disabled | ✅ Implemented | Both adapters short-circuit. |

The apply-phase evidence validator deviation is present exactly as reported: `test-output` accepts case-insensitive substrings `PASS`, `FAIL`, or `ok`; `review-approved` requires the `approved-by:` prefix. It performs no subprocess execution. This shape validation passed its current tests, but it does not repair the missing evidence payload.

### Coherence (Design)

| Decision | Followed? | Notes |
|---|---|---|
| Policy lives in Go core | ⚠️ Partial | Core owns policy, but adapters prevent first-call execution. |
| Exact deterministic normalization | ✅ Yes | Implementation matches the exact ASCII punctuation set and preservation rules. |
| Typed pure evidence registry | ✅ Yes | Reported PASS/FAIL/ok and approved-by deviation implemented. |
| Session first, stale limit 50, ordered `(LastTS,id)` | ⚠️ Partial | Ordering/limit/default pass; configurable stale threshold and empty-session behavior fail. |
| Evidence carried atomically in update + done | ❌ No | Valid evidence payload is discarded. |
| First adapter invocation unmarked; future children guarded | ❌ No | Both adapters mark the first core invocation. |
| Fixed argv, explicit cwd, ignored stdin, bounded output, timeout | ⚠️ Partial | Argv/cwd/stdin are safe; OpenCode has no timeout and reads unbounded stdout. |
| Failure isolation | ✅ Yes | OpenCode catches failures; Claude exits zero for pre-exec diagnostics. |
| Future Codex contract only | ✅ Yes | No Codex implementation was added. |

### TDD Compliance

| Check | Result | Details |
|---|---|---|
| TDD Evidence reported | ✅ | Apply-progress contains the required table. |
| All RED tasks have test files | ✅ | 10/10 RED task rows reference existing test files; all 25 task rows are present. |
| RED confirmed (tests exist) | ✅ | 10/10 referenced test files exist, including untracked files in the candidate. |
| GREEN confirmed (reported suites pass) | ✅ | Go and Bun suites pass now. |
| Triangulation adequate | ⚠️ | Normalization reports 11 passing subcases but current source has 8 table subcases plus 2 standalone cases; several adapter assertions do not exercise production spawn behavior. |
| Safety Net for modified files | ⚠️ | Apply evidence records N/A on modified implementation rows including event queries, CLI, and OpenCode despite package-level prior-test claims elsewhere. |

**TDD Compliance**: 4/6 checks passed. Passing suites do not establish behavioral compliance where assertions bypass production paths.

### Test Layer Distribution

| Layer | Tests | Files | Tools |
|---|---:|---:|---|
| Unit | 32 | 4 | Go `testing`, Bun |
| Integration | 24 | 6 | Go `testing` + in-memory SQLite |
| E2E | 0 | 0 | Not available |
| **Total** | **56** | **10** | |

The safe external runtime harnesses add process-level evidence but are not repository test files.

### Changed File Coverage

Go coverage is statement coverage; Bun produced no per-file coverage report.

| File | Statement % | Uncovered lines | Rating |
|---|---:|---|---|
| `internal/cli/add.go` | 85.7% | L31-33, L36-41, L47-49, L76-78, L100-102, L115-117 | ⚠️ Acceptable |
| `internal/cli/db.go` | 75.0% | L18 | ⚠️ Low |
| `internal/cli/done.go` | 76.9% | L20-22, L28-33, L36-41, L44-46 | ⚠️ Low |
| `internal/cli/list.go` | 81.6% | L23-28, L34-40, L46-48, L50-52 | ⚠️ Acceptable |
| `internal/cli/reconcile.go` | 60.0% | L25-30, L42-44, L48-50, L55-67 | ⚠️ Low |
| `internal/cli/root.go` | 100.0% | — | ✅ Excellent |
| `internal/event/queries.go` | 74.7% | Multiple ranges; notably L599-601, L628-634, L654-656, L679-681, L690-692, L721-723, L733-745 | ⚠️ Low |
| `internal/reconcile/evidence.go` | 81.2% | L9-14 | ⚠️ Acceptable |
| `internal/reconcile/model.go` | N/A | Type declarations only | ➖ N/A |
| `internal/reconcile/normalize.go` | 100.0% | — | ✅ Excellent |
| `internal/reconcile/service.go` | 72.3% | L48-57, L66-69, L86-88, L111-119 | ⚠️ Low |

**Average changed Go statement coverage**: 76.4% (424/555 statements).  
**Changed coverage extraction hash**: `sha256:58b16b81e76e766700c3f6236da73da4852d089bfa144b0aa09997d502c6a17d`.

### Assertion Quality

| File | Line | Assertion | Issue | Severity |
|---|---:|---|---|---|
| `integrations/opencode/kanban.test.ts` | 157-167 | Reconcile call-count assertion | Test claims inherited marker coverage but injects a mock that records no environment; production spawn path is never exercised and the real harness disproves it. | CRITICAL |
| `integrations/opencode/threat.test.ts` | 9-18 | Assertions inside `for (installLines)` | Ghost-loop risk: zero matching install lines would execute zero assertions and pass. | CRITICAL |
| `integrations/opencode/threat.test.ts` | 21-26 | Source-string absence checks | Couples to source text and does not execute installer/selection behavior. | WARNING |
| `internal/cli/done_test.go` | 27-54 | Event-count assertions only | Does not assert the required evidence payload; production currently drops it. | CRITICAL |
| `internal/reconcile/service_test.go` | 242-251 | `snapshotsEqual` compares IDs only | Original idempotency test does not compare full observable state; external harness supplied the missing proof. | WARNING |

**Assertion quality**: 3 CRITICAL, 2 WARNING.

### Quality Metrics

**Formatter**: No format mutation was run; source was inspected only.  
**Linter**: ➖ Not configured.  
**Type checker / vet**: ✅ `go vet ./...` clean.  
**Bun runner**: ✅ Operational (`bun 1.3.14`, 12/12 tests passed).  
**Shell syntax**: ✅ `bash -n` passed.

### Work-Unit Evidence

| Unit | Apply claim | Independent result |
|---|---|---|
| Core | Focused Go tests + reconcile test harness pass | Focused tests pass; configurable threshold and empty-session semantics fail in process-level harness. |
| CLI | Focused CLI tests + JSON harness pass | Focused tests pass; valid completion drops evidence payload. |
| Adapters | 12 Bun tests + session-close harness pass | Bun tests pass, but production OpenCode and Claude invocations force a core no-op; reported harness covered the mock, not the production spawn boundary. |

All 25 checkboxes are checked, but work-unit runtime evidence is contradicted by independent execution. The accepted size exception covers review size only; it does not waive correctness.

### Issues Found

#### CRITICAL

1. **Adapters disable their own first reconciliation.** `integrations/opencode/kanban.ts:52-54` and `integrations/claude/kanban.sh:30-32` set `KB_RECONCILING=1` before invoking `kb reconcile`; `internal/reconcile/service.go:43-46` therefore returns `no_op:true`. OpenCode and Claude close scenarios fail.
2. **OpenCode does not return reconcile output to the host.** `integrations/opencode/kanban.ts:157` awaits and discards the result; the production harness returned `null`.
3. **Valid completion evidence is not persisted.** `internal/event/queries.go:718-742` leaves `metaJSON` empty on valid evidence, contradicting the required `task.done` evidence payload.
4. **Configurable stale review is ignored.** `internal/reconcile/service.go:65` omits `StaleAfter`; `internal/event/queries.go:631` hard-codes seven days. A two-day task was absent with `--stale-after 86400`.
5. **Empty session IDs claim all unowned tasks.** `ListBySession` compares ownership to `""`; the design requires empty session IDs to own nothing.
6. **Strict-TDD adapter evidence is not trustworthy.** The marker test bypasses production spawn behavior, and the threat-matrix loop can pass with zero assertions. Independent runtime evidence contradicts the claimed GREEN behavior.

#### WARNING

1. OpenCode subprocess handling lacks the design's timeout and bounded-output guarantees.
2. Five changed Go implementation files are below 80% statement coverage.
3. Apply TDD safety-net entries are N/A for several modified implementation rows.
4. The apply-progress normalization GREEN count says 11 subcases; current tests expose 10 effective cases (8 table cases plus 2 standalone tests).
5. The `test-output` validator uses substring matching and treats `FAIL` as shape-valid closure evidence, exactly as the reported deviation states; this is broader than proof-of-success semantics and should remain an explicit product decision.

#### SUGGESTION

None. Substantive failures must be resolved before optional improvements.

### Canonical Verification Evidence Preimage

The following 1,813 bytes are the exact preimage whose SHA-256 is the envelope's `evidence_revision`. Preserve these bytes for native continuation:

```yaml
schema: gentle-ai.verification-evidence-preimage/v1
change: cross-host-task-reconciliation
candidate_manifest_hash: sha256:4e4ab61f9391c89ecd312cb0c0ebc3786adafe1c0b4d560688bd06bc2571a22f
candidate_manifest_bytes: 4259
requirements_total: 17
requirements_compliant: 12
scenarios_total: 25
scenarios_compliant: 21
test_command: go test ./...
test_exit_code: 0
test_output_hash: sha256:f60eeb21d5af093a312acec509ed2924eefb9201a223d16b06906cca283ea67b
uncached_test_command: go test -count=1 ./...
uncached_test_exit_code: 0
uncached_test_output_hash: sha256:7a1aa7779fd54449cdeb9383636571aeb97c18e2f64b98c623de14de800e5465
build_command: go build ./...
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
bun_test_command: bun test integrations
bun_test_exit_code: 0
bun_test_output_hash: sha256:c74869624c342e844d2244355c6240f1fc94bab24f0518ef1316d425ef18a34a
runtime_harness_command: python /tmp/opencode/kanban-spec-harness.py
runtime_harness_exit_code: 1
runtime_harness_output_hash: sha256:16796fb94cf6d24fe605bbcd7a8ddbbf0a97527ab1de941952165774d75c9ba8
opencode_runtime_harness_command: bun /tmp/opencode/opencode-runtime-harness.ts
opencode_runtime_harness_exit_code: 0
opencode_runtime_harness_output_hash: sha256:69f143cbdb29f4a25a8ad4ec85e71ba734d15fcec237f51b6e71f48f4f37e0ae
claude_runtime_harness_command: KB_SESSION_ID='ses; touch /tmp/pwned' KB_PROJECT_DIR='/repo with spaces' PATH="/tmp/opencode:$PATH" bash integrations/claude/kanban.sh
claude_runtime_harness_exit_code: 0
claude_runtime_harness_output_hash: sha256:0d4234adf09a476af3dc71ee7ead2c92df9ad4720af334b822c8190603e0b87b
core_recursion_harness_exit_code: 0
core_recursion_harness_output_hash: sha256:bab9396e2756d11152bd159756da5ef477e5bddb8b04ecce4d9f2eed458c2b96
verdict: fail
```

**Preimage SHA-256**: `sha256:dbbce48744baf4144038553c8730fa528d6f173b1e4615c290a9e3b6e282ecd4`  
**Candidate manifest SHA-256**: `sha256:4e4ab61f9391c89ecd312cb0c0ebc3786adafe1c0b4d560688bd06bc2571a22f` (4,259 bytes; verify-report excluded to avoid self-reference).

### Verdict

**FAIL** — 5 requirements and 4 scenarios are not compliant; independent runtime evidence contradicts adapter and closure claims.
