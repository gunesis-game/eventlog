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
- [Acceptance matrix](../../../project/acceptance.md) — agreed bottom-up test layers and pass conditions.
Approved outcomes are integrated into the relevant specification topics. This handoff is only the current cycle checkpoint; it is not a second decision ledger or specification.

## Current state

The public consumer API remains:
    PrepareLog[TEvent, TChannel](ctx, newChannel) (TEvent, TChannel, error)
    Notifications() <-chan Event
    Errors() <-chan error
Both generic arguments are concrete pointer types constrained by their respective interfaces. The Channel notification stream carries heterogeneous concrete Event values through the common Event interface. The approved identity, registration, persistence, recovery, Reader, and internal queue design is captured in the architecture.

## Authoritative-state reconciliation

The current requirements, architecture, decisions, acceptance matrix, and benchmark strategy have been checked together. The following is the reconciled Define state.

### Resolved

- D-026: process-scoped runtime ownership, preparation Context scope, shared-worker ownership, and terminal Reader error behavior.
- D-027: Channel snapshots use the identity-keyed bounded FIFO mechanism; consumer synchronization is required for concurrent multi-field mutation.
- D-028: one continuous committed-sequence Reader progression; wake-up hints only trigger authoritative-log rechecks.
- D-030–D-031: shutdown-drain failure reporting, fixed internal grace period, timeout behavior, retained pending history, and sender exit before stream closure.
- D-032: retry the same pending occurrence with bounded backoff and finite retry limit; no Reader advancement or overtaking; retry exhaustion terminates only the affected Channel Reader.
- D-033: bounded notification backlog with Reader backpressure; persistence continues and the authoritative append log remains the recovery source.
- The acceptance matrix and benchmark workload scenarios are agreed and recorded in [acceptance.md](../../../project/acceptance.md) and [benchmarks.md](../../../project/benchmarks.md).

### Unresolved

- Exact internal runtime-shutdown trigger. The public API has no shutdown method, and process-wide signal interception is not approved.
- Remaining architecture details: UUID identity marker/tag, metadata UTF-8 policy, canonical alias/instantiated-generic naming, maximum framing sizes, and exact internal registry-key details.
- Measurable performance and memory targets, representative target environments, and benchmark measurement/acceptance criteria.

### Next action

Resolve the genuine remaining design questions, agree measurable performance/memory targets and measurement criteria, then perform the final cross-document consistency review. Do not begin implementation or exit Define until the remaining work is settled and you explicitly approve the phase transition.

## Verification

- No implementation, Go build, or Go tests have been run.

## Next action

Bounded notification backlog with Reader backpressure is approved and recorded as D-033. The complete bottom-up acceptance matrix is now agreed and recorded in [acceptance.md](../../../project/acceptance.md), covering disk/storage, persistence/transactions, metadata/recovery, core event-log behavior, Channel/Reader, notification delivery, runtime lifecycle, and public API/end-to-end integration. Next, define benchmark workloads and measurable performance/memory targets, then complete the final consistency review. D-025 through D-033 are recorded. Do not begin implementation until Define is complete and the user explicitly approves exit.
