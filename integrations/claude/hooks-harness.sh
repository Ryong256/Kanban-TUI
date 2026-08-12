#!/usr/bin/env bash
# Runtime harness for the session-start and posttool-memsave hooks.
#
# Both are best-effort and must never break a session, so every failure mode is
# checked here: missing project, wrong tool, unknown observation type, and the
# kill switch.
set -euo pipefail
DIR="$(dirname "$0")"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
LOG="$TMP/kb.log"

# Fake kb: records its argv and answers the two subcommands the hooks call.
cat > "$TMP/kb" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$KB_HARNESS_LOG"
case "$1" in
  detect-project) [[ "${KB_HARNESS_NO_PROJECT:-}" == "1" ]] && exit 1; echo "proj" ;;
  list) printf '%s\n' "${KB_HARNESS_LIST:-#1  a task}" ;;
  note) : ;;
esac
exit 0
EOF
chmod +x "$TMP/kb"
export PATH="$TMP:$PATH" KB_HARNESS_LOG="$LOG"
fail() { echo "FAIL: $1" >&2; exit 1; }

# --- session-start -------------------------------------------------------
ctx() { printf '{"cwd":"%s"}' "$PWD" | bash "$DIR/session-start.sh" | jq -r '.hookSpecificOutput.additionalContext // ""'; }

# The briefing is built with an unquoted heredoc, so a stray backtick or $( in
# the prose runs as a command and silently truncates what the agent receives.
# Nothing downstream fails, which is why this has to be checked here.
stderr=$(printf '{"cwd":"%s"}' "$PWD" | bash "$DIR/session-start.sh" 2>&1 >/dev/null)
[[ -z "$stderr" ]] || fail "session-start wrote to stderr (shell expansion in the briefing?): $stderr"

out=$(ctx)
[[ -n "$out" ]] || fail "session-start produced no context"
grep -q '#1  a task' <<<"$out" || fail "context omits the open task list"
grep -q -- '--closure' <<<"$out" || fail "context teaches kb add without --closure, which now fails"
grep -q -- '--evidence' <<<"$out" || fail "context teaches kb done without --evidence"

# Every command the briefing teaches must be one kb actually accepts.
grep -qF 'kb note "..."' <<<"$out" || fail "context lost the note command"
grep -qF 'kb move <id>' <<<"$out" || fail "context lost the move command"

# The task-vs-note rule has to survive even when the board is empty: that is
# exactly when the agent is about to file its first row.
out=$(KB_HARNESS_LIST="no open tasks" ctx)
grep -q 'A TASK is future work' <<<"$out" || fail "empty board dropped the task-vs-note rule"

[[ -z "$(printf '{"cwd":"%s"}' "$PWD" | KB_HOOKS_DISABLED=1 bash "$DIR/session-start.sh")" ]] \
  || fail "KB_HOOKS_DISABLED did not silence session-start"

# --- posttool-memsave ----------------------------------------------------
memsave() {
  printf '{"tool_name":"%s","cwd":"%s","tool_input":{"title":"%s","type":"%s","topic_key":"t"}}' \
    "$1" "$PWD" "${3:-a title}" "$2" | bash "$DIR/posttool-memsave.sh"
}
notes() { grep -c '^note ' "$LOG" 2>/dev/null || true; }

: > "$LOG"; memsave "mcp__engram__mem_save" "bugfix"
[[ "$(notes)" == "1" ]] || fail "a bugfix observation was not mirrored as a note"
grep -q 'note \[engram\]' "$LOG" || fail "mirrored row is not a note"

# mem_save_prompt matches the regex but must never be mirrored.
: > "$LOG"; memsave "mcp__engram__mem_save_prompt" "bugfix"
[[ "$(notes)" == "0" ]] || fail "mem_save_prompt was mirrored"

# Observation types that carry no work are skipped.
: > "$LOG"; memsave "mcp__engram__mem_save" "manual"
[[ "$(notes)" == "0" ]] || fail "an unlisted observation type was mirrored"

# Without a resolvable project, writing would land under the wrong name.
: > "$LOG"; KB_HARNESS_NO_PROJECT=1 memsave "mcp__engram__mem_save" "decision"
[[ "$(notes)" == "0" ]] || fail "wrote a note despite an unresolvable project"

: > "$LOG"; KB_HOOKS_DISABLED=1 memsave "mcp__engram__mem_save" "bugfix"
[[ "$(notes)" == "0" ]] || fail "KB_HOOKS_DISABLED did not silence posttool-memsave"

echo "OK"
