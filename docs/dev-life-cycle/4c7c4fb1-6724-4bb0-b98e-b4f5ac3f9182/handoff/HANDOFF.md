# Cycle 1 — Current handoff

## Cycle identity and status

- **Cycle ID:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182
- **Sequence / execution order:** 1 / 1
- **Workflow:** standard
- **Registry status / phase:** in-progress / define
- **Last updated:** 2026-11-24

## Objective

Complete eventlog's requirements and technical design before implementation.

## Current repository state

- **Branch:** main
- **Implementation state:** no implementation has started.
- **Build/test state:** no Go build or Go tests have started.
- **Cycle registry:** Cycle 1 remains in progress at Define.

## Authoritative project records

- [Requirements](../../../project/requirements.md) — product behavior and acceptance expectations.
- [Architecture](../../../project/architecture.md) — current approved technical design.
- [Project decisions](../../../project/decisions.md) — durable decision register and provenance.
Approved outcomes are integrated into the relevant specification topics. This handoff is only the current cycle checkpoint; it is not a second decision ledger or specification.

## Current state

The public consumer API remains:
    PrepareLog[TEvent, TChannel](ctx, newChannel) (TEvent, TChannel, error)
    Notifications() <-chan Event
    Errors() <-chan error
Both generic arguments are concrete pointer types constrained by their respective interfaces. The Channel notification stream carries heterogeneous concrete Event values through the common Event interface. The approved identity, registration, persistence, recovery, Reader, and internal queue design is captured in the architecture.

## Remaining Define work

- Runtime process scope, preparation Context scope, shared-worker ownership, and terminal Reader error behavior are approved and recorded as D-026. The exact internal shutdown trigger and ordered graceful cleanup remain unresolved.
- Channel snapshots now use the same identity-keyed bounded FIFO dispatch mechanism as Event snapshots (D-027); consumer synchronization remains required for concurrent multi-field mutation.
- Delivery retry policy is approved and recorded as D-032: retry the same pending occurrence with internal bounded backoff and a finite retry limit; do not advance the Reader or deliver later occurrences while retrying. Retry exhaustion terminates that Channel Reader and reports through Errors(); committed pending history remains recoverable, and other Channel Readers continue independently. Numeric limits and classification remain internal. D-030 defines shutdown-drain failure reporting; D-031 defines a fixed internal graceful-drain grace period, timeout abort, best-effort Errors() reporting, retained pending history, and sender exit before stream closure. No public timeout setting.
- Complete acceptance matrix and benchmark/performance/memory targets.
- Final consistency review and explicit approval to exit Define.

## Verification

- No implementation, Go build, or Go tests have been run.

## Next action

Bounded notification backlog with Reader backpressure is now approved and recorded as D-033: the in-memory pending buffer is finite, a full buffer holds the Reader until capacity returns, persistence continues, committed pending history remains authoritative in the append log, and no second durable queue is introduced. Next, complete the acceptance matrix, benchmark/performance/memory targets, and final consistency review. D-025 through D-033 are recorded. Do not begin implementation until Define is complete and the user explicitly approves exit.
