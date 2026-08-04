# kb — personal kanban

Event-sourced personal kanban that captures tasks and scope evolution from your Claude Code sessions automatically.

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
kb project add|list|rm      manage project registry
```

## TUI

Run `kb view` (alias `kb v`) to open the Bubbletea kanban board. Project tabs at the bottom, vim-style navigation (`h/j/k/l`), `1-5` to jump columns, `H/L` to move the selected task. Use `-a` to view tasks across all projects, `-p <name>` to scope to a specific project.

## Storage

`~/.local/share/kanban/db.sqlite` — SQLite, append-only events table with materialized views for current state.

## Capture

Two layers:

1. **Passive** — `PostToolUse` hook on engram `mem_save` mirrors saves into events.
2. **Active** — `Stop` hook returns a system reminder forcing the agent to audit for unsaved tasks/scope shifts at every stop.

## License

MIT — see [LICENSE](LICENSE).
