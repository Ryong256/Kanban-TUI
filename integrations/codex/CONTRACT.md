# Codex Adapter Contract (Future Implementation)

This document defines the contract a future Codex adapter must implement. No Codex-specific code lives in the repository today.

## Entry Point

The adapter is invoked by the host when a session ends. It must call the `kb` core binary installed on the user's machine.

```
kb reconcile --project <PROJECT> --session-id <SESSION_ID> --json
```

## Environment Markers

- `KB_HOOKS_DISABLED=1` — adapter must exit immediately without invoking `kb`.
- `KB_RECONCILING=1` — set by the **core** on its own process once a reconcile run starts, so that any `kb` the run spawns inherits it and returns a no-op JSON response. The adapter must **not** set it on the `kb` it spawns: that would suppress the first run instead of the recursive one. The adapter reads it only to stay quiet when it is itself running inside a reconcile pass.

## Input

The host provides, at minimum:

- `session_id` — the Codex session identifier.
- `project_dir` — the working directory of the session.

Project resolution is delegated to `kb detect-project <project_dir>`.

## Output Schema

The adapter returns the JSON produced by `kb reconcile`. The schema is versioned and stable:

- Schema file: `integrations/shared/reconcile.schema.json`
- Current version: `1`

Top-level fields:

| Field | Type | Description |
|---|---|---|
| `schema_version` | integer | Always `1` for this contract. |
| `project` | string | Resolved project name. |
| `session_id` | string | Session that was reconciled. |
| `dry_run` | boolean | Whether the run was read-only. |
| `no_op` | boolean | Present when the run short-circuited. |
| `session_owned` | array | Tasks owned by the session. |
| `stale_review` | array | Bounded stale tasks for project-wide review. |
| `errors` | array | Non-fatal diagnostic strings. |

## Failure Behavior

The adapter must never block the host. On any failure it returns a diagnostic JSON object and exits with code `0`.

```json
{
  "schema_version": 1,
  "errors": ["description of failure"]
}
```

## Scope

The adapter is a thin invoker. All policy lives in the Go core.
