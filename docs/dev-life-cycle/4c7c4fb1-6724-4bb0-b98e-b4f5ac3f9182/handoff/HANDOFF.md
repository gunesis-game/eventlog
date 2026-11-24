# Cycle 1 — Current handoff

## Cycle identity and status

- **Cycle ID:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182
- **Sequence / execution order:** 1 / 1
- **Workflow:** standard
- **Registry status / phase:** in-progress / define
- **Last updated:** 2026-11-25

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

The runtime is process-lifetime scoped under D-034; there is no library-managed graceful shutdown at process exit. The public consumer API remains:
    PrepareLog[TEvent, TChannel](ctx, newChannel) (TEvent, TChannel, error)
    Notifications() <-chan Event
    Errors() <-chan error
Both generic arguments are concrete pointer types constrained by their respective interfaces. The Channel notification stream carries heterogeneous concrete Event values through the common Event interface. The approved identity, registration, persistence, recovery, Reader, and internal queue design is captured in the architecture.

## Authoritative-state reconciliation

The current requirements, architecture, decisions, acceptance matrix, and benchmark strategy have been checked together. The following is the reconciled Define state.

### Reconciliation evidence

- **Repository revision inspected before this decision:** `8a142a6b2483f11ac6e248835c14bef60a828aad` (`main`).
- **Project records reviewed in full:** `docs/project/requirements.md`, `architecture.md`, `decisions.md`, `acceptance.md`, and `benchmarks.md`.
- **Cycle records reviewed:** this plan, current handoff, and `docs/dev-life-cycle/cycle-index.json`.
- **Decision register:** D-001 through D-039 are Recorded; D-029 is refined by D-032 and D-030/D-031 are historical and superseded for process termination by D-034.
- **Cross-document corrections made:** stale Channel snapshot, Reader wake-up, retry, and backlog open-item claims were reconciled; the duplicate runtime-shutdown section was consolidated; D-034 replaces the former shutdown-drain sequence with process-lifetime runtime ownership and recovery after process termination; D-035 assigns UUID identity tag `0x03`; D-036 requires valid UTF-8 metadata keys and values and rejects invalid bytes without normalization; D-037 settles reflected Go type naming, alias resolution, opaque instantiated generic names, and the cross-toolchain compatibility limitation. D-038 sets explicit v1 canonical identity framing limits; D-039 leaves exact registry-key representation as an implementation detail while preserving the observable registration, identity, reuse, and Channel-isolation rules. The benchmark strategy was aligned with D-034 process-lifetime behavior, and initial Typical/Heavy/Stress workload profiles plus per-process memory and disk budgets were approved and recorded.
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

### Resolved

- D-026: process-scoped runtime ownership, preparation Context scope, shared-worker ownership, and terminal Reader error behavior.
- D-027: Channel snapshots use the identity-keyed bounded FIFO mechanism; consumer synchronization is required for concurrent multi-field mutation.
- D-028: one continuous committed-sequence Reader progression; wake-up hints only trigger authoritative-log rechecks.
- D-034: process-lifetime runtime ownership, no library-managed shutdown/drain at process exit, and recovery of committed pending notifications. D-030–D-031 are historical and superseded for process termination; their graceful-drain sequence is not part of the current runtime contract.
- D-032: retry the same pending occurrence with bounded backoff and finite retry limit; no Reader advancement or overtaking; retry exhaustion terminates only the affected Channel Reader.
- D-033: bounded notification backlog with Reader backpressure; persistence continues and the authoritative append log remains the recovery source.
- The acceptance matrix and benchmark workload scenarios are agreed and recorded in [acceptance.md](../../../project/acceptance.md) and [benchmarks.md](../../../project/benchmarks.md).

### Unresolved

- Benchmark measurement/acceptance methodology (repetitions, warm-up, statistical treatment/variance) and any justified hard performance/memory pass/fail thresholds.

### Next action

Agree benchmark measurement/acceptance methodology and any justified hard thresholds, then perform the final cross-document consistency review. Do not begin implementation or exit Define until the remaining work is settled and you explicitly approve the phase transition.

## Verification

- No implementation, Go build, or Go tests have been run.


## Latest resumption checkpoint — 2026-11-25

- **Repository revision inspected:** `9b8c20d5eadd9ddaa7da34a0c8b4d91d81a6f064` (`main`). The latest commit reconciles this handoff with the approved benchmark profiles and resource budgets.
- **Cycle state verified:** `cycle-index.json` still records Cycle 1 as `in-progress / define`; no new cycle was created.
- **Repository scope verified:** the current tree contains project/governance documentation, validation and cleanup automation, and governance tests; it contains no Go implementation, `go.mod`, Go test suite, or project build workflow. This is consistent with the current Define phase; implementation has not started.
- **CI evidence:** Steering state validation and workflow-history cleanup both completed successfully for `9b8c20d5eadd9ddaa7da34a0c8b4d91d81a6f064`. These validate governance only and are not Go build/test evidence.
- **Benchmark policy verified:** Typical/Heavy/Stress workload profiles, per-process memory budgets (512 MiB / 1 GiB / 2 GiB), and the 4 GiB per-workload disk cap are recorded. They are initial diagnostic budgets, not measured capabilities or hard performance gates. The Linux main-branch smoke suite remains one execution per scenario; extended Linux/Windows measurements are manual, with no scheduled runs.
- **Consistency issue to resolve before Define exit:** `benchmarks.md` describes the initial memory budgets as diagnostic rather than pass/fail limits, while `requirements.md`, `architecture.md`, and `performance-strategy.md` still describe measurable memory targets as open. The final reconciliation must state consistently whether these budgets are diagnostic guardrails only or approved acceptance limits; no stronger guarantee is inferred.
- **Next action:** obtain approval for a resource-bounded measurement protocol and the treatment of performance/memory thresholds, then update the owning benchmark/performance specifications and decision provenance, fetch back and verify the exact revisions, and complete the final cross-document consistency review. Define remains open; no implementation, Go build, Go tests, or benchmark measurements have run.
