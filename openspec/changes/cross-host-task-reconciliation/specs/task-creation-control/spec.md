# task-creation-control Specification

## Purpose

Gates task creation with verifiable closure condition, normalization-based duplicate blocking, and session-scoped placement.

## Requirements

| Requirement | Rule |
|---|---|
| Closure Condition Declaration | `kb add` MUST reject when `--closure` or `--evidence` is missing; no event appended |
| Normalized Duplicate Blocking | Block when an active task has normalization-equivalent (project, scope, title, closure); basic normalization only — fuzzy matching, embeddings, NLP MUST NOT be used |
| Session-Scoped Placement | `--session-id` MAY be provided; `task.new` event carries `session_id` when present |

### Requirement: Closure Condition Declaration

The system MUST reject `kb add` when no closure condition or evidence type is declared.

#### Scenario: Missing closure condition

- GIVEN a user invokes `kb add --title "X"` without `--closure` or `--evidence`
- WHEN the command is processed
- THEN the system returns a non-zero exit code and an error message naming the missing fields
- AND no event is appended to the log

#### Scenario: Valid declaration

- GIVEN `kb add --title "X" --closure "tests pass" --evidence "test-output"`
- WHEN processed
- THEN a `task.new` event is appended with `closure_condition` and `evidence_type` fields
- AND the task is placed in the active column

### Requirement: Normalized Duplicate Blocking

The system MUST compare (project, scope, title, closure) after deterministic basic normalization. Basic normalization MUST ignore letter case, redundant whitespace, and trivial punctuation. Fuzzy matching, embeddings, NLP semantic similarity, and inferred intent MUST NOT be used. Tasks with equivalent normalized values MUST be blocked as duplicates; tasks with materially different normalized values MUST be allowed.

#### Scenario: Exact duplicate blocked

- GIVEN an active task with title "X", scope "pkg/a", closure "tests pass"
- WHEN `kb add --title "X" --scope "pkg/a" --closure "tests pass" --evidence "test-output"` is invoked
- THEN the system rejects with a duplicate-blocked error listing the existing task ID
- AND no event is appended

#### Scenario: Case-variant duplicate blocked

- GIVEN an active task with title "Fix Auth Bug", scope "pkg/a", closure "tests pass"
- WHEN `kb add --title "fix auth bug" --scope "pkg/a" --closure "Tests Pass" --evidence "test-output"` is invoked
- THEN the system rejects as duplicate

#### Scenario: Whitespace-variant duplicate blocked

- GIVEN an active task with title "Fix Auth Bug", scope "pkg/a", closure "tests pass"
- WHEN `kb add --title "  Fix   Auth  Bug " --scope "pkg/a" --closure "tests  pass" --evidence "test-output"` is invoked
- THEN the system rejects as duplicate

#### Scenario: Punctuation-variant duplicate blocked

- GIVEN an active task with title "Fix auth bug", scope "pkg/a", closure "tests pass"
- WHEN `kb add --title "Fix, auth bug." --scope "pkg/a" --closure "tests pass!" --evidence "test-output"` is invoked
- THEN the system rejects as duplicate

#### Scenario: Materially different task allowed

- GIVEN an active task with title "Fix Auth Bug", scope "pkg/a", closure "tests pass"
- WHEN `kb add --title "Add Auth Tests" --scope "pkg/a" --closure "coverage > 80%" --evidence "test-output"` is invoked
- THEN creation proceeds normally

#### Scenario: Different scope allowed

- GIVEN an active task with title "X", scope "pkg/a", closure "tests pass"
- WHEN `kb add --title "X" --scope "pkg/b" --closure "tests pass" --evidence "test-output"` is invoked
- THEN creation proceeds normally

### Requirement: Session-Scoped Placement

The system MAY accept `--session-id`; when provided, the task is tagged with session ownership. Tasks without `--session-id` remain unowned.

#### Scenario: Session-owned task

- GIVEN `kb add --title "X" --session-id "ses_abc" --closure "c" --evidence "e"`
- WHEN processed
- THEN the `task.new` event includes `session_id="ses_abc"`
