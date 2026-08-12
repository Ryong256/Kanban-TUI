#!/usr/bin/env bash
# Claude Code kanban adapter — thin invoker of the kb core.
set -euo pipefail

if [[ "${KB_HOOKS_DISABLED:-}" == "1" ]]; then
  exit 0
fi

if [[ "${KB_RECONCILING:-}" == "1" ]]; then
  echo '{"schema_version":1,"no_op":true}'
  exit 0
fi

# Claude sets these via mcp hook context.
SESSION_ID="${KB_SESSION_ID:-}"
PROJECT_DIR="${KB_PROJECT_DIR:-${PWD}}"

if [[ -z "$SESSION_ID" ]]; then
  echo '{"schema_version":1,"errors":["session-id is required"]}' >&2
  exit 0
fi

# Resolve project read-only; never auto-register from an adapter.
PROJECT="$(kb detect-project "$PROJECT_DIR" 2>/dev/null || true)"
if [[ -z "$PROJECT" ]]; then
  echo '{"schema_version":1,"errors":["project not detected"]}' >&2
  exit 0
fi

# Run reconciliation. The adapter itself is unmarked; only a core child would inherit
# KB_RECONCILING=1 if the core ever spawned a nested reconcile.
exec kb reconcile \
  --project "$PROJECT" \
  --session-id "$SESSION_ID" \
  --json
