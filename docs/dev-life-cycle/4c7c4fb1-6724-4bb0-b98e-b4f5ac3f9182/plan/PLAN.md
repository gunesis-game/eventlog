# Cycle 1 — Define plan

## Identity and status
- **Cycle ID:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182
- **Sequence / execution order:** 1 / 1
- **Workflow:** standard
- **Status / phase:** in-progress / define
- **Created:** 2026-11-22
- **Updated:** 2026-11-24

## Objective

Define eventlog's product requirements, public API, persistence guarantees, internal design, and acceptance criteria before implementation.

## Scope

This cycle establishes the library's product and technical baseline: public contracts, configuration, field selection, identity, persistence, recovery, replay/live delivery, runtime lifecycle, portability, and quality criteria. Implementation, builds, Go tests, dependency additions, and multi-process or distributed writers are outside this cycle's Define scope.

## Authoritative project documents

- [Requirements](../../../project/requirements.md) define product behavior and acceptance expectations.
- [Architecture](../../../project/architecture.md) defines the approved technical design.
- [Project decisions](../../../project/decisions.md) is the durable decision register and provenance record.
- [Acceptance matrix](../../../project/acceptance.md) defines the agreed bottom-up scenarios and pass conditions.

The specifications contain curated current rules. This plan tracks cycle scope and unresolved work; it does not reproduce the decision ledger or append approval/checkpoint narratives.

## Current design baseline

The consumer-facing API remains:
- PrepareLog[TEvent, TChannel](ctx, newChannel) (TEvent, TChannel, error).
- TEvent and TChannel are concrete pointer types constrained by Event and Channel.
- newChannel has type func(chan Event) TChannel.
- Channel exposes Notifications() <-chan Event and Errors() <-chan error.
- The library owns persistence, replay, change detection, dispatch, and runtime lifecycle.

Approved project decisions are recorded in the project decision register and integrated into the current requirements and architecture. The register is provenance, not a substitute for those specifications.

## Authoritative-state reconciliation

The current requirements, architecture, decision register, acceptance matrix, and benchmark strategy have been reviewed together against the active cycle records.

### Resolved

- Public preparation and heterogeneous Channel notification/error contracts: D-001–D-003 and current requirements/architecture.
- Runtime process scope, preparation Context scope, shared-worker ownership, and terminal Reader error behavior: D-026.
- Channel snapshot FIFO/identity/consistency rules and consumer-owned synchronization for concurrent multi-field mutation: D-027.
- Continuous committed-sequence Reader progression and wake-up rechecking: D-028.
- Retry of the same pending occurrence with bounded backoff/finite retry limit and terminal per-Channel failure: D-032.
- Shutdown drain failure reporting, fixed internal grace period, retained pending history, and sender exit before stream closure: D-030–D-031.
- Bounded notification backlog and Reader backpressure without blocking persistence or adding a second durable queue: D-033.
- Bottom-up acceptance scenarios and benchmark workload scenarios: [acceptance.md](../../../project/acceptance.md) and [benchmarks.md](../../../project/benchmarks.md).

### Unresolved

1. Exact internal runtime-shutdown trigger; no public shutdown API or process-wide signal interception is approved.
2. Remaining architecture details listed in [architecture.md](../../../project/architecture.md): UUID identity marker/tag, metadata UTF-8 policy, canonical alias/instantiated-generic naming, maximum framing sizes, and exact internal registry-key details.
3. Measurable performance and memory targets, representative target environments, and benchmark measurement/acceptance criteria.

### Explicitly not open

Channel snapshot consistency, Reader wake-up behavior, retry policy, shutdown-drain policy, and notification backlog/backpressure are not remaining design questions. Their numeric queue/retry parameters are internal implementation/test policy.

### Next Define work

Resolve the genuine remaining design questions, agree measurable performance/memory targets and their measurement criteria, then perform the final cross-document consistency review. Define exit still requires your explicit approval.

## Acceptance criteria

- [x] Product purpose, ownership, scope, and platform constraints are established.
- [x] Public preparation and Channel notification/error contracts are approved.
- [x] Configuration, field selection, identity, persistence, recovery, Reader, and queue design decisions are recorded.
- [ ] Exact internal runtime-shutdown trigger and remaining architecture details are resolved or explicitly deferred.
- [x] Bottom-up acceptance matrix and scenario pass conditions are agreed and recorded.
- [x] Benchmark workload scenarios are defined and approved.
- [ ] Measurable performance/memory targets and measurement/acceptance criteria are agreed.
- [ ] Requirements and architecture are coherent, internally consistent, and implementation-ready.
- [ ] User explicitly approves exit from Define before implementation begins.

## Verification

No implementation, Go build, or Go tests have started.
