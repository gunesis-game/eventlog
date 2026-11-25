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
| Benchmark measurement protocol, evidence retention, and comparable-run selection | Resolved | D-040 approves the warm-up/five-repetition protocol and diagnostic ceilings; D-041 approves 20-day artifact retention and comparable-run selection in `benchmarks.md`. Numeric product targets remain deferred pending representative baselines |

### Resolved

- D-026: process-scoped runtime ownership, preparation Context scope, shared-worker ownership, and terminal Reader error behavior.
- D-027: Channel snapshots use the identity-keyed bounded FIFO mechanism; consumer synchronization is required for concurrent multi-field mutation.
- D-028: one continuous committed-sequence Reader progression; wake-up hints only trigger authoritative-log rechecks.
- D-034: process-lifetime runtime ownership, no library-managed shutdown/drain at process exit, and recovery of committed pending notifications. D-030–D-031 are historical and superseded for process termination; their graceful-drain sequence is not part of the current runtime contract.
- D-032: retry the same pending occurrence with bounded backoff and finite retry limit; no Reader advancement or overtaking; retry exhaustion terminates only the affected Channel Reader.
- D-033: bounded notification backlog with Reader backpressure; persistence continues and the authoritative append log remains the recovery source.
- The acceptance matrix and benchmark workload scenarios are agreed and recorded in [acceptance.md](../../../project/acceptance.md) and [benchmarks.md](../../../project/benchmarks.md).

### Unresolved

- Final cross-document consistency review, including the benchmark/requirements/architecture/performance-strategy/acceptance alignment.
- Numeric product performance/memory targets remain intentionally deferred until representative baselines exist; any proposed targets require a separate approval and decision record. This is a post-implementation evidence task, not authorization to invent targets during Define.

### Next action

Complete the final cross-document consistency review. Benchmark measurement, artifact retention, and comparable-run selection are now approved and recorded. Establish representative baselines after implementation exists, then propose any justified numeric product targets as a separate decision. Do not exit Define until its remaining design work is settled and you explicitly approve the phase transition.

## Verification

- No implementation, Go build, or Go tests have been run.


## Latest resumption checkpoint — governance adoption, 2026-11-25

- **Repository revision inspected:** `77652b73b474f30ed4248ff5fb9af2b9ee456e5d` (`main`).
- **Template adoption:** `TEMPLATE_VERSION` is `1.6.0`, matching `gunesis-game/project-steering-template`. The lifecycle README, registry schema, and standard plan/handoff templates match the template revision. The repository-specific `steering.md` intentionally retains eventlog's absolute main-only policy. The validator retains eventlog-specific project decision-register checks and its adoption-reference check now matches the template's document ownership.
- **Execution-lock removal:** the active repository tree has no execution-lock registry/file, and the active steering/lifecycle specification has no execution-lock claim/release protocol. The cycle registry and handoff are the only active progress records. References in the archived 2026-11-24 handoff history describe the retired mechanism and are historical evidence, not active instructions.
- **Cycle state verified:** Cycle 1 remains `in-progress / define`; no new cycle was created and no phase transition occurred.
- **Implementation state verified:** the repository still contains no Go implementation, `go.mod`, Go test suite, or project build workflow. No implementation, Go build, Go tests, or benchmark measurements have run.
- **CI evidence:** steering validation and workflow-history cleanup both completed successfully for this exact revision. The preceding validation run on `11dbb389aba9a785771ab42734a29d88c4997a93` failed because the validator expected a project-strategy reference in the lifecycle README that the template no longer owns there; the validator expectation was corrected in `77652b7`, and the latest validation passed. These workflows validate governance only, not Go implementation.
- **Benchmark policy remains the next Define item:** Typical/Heavy/Stress profiles, initial diagnostic memory budgets (512 MiB / 1 GiB / 2 GiB), and the 4 GiB per-workload disk cap remain recorded. They are diagnostic guardrails, not measured capabilities or hard performance gates. The unresolved work is the resource-bounded measurement/acceptance methodology and consistent treatment of performance/memory thresholds across the owning specifications.
- **Next action:** agree the measurement protocol and whether any hard performance/memory thresholds are justified; then reconcile the benchmark/performance specifications and decision provenance, fetch back and verify them, and complete the final cross-document consistency review. Define remains open; do not begin implementation or exit Define until the remaining work is settled and you explicitly approve the phase transition.


## Latest resumption checkpoint — benchmark protocol approved, 2026-11-25

- **Approved decision:** D-040 records the user's approval of the benchmark measurement protocol and diagnostic resource-ceiling policy.
- **Measurement protocol:** full/manual workloads use one unreported warm-up followed by five measured repetitions; retain individual samples and report median, min-to-max range, and median absolute deviation (MAD). PR smoke scenarios run once and remain informational.
- **Resource policy:** the Typical/Heavy/Stress process-memory budgets and 4 GiB per-workload disk cap are diagnostic execution ceilings, not product acceptance thresholds. An exceeded ceiling makes that run incomplete/resource-limited; it is not a product acceptance failure. No numeric performance or product-memory threshold may fail CI.
- **Specifications updated and fetched back exactly:** `benchmarks.md`, `performance-strategy.md`, `requirements.md`, and `architecture.md`. D-040 points to the verified benchmark specification revision.
- **Benchmark evidence policy approved:** D-041 sets benchmark artifact retention to 20 days. Failed and incomplete/resource-limited evidence is retained during that period for investigation; cancelled runs need not be retained. Baselines must be successful, complete, correctness-valid, and materially comparable; Linux and Windows remain separate, and material environment/workload changes start new comparison series.
- **Still open:** final cross-document consistency review. Numeric product targets remain deferred until representative baselines exist and require a separate approval. Cycle 1 remains in Define; implementation has not started.


## Latest resumption checkpoint — benchmark evidence policy approved, 2026-11-25

- **Approved decision:** D-041 records 20-day benchmark artifact retention and comparable-run selection.
- **Retention:** retain benchmark evidence from successful, failed, and incomplete/resource-limited runs for 20 days; cancelled runs need not be retained. Smoke summaries and environment metadata are retained; raw smoke output is optional.
- **Comparison:** use only the latest successful, complete, correctness-valid run with matching workload parameters and a materially comparable environment. Keep Linux and Windows separate; material environment or workload changes create separate comparison series.
- **Specifications updated:** benchmarks.md is authoritative; performance-strategy.md is aligned; D-041 records provenance.
- **Still open:** final cross-document consistency review. Numeric product targets remain deferred until representative baselines exist and require separate approval. Cycle 1 remains in Define; implementation has not started.


## Latest resumption checkpoint — cross-document reconciliation, 2026-11-25

- **Benchmark policy:** D-040 and D-041 are recorded. Full/manual measurements use one unreported warm-up and five measured repetitions; smoke scenarios run once. Benchmark artifacts are retained for 20 days. Baseline selection requires successful, complete, correctness-valid and materially comparable evidence.
- **Acceptance alignment:** acceptance.md now explicitly separates correctness acceptance from the approved benchmark methodology and defers numeric product targets until representative baselines exist.
- **Cross-document review:** requirements, architecture, acceptance, benchmark strategy, performance strategy, and decisions were inspected for stale measurement-policy language. The acceptance strategy's former claim that workload and measurement methodology remained unapproved was corrected.
- **Remaining Define work:** finish the final semantic consistency review and any remaining design decisions. Numeric product performance/memory targets are deferred until implementation and representative measurements exist; no implementation or benchmark measurements have started.
