# Cycle 1 — Current handoff (2026-11-24)

## State
- **Cycle:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182 (sequence 1, execution order 1)
- **Workflow / phase:** standard / define
- **Registry:** paused at Define after this handoff
- **Execution lock:** released after checkpoint
- **Implementation:** none; no build or Go tests started.
- **Latest resume checkpoint:** lock claim commit `901769611bbd6469accaddac74bd843c2607d423`; cycle resumed in Define, then paused pending user direction/continued design work.

## Correction after user review
The previous cleanup was wrong: it optimized for file size and stripped substantive requirements/design from active project docs, leaving the archive as the place to find them. The user explicitly said this was not simply moving content and that architecture must retain the complete design. This pass restores and curates the active docs. Active specifications must remain complete and topic-oriented; archives are provenance only.

## What was restored and curated
- `docs/project/requirements.md`: full functional requirements, ownership/concurrency, Event/Channel identity and occurrence semantics, primitive Fields constraints and selector validation, public API and runtime configuration, append/transaction guarantees, versioned typed persistence and schemas, channel-scoped reader behavior, change detection/notification delivery and backlog, storage/manifest/index/recovery/compaction, durability and platform/non-goals, and remaining Define questions.
- `docs/project/architecture.md`: full system boundary/public contract, component responsibilities, record taxonomy/identity/HEAD, typed field representation and permanent tags, UUID v5 namespace/domains and framing, append/commit flow, detailed recovery and duplicate reconciliation, replay/live reader, scheduler/notification dispatcher, storage/manifest/index, durability, reader-safe compaction, portability and unresolved details.
- Active documents were reorganized by subject and reflowed for readability. Superseded decisions (e.g. full-snapshot identity, notification drop-on-full, consumer-facing Log/Append/Read/Close APIs) are not current instructions; latest approved decisions govern.
- Pre-cleanup plan, handoff, requirements and architecture snapshots remain preserved in the Cycle 1 `history/` directory; no archive/checkpoint was deleted.

## Review and verification
- Read steering and lifecycle rules, registry/lock, Cycle 1 plan/handoff, requirements and architecture. The GitHub connector supports file fetch/update but no directory-tree listing or local checkout/status access was available in this session.
- Confirmed main branch lock was released, claimed Cycle 1, and confirmed the claim on main before updating the registry. Registry was updated to in-progress after claim.
- Prior lifecycle validation passed on `7243ec1bc8e3950c5376882333849e27b662c624` (workflow run 45316732733). That validator run predates this resume/claim checkpoint; no new validator run or Go tests have been executed.
- No implementation, build, or Go tests have started.

## Working proposal added (not approved)
A focused design proposal was added to the Cycle 1 plan: pointer-backed prepared values and stable schemas; explicit one-identity-field selectors; shared preparation lifecycle; typed errors; immutable identity after prepare; post-commit typed notification snapshots; durable disk-backed backlog; receive-only notification accessor; and a test/acceptance matrix. These are recommendations only and have not been promoted to approved requirements/architecture.

## Remaining Define work
1. Final Go-valid API/interface/selector signatures, generic constraints, pointer/value and occurrence conventions.
2. Prepared-value association and internal lifecycle/cancellation/resource/error behavior.
3. Designated identity-field mutation transition rules and snapshot/index/Channel association effects.
4. Notification accessor/message/error API, backlog durability/retention/replay boundary, retry termination, ordering and drain-failure behavior.
5. Snapshot consistency and consumer synchronization details.
6. Acceptance matrix, benchmarks and performance/memory targets; reconcile requirements/architecture and request explicit approval before implementation.

## Safe resume
Cycle 1 remains in Define. Continue with bounded design proposals for the unresolved API/lifecycle/identity/notification questions, then reconcile active requirements and architecture and seek explicit approval to exit Define. Do not start implementation before that approval.
