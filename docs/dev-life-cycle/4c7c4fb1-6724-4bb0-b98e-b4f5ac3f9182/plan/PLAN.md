# Cycle 1 — Implementation plan

## Identity and status
- **Cycle ID:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182
- **Sequence / execution order:** 1 / 1
- **Workflow:** standard
- **Status / phase:** in-progress / implement
- **Created:** 2026-11-22
- **Updated:** 2026-11-25

## Objective

Implement eventlog according to the approved requirements, public API, architecture, acceptance criteria, testing strategy, performance strategy, and Git workflow.

## Scope

Implement the approved single-process, disk-backed append-only event log in architecture-aligned, testable increments. Preserve the approved consumer-facing API and behavior. Multi-process/distributed writers and any public API changes remain out of scope unless separately approved.

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
| Benchmark measurement protocol and evidence policy | Resolved | D-040 approves the measurement/statistical protocol and diagnostic ceilings; D-041 approves 20-day artifact retention and comparable-run selection. Numeric product targets are deferred until representative implementation baselines exist |

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

### Define exit evidence

The user explicitly approved exiting Define and transitioning Cycle 1 to Implement. D-040/D-041 (benchmark methodology/evidence), D-042 (testing strategy), and D-043 (Git workflow) are recorded and reflected in their owning specifications. Final semantic review found no remaining Define-level contradiction. Repository-protection/ruleset and signing/authorship verification remain operational follow-ups; project Go test/build/lint workflows, suite locations, exact commands, and runner configuration are implementation tasks. Numeric product performance/memory targets remain deferred until representative baselines and separate approval.

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
- [x] Project testing strategy is explicitly approved and recorded as D-042; implementation-specific suite locations, commands, and CI configuration remain deferred until implementation.
- [x] Numeric product performance/memory targets are explicitly deferred until representative post-implementation baselines; diagnostic resource ceilings are not acceptance thresholds.
- [x] Requirements and architecture are coherent, internally consistent, and implementation-ready after final cross-document review; implementation-specific operational follow-ups remain explicitly deferred.
- [x] User explicitly approved exit from Define and transition to Implement.

## Implement phase

### Governing revisions and gates

- Architecture: `docs/project/architecture.md` (current main revision at phase entry).
- Requirements: `docs/project/requirements.md`.
- Acceptance scenarios: `docs/project/acceptance.md`.
- Coding standard: `docs/standards/coding.md` (blob `19e2bebe182ec6d7b3e9fa645a5469f07b5ee4ac`).
- Testing strategy: D-042, `docs/project/testing-strategy.md`; fast, integration, and mandatory full-verification tiers. No arbitrary coverage threshold.
- Performance strategy: D-040/D-041, `docs/project/performance-strategy.md` and `docs/project/benchmarks.md`; smoke is informational, full/manual uses one warm-up plus five measured repetitions; numeric product targets deferred.
- Git workflow: D-043, `docs/project/git-workflow.md`; main-only direct commits, no branches or pull requests, never bypass protections.

### Ordered implementation work

1. [x] Inspect repository contents and Go toolchain/runtime evidence; establish module metadata. The module is `github.com/gunesis-game/eventlog`, declares `go 1.18` as the language floor required by generics, and has no external dependencies. CI resolves stable Go; the initial verified runner toolchain was Go 1.27.4.
2. [ ] Define canonical v1 record/field schemas and their repository path from the approved architecture; then implement strict record serialization and framing with focused tests. The architecture requires canonical standalone schemas embedded with `go:embed`, but none exist in the repository yet.
3. Implement storage and canonical framing primitives bottom-up, with focused tests for file operations, encoding bounds, checksums/validation, and corruption handling as specified by architecture and acceptance.
4. Implement append transaction commit/recovery and metadata reconstruction; test atomicity, interrupted writes, restart recovery, and authoritative committed sequence.
5. Implement Event identity, typed field encoding/reconstruction, registration, and Channel snapshot/change observation according to the public API and architecture.
6. Implement per-Channel Reader progression, replay-to-live handoff, bounded notification backlog, retry/backoff, and terminal error behavior; test ordering, isolation, backpressure, and the documented crash-window duplicate boundary.
7. Implement process-lifetime runtime ownership and `PrepareLog` integration; add consumer-defined Event/Channel public API and lifecycle/recovery tests without adding a public shutdown control.
8. [x] Establish initial verified Go test/build/vet commands and Linux/Windows CI. The first run passed on both platforms; no acceptance test files exist yet. Expand suite locations and full-verification gates as implementation adds tests. Keep governance CI evidence distinct from Go verification.
9. Add approved benchmark workloads and evidence capture after representative behavior exists; do not invent numeric product performance or memory thresholds.

Implementation proceeds in small increments. At each boundary, update the relevant fast tests and acceptance evidence. Changes that require an architectural or public API deviation stop for explicit design approval before implementation continues.

## Verification

Initial implementation setup is committed: `go.mod`, root package documentation, and Linux/Windows Go verification workflow. `go test ./...`, `go build ./...`, and `go vet ./...` passed on Go 1.27.4 for both platforms at `4fbf2837906b0e25c635ef00837cc711c525f374`; the test command reported no test files. These are scaffold checks, not acceptance coverage. Verified commands are recorded in `docs/project/commands.md`. The Implement exit gate remains the mandatory full-verification suite plus applicable performance checks and coding-standard review on the exact final implementation SHA.
