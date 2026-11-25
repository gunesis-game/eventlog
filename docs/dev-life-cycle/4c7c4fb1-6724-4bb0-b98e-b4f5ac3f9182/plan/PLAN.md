# Cycle 1 — Implementation plan

## Identity and status
- **Cycle ID:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182
- **Sequence / execution order:** 1 / 1
- **Workflow:** standard
- **Status / phase:** in-progress / implement
- **Updated:** 2026-11-25

## Objective and scope
Implement eventlog according to approved requirements, public API, architecture, acceptance criteria, testing strategy, performance strategy, and Git workflow. Implement the approved single-process disk-backed append-only event log in small, testable increments. Preserve consumer-facing API and behavior. Multi-process/distributed writers, public API changes, and any public shutdown API remain out of scope absent separate approval.

## Governing project documents
- [Requirements](../../../project/requirements.md)
- [Architecture](../../../project/architecture.md)
- [Decision register](../../../project/decisions.md)
- [Acceptance matrix](../../../project/acceptance.md)
- [Testing strategy](../../../project/testing-strategy.md)
- [Performance strategy](../../../project/performance-strategy.md) and [benchmarks](../../../project/benchmarks.md)
- [Git workflow](../../../project/git-workflow.md)

## Approved public API constraints
- `PrepareLog[TEvent, TChannel](ctx, newChannel) (TEvent, TChannel, error)`
- `Notifications() <-chan Event` and `Errors() <-chan error`
- Consumer-defined concrete pointer types implementing Event and Channel; heterogenous concrete Events flow through the shared Event interface.
- No public shutdown API and no execution-lock concept.

## Implementation progress (verified against current main tree and recent commits)
1. [x] Module/toolchain and Linux/Windows Go CI scaffold.
2. [x] Canonical v1 schemas, schema embedding/validation, serialization, and record framing with focused tests.
3. [x] Storage and record validation primitives; checksum helper and transaction validation are present.
4. [x] Transaction/stream validation and global sequence continuation work are present, with recent tests/commits.
5. [ ] Event identity, typed-field encoding/reconstruction, registration, and Channel snapshot/change observation. Review current code/tests before continuing; do not assume all subrequirements are complete.
6. [ ] Reader progression, replay/live handoff, notification backlog, retry/backoff, terminal errors, and documented crash-window boundary. Audit existing code/tests and finish gaps.
7. [ ] Runtime ownership, `PrepareLog` integration, consumer API/lifecycle/recovery acceptance tests.
8. [ ] Full verification on exact SHA, Linux and Windows; CI configuration/test-suite expansion as required. Keep governance validation distinct from Go tests.
9. [ ] Benchmarks and evidence capture after representative behavior exists; numeric product targets remain deferred until representative baselines and separate approval.

## Next action
Resume from the latest repository checkpoint (main at `aef9584f7095dc25fe409e80fe43b4ddac2ae7a6`). Review the stream validator's zero starting-sequence boundary and test coverage, verify the current CI status against the exact SHA, correct any uncovered defect, then update this plan and handoff at the next coherent checkpoint. Do not infer CI completion from commit messages or prior chat claims.

## Phase exit
Implement exit requires the mandatory full verification suite, applicable performance checks, coding-standard review, and evidence tied to the exact final implementation SHA. Do not transition phases until the documented gate is satisfied and user authorization is obtained where required.
