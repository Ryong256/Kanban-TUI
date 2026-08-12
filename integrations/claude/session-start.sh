#!/usr/bin/env bash
# kanban-session-start.sh — SessionStart hook that briefs Claude with the
# current project's open tasks and the rules for writing to kb. Read-only and
# best-effort.
#
# Disable: export KB_HOOKS_DISABLED=1
set -euo pipefail

[[ "${KB_HOOKS_DISABLED:-0}" == "1" ]] && exit 0
command -v kb >/dev/null 2>&1 || exit 0
command -v jq >/dev/null 2>&1 || exit 0

input=$(cat)
cwd=$(printf '%s' "$input" | jq -r '.cwd // ""' 2>/dev/null || true)
[[ -z "$cwd" || ! -d "$cwd" ]] && exit 0

open=$(cd "$cwd" && kb list 2>/dev/null || true)

# The procedure is emitted even with an empty board: a project with no open
# tasks is exactly the case where the agent is about to create the first one,
# and it needs the task-vs-note rule before it does.
if [[ -z "$open" || "$open" == "no open tasks" ]]; then
    open_block="KANBAN — no open tasks for this project."
else
    open_block="KANBAN — open tasks for this project:

$open"
fi

context=$(cat <<EOF
$open_block

KANBAN PROCEDURE (active during this session):

FIRST, decide what you are recording. This is the rule that matters most:

  A TASK is future work. It must name something that is NOT done yet and has a
  done condition someone can check. If you cannot finish the sentence "this is
  closed when ___", it is NOT a task.

  A NOTE is something that already happened: a PR that merged, a root cause you
  confirmed, a decision you made, a state of the world you observed. Notes never
  enter the board and never need closing.

  Anything phrased in past tense or as a status report — "shipped X", "X is
  ARCHIVED", "merged #123", "wrote the terraform", "X proven broken" — is a NOTE.
  Filing those as tasks is the single biggest source of backlog rot. When in
  doubt, write a note: an unwritten task costs one reminder, a fake task costs
  a permanent row nobody can ever close.

COMMANDS:
- Something happened, nothing to act on → kb note "..." --body="detail" --scope="<feature>"
- Real future work discovered        → kb add "..." --body="why" --scope="<feature>" --closure="this is closed when ___" --evidence=test-output
- Work started/progressed on a task  → kb move <id> <status>   (backlog, in_progress, testing, done)
- Task actually finished             → kb done <id> --evidence "<the real output that proves it>"
- Scope shift                        → kb event --type=scope.shift --title="from X to Y" --body="reason" --scope="<feature>"
- Scope expansion                    → kb event --type=scope.expand --title="..." --body="trigger" --scope="<feature>"

CLOSURE AND EVIDENCE:
  --closure is the sentence "this is closed when ___". A task without one is a
  wish, not a task, and kb refuses to create it.
  --evidence declares what will prove it: test-output or review-approved.
  On kb done, the evidence must actually prove the work: real test-runner output
  showing a pass, or "approved-by:<name>". A failing run, or prose that merely
  contains the word "ok", is rejected and the task stays open flagged
  completion-unverified. Do not invent evidence to close a row.
  Tasks filed before closure conditions existed declare none, and close with a
  plain "kb done <id>" and no evidence.

BEFORE creating a task, check the open list above. If what you are about to file
is already there, move it or close it instead of adding a second row. Duplicates
are the second biggest source of rot.

WHEN YOU FINISH the work an open task describes, close it in the same turn:
kb done <id>. Do not defer it to the end of the session — by then you will not
remember which id it was.

NEVER reference kanban tasks anywhere in the repo — not in git commits, PR
titles/bodies, code comments, identifiers, or docs. kb is a local-only tool and
those task IDs are meaningless to anyone cloning the repo.
EOF
)

jq -n --arg ctx "$context" '{
  hookSpecificOutput: {
    hookEventName: "SessionStart",
    additionalContext: $ctx
  }
}'
