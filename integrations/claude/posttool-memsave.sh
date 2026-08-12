#!/usr/bin/env bash
# kanban-posttool-memsave.sh — PostToolUse hook that mirrors engram mem_save
# observations into the kanban event log as NOTES.
#
# These are always notes, never tasks. An engram observation records something
# that already happened (a bugfix, a decision, a discovery); none of them carry
# future work with a done condition. Mirroring them as task.new is what buried
# the backlog under hundreds of unclosable rows.
#
# Triggered on: PostToolUse, matcher "mem_save$"
# Disable: export KB_HOOKS_DISABLED=1
set -euo pipefail

[[ "${KB_HOOKS_DISABLED:-0}" == "1" ]] && exit 0
command -v kb >/dev/null 2>&1 || exit 0
command -v jq >/dev/null 2>&1 || exit 0

input=$(cat)

# Guard the matcher: PostToolUse matchers are regex, and mem_save_prompt must
# not be mirrored.
tool=$(printf '%s' "$input" | jq -r '.tool_name // ""' 2>/dev/null || true)
[[ "$tool" != *mem_save ]] && exit 0

title=$(printf '%s' "$input" | jq -r '.tool_input.title // ""' 2>/dev/null || true)
obs_type=$(printf '%s' "$input" | jq -r '.tool_input.type // "manual"' 2>/dev/null || true)
scope=$(printf '%s' "$input" | jq -r '.tool_input.topic_key // ""' 2>/dev/null || true)

[[ -z "$title" ]] && exit 0

# Resolve project using kb detect-project against the hook's cwd.
# This gives us the kanban registry name (e.g. "kanban", "LAYA-IA-SDK"),
# avoiding the basename/engram-project-name mismatch.
project=""
cwd=$(printf '%s' "$input" | jq -r '.cwd // ""' 2>/dev/null || true)
if [[ -n "$cwd" && -d "$cwd" ]]; then
    project=$(kb detect-project "$cwd" 2>/dev/null || true)
fi

# Degrade gracefully: if kb detect-project returns empty or fails, skip the
# event rather than writing with a wrong project name.
[[ -z "$project" ]] && exit 0

# Mirror only the observation types that describe real work.
case "$obs_type" in
    bugfix|decision|architecture|discovery|pattern|config)
        kb note "[engram] $title" \
            --body="auto-captured from engram mem_save (type: $obs_type)" \
            --scope="$scope" \
            -p "$project" \
            --source=hook-post 2>/dev/null || true
        ;;
    *)
        exit 0
        ;;
esac

exit 0
