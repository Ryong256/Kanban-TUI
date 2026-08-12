# session-reconciliation Specification

## Purpose

Session-priority reconciliation at close, bounded project-wide stale review, and staleness visibility in listing.

## Requirements

| Requirement | Rule |
|---|---|
| Session-First Pass | `kb reconcile --session-id S` first considers only tasks owned by S |
| Bounded Stale Review | After session pass, MAY review project-wide tasks with last-update > threshold (default 7d) |
| Staleness in List | `kb list` MUST render staleness age for active tasks past threshold |
| Dry-Run Mode | `--dry-run` MUST NOT append events; report only |

### Requirement: Session-First Pass

`kb reconcile --session-id S` MUST consider only tasks owned by S on the first pass.

#### Scenario: Session-owned priority

- GIVEN session S owns tasks T1 (active) and T2 (flagged unverified)
- WHEN `kb reconcile --session-id S --json` is invoked
- THEN the output's `session_owned` array contains T1 and T2
- AND no non-session tasks appear in `session_owned`

### Requirement: Bounded Stale Review

After the session pass, the system MAY review project-wide stale tasks, bounded to tasks with last-update older than a configurable threshold (default 7 days).

#### Scenario: Stale tasks surfaced

- GIVEN task T3 in active, last updated 10 days ago, not owned by S
- WHEN `kb reconcile --session-id S --project P --json` completes
- THEN the output's `stale_review` array contains T3

#### Scenario: Recent tasks excluded

- GIVEN task T4 in active, last updated 2 days ago
- WHEN reconcile runs
- THEN T4 is excluded from `stale_review`

### Requirement: Staleness in List

`kb list` MUST render staleness age for active tasks past the threshold.

#### Scenario: Staleness shown

- GIVEN active task T5 last updated 8 days ago
- WHEN `kb list` is invoked
- THEN T5's row includes an age indicator (e.g. "8d stale")

### Requirement: Dry-Run Mode

`kb reconcile --dry-run` MUST NOT append events; it only reports what would be acted upon.

#### Scenario: Dry-run is read-only

- GIVEN any task state
- WHEN `kb reconcile --dry-run --json` is invoked
- THEN the event log length is unchanged after the command
