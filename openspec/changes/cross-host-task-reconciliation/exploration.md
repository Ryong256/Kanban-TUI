# Exploration: Cross-Host Task Reconciliation

## Current State

### Architecture

The `kanban-tui` project is an event-sourced personal kanban built in Go with a host-neutral core. Only one AI-host integration exists: an OpenCode TypeScript plugin at `integrations/opencode/kanban.ts`. No Claude Code or Codex integration exists. The core (`cmd/kb`, `internal/{event,db,cli,tui}`) has zero host dependencies.

The events table (`internal/db/migrations/0001_init.sql`) already has `session_id TEXT` and `source TEXT` columns, seeded by the original Claude Code design, but these fields are not populated by any current code path — `kb add` (`internal/cli/add.go`) hardcodes `Source: "manual"` and leaves `SessionID` empty.

### OpenCode Integration (the only live host)

The plugin at `integrations/opencode/kanban.ts` uses three hooks:

1. **`session.idle`**: Fires once per top-level session. Runs `kb list --project <p>`, injects a one-shot `AUDIT_PROMPT` via `client.session.promptAsync()`. The `auditDispatched` Set prevents re-entry. On failure, the marker is cleared so a future idle event retries. Cleanup on `session.deleted`.

2. **`experimental.chat.system.transform`**: On every prompt turn, runs `kb list --project <p>` and appends the `OPERATING_PROCEDURE` plus the current open task list into the system prompt block `<!-- kb-opencode-guidance:start -->...<!-- kb-opencode-guidance:end -->`.

3. **`tool.execute.after`**: Mirrors `engram_mem_save` calls with six accepted types (`bugfix, decision, architecture, discovery, pattern, config`) into `kb note` with `--source hook-post`.

### Root Causes of Drift

**Creation Drift — why agents create excessive tasks:**

1. **No structural duplicate guard**: `kb add` (`internal/event/event.go:Add`) validates type, project, and title non-empty, but does no similarity deduplication. The `OPERATING_PROCEDURE` asks agents to "Search the current project tasks before adding work and avoid duplicates," but this is prompt-only guidance — LLMs interpret it loosely, especially across restarts and parallel sessions.

2. **Prompt-based guidance is the only gate**: The task list appears in the system prompt once per turn, but the agent decides whether to scan it. There is no structured pre-add check that says "this task looks like an existing open task."

3. **No session-ownership tracking**: `Insert.SessionID` exists in the Go struct but is never populated by `kb add`. The CLI does not expose a `--session-id` flag. Without session attribution, there is no way to answer "did my session already create this?" or "which session created those 12 stale tasks?"

4. **The audit prompt adds tasks too**: `AUDIT_PROMPT` explicitly asks the agent to "Add only genuinely missing future work." An agent that is overly helpful will add tasks during the audit — the same mechanism meant to close tasks can ironically create more.

**Closure Drift — why completed work stays open:**

1. **One-shot-per-session audit**: `auditDispatched` is added before the prompt is sent and only cleared on failure. After a successful audit dispatch, the session gets no more closure prompts. If the AI misses tasks in that one shot, they stay open forever.

2. **No lifecycle-hook on session end**: `session.deleted` only cleans caches (`sessionKinds`, `tasksBySession`, `auditDispatched`). It does not trigger a final reconciliation or closure pass.

3. **No evidence gate for closure**: The audit prompt says "Record completed, confirmed, or observed facts as notes instead of tasks" but does not demand evidence (commits, PR merges, test results). An AI can hallucinate a closure without proof.

4. **Cross-scope visibility**: The audit shows ALL open tasks regardless of scope. An agent working on `feature-X` sees 15 tasks from `feature-Y` and has no context to close them. Tasks from orphaned sessions accumulate indefinitely.

5. **`kb list` has no staleness signal**: The list command shows `created_ts` but not `last_modified` or session ownership. Adapters have no way to surface "tasks untouched for 14 days."

6. **No automatic TUI-driven cleanup**: The TUI prunes done-column tasks after 7 days but offers no equivalent for backlog rot.

### Existing Safeguards

- **`KB_HOOKS_DISABLED=1`**: Kill switch that disables all adapter hooks.
- **Concurrent idle guard**: The `auditDispatched.has(sessionID)` check (before and after `isTopLevel`) combined with `auditDispatched.add(sessionID)` prevents concurrent idle handlers from dispatching duplicate audits.
- **Child-session filtering**: `isTopLevel()` checks `sessionKinds` before acting; child sessions are ignored.
- **Graceful degradation**: All hooks are wrapped in try/catch. Kanban automation never breaks OpenCode.

## Affected Areas

