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

The specifications contain curated current rules. This plan tracks cycle scope and unresolved work; it does not reproduce the decision ledger or append approval/checkpoint narratives.

## Current design baseline

The consumer-facing API remains:
- PrepareLog[TEvent, TChannel](ctx, newChannel) (TEvent, TChannel, error).
- TEvent and TChannel are concrete pointer types constrained by Event and Channel.
- newChannel has type func(chan Event) TChannel.
- Channel exposes Notifications() <-chan Event and Errors() <-chan error.
- The library owns persistence, replay, change detection, dispatch, and runtime lifecycle.

Approved project decisions are recorded in the project decision register and integrated into the current requirements and architecture. The register is provenance, not a substitute for those specifications.

## Remaining Define work

1. Finalize internal runtime lifecycle, cancellation, reader/resource cleanup, and terminal-error behavior.
2. Resolve remaining Channel mutation and snapshot-consistency details.
3. Finalize internal backlog/replay boundaries, retry termination, and shutdown-drain failure behavior. Retry policy is now approved and recorded as D-032.
4. Complete the acceptance matrix, benchmark scenarios, and measurable performance/memory targets.
5. Reconcile requirements, architecture, and decisions; then request explicit approval to exit Define.

## Acceptance criteria

- [x] Product purpose, ownership, scope, and platform constraints are established.
- [x] Public preparation and Channel notification/error contracts are approved.
- [x] Configuration, field selection, identity, persistence, recovery, Reader, and queue design decisions are recorded.
- [ ] Remaining lifecycle, delivery failure, and snapshot-consistency questions are resolved or explicitly deferred.
- [ ] Acceptance matrix and benchmark plan are complete.
- [ ] Requirements and architecture are coherent, internally consistent, and implementation-ready.
- [ ] User explicitly approves exit from Define before implementation begins.

## Verification

No implementation, Go build, or Go tests have started.
