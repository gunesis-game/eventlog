# Cycle 1 — Current handoff (2026-11-24)

## State
- **Cycle:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182 (sequence 1, execution order 1)
- **Workflow / phase:** standard / define
- **Registry:** paused
- **Execution lock:** released in this checkpoint commit.
- **Implementation:** none; no build or Go tests started.
- **Latest documentation commit:** `0fd02e0562d0b5e2cd5597f861f1696d781d3bcb`; final checkpoint/release commit will supersede it.

## Correction after user review
The previous cleanup was wrong: it optimized for file size and stripped substantive requirements/design from active project docs, leaving the archive as the place to find them. The user explicitly said this was not simply moving content and that architecture must retain the complete design. This pass restores and curates the active docs. Active specifications must remain complete and topic-oriented; archives are provenance only.

## What was restored and curated
- `docs/project/requirements.md`: full functional requirements, ownership/concurrency, Event/Channel identity and occurrence semantics, primitive Fields constraints and selector validation, public API and runtime configuration, append/transaction guarantees, versioned typed persistence and schemas, channel-scoped reader behavior, change detection/notification delivery and backlog, storage/manifest/index/recovery/compaction, durability and platform/non-goals, and remaining Define questions.
- `docs/project/architecture.md`: full system boundary/public contract, component responsibilities, record taxonomy/identity/HEAD, typed field representation and permanent tags, UUID v5 namespace/domains and framing, append/commit flow, detailed recovery and duplicate reconciliation, replay/live reader, scheduler/notification dispatcher, storage layout/manifest example/index, durability, reader-safe compaction, portability and unresolved details.
- Active documents were reorganized by subject and reflowed for readability. Superseded decisions (e.g. full-snapshot identity, notification drop-on-full, consumer-facing Log/Append/Read/Close APIs) are not current instructions; latest approved decisions govern.
- Pre-cleanup plan, handoff, requirements and architecture snapshots remain preserved in the Cycle 1 `history/` directory; no archive/checkpoint was deleted.

## Review and verification
- Read `steering.md`, lifecycle instructions, active docs, plan/handoff, registry/lock and archived original docs.
- Compared the original project docs and archived plan decision headings to the curated current docs; checked that major design topics and current decisions were retained and superseded proposals reconciled.
- Lifecycle validation passed on `7243ec1bc8e3950c5376882333849e27b662c624` (workflow run 45316732733). This checks the registry/lock, not semantic project documentation. Later docs-only commits did not trigger the workflow because its path filters exclude `docs/project/**`; no manual workflow dispatch was available.
- A temporary claim commit CI run failed because the lock was active while registry status was still paused (`must match exactly one in-progress registry cycle`). Registry was corrected in 7243ec1; validation then passed.
- No implementation, build or Go tests were started.

## Remaining Define work
1. Final Go-valid API/interface/selector signatures, generic constraints, pointer/value and occurrence conventions.
2. Prepared-value association and internal lifecycle/cancellation/resource/error behavior.
3. Designated identity-field mutation transition rules and snapshot/index/Channel association effects.
4. Notification accessor/payload/error API, backlog persistence/replay boundary, retry termination, ordering and shutdown drain-failure semantics.
5. Snapshot consistency and consumer synchronization details.
6. Acceptance matrix, benchmarks and performance/memory targets; reconcile requirements/architecture and request explicit approval before implementation.

## Safe resume
Resume Cycle 1 in Define after claiming the lock. Active project requirements and architecture are the authoritative full design; history is for provenance/checkpoint recovery only.
