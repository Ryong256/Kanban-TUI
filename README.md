# kb — personal kanban

Event-sourced personal kanban with optional automation adapters for Claude Code and OpenCode.

## Why

Per-project `TODO.md` files lose tasks that emerge mid-session and the *evolution* of how scope changed. `kb` centralizes this across all projects and surfaces it passively in waybar so you don't have to go looking.

## Model

Append-only event log, not a mutable task list. State is reconstructed from events.

| Event type     | Meaning                                        |
| -------------- | ---------------------------------------------- |
| `task.new`     | Task discovered                                 |
| `task.done`    | Task completed (refs the `task.new` id)         |
| `task.update`  | Task updated (refs the `task.new` id)           |
| `scope.shift`  | Pivoted from one direction to another           |
| `scope.expand` | Initial scope grew to cover something new       |
| `note`         | Something that happened; no status, never on the board |

A task is future work with a checkable done condition. A note is a record of
something already finished — a merged PR, a confirmed root cause, a decision.
Notes stay out of `kb list`, `kb count`, and the board, so recording history
never inflates the backlog.

## Install

```
go install github.com/Ryong256/kanban/cmd/kb@latest
kb init
```

Requires Go 1.26+. Binary lands in `$GOBIN` (usually `~/go/bin` or `~/.local/bin` if you set `GOBIN`).

## Architecture

`kb` core is host-neutral. The CLI, event store, project detection, and TUI do
not depend on an AI host. Automation lives at the host boundary and invokes the
same public CLI commands a person can run.

| Layer | Responsibility |
| ----- | -------------- |
| `kb` core | Project-scoped tasks, notes, events, storage, and TUI |
| Claude Code adapter | Existing `PostToolUse` passive capture and `Stop` audit lifecycle |
| OpenCode adapter | System guidance, idle audit turn, and Engram memory mirroring |

The adapters are independent. Installing OpenCode support does not remove or
modify Claude Code behavior, and the OpenCode adapter does not modify the
Engram plugin.

## Automation adapters

### Claude Code

Existing Claude Code hooks remain compatible. `PostToolUse` mirrors supported
Engram `mem_save` calls into `kb`, while `Stop` asks the agent to audit tasks and
scope before ending. Claude Code owns that hook lifecycle and configuration;
the OpenCode installer below does not touch it.

### OpenCode

The repository-owned plugin source is
[`integrations/opencode/kanban.ts`](integrations/opencode/kanban.ts). Install it
for the current user with:

```sh
make install-opencode
```

The target copies the plugin to
`${XDG_CONFIG_HOME:-$HOME/.config}/opencode/plugins/kanban.ts`, creating only
the plugin directory it needs. OpenCode 1.18.16 discovers TypeScript files in
that directory automatically, so no `opencode.json` edit is required.

Quit and restart OpenCode after installation. Plugins are loaded at startup and
an already-running process will not see the new file.

OpenCode has a different lifecycle from Claude Code:

- Every top-level system transform receives current project-scoped open tasks
  and the task-versus-note procedure, including after later turns or compaction.
- The first top-level `session.idle` refreshes tasks and dispatches one synthetic
  audit turn. Child sessions are ignored and a failed dispatch may retry on the
  next idle event.
- Successful `engram_mem_save` calls for bug fixes, decisions, architecture,
  discoveries, patterns, and configuration are mirrored as `kb note` entries.
  Prompt saves, preferences, and unknown types are not mirrored.
- Project detection is explicit. If `kb detect-project` returns no project, the
  adapter stops and never runs an unscoped `kb list`.

Set `KB_HOOKS_DISABLED=1` before starting OpenCode to disable the adapter. A
missing `kb` binary, failed project detection, or adapter subprocess error is a
best-effort no-op and does not interrupt OpenCode.

## Commands

```
kb init                     create db at ~/.local/share/kanban/db.sqlite
kb add <title>              add a task manually
kb note <title>             record something that happened (never hits the board)
kb list | kb today          show open tasks
kb notes                    show recent notes
kb done <id>                mark a task done
kb rm <id...>               delete a task or note permanently
kb demote <id...>           reclassify a task as a note
kb move <id> <status>       move task between columns (backlog, in_progress, testing, complete, done)
kb event --type=... ...     generic event (used by hooks)
kb count                    print open count (waybar)
kb scope <name>             timeline for a scope
kb project add|list|rm      manage project registry (rm --purge deletes its events)
```

## TUI

Run `kb view` (alias `kb v`) to open the Bubbletea kanban board. Project tabs at
the bottom, vim-style navigation (`h/j/k/l`), `1-4` to send the selected task to
a column, `H/L` to move it one column over. Press `?` for the full keymap, `/` to
filter, `d` to delete a task, `n` to turn it into a note, `D` to delete the
active project. Use `-a` to view tasks across all projects, `-p <name>` to scope
to a specific project.

The focused column takes about half the width so its titles stay readable, and
the rest render as a preview. `IN PROGRESS` and `TESTING` carry WIP limits and
turn red when breached; tasks in them show how long they have sat there. `DONE`
shows the last 7 days — the all-time count lives in the header as `N older`.
A `~` marks a task whose scope has `scope.shift`/`scope.expand` events.

## Storage

`~/.local/share/kanban/db.sqlite` — SQLite, append-only events table with materialized views for current state.

## License

MIT — see [LICENSE](LICENSE).
