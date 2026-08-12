# task-closure-evidence Specification

## Purpose

Closure requires task-declared evidence; unproved completions stay open with a visible signal; lifecycle transitions are honored.

## Requirements

| Requirement | Rule |
|---|---|
| Evidence-Proved Closure | `kb done <id>` MUST be rejected without evidence matching the declared `evidence_type` |
| Flagged Unproved Completion | Unverifiable evidence → task stays open, flag `completion-unverified`, visible in `kb list` |
| Lifecycle Transition Rules | Required transitions honored; inapplicable intermediate states MAY be skipped with evidence |

### Requirement: Evidence-Proved Closure

The system MUST accept `kb done <id>` only when the supplied evidence satisfies the task's declared `evidence_type`.

#### Scenario: Evidence satisfies declaration

- GIVEN task T with `evidence_type="test-output"`
- WHEN `kb done T --evidence "PASS ./..."` is invoked
- THEN a `task.done` event is appended with the evidence payload
- AND the task transitions to done

#### Scenario: Missing evidence rejected

- GIVEN task T with `evidence_type="test-output"`
- WHEN `kb done T` is invoked without `--evidence`
- THEN the system rejects with an error naming the missing evidence
- AND no `task.done` event is appended

#### Scenario: Failing run is not proof of closure

- GIVEN task T with `evidence_type="test-output"`
- WHEN `kb done T --evidence "FAIL ./..."` is invoked
- THEN the evidence does not validate
- AND the task is flagged `completion-unverified` rather than closed

#### Scenario: Task with no declaration closes without evidence

- GIVEN task T created before evidence declarations existed, with no `evidence_type`
- WHEN `kb done T` is invoked
- THEN the task closes
- AND no evidence is demanded, because none was ever declared

#### Scenario: Prose is not proof of closure

- GIVEN task T with `evidence_type="test-output"`
- WHEN evidence is prose that merely contains `pass`, `ok` or `fail` as a substring, such as "password" or "the build is broken"
- THEN the evidence does not validate
- AND the task is flagged `completion-unverified` rather than closed

### Requirement: Flagged Unproved Completion

When evidence is supplied but cannot be validated against the declared type, the task MUST remain open and be flagged as `completion-unverified`.

`completion-unverified` is a flag, not a status. It MUST NOT be written to the task's status: a status outside the enum drops the task from the board, which hides exactly the task that needs attention.

#### Scenario: Unverifiable evidence

- GIVEN task T with `evidence_type="test-output"` and evidence "manual check"
- WHEN `kb done T --evidence "manual check"` is invoked
- THEN the task stays in active and gains flag `completion-unverified`
- AND the task keeps its lifecycle status and stays visible on the board
- AND `kb list` renders the flag visibly

#### Scenario: A later valid attempt clears the flag

- GIVEN task T flagged `completion-unverified`
- WHEN `kb done T --evidence "PASS ./..."` is invoked with evidence that validates
- THEN the task closes
- AND the flag is no longer reported for it

### Requirement: Lifecycle Transition Rules

Required transitions (new → active → done) MUST be honored. Inapplicable intermediate states MAY be skipped when evidence justifies the skip.

#### Scenario: Skip intermediate state

- GIVEN task T in active with `evidence_type="review-approved"`
- WHEN `kb done T --evidence "approved-by:alice"` is invoked
- THEN the transition to done is accepted
