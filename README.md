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

## Commands

```
kb init                     create db at ~/.local/share/kanban/db.sqlite
kb add <title>              add a task manually
kb list | kb today          show open tasks
kb done <id>                mark a task done
kb event --type=... ...     generic event (used by hooks)
kb count                    print open count (waybar)
kb scope <name>             timeline for a scope
```

## Storage

`~/.local/share/kanban/db.sqlite` — SQLite, append-only events table with materialized views for current state.

## Capture

Two layers:

1. **Passive** — `PostToolUse` hook on engram `mem_save` mirrors saves into events.
2. **Active** — `Stop` hook returns a system reminder forcing the agent to audit for unsaved tasks/scope shifts at every stop.
