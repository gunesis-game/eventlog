# Cycle 1 — Current handoff (2026-11-24)

## State
- **Cycle:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182 (sequence 1, execution order 1)
- **Workflow / phase:** standard / define
- **Registry:** paused at Define after this handoff
- **Execution lock:** released after checkpoint
- **Implementation:** none; no build or Go tests started.
- **Latest checkpoint:** proposal commit `c57fbdf2f336d0c3c6370233249bdf59a8f22896`; handoff update `68b42ab313c84617dd13832e529ee8f01ee82846`; registry pause commit `1d2d691350e71f7b7d2f11349d57951aa5e36074`; lock release commit `e0dd9ad1a53777376f3eae3d36394b8897bbcfd2`.

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

## Follow-up proposal checkpoint — 2026-11-24
- Expanded the working proposal in `plan/PLAN.md` with a concrete generic API recommendation: pointer type arguments (e.g. `PrepareLog[*MyEvent,*MyChannel]`), fresh non-nil allocations, pointer receiver support, reject non-pointer type arguments, shared live instance pair, process-wide configured owner, per-caller Context cancellation, retryable shared initialization failures, typed identity mutation errors, and receive-only durable ordered notification channel.
- Added a decision table for pointer enforcement, prepared-instance reuse, init timeout ownership, identity mutation severity, notification acknowledgment/backlog/error accessor, and schema drift.
- These remain recommendations only; active requirements and architecture are unchanged. No code/build/test started.
- Proposal commit: `3ab160061d562bf8ab63cb5d5c679a9db8b0234c`.

## Identity/notification decision proposal checkpoint — 2026-11-24
- Started requested work by resuming Cycle 1 Define, claiming the execution lock and updating the registry to in-progress.
- Added concrete identity-mutation proposal: Event-scoped quarantine on Event identity changes; Channel-scoped quarantine blocking events on that Channel after Channel identity changes; preserve last committed snapshots; unaffected channels continue; no implicit auto-clear; avoid repeated fault spam.
- Added notification contract proposal: at-least-once delivery, opaque delivery token, immutable occurrence snapshots, explicit Ack after consumer handling, durable pending/ack ledger, replay unacked in order, duplicates possible between send and Ack, bounded channel with durable backlog, one in-flight per channel, typed bounded diagnostics, shutdown drain deadline and typed error while retaining pending items.
- Important rationale: exactly-once consumer processing cannot be guaranteed across crash between receiving/processing and acknowledgment without consumer transactional integration. Ack makes semantics honest and recoverable but adds API and persistence cost.
- Plan commit: `4e6b5877120d657ccd2e534ac953b4c31ff9ff89`.
- Proposals only; active requirements/architecture remain unchanged. No implementation/build/tests started.


## Approved notification public API decision — 2026-11-24

- The consumer-facing notification payload is the concrete Event type `T` only. No public `Notification[T]` wrapper, Event ID, Occurrence ID, delivery token, or consumer-facing Ack API.
- Notifications are channel-scoped, accessed through the Channel abstraction. Public accessor shape remains to be reconciled with the initialization/ownership mechanism.
- Delivery identity, occurrence identity, durable backlog, retries, replay bookkeeping, and acknowledgment of internal dispatch are private implementation details.
- Since the consumer provides no acknowledgment, eventlog cannot claim exactly-once consumer processing. Delivery/recovery semantics must be stated honestly; duplicates may occur in a crash window if internal dispatch is durably recorded after sending.
- API example: `for event := range channel.Notifications() { handle(event) }`.


## Approved notification payload decision — 2026-11-24

User approved the public payload simplification: consumers receive only `T Event`. No public Notification wrapper, event/occurrence IDs, delivery token, or consumer Ack. Notification stream remains scoped to Channel. Internal delivery metadata/backlog/retries/replay bookkeeping remain private. Exactly-once consumer processing is not claimable without consumer acknowledgment/transactional integration; possible duplicate window must be documented honestly. Notification accessor and publisher/receiver channel initialization remain unresolved. Active requirements and architecture were updated to record this approval; no implementation/build/Go tests started.


## Current checkpoint

- Cycle resumed under lock, approved notification payload decision documented in active requirements/architecture and plan/handoff.
- Completed work: approval checkpoint and doc updates only.
- Remaining Define: notification stream initialization/ownership and dispatch semantics; API/lifecycle and identity mutation details; acceptance matrix/benchmarks; final reconcile and user approval to exit Define.
- Next action: continue Define on notification channel construction/ownership; then reconcile complete requirements/architecture and plan/handoff before requesting exit-Define approval.


## Resume checkpoint — notification construction and remaining design proposals (2026-11-24)
- Re-read active requirements/architecture, Cycle 1 plan and handoff after lock claim. Cycle remains Define; no implementation/build/test started.
- Proposed notification accessor mechanism: Channel embeds library-owned `eventlog.Notifications[TEvent]`, initialized by PrepareLog, exposing receive-only `Notifications()`; helper is excluded from persisted Fields. This keeps Event-only payload and avoids public Ack/token metadata.
- Proposal recommends pointer type arguments, stable pair reuse, per-caller Context cancellation with library-owned shared init, and object-scoped quarantine for identity mutation.
- Acceptance matrix/benchmark scenarios added to proposal. None promoted to active docs; requires explicit user approval.
- Next: user review of proposal, then finalize remaining API/identity/delivery diagnostics/shutdown decisions, acceptance/benchmark matrix, reconcile active requirements and architecture, and seek explicit approval to exit Define. Implementation still prohibited.
