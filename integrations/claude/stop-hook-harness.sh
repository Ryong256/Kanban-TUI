#!/usr/bin/env bash
# Runtime harness: the Stop hook must translate adapter JSON into a hook decision,
# stay silent when there is nothing to say, and never block twice in one session.
set -euo pipefail
HOOK="$(dirname "$0")/stop-hook.sh"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
export XDG_RUNTIME_DIR="$TMP"

# kb only has to exist for the hook's preflight check.
printf '#!/usr/bin/env bash\nexit 0\n' > "$TMP/kb"
chmod +x "$TMP/kb"
export PATH="$TMP:$PATH"

fake_adapter() {
    printf '#!/usr/bin/env bash\ncat <<'"'"'JSON'"'"'\n%s\nJSON\n' "$1" > "$TMP/adapter"
    chmod +x "$TMP/adapter"
}
run() {
    printf '{"session_id":"%s","cwd":"%s"}' "$1" "$PWD" | KB_ADAPTER="$TMP/adapter" bash "$HOOK"
}
fail() { echo "FAIL: $1" >&2; exit 1; }

# A stale task must reach the agent as a blocking decision naming that task.
fake_adapter '{"schema_version":1,"project":"p","session_id":"s","session_owned":[],
  "stale_review":[{"id":42,"title":"old thing","status":"backlog","stale_age_hours":240}],"errors":[]}'
out=$(run ses-stale)
[[ "$(printf '%s' "$out" | jq -r '.decision')" == "block" ]] || fail "expected a blocking decision: $out"
printf '%s' "$out" | jq -r '.reason' | grep -q '#42' || fail "reason does not name the stale task"
printf '%s' "$out" | jq -r '.reason' | grep -q '10d' || fail "reason does not carry the stale age"

# Second stop in the same session must not block again.
[[ -z "$(run ses-stale)" ]] || fail "hook blocked twice in one session"

# An empty board has nothing to audit.
fake_adapter '{"schema_version":1,"project":"p","session_id":"s","session_owned":[],"stale_review":[],"errors":[]}'
[[ -z "$(run ses-empty)" ]] || fail "hook blocked with nothing to report"

# A suppressed nested run is silent.
fake_adapter '{"schema_version":1,"no_op":true}'
[[ -z "$(run ses-noop)" ]] || fail "hook blocked on a no-op run"

# An unverified completion must be surfaced.
fake_adapter '{"schema_version":1,"project":"p","session_id":"s",
  "session_owned":[{"id":7,"title":"claimed done","status":"in_progress","flags":["completion-unverified"]}],
  "stale_review":[],"errors":[]}'
out=$(run ses-flag)
printf '%s' "$out" | jq -r '.reason' | grep -q 'sin verificar' || fail "reason omits the unverified completion"

# Garbage from the adapter must never block the user's session.
fake_adapter 'not json at all'
[[ -z "$(run ses-garbage)" ]] || fail "hook blocked on unparseable adapter output"

# The kill switch wins over everything.
fake_adapter '{"schema_version":1,"stale_review":[{"id":1,"title":"x","status":"backlog","stale_age_hours":999}]}'
[[ -z "$(KB_HOOKS_DISABLED=1 run ses-off)" ]] || fail "KB_HOOKS_DISABLED did not silence the hook"

echo "OK"
