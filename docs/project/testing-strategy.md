# Project Testing Strategy

## Purpose and status

This document is the project-specific implementation of the [Testing Standard](../standards/testing.md). The approved acceptance scenarios are maintained in [acceptance.md](acceptance.md). This strategy maps those scenarios to architectural boundaries, feedback tiers, and verification evidence.

**Current status:** Acceptance scenarios are defined, but implementation and test execution have not started. The repository currently has a steering-governance validation workflow; it does not yet define a project Go test workflow or verified Go test commands. Do not report acceptance coverage as implemented or passing until executable tests have been added and run.

## Architecture and test ownership

The system boundary and component responsibilities are defined in [architecture.md](architecture.md). The acceptance layers are the test ownership map:

| Component / boundary | Direct dependants / consumers | Focused tests | Contract / integration tests | Notes |
|---|---|---|---|---|
| Disk and storage | Persistence and transaction layer | D-01–D-11 | Platform storage tests | Filesystem behavior, I/O errors, Linux and Windows parity |
| Persistence format and transactions | Recovery and metadata | F-01–F-06, T-01–T-06 | Interrupted transaction recovery | Distinguish incomplete trailing transactions from complete malformed records |
| Metadata and recovery | Event-log runtime | M-01–M-09 | Restart and reconstruction tests | Committed append history remains authoritative |
| Core Event/Channel behavior | Channel Reader | L-01–L-12 | Append, identity, concurrency and restart | Preserve transaction atomicity and global sequence |
| Channel and Reader | Notification delivery | R-01–R-14 | Replay-to-live continuity | Each Reader progresses independently |
| Notification delivery | Consumer Channel interface | N-01–N-14 | FIFO, retry, backpressure and error reporting | Validate bounded memory and the documented at-least-once/duplicate boundary |
| Runtime lifecycle | Public preparation API | RT-01–RT-14 | Initialization, cancellation, shutdown and recovery | Test internal lifecycle without adding public controls |
| Public API and integration | Library consumers | API-01–API-13 | Consumer-defined Event/Channel end-to-end tests | Public API compatibility is an explicit acceptance concern |

## Test suites and tiers

| Tier | Suite(s) | Trigger / impact rule | Environment / command | Blocking? |
|---|---|---|---|---|
| Fast feedback | Focused tests for the changed storage, format, identity, Reader, notification or lifecycle boundary and its direct dependants | Every meaningful implementation iteration | Commands are not yet verified; record them in [commands.md](commands.md) when established | Yes |
| Integration feedback | Cross-boundary tests for transaction/recovery, replay-to-live continuity, notification delivery and consumer API contracts | Any change crossing a component boundary or affecting a public contract | Commands and suite locations are not yet established | Yes when the affected contract is in scope |
| Full verification | All acceptance layers, public API compilation, recovery/fault-injection cases and supported-platform checks | Mandatory implementation exit gate | Commands, runners and workflow are not yet established | Yes |

## Test design and quality policy

- Implement acceptance tests from the lowest storage boundary upward; higher-level tests must not mask lower-level failures.
- Cover success, invalid input, corruption, interruption, concurrency, recovery and resource cleanup where specified by the acceptance matrix.
- Test actual consumer-defined Event and Channel types at the public API boundary.
- Exercise Linux and Windows for the platform-sensitive storage and integration guarantees.
- Keep fault injection and process-termination controls test-only; do not add them to the public API.
- Verify deterministic ordering, transaction atomicity, no missed or duplicated history at replay/live transition, bounded notification memory, and explicit error behavior.
- Use real filesystem behavior for storage and recovery tests. Use fakes only where they isolate a boundary without replacing the behavior under test.
- Avoid timing-only assertions where synchronization or observable state can establish the condition more reliably.
- Coverage percentages and other numeric quality thresholds have not been approved; do not invent or enforce them.

## CI and evidence

- Current workflow: [Steering state validation](../../.github/workflows/steering-validation.yml). It validates governance state only; it is not a project test suite.
- No project test workflow, required Go test job, test report artifact, or platform matrix is currently configured.
- When project CI is introduced, document exact workflow paths, triggers, required jobs, and platform coverage here before treating CI as an acceptance gate.
- Associate every result with the exact commit SHA. Distinguish success, failure, skipped, cancelled, inaccessible, and not-run results.
- A missing or incomplete required result is not a pass.

## Exclusions and approvals

No acceptance layer is excluded. Performance measurement is separate and governed by [performance-strategy.md](performance-strategy.md) and [benchmarks.md](benchmarks.md). Any future exclusion, non-blocking suite, or reduced platform check requires rationale, risk, compensating evidence, approval, and a decision ID.

## Define approval checklist

- [x] Acceptance layers and observable scenarios are documented in [acceptance.md](acceptance.md).
- [ ] Executable suite locations and architecture-to-test ownership are confirmed during implementation.
- [ ] Fast, integration, and full-verification commands and triggers are verified.
- [ ] Linux and Windows test execution is configured.
- [ ] Full verification is mandatory before implementation exit.
- [ ] Exclusions and non-blocking checks have documented rationale and approval.
- [ ] The completed strategy is approved and recorded in the decision ledger.
