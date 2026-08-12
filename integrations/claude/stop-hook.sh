#!/usr/bin/env bash
# Claude Code Stop hook — turns a reconcile run into an audit the agent can act on.
#
# kanban.sh speaks the adapter contract: it prints schema-v1 JSON. Claude Code
# hooks speak a different one — stdout is a control channel read for `decision`
# and `reason`. This wrapper is the translation between the two, and it is the
# only place that knows about Claude Code's hook protocol.
#
# Disable: export KB_HOOKS_DISABLED=1
# Reset the per-session marker: rm "$XDG_RUNTIME_DIR/kb-hooks/reconcile-<session>"
set -euo pipefail

[[ "${KB_HOOKS_DISABLED:-}" == "1" ]] && exit 0
command -v kb >/dev/null 2>&1 || exit 0
command -v jq >/dev/null 2>&1 || exit 0

ADAPTER="${KB_ADAPTER:-$(dirname "$0")/kanban.sh}"
[[ -x "$ADAPTER" ]] || exit 0

input=$(cat)
session_id=$(printf '%s' "$input" | jq -r '.session_id // ""' 2>/dev/null || true)
cwd=$(printf '%s' "$input" | jq -r '.cwd // ""' 2>/dev/null || true)
[[ -z "$session_id" ]] && exit 0
[[ -n "$cwd" && -d "$cwd" ]] || cwd="$PWD"

# One audit per session. Without the marker the hook blocks the stop it just
# caused, and the session can never end.
marker_dir="${XDG_RUNTIME_DIR:-/tmp}/kb-hooks"
mkdir -p "$marker_dir" 2>/dev/null || exit 0
marker="$marker_dir/reconcile-$session_id"
[[ -f "$marker" ]] && exit 0
find "$marker_dir" -type f -mtime +7 -delete 2>/dev/null || true
touch "$marker"

result=$(KB_SESSION_ID="$session_id" KB_PROJECT_DIR="$cwd" "$ADAPTER" 2>/dev/null || true)
printf '%s' "$result" | jq -e . >/dev/null 2>&1 || exit 0

# A suppressed nested run has nothing to say.
[[ "$(printf '%s' "$result" | jq -r '.no_op // false')" == "true" ]] && exit 0

owned=$(printf '%s' "$result" | jq -r '
  .session_owned // [] | map("  #\(.id) [\(.status)] \(.title)") | join("\n")')
stale=$(printf '%s' "$result" | jq -r '
  .stale_review // [] | map("  #\(.id) [\(.status)] \(.title) — \((.stale_age_hours // 0) / 24 | floor)d sin moverse") | join("\n")')
flagged=$(printf '%s' "$result" | jq -r '
  [(.session_owned // []) + (.stale_review // []) | .[] | select((.flags // []) | index("completion-unverified"))]
  | map("  #\(.id) \(.title)") | join("\n")')

# Nothing owned, nothing stale, nothing flagged — let the session end quietly.
[[ -z "$owned" && -z "$stale" && -z "$flagged" ]] && exit 0

sections=""
[[ -n "$owned" ]] && sections+="Tareas de esta sesión:

$owned

¿Alguna quedó terminada? → kb done <id> --evidence \"<salida real del test>\"
¿Alguna avanzó sin terminarse? → kb move <id> <status>

"
[[ -n "$flagged" ]] && sections+="Cierres sin verificar (la evidencia no validó):

$flagged

Corregí la evidencia o dejá la tarea abierta. No la cierres a mano.

"
[[ -n "$stale" ]] && sections+="Sin moverse hace tiempo:

$stale

¿Siguen siendo trabajo real? Si no, kb rm <id>. Si sí, movelas o dejalas.

"

reason=$(cat <<EOF
Kanban reconcile (una sola vez por sesión).

${sections}Trabajo futuro nuevo, sólo si tiene condición de cierre verificable:
  kb add "..." --closure="esto se cierra cuando ___" --evidence=test-output

Lo que ya pasó y no requiere acción va como nota, NO como tarea:
  kb note "..." --body="detalle"

Si no aplica nada, respondé "reconcile-clean".
EOF
)

jq -n --arg msg "$reason" '{decision: "block", reason: $msg}'
