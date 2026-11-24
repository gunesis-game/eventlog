# Testing Standard

## Purpose and authority

This standard defines the minimum engineering expectations for designing, selecting, executing, and evidencing tests. It applies to every derived project. The project-specific test strategy at `docs/project/testing-strategy.md` defines the actual suites, commands, architecture-to-test relationships, environments, and gates.

Testing is a design and engineering activity, not a final phase added after implementation. Test design begins during Define and evolves with the implementation.

## Core principles

- Tests must verify observable behavior and meaningful contracts, not merely mirror implementation details.
- Prefer deterministic, repeatable, isolated tests with explicit setup and cleanup.
- Make failures diagnosable: identify the behavior, component, inputs, and relevant environment.
- Keep test data controlled and representative; avoid hidden dependence on external mutable services or wall-clock timing.
- Test public interfaces and component boundaries, including compatibility and error behavior where relevant.
- Avoid redundant tests that add maintenance cost without meaningful additional confidence.
- Do not weaken or skip required checks merely to obtain a green result. Record justified exclusions in the project strategy and obtain approval where they change an accepted quality gate.

## Test design and levels

Projects select applicable test levels and define their purpose and boundaries. Common levels include:

- **Unit/component:** focused behavior within one component, using controlled collaborators.
- **Contract/interface:** compatibility and behavioral guarantees at public or internal boundaries.
- **Integration:** interactions among components, infrastructure, persistence, or external systems.
- **End-to-end/system:** important user-visible or system-wide workflows.
- **Regression:** retained coverage for previously fixed defects and critical behavior.
- **Platform/compatibility:** supported runtime, operating system, architecture, or dependency combinations.
- **Property/fuzz/stress:** broad input spaces, invariants, robustness, and load-sensitive behavior where applicable.

The project strategy must identify which levels apply, what each suite covers, and what is intentionally out of scope. A test's label alone does not establish its coverage.

## Architecture-aware test partitioning

A project must document its architectural components, dependencies, public contracts, and the tests that protect them. Use that map to select focused feedback:

1. A change isolated to one component runs that component's focused tests.
2. Changes affecting a component boundary run the relevant contract and integration tests.
3. Changes to a shared foundation, public interface, or cross-cutting behavior run tests for all affected downstream dependants and consumers.
4. Cross-component changes run the affected integration and end-to-end paths.
5. Unrelated suites need not run on every iteration, provided the selection rationale is documented and the full required suite remains a mandatory gate.

Selection must be based on known architectural impact, not solely on changed file paths or directory naming. When impact cannot be established confidently, broaden the selection or run the full relevant suite.

## Feedback tiers and gates

The project strategy must define explicit feedback tiers, their triggers, suites, expected duration, and blocking behavior. At minimum:

- **Fast feedback:** changed component, directly affected dependants, and relevant interface/contract tests. Run on each meaningful implementation iteration.
- **Integration feedback:** affected component boundaries and important end-to-end paths. Run when boundary or cross-component impact warrants it and at defined integration checkpoints.
- **Full verification:** all required test levels, supported platform checks, regression suites, and other required quality checks. This is mandatory before implementation exit and transition from Implement to Verify.

A green fast-feedback result does not replace integration checks or full verification. A skipped, cancelled, missing, or still-running required check is not a pass. Failed required feedback blocks progression until diagnosed and resolved or explicitly handled through an approved exception.

## Test quality

The project strategy must address, where applicable:

- Test naming, organization, ownership, and traceability to requirements or contracts.
- Isolation, fixtures, test data lifecycle, cleanup, and safe parallel execution.
- Determinism, time/randomness control, retries, flakiness detection, and quarantine policy.
- Mocks, fakes, simulators, and real dependencies: use the least coupled approach that still verifies the required behavior.
- Security-sensitive, boundary, invalid-input, concurrency, cancellation, and failure-path coverage.
- Coverage metrics as diagnostic signals; coverage percentages are not substitutes for behavioral adequacy.
- Test environment reproducibility and dependency/version control.

Flaky tests must be investigated and corrected; repeated retries must not conceal instability. Any temporary quarantine needs an owner, rationale, and removal condition.

## Evidence and reporting

Every reported test result must identify the exact commit SHA, workflow/run or local command, selected suites, environment where material, result, and relevant failure evidence. Distinguish passed, failed, skipped, cancelled, not triggered, inaccessible, and not run. A workflow being triggered or a commit being accepted is not evidence of success.

For CI, inspect the actual run and relevant jobs, steps, logs, and artifacts. Confirm that the run tested the exact revision under discussion. If a workflow tests a different SHA, it is not evidence for the current revision.

## Project strategy and exceptions

Before implementation begins, Define must approve `docs/project/testing-strategy.md`. It records the project's architecture-to-test map, suite definitions, tiers, triggers, commands, environments, CI workflows, required gates, and justified exclusions. If the project has no meaningful applicability for a test level, document why rather than silently omitting it.

Changes to required coverage, suite selection, or blocking gates are material quality-policy decisions and must be approved and recorded through the project's decision process.
