# Cycle 1 — Current handoff (2026-11-24)

## State
- **Cycle ID:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182
- **Sequence / execution order:** 1 / 1
- **Workflow / phase:** standard / define
- **Registry:** in-progress
- **Lock:** active for ChatGPT-GPT-6, published on shared `main` in claim commit `54d0f5fb897aa7ef0aa416776093b881af4d73f6`.
- **Implementation:** none; no build or Go tests started.
- **Cleanup/reconciliation:** current plan, handoff, requirements, and architecture consolidated; all prior plan, handoff, requirements, and architecture snapshots preserved in `history/`.
- **Documentation cleanup commit:** `98a3b17784c13419ccda7c2ac78e0081b770967e` (shared `main`, confirmed).
- **Lifecycle CI:** run `45315940750` succeeded for that exact cleanup commit; `Validate cycle registry and execution lock` passed. This validates lifecycle records, not documentation semantics or Go code.

## Public API/configuration approved
```go
event, channel, err := eventlog.PrepareLog[MyEvent, MyChannel](ctx)
```
Only this preparation call is approved. Consumers receive Event/Channel values; internal Log owns operations and lifecycle. Runtime configuration is mandatory via one-time immutable `Config(options ...Option) error` and required `WithStorageDirectory(path)`. Validation rejects nil/duplicate/missing/invalid settings. Paths resolve and clean to absolute during Config; storage creation occurs in PrepareLog. `ErrConfigNotInitialized` and `ErrConfigAlreadySet` are approved. Context is for cancellation/deadline, not configuration.

## Approved design checkpoint
- Single-process ownership, concurrent goroutines, primitive explicit `Fields()` selectors.
- Canonical UUID v5 Event/Channel identity. Event ID includes type/schema/consumer-designated identity field name+value; non-identity data changes create immutable occurrences. Latest occurrence is HEAD. Deduplicate identical physical records.
- Versioned NDJSON channel/event/commit records; global sequence; commit markers; checksum; strict corruption detection and incomplete-tail rollback.
- Manifest-selected immutable segment generations, rebuildable index, transaction-boundary rotation and reader-pinned compaction.
- Sync every 1 second or 1 MiB; Flush/Close are durability boundaries.
- Channel-scoped replay/live reader, internal cursor, type reconstruction/filtering and gap-free transition.
- One 100 ms internal polling scheduler; consumer synchronizes own field changes.
- Notifications: immutable snapshots emitted only after persistence, guaranteed eventual delivery, no persistence/scheduler blocking, recovered backlog first, orderly shutdown drain, crash replay and typed errors after persistent failure.

## Remaining Define work
1. Settle Go-valid API/interface signatures, selector contract, generic constraints, pointer/value and occurrence shapes.
2. Define prepared-value association, lifecycle, cancellation, reader/resource cleanup and terminal errors.
3. Specify identity-field mutation handling and Channel mutation semantics.
4. Finalize notification API/error shapes, backlog durability, replay boundary, retry termination, error ordering and drain-failure semantics.
5. Specify snapshot consistency and consumer synchronization.
6. Complete acceptance matrix and benchmarks; reconcile documents and seek explicit Define approval before implementation.

## Documentation cleanup and checkpoint preservation
Pre-cleanup plan and handoff were preserved verbatim in the cycle's `history/` directory. Active plan/handoff now contain only current baseline, open decisions and resume steps. Historical notes are evidence, not instructions; later approved decisions supersede earlier conflicting proposals.

## Reconciliation
- [ ] Verify project docs, plan, handoff and registry state.
- [ ] Run lifecycle validator if available; otherwise document limitation.
- [ ] Review changed files and link targets.
- [ ] Record cleanup commit and verification.
- [ ] Keep Cycle 1 in Define; do not implement.
