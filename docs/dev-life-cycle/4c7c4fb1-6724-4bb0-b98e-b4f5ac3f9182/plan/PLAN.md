# Cycle 1 — Define plan

## Identity and status
- **Cycle ID:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182
- **Sequence / execution order:** 1 / 1
- **Workflow:** standard
- **Status / phase:** in-progress / define
- **Created:** 2026-11-22
- **Updated:** 2026-11-25

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

The runtime is process-lifetime scoped under D-034; it has no library-managed graceful shutdown at process exit. The consumer-facing API remains:
- PrepareLog[TEvent, TChannel](ctx, newChannel) (TEvent, TChannel, error).
- TEvent and TChannel are concrete pointer types constrained by Event and Channel.
- newChannel has type func(chan Event) TChannel.
- Channel exposes Notifications() <-chan Event and Errors() <-chan error.
- The library owns persistence, replay, change detection, dispatch, and runtime lifecycle.

Approved project decisions are recorded in the project decision register and integrated into the current requirements and architecture. The register is provenance, not a substitute for those specifications.

## Authoritative-state reconciliation

### Reconciliation evidence

- **Repository revision inspected before this decision:** `edcb72595d3c06f48d3fc67244fa1df45e22ad6a` (`main`).
- **Project records reviewed in full:** `docs/project/requirements.md`, `architecture.md`, `decisions.md`, `acceptance.md`, and `benchmarks.md`.
- **Cycle records reviewed:** this plan, current handoff, and `docs/dev-life-cycle/cycle-index.json`.
- **Decision register:** D-001 through D-037 are Recorded; D-029 is refined by D-032 and D-030/D-031 are historical and superseded for process termination by D-034.
- **Cross-document corrections made:** stale Channel snapshot, Reader wake-up, retry, and backlog open-item claims were reconciled; the duplicate runtime-shutdown section was consolidated; D-034 replaces the former shutdown-drain sequence with process-lifetime runtime ownership and recovery after process termination; D-035 assigns UUID identity tag `0x03`; D-036 requires valid UTF-8 metadata keys and values and rejects invalid bytes without normalization; D-037 settles reflected Go type naming, alias resolution, opaque instantiated generic names, and the cross-toolchain compatibility limitation.
- **Validation boundary:** this is a semantic document reconciliation. No implementation, Go build, Go tests, or benchmark measurements have been run.

| Candidate item | Classification | Current evidence / next action |
|---|---|---|
| Channel snapshot ordering, identity immutability, concurrent mutation boundary | Resolved | D-027 and architecture section “Channel mutation and snapshot consistency” |
| Reader progression and wake-up rechecking | Resolved | D-028 and architecture section “Reader behavior” |
| Delivery retry and terminal failure | Resolved | D-029 refined by D-032 |
| Process-lifetime runtime ownership and process-termination recovery | Resolved | D-034 supersedes D-030/D-031 graceful-drain behavior for process termination |
| Bounded notification backlog and Reader backpressure | Resolved | D-033 |
| Acceptance scenarios and benchmark workload scenarios | Resolved | `acceptance.md` and `benchmarks.md` |
| UUID identity marker/tag | Resolved | D-035 and architecture sections “UUID identity representation” and “Canonical identity byte composition” |
| Canonical alias/generic type naming | Resolved | D-037 and architecture section “Canonical identity encoding” |
| Metadata UTF-8 policy | Resolved | D-036 and architecture section “Metadata UTF-8 policy” |
| Maximum framing sizes | Unresolved | Set explicit bounds for each u32-framed component/count before implementation |
| Exact internal registry-key composition | Unresolved/internal design detail | Confirm whether existing identity/registration rules are sufficient and defer exact representation to implementation if no observable contract depends on it |
| Numeric performance/memory targets and measurement criteria | Unresolved | Agree target environments, resource ceilings, repetitions, warm-up, and acceptance methodology |

The current requirements, architecture, decision register, acceptance matrix, and benchmark strategy have been reviewed together against the active cycle records.

### Resolved

- Public preparation and heterogeneous Channel notification/error contracts: D-001–D-003 and current requirements/architecture.
- Runtime process scope, preparation Context scope, shared-worker ownership, and terminal Reader error behavior: D-026.
- Channel snapshot FIFO/identity/consistency rules and consumer-owned synchronization for concurrent multi-field mutation: D-027.
- Continuous committed-sequence Reader progression and wake-up rechecking: D-028.
- Retry of the same pending occurrence with bounded backoff/finite retry limit and terminal per-Channel failure: D-032.
- Process-lifetime runtime ownership, no library-managed shutdown/drain at process exit, and recovery of committed pending notifications: D-034. D-030–D-031 are historical and superseded for process termination; their graceful-drain sequence is not part of the current runtime contract.
- Bounded notification backlog and Reader backpressure without blocking persistence or adding a second durable queue: D-033.
- Bottom-up acceptance scenarios and benchmark workload scenarios: [acceptance.md](../../../project/acceptance.md) and [benchmarks.md](../../../project/benchmarks.md).

### Unresolved

1. Remaining architecture details listed in [architecture.md](../../../project/architecture.md): maximum framing sizes and exact internal registry-key details.
2. Measurable performance and memory targets, representative target environments, and benchmark measurement/acceptance criteria.

### Explicitly not open

Channel snapshot consistency, Reader wake-up behavior, retry policy, process-lifetime runtime ownership, and notification backlog/backpressure are settled design decisions. Their numeric queue/retry parameters are internal implementation/test policy.

### Next Define work

Resolve maximum framing sizes and the remaining registry-key question, agree measurable performance/memory targets and their measurement criteria, then perform the final cross-document consistency review. Define exit still requires your explicit approval.

## Acceptance criteria

- [x] Product purpose, ownership, scope, and platform constraints are established.
- [x] Public preparation and Channel notification/error contracts are approved.
- [x] Configuration, field selection, identity, persistence, recovery, Reader, and queue design decisions are recorded.
- [x] Process-lifetime runtime ownership and process-termination recovery are specified; no internal shutdown trigger is required.
- [ ] Remaining architecture details are resolved or explicitly deferred.
- [x] Bottom-up acceptance matrix and scenario pass conditions are agreed and recorded.
- [x] Benchmark workload scenarios are defined and approved.
- [ ] Measurable performance/memory targets and measurement/acceptance criteria are agreed.
- [ ] Requirements and architecture are coherent, internally consistent, and implementation-ready.
- [ ] User explicitly approves exit from Define before implementation begins.

## Verification

No implementation, Go build, or Go tests have started.