- `integrations/opencode/kanban.ts` (252 lines) — the entire adapter: session lifecycle, system prompt injection, audit dispatch, memory mirroring. Will need significant restructuring to consume a shared prompt artifact and to add session-end hooks.
- `integrations/opencode/kanban.test.ts` (263 lines) — comprehensive test suite covering disabled hooks, idle audit, recursion prevention, concurrency, child sessions, guidance injection. All tests must evolve for the new architecture.
- `internal/cli/add.go` — must expose `--session-id` flag and optionally support a duplicate check mode.
- `internal/cli/list.go` — may need `--stale` or `--since` flags to surface staleness.
- `internal/cli/done.go` / `internal/cli/count.go` — may need new subcommands or flags for reconciliation.
- `internal/event/event.go` — `Insert` struct already has `SessionID` but it is unused. No change needed to the struct; the change is in cli wiring.
- `internal/event/queries.go` — `ListOpen`, `CountOpen`, `MoveTask` are stable. May need new query functions (e.g., `ListStale`, `ListBySession`).
- `internal/cli/root.go` — may register new commands like `kb reconcile`.
- `internal/db/migrations/` — append-only. No migration needed unless a new event type is added. Current types (`task.new`, `task.done`, `task.update`, `scope.shift`, `scope.expand`, `note`) cover the reconciliation domain.
- `Makefile` — must include `install-claude` or `install-claude-code` target.
- (NEW) `integrations/claude/` — Claude Code lifecycle hooks (shell scripts or CLAUDE.md augmentation).
- (NEW) `integrations/shared/` — shared prompt/procedure artifacts consumed by all adapters.
- (NEW, optional) `internal/cli/reconcile.go` — if a host-neutral `kb reconcile` command is the chosen approach.

