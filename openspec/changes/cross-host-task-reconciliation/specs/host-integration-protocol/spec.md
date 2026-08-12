# host-integration-protocol Specification

## Purpose

Provider-neutral core with thin host adapters; OpenCode and Claude adapters now; stable Codex contract without implementation.

## Requirements

| Requirement | Rule |
|---|---|
| Provider-Neutral Core | Reconciliation logic MUST live in Go core; adapters are thin invokers |
| OpenCode Adapter | Existing integration MUST be refactored to a thin invoker of the core |
| Claude Adapter | New adapter MUST invoke the same core entry points |
| Codex Contract | Stable versioned contract documented; no implementation required |
| Recursion Guard | Core detects `KB_RECONCILING=1` env marker and short-circuits |
| Idempotency | Reconciliation with same inputs yields the same observable state |
| Hooks Disabled | `KB_HOOKS_DISABLED=1` disables all adapter hooks instantly |

### Requirement: Provider-Neutral Core

The reconciliation and creation/closure logic MUST live in the Go core. Adapters MUST be thin invokers calling core entry points.

#### Scenario: Adapter is a thin invoker

- GIVEN the OpenCode adapter receives a session-close hook
- WHEN it invokes the core reconcile entry point
- THEN the core performs all logic; the adapter only marshals input/output

### Requirement: OpenCode Adapter

The existing OpenCode integration MUST be refactored to a thin invoker of the core.

#### Scenario: OpenCode session close

- GIVEN an OpenCode session ending with session-id S
- WHEN the adapter's close hook fires
- THEN `kb reconcile --session-id S --json` is invoked and its output is returned to the host

### Requirement: Claude Adapter

A new Claude Code adapter MUST be provided, invoking the same core entry points.

#### Scenario: Claude session close

- GIVEN a Claude Code session ending with session-id S
- WHEN the adapter's close hook fires
- THEN the same reconcile entry point is invoked and output returned

### Requirement: Codex Contract

A stable, versioned contract for a future Codex adapter MUST be documented. No implementation is required now.

#### Scenario: Contract document exists

- GIVEN the integrations directory
- WHEN inspected
- THEN a `codex/CONTRACT.md` file describes the entry point, input schema, output schema, and version

### Requirement: Recursion Guard

Adapters MUST NOT trigger reconciliation that re-invokes the adapter. The core MUST detect recursion via an environment marker and short-circuit.

#### Scenario: Recursion blocked

- GIVEN `KB_RECONCILING=1` is set in the environment
- WHEN an adapter invokes the core reconcile entry point
- THEN the core returns immediately with a no-op result
- AND no events are appended

### Requirement: Idempotency

Reconciliation MUST be idempotent: running it twice with the same inputs produces the same observable state.

#### Scenario: Double reconcile

- GIVEN session S with task T1 flagged unverified
- WHEN `kb reconcile --session-id S` is run twice
- THEN T1's final state after the second run equals its state after the first

### Requirement: Hooks Disabled Escape Hatch

Setting `KB_HOOKS_DISABLED=1` MUST disable all adapter hooks instantly.

#### Scenario: Hooks disabled

- GIVEN `KB_HOOKS_DISABLED=1` in the environment
- WHEN a session-close event occurs
- THEN no adapter runs and no reconcile is invoked
