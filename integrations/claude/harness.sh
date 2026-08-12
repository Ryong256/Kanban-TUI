#!/usr/bin/env bash
# Runtime harness: Claude adapter must invoke reconcile unmarked and preserve literal argv.
set -euo pipefail
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
LOG="$TMP/log"
KB="$TMP/kb"
cat > "$KB" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "${KB_HARNESS_LOG}"
[[ "$1" == "detect-project" ]] && { echo "proj"; exit; }
[[ "$1" == "reconcile" ]] && {
  [[ "${KB_RECONCILING:-}" == "1" ]] && { echo '{"schema_version":1,"no_op":true}'; exit; }
  echo '{"schema_version":1,"project":"proj","session_id":"'"$3"'","session_owned":[],"stale_review":[],"errors":[]}'
  exit
}
exit 1
EOF
chmod +x "$KB"
export KB_HARNESS_LOG="$LOG" PATH="$TMP:$PATH" KB_SESSION_ID='ses; touch /tmp/pwned' KB_PROJECT_DIR='/repo with spaces'
out=$(bash "$(dirname "$0")/kanban.sh")
[[ "$out" == *'"no_op":true'* ]] && { echo "FAIL: first reconciliation suppressed" >&2; exit 1; }
[[ "$out" != *'"schema_version":1'* ]] && { echo "FAIL: missing schema-v1 output" >&2; exit 1; }
call=$(grep '^reconcile ' "$LOG" || true)
[[ "$call" == 'reconcile --project proj --session-id ses; touch /tmp/pwned --json' ]] || { echo "FAIL: argv altered: $call" >&2; exit 1; }
echo "OK"