Files NOT affected: `internal/tui/` (TUI is read-only for this change — board rendering doesn't change), `internal/db/db.go`, `internal/store/`, `cmd/kb/`.

## Approaches

### 1. Duplicated Host-Specific Prompts

Each host adapter carries its own copy of the `OPERATING_PROCEDURE` and `AUDIT_PROMPT` text. OpenCode gets a TypeScript plugin; Claude Code gets `/claude/stop.sh` and `/claude/pre-prompt.sh` scripts with the same text baked in. No new Go code.

- **Pros**: Fastest to ship; zero Go changes; each host can tune its own prompt independently.
- **Cons**: Prompt text drifts across hosts; bug fixes in procedure must be applied in N places; Claude Code hooks are bash scripts that call `kb` — fragile quoting/escaping; no shared validation logic; Codex would add a third copy; LLM interpretation variance means one host may close aggressively while another ignores tasks.
- **Effort**: Low

### 2. Shared Prompt/Procedure Artifact Consumed by Adapters

A single `integrations/shared/reconcile.md` file holds the canonical procedure + audit prompt + evidence gates. Each host adapter reads this file at runtime and injects it. Claude Code hooks source the file via `cat`; OpenCode reads it via `Bun.file()`. No new Go commands; thin adapter wrappers.

- **Pros**: Single source of truth for prompt text; bug fixes apply to all hosts; adapters are thin; minimal Go changes (maybe none).
- **Cons**: Still purely prompt-based — relies entirely on LLM interpretation for closure decisions; no machine-checkable guard against missing session IDs or stale tasks; adapters must handle file-read errors; Claude Code hooks are still shell scripts with quoting risks; no structural deduplication at the `kb add` level.
- **Effort**: Low–Medium

### 3. Host-Neutral Reconciliation Command with Thin Adapters

A new `kb reconcile` command in the Go core that implements structured reconciliation heuristics. Adapters only invoke `kb reconcile --project <p> --session-id <s>` and forward the output. The Go command handles:
- Listing open tasks with staleness metadata (days since creation, days since last event, session ownership where known)
- Recommending closure candidates based on configurable thresholds
- Providing a machine-readable `--json` output for adapters to parse
- Supporting `--dry-run` vs. `--apply` modes
- Optionally performing duplicate detection on `kb add --dedupe` before creating new tasks

Thin adapters call the same command at session start (context injection) and session end (reconciliation).

- **Pros**: Reconciliation logic lives in Go — testable, type-safe, single source of truth; provider-neutral protocol (any host that can run `kb` works); adapters shrink to ~30 lines each; Codex needs only to run `kb reconcile`; LLM interpretation limited to "what to do about these recommended closures" not "how to find stale tasks"; session-ownership tracking becomes structural; evidence gates can be encoded in the command output (e.g., "task #42 was created by session `ses_abc` 14 days ago, last touched 14 days ago, no related events").
- **Cons**: Largest Go changes; new CLI command surface; JSON output format must be stable; `--session-id` wiring through CLI; `kb add --dedupe` is a non-trivial feature (similarity matching); more upfront design; `kb reconcile` adds a new system command to the public CLI.
- **Effort**: Medium–High

### Comparison Table

| Dimension | Approach 1 (Duped Prompts) | Approach 2 (Shared Artifact) | Approach 3 (Reconcile Command) |
|-----------|---------------------------|------------------------------|-------------------------------|
| Prompt consistency | ❌ Fragile | ✅ Single file | ✅ Go-embedded |
| Closure reliability | ❌ LLM-only | ❌ LLM-only | ✅ Heuristic + LLM |
| Session ownership | ❌ Not tracked | ❌ Not tracked | ✅ `--session-id` |
| Staleness detection | ❌ Ad-hoc | ❌ Ad-hoc | ✅ Structured query |
| Testability | ❌ Adapter-level only | ⚠️ Adapter + file content | ✅ `go test` + adapter |
| Codex readiness | ❌ Third copy | ⚠️ Needs file access | ✅ `kb reconcile` |
| Effort | Low | Low–Medium | Medium–High |
| Duplicate prevention | ❌ Prompt only | ❌ Prompt only | ✅ `kb add --dedupe` |
| Adapter complexity | High (fragile) | Medium | Low (~30 lines) |
| Migration path | — | Can evolve to #3 | Terminal state |

## Recommendation

**Approach 3 (Host-Neutral Reconciliation Command with Thin Adapters)** with one staged delivery:

**Stage 1 (this change)**: Ship the `kb reconcile` command and the thin adapters. This gives immediate structural improvement for OpenCode and Claude Code. The reconciliation command handles stale-task detection, session-aware lists, and structured output.

**Stage 2 (future change)**: Add `kb add --dedupe` and deeper task-lifecycle guards. Duplicate prevention is separable — the creation drift and closure drift are independent problems. Address creation drift after the reconciliation pipeline is proven.

### Why not Approach 2 alone

Approach 2 solves prompt consistency but not the structural gap: without session tracking and staleness heuristics, the system still depends entirely on LLM interpretation of text. The user specifically asked for a "protocol," not a "prompt."

### Specific Design Decisions

1. **`kb reconcile` command signature**: `kb reconcile --project <p> --session-id <s> [--dry-run] [--json]`
   - Without `--dry-run`: emits human-readable closure recommendations.
   - With `--dry-run` + `--json`: emits a JSON array of recommended actions (close, move, note, review).
   - Does NOT auto-apply — the AI host or human must confirm. This is an audit tool, not an automated closer.

2. **Session ownership via `kb add --session-id`**: Add a `--session-id` flag to `kb add` that populates `Insert.SessionID`. The flag is optional — manual `kb add` still works without it. Adapters always pass it.

3. **Staleness query in `queries.go`**: A new `ListStale(d *sql.DB, project string, sinceHours int)` function that returns tasks with no `task.update` or `task.done` events in the last N hours, grouped by session where possible.

4. **Recursion prevention**: The `kb reconcile` output includes a header `KB_RECONCILE_RUN=1`. Adapters check for this header in the last assistant message before issuing commands. If present, skip the next reconcile and continue. Combined with `KB_HOOKS_DISABLED=1`.

5. **Evidence gate**: The reconciliation prompt emitted by `kb reconcile` demands specific evidence before closing: "For each closure recommendation, cite the commit hash, merged PR URL, or test result that confirms completion. Do not close any task without evidence."

6. **Claude Code integration**: A `/claude/KANBAN.md` file with startup instructions plus `/claude/hooks/stop.sh` that invokes `kb reconcile`. Claude Code's native hook mechanism makes this straightforward.

7. **Codex path**: Document the protocol contract: "Any host that can run `kb reconcile --project <p> --session-id <s> --json` and inject its output into the agent context is a compatible host." No adapter code needed for Codex — just documentation.

## Risks

- **`kb reconcile` output sensitivity**: If the JSON output leaks task IDs or internal state unexpectedly, it could pollute the AI context. Mitigation: the `--json` output is explicitly designed for machine consumption with a documented schema.
- **Session-ID spoofing**: Any adapter (including malicious ones) can pass any `--session-id`. Mitigation: session IDs are advisory metadata, not an auth mechanism. The system treats untrusted session IDs as categorization hints, not identity claims.
- **Shell-escape in Claude Code hooks**: Claude Code hooks are bash scripts. If a project name contains spaces or special chars, `kb reconcile --project "$PROJECT"` must quote correctly. Mitigation: the adapter template includes proper quoting; test with pathological project names.
- **Perf impact of `kb list` on every prompt turn**: The OpenCode adapter already runs `kb list` per turn. A `kb reconcile` call is similar cost (one SQL query, no writes in dry-run mode). Acceptable for a personal kanban.
- **`kb add --dedupe` scope creep**: Similarity matching is non-trivial. The recommendation defers this to Stage 2 to avoid blocking Stage 1 on a hard NLP problem.

## Ready for Proposal

**Yes.** The exploration is complete. The orchestrator should proceed to `sdd-propose` with:

1. **Recommended approach**: Host-Neutral Reconciliation Command (Approach 3), Stage 1 scope.
2. **Key scoping decisions**: Defer `kb add --dedupe` to a future change. This change is about reconciliation (closure drift), not creation prevention.
3. **Adapter count**: OpenCode (refactor existing), Claude Code (new).
4. **User to confirm**: The two-stage delivery plan (reconcile first, dedupe later) before proceeding to proposal.
