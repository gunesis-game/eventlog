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

- **Repository revision inspected before this decision:** `8a142a6b2483f11ac6e248835c14bef60a828aad` (`main`).
- **Project records reviewed in full:** `docs/project/requirements.md`, `architecture.md`, `decisions.md`, `acceptance.md`, and `benchmarks.md`.
- **Cycle records reviewed:** this plan, current handoff, and `docs/dev-life-cycle/cycle-index.json`.
- **Decision register:** D-001 through D-041 are Recorded (D-040 and D-041 cover the approved benchmark protocol and evidence policy); D-029 is refined by D-032 and D-030/D-031 are historical and superseded for process termination by D-034.
- **Cross-document corrections made:** stale Channel snapshot, Reader wake-up, retry, backlog, and lifecycle claims were reconciled; D-034 establishes process-lifetime runtime ownership and recovery after process termination; D-035–D-039 settle UUID identity framing, metadata UTF-8, type naming, identity bounds, and observable registry behavior. D-040 approves one warm-up plus five measured repetitions for full/manual benchmarks, one informational smoke execution, summary statistics, and diagnostic resource ceilings. D-041 approves 20-day artifact retention and materially comparable baseline selection. The acceptance strategy now distinguishes normal replay-to-live continuity from permitted crash-window duplicates, and architecture explicitly specifies per-Channel stream closure after a terminal Reader error while preserving no stream closure merely for process termination.
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
| Maximum framing sizes | Resolved | D-038 and architecture section “Canonical identity framing limits (v1)” |
| Exact internal registry-key composition | Resolved/deferred to implementation | D-039: representation is internal; architecture specifies required registration, identity, reuse, and Channel-isolation behavior |
| Numeric performance pass/fail thresholds and statistical measurement criteria | Unresolved | Initial workload profiles and resource budgets are approved in `benchmarks.md`; still agree repetitions, warm-up, variance/acceptance methodology, and whether any thresholds are appropriate |

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

No additional product or architecture design decision is currently identified by the cross-document review. Numeric product performance and memory targets are intentionally deferred until an implementation exists and representative baselines can be measured; they are not a prerequisite to completing Define and must not be invented from the diagnostic runner ceilings.

### Explicitly not open

Channel snapshot consistency, Reader wake-up behavior, retry policy, process-lifetime runtime ownership, and notification backlog/backpressure are settled design decisions. Their numeric queue/retry parameters are internal implementation/test policy.

### Remaining Define work

Complete the final semantic review of the reconciled specifications and cycle records. If no contradiction or unrecorded design decision remains, present the Define exit assessment for your explicit approval. No implementation or phase transition is authorized by this plan update.

## Acceptance criteria

- [x] Product purpose, ownership, scope, and platform constraints are established.
- [x] Public preparation and Channel notification/error contracts are approved.
- [x] Configuration, field selection, identity, persistence, recovery, Reader, and queue design decisions are recorded.
- [x] Process-lifetime runtime ownership and process-termination recovery are specified; no internal shutdown trigger is required.
- [x] Remaining architecture details are resolved or explicitly deferred.
- [x] Bottom-up acceptance matrix and scenario pass conditions are agreed and recorded.
- [x] Benchmark workload scenarios are defined and approved.
- [x] Initial benchmark workload profiles and resource budgets are agreed and recorded.
- [x] Benchmark measurement protocol, statistical reporting, artifact retention, and comparable-run selection are agreed and recorded in D-040/D-041.
- [x] Numeric product performance/memory targets are explicitly deferred until representative post-implementation baselines; diagnostic resource ceilings are not acceptance thresholds.
- [ ] Requirements and architecture are coherent, internally consistent, and implementation-ready.
- [ ] User explicitly approves exit from Define before implementation begins.

## Verification

No implementation, Go build, or Go tests have started.
