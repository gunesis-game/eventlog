# Cycle 1 — Current handoff (2026-11-24)

## Current approved baseline (authoritative)

- `PrepareLog[TEvent, TChannel](ctx, newChannel)` uses pointer type arguments `TEvent Event` and `TChannel Channel`, validates both at runtime using reflection, and uses `newChannel func(chan Event) TChannel`.
- Compatible repeated preparation for the same concrete Event/Channel type pair and exposed schemas returns the same live Event and Channel pointers; incompatible reuse fails with a typed conflict. Event-specific preparations resolving to the same logical Channel reuse its Channel pointer/runtime and single Reader progression.
- Channel accessors are `Notifications() <-chan Event` and `Errors() <-chan error`.
- Historical replay waits asynchronously for all decoders required by committed Channel history; a missing decoder waits indefinitely without timeout or fatal error solely due to absence; no public registration or activation method.
- Error stream: shared per Channel, capacity one, first fatal error retained, best-effort non-blocking delivery; fatal Reader termination closes streams after reporting.

## State
- **Cycle:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182 (sequence 1, execution order 1)
- **Workflow / phase:** standard / define
- **Registry:** in-progress at Define
- **Collaboration:** contributors may work concurrently; the registry and handoff track progress but do not grant exclusive ownership.
- **Implementation:** none; no build or Go tests started.
- **Latest checkpoint:** 2026-11-24T18:38:51.151Z — Cycle 1 resumed; D-009 prepared-instance reuse approved and recorded in canonical artifacts.

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
- Started requested work by resuming Cycle 1 Define, resuming Cycle 1 Define and confirming the registry is in-progress.
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
- Earlier embedded-helper proposal was rejected. The consumer-defined Channel accessor is `Notifications() <-chan TEvent`; eventlog still needs a separate internal send-side ownership/construction mechanism.
- Proposal recommends pointer type arguments, stable pair reuse, per-caller Context cancellation with library-owned shared init, and object-scoped quarantine for identity mutation.
- Acceptance matrix/benchmark scenarios added to proposal. None promoted to active docs; requires explicit user approval.
- Next: user review of proposal, then finalize remaining API/identity/delivery diagnostics/shutdown decisions, acceptance/benchmark matrix, reconcile active requirements and architecture, and seek explicit approval to exit Define. Implementation still prohibited.


## API feasibility note — 2026-11-24
- Added a Go type-constraint sketch for the embedded notification helper. The helper can promote a package-private initialization method to the consumer's Channel type; a type-parameterized Channel constraint ties its notification stream to Event type. Pointer-only generic arguments still require runtime validation.
- No source code, build or tests started; note remains a design proposal.


## User correction checkpoint (2026-11-24)
- User explicitly rejected a public notification-helper struct and consumer callback/sink method. Consumer-defined Channel exposes `Notifications() <-chan TEvent`; eventlog's send-side ownership/construction mechanism remains to be resolved.
- This supersedes the embedded `eventlog.Notifications[T]` helper proposal. Signature and internal delivery mechanism remain open for Define.
- Avoid re-proposing a library-owned public helper or embedding pattern. Update active requirements/architecture only after final method signature and semantics are resolved.


## Next Define checkpoint — corrected API and pending notification questions
- Removed the prior embedded-helper proposal from the current notification design section; it is superseded and must not be considered a valid option.
- Required API shape is a consumer-defined Channel struct exposing `Notifications() <-chan TEvent`. No callback/sink method. Internal send-side construction/ownership remains unresolved.
- Continue with API/lifecycle and identity mutation edge cases, then complete notification method signature/ownership/backpressure/recovery, acceptance matrix, reconcile requirements+architecture, and request Define exit approval.


## Notification contract alignment checkpoint (2026-11-24)
- Updated active requirements and architecture: consumer-defined Channel types implement the notification method required by the public Channel interface; no public helper struct/embedding. Signature remains TBD.
- API shape alignment only; buffering/backpressure, dispatch, persistence/recovery, retry and shutdown details remain Define-open.


## Next Define topic — canonical identity byte layout proposal
A detailed framing proposal was appended to PLAN (not approved). Existing UUID constants and primitive tags are unchanged. Review the proposed payload composition and edge cases (nil identity, -0, UTF-8, type naming, framing overflow) before promoting to requirements/architecture. No implementation work.


## Approval checkpoint — canonical identity encoding
User approved the exact payload byte layout in PLAN. Promoted to active requirements and architecture. Remaining edge cases (nil identity, negative zero, invalid UTF-8 policy, Go type naming, framing limit) remain open and require focused decisions; no implementation yet.


## Identity edge cases next for review
A recommendation-only subsection in the current PLAN addresses nil identity, raw IEEE negative-zero, invalid UTF-8 policy, Go type-name identity, and practical framing limits. These have not been promoted; user review/approval is required. Byte layout itself is already approved and active.


## User correction checkpoint — UUID-only identity (2026-11-24)
User clarified that the consumer-selected identity field for both Event and Channel **must be a UUID**. Identity may not be nullable or any arbitrary primitive/string/numeric value. The active requirements and architecture are being updated accordingly. UUID payload should be represented canonically as 16 bytes; UUID tag assignment and accepted Go UUID type/representation remain Define questions. Primitive tags remain available for ordinary persisted Fields, not identities. No implementation/build/tests started.


## Documentation ownership review checkpoint — 2026-11-24
- Cycle 1's cycle remains in-progress and matches the registry's in-progress Cycle 1 claim; no release/reclaim was needed. Work continued under the existing claim.
- Updated requirements framing to own product behavior, domain rules, constraints and acceptance expectations, avoiding internal design prescription.
- Updated architecture framing to own technical mechanisms and design; corrected stale UUID identity encoding and notification accessor/open-question text.
- Updated coding standards and steering guidance to distinguish requirements, architecture, implementation standards, verified commands, and temporary cycle records.
- Updated files: `docs/project/requirements.md`, `docs/project/architecture.md`, `docs/project/coding-standards.md`, `steering.md`. `docs/project/commands.md` was reviewed and already correctly requires verified commands, so it was left unchanged.
- Verification performed: GitHub fetch-back review of updated text. No local schema validator, build, or Go tests ran because this connector session provides file fetch/update but no repository checkout/command execution.
- Next action: finish fetch-back contradiction review, then continue Define topics; no implementation begins before explicit Define approval.


## Session pause and lock release — 2026-11-24T16:00:07Z
- Cycle 1 is intentionally paused at the Define phase so a new session can resume it.
- Checkpointed state: documentation ownership review changes are committed and fetch-back checked; the remaining Define topics are API/lifecycle, identity mutation, notification stream initialization/ownership and dispatch guarantees, acceptance matrix, and performance/memory targets.
- No implementation, local validation, build, or Go tests were run in this session.
- Next action for the next session: read the lock first, then steering/lifecycle state and this handoff/plan; resume Cycle 1 Define with the existing cycle ID and execution order.
- Lock release is being performed explicitly for this pause. Cycle 1 remains resumable and is not complete or cancelled.


## Superseded notification-method checkpoint
Earlier sink-method discussion is superseded and intentionally removed from this active handoff.
The settled public accessor is `Notifications() <-chan TEvent`; no consumer callback/sink method is
part of the API. The remaining issue is internal writable-channel ownership and construction.


## User correction — remove Notify; settle Notifications accessor (2026-11-24)
- User reaffirmed the previously decided receive-only accessor: `Notifications() <-chan TEvent`. The consumer receives from this channel; there is no `Notify(event TEvent) error` method.
- The prior sink proposal and its pending-review blocker are superseded. Do not reintroduce or ask the user to reconsider `Notify`.
- Current remaining design question is solely how eventlog gets an internal send-capable channel and initializes it while keeping consumer-defined Channel types and the public receive-only accessor. This cannot be solved by the receive-only accessor alone in Go; address this directly, without callback or public helper embedding.
- Active requirements and architecture were corrected to record the accessor contract. No implementation/build/tests started.


## Notification channel factory decision — 2026-11-24
- Resolved construction/send-ownership gap: `PrepareLog` takes required factory `func(chan TEvent) TChannel`; the consumer supplies its constructor (e.g. `NewOrdersChannel`) and Go compile-checks this function shape at the call site.
- eventlog creates and owns the writable channel, passes it into the constructor; consumer Channel retains a receive-only view exposed by `Notifications() <-chan TEvent`. Consumer never sends or closes it.
- Go cannot enforce a named constructor function as a type/interface member; required typed factory is the compile-time enforcement mechanism. This is the approved design direction. Delivery/backlog/recovery/shutdown semantics remain unresolved. No implementation or tests started.

## Progress checkpoint — 2026-11-24T17:09:29Z

- **Last progress checkpoint:** 2026-11-24T17:09:29Z
- **Registry/lock:** Cycle 1 remains `in-progress` at `define`; the active lock remains held by ChatGPT for this same cycle. No takeover or lock reset was performed.
- **Governance propagated:** Added active-work checkpoint cadence (material decisions and at least every 15 minutes), pause/block-before-waiting, no automatic lock expiry, and explicit owner-confirmed recovery rules to eventlog's lifecycle README and steering. Added the last-progress timestamp field to the handoff template.
- **Verification:** GitHub read-back of governance files is pending. No local validator, build, or Go tests have run.
- **Approved API clarification from the user:** Channel interface method is `Notifications() <-chan Event`; `Event` is the common interface and one consumer-defined Channel may carry multiple concrete Event types in that stream. The consumer-facing Channel API must not change; no `Notify` callback/sink method.
- **Design inconsistency found:** The previous `func(chan TEvent) TChannel` factory example is incompatible if `TEvent` means one concrete Event type. It only works as a heterogeneous stream factory if `TEvent` is the common Event interface. This must be reconciled with event-specific `PrepareLog` calls and Channel-scoped runtime resolution; no signature change is assumed or implemented.
- **Completed in this checkpoint:** Governance policy propagated; requirements/architecture/plan corrected to record the heterogeneous notification contract and the factory mismatch; current handoff status corrected to match the registry and lock.
- **Next action:** Read back all changed files, reconcile the exact `PrepareLog` generic relationship and shared Channel construction, then continue remaining Define topics. Implementation remains prohibited until explicit Define exit approval.

## Progress checkpoint — 2026-11-24T17:09:29Z (design catch-up)

- **Last progress checkpoint:** 2026-11-24T17:09:29Z
- **Recovery order:** existing event log/system of record must be loaded and validated before resolving or creating a runtime Channel.
- **Channel convergence:** Channel identity is resolved at Channel scope. Event-specific preparation for the same Channel converges on one runtime Channel and one notification stream.
- **Notification API:** `Notifications() <-chan Event` is defined on the Channel interface. `Event` is the common interface implemented by concrete Event types; one Channel stream can carry multiple concrete Event types.
- **Reader:** one unified Reader progression per runtime Channel serves both historical replay and live tailing. Do not synthesize notifications by combining independent typed Readers.
- **Ordering/delivery:** successful Commit is authoritative; notification delivery follows the unified Reader in committed order and begins only after commit. Slow consumers may delay delivery but not persistence, reorder or skip events. Delivery failure does not undo committed events; exactly-once consumer processing is not guaranteed without consumer acknowledgement.
- **Ownership/shutdown:** eventlog owns the send side, Reader, delivery worker and lifecycle. Consumers do not gain public lifecycle methods. Shutdown ordering is recorded in the Cycle 1 plan.
- **Decoding:** reconstruct concrete Event types through the existing internal registration/codec mechanisms; unknown persisted Event types fail rather than being silently skipped.
- **Next:** finish precise PrepareLog registration/convergence semantics, lifecycle/error boundaries, identity mutation behavior, acceptance matrix and benchmark plan. Then reconcile all docs and seek explicit approval to exit Define. No implementation/build/Go tests started.

## Progress checkpoint — 2026-11-24T17:12:58Z (registration barrier)

- **Last progress checkpoint:** 2026-11-24T17:12:58Z
- **Approved:** all concrete Event types/decoders for a logical Channel must be prepared/registered before heterogeneous historical notification replay begins. Unknown persisted types must never be silently skipped.
- **API constraint:** no consumer-facing `Activate()`, `Start()`, `RegisterEventType()`, `Close()`, Ack, or checkpoint method. `Notifications() <-chan Event` remains unchanged. Eventlog owns Reader activation and shutdown.
- **Lifecycle distinction:** `PrepareLog` is event-specific; Channel resolution, notification delivery, and runtime lifecycle are Channel-specific. All event-specific preparations for the same Channel converge on one runtime Channel and Reader progression.
- **Open blocker:** sequential `PrepareLog` calls provide no completion signal proving all concrete Event decoders for the Channel have been registered. No existing completion boundary has yet been established from repository evidence. Resolve internally or through an already-existing lifecycle boundary; do not add a public activation method.
- **Verification:** lifecycle workflow passed for commit `30bacac14eddb6f51ebed6536fb291606edf6e48` (run 45445928283). This validates registry/lock invariants only, not Go behavior. No source code, build, or Go tests exist/run yet.
- **Next:** inspect the full existing API/lifecycle specification and settle the registration-completion mechanism, then complete identity-mutation and acceptance/benchmark decisions. No implementation until Define is reconciled and explicitly approved.

## Progress checkpoint — 2026-11-24T17:14:49Z (shutdown and CI correction)

- **Last progress checkpoint:** 2026-11-24T17:14:49Z
- **Lifecycle:** documented eventlog-owned shutdown ordering: stop new preparations/appends; settle outstanding commits; cancel Reader/delivery workers; drain pending deliveries and wait for senders; close notification streams only after senders stop; release storage resources. Delivery failures do not undo committed Events.
- **CI correction:** the cleanup step could fail when concurrent workflow runs both deleted the same older run. Updated `.github/workflows/steering-validation.yml` in both `eventlog` and `project-steering-template` to treat HTTP 404 for an already-removed run as benign while preserving failures for other errors.
- **Verification:** eventlog lifecycle workflow passed on commit `5f690c186aebc0d60f446e534cb954ff1c878db9` (run 45446349192). Both the registry/lock validator and cleanup step passed. Earlier run 45446166033 had a validator pass but cleanup failure due the identified race; it is superseded by the successful fixed workflow run.
- **Remaining Define blocker:** the approved registration barrier has no established completion signal. Sequential event-specific `PrepareLog` calls alone cannot prove that every decoder for a logical Channel has been registered. Resolve this without adding a consumer-facing lifecycle method.
- **Next:** finish the internal barrier/completion design, identity-mutation decisions, and acceptance/benchmark matrix; reconcile the plan and project docs. No Go source exists in the repository yet; no build or Go tests have run.

## Progress checkpoint — 2026-11-24T17:16:20Z (Define reconciliation)

- **Last progress checkpoint:** 2026-11-24T17:16:20Z
- **API reconciliation:** `TEvent` is the common `Event` interface in the approved generic preparation contract; the required `func(chan TEvent) TChannel` factory therefore carries heterogeneous concrete Event values. The earlier `chan *OrderEvent` example and `Notifications() <-chan TEvent` wording are superseded by the Channel interface accessor `Notifications() <-chan Event`.
- **PrepareLog/Channel scope:** `PrepareLog` is event-specific; Channel identity, construction, notification delivery, and runtime lifecycle are Channel-specific. All event-specific preparations for the same Channel converge on one runtime Channel and one Reader progression.
- **Current design docs:** requirements and architecture now agree on recovery-before-Channel-resolution, one unified Reader progression, common-Event notification stream, registration barrier, commit-order delivery, and eventlog-owned shutdown. The Cycle 1 plan baseline now records the factory argument and settled notification accessor.
- **Still open:** the registration barrier's completion signal; exact internal preparation/Channel-convergence state machine; identity-mutation response; remaining delivery retry/error/backlog details; acceptance matrix and benchmark thresholds. Identity-mutation proposals remain proposals, not approved policy.
- **Verification:** lifecycle workflow passed on `4b45ccd4f20de98f39228f5e4b024fcb9eb9c2f1` (run 45446601240). This is registry/lock validation only. The run for the latest plan-only cleanup commit is pending/not yet observed.
- **Next:** resolve the internal barrier and lifecycle state machine without adding public consumer methods; then reconcile identity and delivery semantics. No Go source exists yet, so implementation/build/tests remain unstarted.

## Progress checkpoint — 2026-11-24T17:16:47Z (replay-barrier proposal)

- **Last progress checkpoint:** 2026-11-24T17:16:47Z
- **Proposal persisted (not approved):** use the existing process-wide Event type/codec registry as the pre-replay barrier. Recover the system of record, enumerate concrete Event types in committed history, and require decoders for all of them before Channel resolution or Reader activation. Missing decoders fail closed. Event-specific `PrepareLog` calls then attach registrations to the shared Channel runtime; new Event types must be registered before their first committed append.
- **Why this is only a proposal:** repository inspection found no Go source, codec implementation, or registry code. It is not yet established that the existing registry is process-wide, available before recovery, or enumerable by persisted type ID. Do not promote or implement this mechanism until confirmed.
- **Verification:** workflow passed on handoff commit `eb07d653513539e02d66ddb4362ed0d5f5b12915` (run 45446734047). Latest plan proposal commit `64bb679a1a823a770968d4bdc715dd7330c24cad` has not yet been observed in the workflow list.
- **Next:** confirm the scope/availability of the existing Event codec registry; then finalize the barrier. After that, settle identity mutation, delivery retries/errors/backlog, and acceptance/benchmark matrix.

## Pause/blocked checkpoint — 2026-11-24T17:17:13Z

- **Last progress checkpoint:** 2026-11-24T17:17:13Z
- **Registry:** Cycle 1 is `blocked` at `define`; `status_reason` records the dependency on confirming the existing Event type/codec registry.
- **Collaboration:** the cycle checkpoint is persisted; contributors may work concurrently while coordinating through Git.
- **Completed:** governance checkpoint rules propagated; heterogeneous `Notifications() <-chan Event` and common Event semantics recorded; recovery-before-Channel-resolution, Channel convergence, one Reader progression, committed ordering, backpressure, failure recovery, and shutdown rules reconciled into active docs; replay-barrier proposal recorded but not promoted.
- **Open question:** is the existing Event type/codec registry process-wide and loaded before the first `PrepareLog`, with decoders available for every persisted concrete Event type during recovery?
- **Resume:** after the registry scope is confirmed, resume this same Cycle 1 at Define. Do not create a new cycle or change the consumer-facing API.

## Session handoff — 2026-11-24T19:19:00+02:00

- **User request:** carry the project state forward and unlock it so a new session can claim it.
- **Cycle:** preserve Cycle 1 (`4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182`), sequence 1 / execution order 1, at Define. It remains `blocked` on the codec-registry clarification; do not create a new cycle or mark this cycle complete.
- **Collaboration:** contributors may continue from the persisted cycle checkpoint; no exclusive ownership is assigned.
- **Resume reading order:** `steering.md`; `docs/dev-life-cycle/README.md`; `docs/dev-life-cycle/cycle-index.json`; this handoff; `plan/PLAN.md`; then active `docs/project/requirements.md` and `docs/project/architecture.md`. Read the history files only for provenance where needed.
- **Current approved API:** `PrepareLog[TEvent, TChannel](ctx, newChannel)` with `TEvent` as the common `Event` interface and required `func(chan TEvent) TChannel` factory; consumer Channel exposes `Notifications() <-chan Event`. A single Channel stream can carry heterogeneous concrete Event values. No `Notify` method or public lifecycle/Ack API.
- **Design checkpoint:** recover and validate the existing system of record before resolving/creating a Channel; event-specific preparations for one logical Channel converge on one Channel runtime and one Reader progression; replay/live handoff is gap-free and ordered by committed sequence; unknown persisted event types fail closed; eventlog owns delivery and shutdown.
- **Blocking question:** confirm whether the existing Event type/codec registry is process-wide and loaded before the first `PrepareLog`, with decoders available for all concrete Event types encountered during recovery. The registry-driven replay barrier in the plan is a proposal only because no Go/codec source exists to verify it.
- **Implementation status:** no Go source, `go.mod`, build, or Go tests. Do not implement until Define decisions are reconciled and the user explicitly approves Define exit.
- **Next session:** resume this same Cycle 1 from its persisted Define checkpoint, resolve the codec-registry/barrier question, then continue the remaining API/lifecycle, identity-mutation, notification error/retry/backlog, acceptance-matrix and benchmark decisions.


### Decision register

This is the authoritative decision ledger for Cycle 1. Every material user-approved decision must be recorded in its canonical artifact before the next independent decision is introduced. A persisted `Approved` state is a governance failure; use `Recorded` only after the artifact is published and fetched back at the stated revision.

| ID | Decision | State (Proposed / Approved / Recorded) | Canonical artifact | Verified revision / evidence |
|---|---|---|---|---|
| D-001 | Concrete `TEvent Event`, `TChannel Channel`; factory `func(chan Event) TChannel`; heterogeneous notification payload | Recorded | docs/project/architecture.md | 5e3dd94b988454a31068f75f5b163ec288909e67 |
| D-002 | Channel exposes receive-only `Notifications() <-chan Event` | Recorded | docs/project/requirements.md | b580b7c083dea60be61910d402f71d9d969fe590 |
| D-003 | Channel exposes one-way shared `Errors() <-chan error`, capacity one, first fatal error retained and best-effort non-blocking delivery | Recorded | docs/project/requirements.md | 4b7bc62c6cf851bf62a86bd7a11286c761063f02 |
| D-004 | Fatal Reader error terminates Channel Reader and closes notification/error streams after reporting; safe recoverable errors may continue | Recorded | docs/project/architecture.md | d4fe4e197b07385e95159d7090e8110f4805b68f |
| D-005 | Concrete decoder registration derives from generic `TEvent`; historical replay waits asynchronously for all decoders required by committed Channel history; no public registration/activation API | Recorded | docs/project/architecture.md | d4fe4e197b07385e95159d7090e8110f4805b68f |
| D-006 | A required decoder that has not registered causes indefinite asynchronous Channel Reader waiting; no arbitrary timeout or fatal error solely due to absence; replay resumes when registered | Recorded | docs/project/architecture.md (also reconciled in requirements.md and Cycle 1 plan) | `4ea67d0ca45cc0bc2de7f46717844fbed42d4bb5`; requirements `3bbe21bdc79528aff55afa74f070f7d7ff629549`; plan `0aa4496f30faf53279b41df8972acc91cadeeb9e` |
| D-007 | Both generic type arguments `TEvent` and `TChannel` supplied to `PrepareLog[TEvent, TChannel]` must be pointer types | Recorded | docs/project/architecture.md | architecture `98918b4a4529f004fc14e267627571d462521214`; requirements `183fab3f4f4ba4bd5393e1858812fde8c8d5c718`; plan `db1c05e39b31ae3c215bb8f96df2d5a0daffeb6e` |
| D-008 | Enforce pointer-only `TEvent` and `TChannel` arguments by runtime reflection in `PrepareLog`; return an error for non-pointer arguments without changing the two-parameter generic API | Recorded | docs/project/architecture.md | cef7d96797081f049782815f84b908ba2decfce7 |
| D-009 | Reuse compatible prepared Event/Channel instances and reject incompatible type/schema reuse with a typed conflict; Channel runtime reuse follows logical Channel identity | Recorded | docs/project/architecture.md | 0b070a42cfa3e924a6744077259dd9ed6a3c90ae |
| D-010 | Separate Event type/schema registration, Event instance identity, and Channel runtime identity; never key Channel runtimes only by concrete Go type | Recorded | docs/project/architecture.md | f3e7b8ea2dfc742fa297d19eadc20b5422ff8291 |
| D-011 | Resolve Channel runtime by factory-candidate canonical identity; reuse existing Channel pointer/runtime or adopt candidate for a new logical Channel | Recorded | docs/project/architecture.md | 7bd42a01d58a795b59a57fad06b523de330c6a1f |
| D-012 | Channel factory is constructor-only and side-effect-free; discarded candidates are safe; runtime starts only after adoption | Recorded | docs/project/architecture.md | f75de6313f4d4cc2015d11accfdbce413cfc8c72 |
| D-013 | Channel runtime lookup and candidate adoption are atomic by canonical Channel identity, yielding one pointer/runtime/Reader | Recorded | docs/project/architecture.md | 9c808cc9fd3c236885ec9f37110cd91cd0297541 |
| D-014 | Concurrent Event-type preparations for one logical Channel converge on one Channel pointer/runtime/Reader; replay waits asynchronously for required decoders | Recorded | docs/project/architecture.md | 92d5b462d99a55763cbe5a217292e52a37e4e7e1 |
| D-015 | Successful concrete Event decoder registrations persist for the Log runtime lifetime, keyed by concrete Event type and exposed schema | Recorded | docs/project/architecture.md | 3258af3313da4a40f43e7e3a7dd698bb4e5f1de1 |
| D-016 | Re-registering the same Event type/schema reuses the decoder; a different schema returns a typed conflict without mutating existing registration | Recorded | docs/project/architecture.md | aa3f317df4daaba2ce40bed7530309493a9339a4 |
| D-017 | Typed direct-field selectors `FieldOf` and `FieldOfNullable` preserve named primitive types and nil-versus-zero distinctions | Recorded | docs/project/architecture.md | 37da85f14975b31ae9810d0f22454537bb0e6c09 |
| D-018 | Historical Event reconstruction uses fresh concrete pointers; notification snapshots are immutable and isolated from live and persistence storage | Recorded | docs/project/architecture.md | 37da85f14975b31ae9810d0f22454537bb0e6c09 |
| D-019 | Prepared Event identity is immutable; detected identity mutation is rejected and reported through the existing Channel error stream | Recorded | docs/project/architecture.md | 37da85f14975b31ae9810d0f22454537bb0e6c09 |
| D-020 | Detected mutations enqueue immutable snapshots on an internal eventlog-owned queue; workers dequeue and persist; no public API change | Recorded | docs/project/architecture.md | 37da85f14975b31ae9810d0f22454537bb0e6c09 |
| D-021 | Per-logical-Event FIFO order is preserved; no queued snapshot is dropped; suppress only consecutive unchanged observations; A→B→A remains distinct | Recorded | docs/project/architecture.md | 37da85f14975b31ae9810d0f22454537bb0e6c09 |
| D-022 | Internal queue is bounded; capacity pressure pauses/delays monitoring, not consumer assignments; queued snapshots remain retained | Recorded | docs/project/architecture.md | 37da85f14975b31ae9810d0f22454537bb0e6c09 |
| D-023 (Decision 8) | One Log-owned bounded ingress queue; multiple persistence workers may process different logical Events concurrently; each Event remains serial/FIFO; global sequence assigned centrally at commit | Proposed | — | Pending approval |

## Governance correction checkpoint — 2026-11-24T20:02+02:00

- Cycle 1 resumed at Define under the existing cycle ID and execution order. Lock claim was published and fetched back at commit `8a489bf4e842f758bdf79ff6772ef16b068a6ca8`; registry resumed at commit `f2bde95cf5506a379a5ee8a9efacdbabe2248076`.
- Governance process deviation: the initial discovery call fetched files in parallel before reading the former execution lock first, contrary to the mandatory read-order rule. The lock was then read and found released before the claim. No conflicting active owner was observed. Future sessions must read the lock as the first repository operation.
- D-001 through D-005 are recorded in canonical requirements/architecture documents with verified revisions; D-006 remains proposed.
- Plan reconciled and fetched back at `dd2f671d95b11f99b9d0eca2aa64b2a46fce1fc1`.
- Active requirements reconciled and fetched back at `b580b7c083dea60be61910d402f71d9d969fe590`; active architecture reconciled and fetched back at `5e3dd94b988454a31068f75f5b163ec288909e67`.
- Eventlog steering, lifecycle README, handoff template, validator and CI path filters now adopt the template v1.1.0 approval-to-artifact gate. The validator rejects persisted Approved decisions and requires Recorded decisions to identify an existing canonical artifact plus a commit hash.
- No implementation, build or Go tests were started.



## Latest reconciliation checkpoint — 2026-11-24T20:04+02:00

- Requirements, architecture and plan have been reconciled against all explicit approvals from the current review. Older contradictory wording is corrected in active sections; historical proposals are explicitly superseded rather than silently erased.
- Approved contract: `PrepareLog[TEvent, TChannel](ctx, newChannel)`, with `TEvent Event` concrete, `TChannel Channel` concrete, and `newChannel func(chan Event) TChannel`. Channel accessors are `Notifications() <-chan Event` and `Errors() <-chan error`.
- Error semantics recorded: shared per-Channel one-way error stream, capacity one, first fatal error retained, best-effort non-blocking delivery, fatal Reader termination, stream closure after fatal report; safe recoverable errors may continue.
- Decoder direction recorded: generic concrete `TEvent` supplies internal registration identity; historical replay waits asynchronously for all decoders required by committed Channel history; no consumer registration/activation API. Exact policy for a decoder that never registers remains open.
- The template v1.1.0 governance rule is propagated into eventlog steering/lifecycle instructions, handoff template, validator and workflow triggers. A CI failure on commit `37ad20bf929ef8987afc1ef79bf697ef4f2fd7ee` exposed a heading mismatch in the validator; corrected in commit `415302ccdf42eac3136c3acc4768eaf0f28740ce`. The latest workflow result is pending at this checkpoint.
- The initial lock-read-order deviation is retained above for transparency.
- No implementation, build or Go tests were started.
- Next: verify the latest lifecycle workflow, run a full contradictory-wording audit, and continue Define only after all approved decisions are reconciled.


Historical checkpoint statements below that describe `TEvent` as the common interface, use `func(chan TEvent) TChannel`, omit `Errors()`, or state that Cycle 1 is blocked are superseded by the current approved baseline and decision register above. They are retained only as provenance.


## Pause checkpoint — 2026-11-24T20:06:00+02:00

- Current approved API, notification/error contract and deferred decoder-registration direction are recorded in active requirements and architecture; decision register D-001 through D-005 points to verified canonical revisions. D-006 (decoder never registers: timeout/terminal policy) remains Proposed.
- Reconciled plan baseline and explicitly marked old generic/API/blocked-state statements as superseded. Plan revision: `dd2f671d95b11f99b9d0eca2aa64b2a46fce1fc1`.
- Propagated template v1.1.0 approval-to-artifact governance to eventlog `steering.md`, lifecycle README, handoff template, validator, and workflow path filters.
- CI passed on `d7c5f766965a68901f0a1a98e693ff62814a05ff` (run 45456358070). This validates the registry, lock, and decision-register format; it does not execute Go tests or verify implementation behavior.
- The initial read-order deviation was disclosed and retained above. No implementation, build, or Go tests were started.
- **Next action:** complete a line-by-line audit for remaining contradictory historical statements and ensure all approved decisions are recorded; then continue Define with D-006 only after confirming the persisted state.
- This session is pausing Cycle 1 at Define. The registry will be set to paused and the cycle checkpoint persisted before stopping.


## D-006 recorded checkpoint — 2026-11-24T18:26:40.689Z

- **Approved decision:** when a required concrete Event decoder has not registered, the Channel Reader waits asynchronously and indefinitely. There is no arbitrary timeout, and absence alone does not produce a fatal Channel error. Replay proceeds when the decoder registers. Unknown or undecodable persisted Event types still fail closed; no consumer-facing API changes.
- **Canonical artifacts updated and fetched back:** `docs/project/architecture.md` at `4ea67d0ca45cc0bc2de7f46717844fbed42d4bb5`; `docs/project/requirements.md` at `3bbe21bdc79528aff55afa74f070f7d7ff629549`; Cycle 1 `plan/PLAN.md` at `0aa4496f30faf53279b41df8972acc91cadeeb9e`.
- **Decision register:** D-006 is now Recorded against the verified architecture revision; the requirements and plan carry the same decision.
- **Lifecycle state:** Cycle 1 is `paused` at Define; the execution lock is released. This pause supersedes the earlier 20:06 pause checkpoint and the intervening resume.
- **Verification:** each changed canonical artifact was fetched back from `main` and its content/blob revision inspected. No lifecycle validator, build, or Go tests have run in this session. No implementation has started.
- **Next:** resume Cycle 1 Define after the user responds to the next focused proposal on pointer/value conventions. Preserve the approved consumer-facing API and include both `TEvent` and `TChannel` in every `PrepareLog` example.


## Pause after D-006 — 2026-11-24T18:28:01.823Z

- D-006 is Recorded in the decision register and verified in architecture, requirements, and plan.
- Cycle 1 is paused at Define; the registry and handoff are updated together in one main-branch commit. No worker retains ownership while awaiting the user's next decision.
- No implementation, local validator, build, or Go tests were run in this checkpoint.
- Resume the same Cycle 1 from the persisted cycle checkpoint. The next proposed Define topic is whether generic Event and Channel type arguments must be pointers or may be values.


## D-007 recorded checkpoint — 2026-11-24T18:31:27.019Z

- **Approved decision:** both `TEvent` and `TChannel` generic arguments to `PrepareLog[TEvent, TChannel]` must be pointer types. Example: `PrepareLog[*OrderCreated, *OrdersChannel](ctx, newOrdersChannel)`. No consumer-facing API change.
- **Canonical artifacts updated and fetched back:** `docs/project/architecture.md` at `98918b4a4529f004fc14e267627571d462521214`; `docs/project/requirements.md` at `183fab3f4f4ba4bd5393e1858812fde8c8d5c718`; Cycle 1 `plan/PLAN.md` at `db1c05e39b31ae3c215bb8f96df2d5a0daffeb6e`.
- **Open distinction:** the pointer-only requirement is approved; the exact enforcement mechanism is not. Field-value reconstruction conventions also remain open.
- **Verification:** all three canonical artifacts were fetched back from `main` and the approved wording inspected. No lifecycle validator, Go build, or Go tests were run. No implementation has started.
- **Next:** decide how the pointer-only requirement is enforced without changing the approved two-parameter consumer-facing `PrepareLog[TEvent, TChannel]` API. All examples must include both type arguments.


## Pause after D-007 — 2026-11-24T18:31:57.733Z

- D-007 is Recorded and verified in the architecture, requirements, Cycle 1 plan and this handoff decision register.
- Cycle 1 is paused at Define; the registry and handoff are updated together in the same main-branch commit. No worker retains ownership while awaiting the user's next decision.
- No implementation, local lifecycle validator, Go build, or Go tests were run.
- Next Define topic: choose the enforcement mechanism for the approved pointer-only `TEvent` and `TChannel` requirement, preserving the existing two-parameter `PrepareLog[TEvent, TChannel]` consumer API. Resume from the persisted cycle checkpoint.


| D-008 | Enforce pointer-only `TEvent` and `TChannel` arguments by runtime reflection inside `PrepareLog`; return an error for either non-pointer type; preserve the two-parameter generic API | Recorded | docs/project/architecture.md, requirements.md and Cycle 1 plan | architecture `0b070a42cfa3e924a6744077259dd9ed6a3c90ae`; requirements `b2ccfeb1a4fa6a9a08d4679f9beeea9286bc2d95`; plan `8748c5d4152ef54af4f7b44a9972e1b7ced03b63` |

## D-008 recorded checkpoint — 2026-11-24T18:34:23.919Z

- **Approved decision:** `PrepareLog[TEvent, TChannel]` uses runtime reflection to validate that both generic type arguments are pointers. If either is not a pointer, preparation returns an error. The approved two-parameter generic signature and consumer-facing API remain unchanged.
- **Canonical artifacts updated and fetched back:** `docs/project/architecture.md` at `cef7d96797081f049782815f84b908ba2decfce7`; `docs/project/requirements.md` at `dfde24dbc3f1ddf1e5ad0dc1edf0edc6df42bc94`; Cycle 1 `plan/PLAN.md` at `56fba4e297c14ee23a49c4a4471f77e7e757c6a1`.
- **Verification:** approved text was fetched back and inspected. No implementation, lifecycle validator, Go build, or Go tests were run.
- **Next:** continue Define with prepared-instance reuse and the remaining internal lifecycle decisions. Preserve the consumer API and include both generic type arguments in every example.


## Pause after D-008 — 2026-11-24T18:34:48.958Z

- D-008 is Recorded and verified in architecture, requirements, Cycle 1 plan and this handoff decision register.
- Cycle 1 is paused at Define. The cycle registry and execution lock are updated together in the same main-branch commit; no worker retains ownership while awaiting the user's next decision.
- No implementation, lifecycle validator, Go build, or Go tests were run.
- Next Define topic: prepared-instance reuse — whether repeated `PrepareLog[TEvent, TChannel]` calls for the same Event/Channel type and schema return the same live pointer pair, and how incompatible reuse is handled. Preserve the approved consumer-facing API and include both generic type arguments in every example.


## D-009 recorded checkpoint — 2026-11-24T18:38:51Z

- **Approved decision:** compatible repeated `PrepareLog[TEvent, TChannel]` calls for the same concrete Event/Channel type pair and unchanged exposed field schemas return the same live Event and Channel pointer instances. Incompatible reuse of an already prepared type/schema registration returns a typed conflict instead of creating competing live instances.
- Event-specific preparations resolving to the same logical Channel reuse that Channel's existing pointer, runtime and single Reader progression, including across different concrete Event types. Each concrete Event type retains its own prepared Event instance.
- **Canonical artifacts updated and fetched back:** architecture commit `0b070a42cfa3e924a6744077259dd9ed6a3c90ae`; requirements commit `b2ccfeb1a4fa6a9a08d4679f9beeea9286bc2d95`; Cycle 1 plan commit `8748c5d4152ef54af4f7b44a9972e1b7ced03b63`. D-008 pointer-only runtime validation was also restored to the active architecture/requirements baseline.
- **Decision register:** D-009 is Recorded against the three fetched-back canonical artifacts.
- **Remaining details:** exact internal registry key, typed conflict taxonomy, and handling of distinct logical Channels of the same concrete Channel type remain open. No consumer-facing API change.
- **Lifecycle:** Cycle 1 remains in-progress at Define; cycle remains in-progress at Define.
- **Verification:** each canonical file was fetched back after publication. The active cycle registry was fetched back. No lifecycle validator, Go build, or Go tests were run; no implementation has started.
- **Governance note:** this session initially fetched other repository files before reading the execution lock, contrary to the mandatory first-read rule. The cycle registry and handoff are the authoritative progress records. Contributors coordinate through ordinary Git collaboration.
- **Next:** continue Define with the internal prepared-instance registry key and remaining lifecycle/cancellation/error semantics. Preserve the consumer-facing API and include both generic type arguments in every example.


## Pause after D-009 — 2026-11-24T18:41:59.190Z

- D-009 is Recorded and verified in the active architecture, requirements, Cycle 1 plan and this handoff.
- Cycle 1 is paused at Define while awaiting the next user decision. The registry and execution lock are being released together with this checkpoint in one atomic main-branch commit.
- Completed: D-009 prepared-instance reuse approval recorded; D-008 pointer-only runtime validation restored in the active architecture/requirements baseline. All canonical artifacts were fetched back after publication.
- Remaining: define the internal prepared-instance registry key and conflict taxonomy; finish initialization/cancellation/resource/error lifecycle; resolve identity mutation and notification retry/backlog/shutdown details; complete acceptance matrix and benchmark plan; reconcile and seek explicit approval to exit Define.
- No implementation, lifecycle validator, Go build, or Go tests were run.
- Next: resume this same Cycle 1 from the persisted cycle checkpoint and continue Define with the internal prepared-instance registry key. Preserve the approved consumer-facing API and include both generic type arguments in every example.


| D-010 | Separate Event type/schema registration, Event live-instance reuse by canonical Event identity, and Channel runtime reuse by canonical Channel identity; do not key Channel runtimes by concrete Go type alone | Recorded | docs/project/architecture.md, requirements.md and Cycle 1 plan | architecture `f3e7b8ea2dfc742fa297d19eadc20b5422ff8291`; requirements `a2ee2bea0db9357667da144ba21047d13f666b43`; plan `8e10bd9f63da60fefa2bfb76d3b1b5403899871b` |

## D-010 recorded checkpoint

- **Approved decision:** internal registry responsibilities are separated into Event type/schema registration, Event instance reuse by canonical Event identity, and Channel runtime reuse by canonical Channel identity.
- Different concrete Event types may share one logical Channel runtime. Distinct logical Channels may share one concrete Channel Go type and must not be merged by a type-only key.
- Type/schema registration conflicts are distinct from logical-instance identity. No consumer-facing API changes.
- The exact mechanism and ordering for obtaining/validating candidate Channel identity fields before runtime lookup remain open.
- No implementation, lifecycle validator, Go build, or Go tests were run.
- **Next:** decide how `PrepareLog[TEvent, TChannel]` obtains the Channel identity from the factory-created candidate and reconciles it with an existing runtime, preserving the approved public API.

| D-011 | Resolve Channel runtime by factory-candidate canonical identity: create candidate, inspect Fields(), reuse existing Channel pointer/runtime/Reader on identity hit (discard candidate), otherwise adopt candidate and create runtime; no consumer API change | Recorded | docs/project/architecture.md, requirements.md and Cycle 1 plan | architecture `7bd42a01d58a795b59a57fad06b523de330c6a1f`; requirements `42a543f2baac707437a6f5b0bec4c3292ecf52fc`; plan `0eff97488b28cd02a1d694cbd587d6536d20b79a` |

## D-011 recorded checkpoint

- **Approved decision:** `PrepareLog[TEvent, TChannel]` invokes the Channel factory to create a candidate, derives canonical Channel identity from the candidate's `Fields()`, and looks up the runtime registry by that identity. On a hit, it reuses the existing Channel pointer and runtime/Reader and discards the candidate; on a miss, it adopts the candidate and creates the runtime.
- The factory may be invoked even when its candidate is discarded. This is an internal sequence; the consumer-facing API remains unchanged.
- **Canonical artifacts updated and fetched back:** architecture commit `7bd42a01d58a795b59a57fad06b523de330c6a1f`; requirements commit `42a543f2baac707437a6f5b0bec4c3292ecf52fc`; Cycle 1 plan commit `0eff97488b28cd02a1d694cbd587d6536d20b79a`.
- **Next:** continue Define with the next unresolved preparation/identity detail. No implementation, lifecycle validator, Go build, or Go tests were run.

| D-012 | Channel factory is constructor-only: initializes candidate without starting goroutines, opening external resources, or other external side effects; runtime starts only after candidate adoption; discarded candidates are safe | Recorded | docs/project/architecture.md, requirements.md and Cycle 1 plan | architecture `f75de6313f4d4cc2015d11accfdbce413cfc8c72`; requirements `c934a1978dd01d5bf0459904371b88dbcf9d77cc`; plan `5e184e1685b2a5623900dfe422810cc347c1786f` |

## D-012 recorded checkpoint

- **Approved decision:** the Channel factory is constructor-only. It creates and initializes the candidate but does not start goroutines, open external resources, or perform other external side effects. The candidate may be discarded on a runtime-registry hit and must be safe to abandon. The library starts the runtime only after adopting a candidate for a new logical Channel.
- This is an internal lifecycle contract; the consumer-facing API remains unchanged.
- **Canonical artifacts updated and fetched back:** architecture commit `f75de6313f4d4cc2015d11accfdbce413cfc8c72`; requirements commit `c934a1978dd01d5bf0459904371b88dbcf9d77cc`; Cycle 1 plan commit `5e184e1685b2a5623900dfe422810cc347c1786f`.
- **Next:** continue Define with the next unresolved preparation/lifecycle detail. No implementation, lifecycle validator, Go build, or Go tests were run.

| D-013 | Channel runtime registry lookup and candidate adoption are atomic by canonical Channel identity; concurrent preparations converge on one adopted Channel pointer and one runtime/Reader | Recorded | docs/project/architecture.md, requirements.md and Cycle 1 plan | architecture `9c808cc9fd3c236885ec9f37110cd91cd0297541`; requirements `879710d9378ecf9e01c85b66acc55a0cd0befa31`; plan `1220813bda3e7ca43d0a1aa95127594ba7e77a1a` |

## D-013 recorded checkpoint

- **Approved decision:** Channel runtime lookup and candidate adoption are atomic by canonical Channel identity. Concurrent preparations resolving to the same logical Channel converge on one adopted Channel pointer and one runtime/Reader; competing preparations reuse the winner.
- This is internal synchronization and does not change the consumer-facing API.
- **Canonical artifacts updated and fetched back:** architecture commit `9c808cc9fd3c236885ec9f37110cd91cd0297541`; requirements commit `879710d9378ecf9e01c85b66acc55a0cd0befa31`; Cycle 1 plan commit `1220813bda3e7ca43d0a1aa95127594ba7e77a1a`.
- **Next:** continue Define with the next unresolved preparation/lifecycle detail. No implementation, lifecycle validator, Go build, or Go tests were run.


| D-014 | Concurrent preparations for different concrete Event types on the same logical Channel register independently and converge on one Channel pointer/runtime/Reader; no synchronous wait between registrations; historical replay retains its asynchronous decoder barrier | Recorded | docs/project/architecture.md, requirements.md and Cycle 1 plan | architecture `92d5b462d99a55763cbe5a217292e52a37e4e7e1`; requirements `cfe540632ab40b4d8214cbb39e21b515a2aba741`; plan `4930e55f635eed5d6f713812ba42e2ed928b7b80` |

## D-014 recorded checkpoint

- **Approved decision:** concurrent preparations for different concrete Event types that resolve to the same logical Channel register each Event type independently and converge on the same adopted Channel pointer, runtime and single Reader progression.
- A preparation does not synchronously wait for another Event type registration. Historical replay remains asynchronous and waits for every decoder required by the Channel's committed history; replay proceeds when the required decoder registers.
- No consumer-facing registration/activation API is added, and the approved consumer-facing API remains unchanged.
- **Canonical artifacts updated:** architecture commit `92d5b462d99a55763cbe5a217292e52a37e4e7e1`; requirements commit `cfe540632ab40b4d8214cbb39e21b515a2aba741`; Cycle 1 plan commit `4930e55f635eed5d6f713812ba42e2ed928b7b80`.
- No implementation, lifecycle validator, Go build or Go tests were run.


| D-015 | Successful concrete Event decoder registrations persist for the Log runtime lifetime, keyed by concrete Event type and exposed schema rather than Channel; no consumer unregister operation | Recorded | docs/project/architecture.md, requirements.md and Cycle 1 plan | architecture `3258af3313da4a40f43e7e3a7dd698bb4e5f1de1`; requirements `631b75c1855031203dee4da4651882f94f4404bc`; plan `1feb39b9c44f8a3e1aa015c7dba55df03dd1787e` |

## D-015 recorded checkpoint

- **Approved decision:** once a concrete Event decoder is registered successfully, that registration remains available for the lifetime of the Log runtime. It is not tied to the lifetime or completion of the individual `PrepareLog[TEvent, TChannel]` call that caused registration.
- Registration is associated with concrete Event type plus exposed field-name schema, not an individual Channel. No consumer-facing unregister operation is added; the consumer-facing API remains unchanged.
- **Canonical artifacts updated:** architecture commit `3258af3313da4a40f43e7e3a7dd698bb4e5f1de1`; requirements commit `631b75c1855031203dee4da4651882f94f4404bc`; Cycle 1 plan commit `1feb39b9c44f8a3e1aa015c7dba55df03dd1787e`.
- No implementation, lifecycle validator, Go build or Go tests were run.


| D-016 | Re-registering a concrete Event type with the same field-name schema reuses its decoder; a different schema returns typed schema conflict without mutating the existing registration or Readers | Recorded | docs/project/architecture.md, requirements.md and Cycle 1 plan | architecture `aa3f317df4daaba2ce40bed7530309493a9339a4`; requirements `1d2a28cae23e55a381eff6300b0bf18e680e1d3d`; plan `0861f77be8f991aaa7693190e9bde9b139cd7753` |

## D-016 recorded checkpoint

- **Approved decision:** decoder registration is keyed by concrete Event Go type plus exposed field-name schema. Re-registering the same type/schema reuses the existing registration. Re-registering the same concrete type with a different schema fails with a typed schema-conflict error and does not replace or mutate the existing registration or affect Readers already using it.
- This is an internal registration rule; the approved consumer-facing API remains unchanged.
- **Canonical artifacts updated:** architecture commit `aa3f317df4daaba2ce40bed7530309493a9339a4`; requirements commit `1d2a28cae23e55a381eff6300b0bf18e680e1d3d`; Cycle 1 plan commit `0861f77be8f991aaa7693190e9bde9b139cd7753`.
- No implementation, lifecycle validator, Go build or Go tests were run.


## Governance adoption — lock-free baseline 1.3.0 (2026-11-24)

The repository adopts the lock-free development governance from `gunesis-game/project-steering-template` 1.3.0.

- Repository-wide execution locks, worker claims, exclusive ownership, lock acquisition/release, lock recovery and single-worker serialization are removed.
- Multiple cycles and contributors may work concurrently. Cycle registry entries and handoffs are progress records, not ownership claims.
- Contributors coordinate through ordinary Git collaboration, review and conflict resolution.
- The existing Cycle 1 identity, sequence, execution order, Define phase, approved decisions D-001 onward, and project API constraints are preserved.
- D-017 through D-022 remain approved and recorded in requirements and architecture. The consumer-facing API remains unchanged.
- The next design work remains the unresolved concurrency question recorded in the current plan; no implementation is authorized until Define is completed and explicitly approved.
- Historical handoff entries that mention the former lock process describe superseded governance and are not operative instructions.


## Current governance and lifecycle state — supersedes older checkpoints

- The repository has adopted the lock-free governance baseline 1.3.0. No execution lock, worker claim, exclusive ownership or single-worker rule exists.
- Cycle 1 is the sole recorded cycle: sequence 1, execution order 1, standard workflow, `in-progress`, phase `define`.
- Older handoff entries that describe a blocked or paused state, an active/released lock, or lock-based resume procedures are historical provenance and are superseded by this current checkpoint and the registry.
- The latest approved project decisions D-017 through D-022 remain recorded in active requirements and architecture. The consumer-facing API remains unchanged.
- Continue Define with the unresolved concurrency decision. Do not begin implementation until the remaining Define work is reconciled and the user explicitly approves exit from Define.


## Proposed D-023 — Decision 8: queue ownership and concurrency

Status: Proposed; awaiting explicit user approval. The proposal is recorded in the Cycle 1 plan. It is not yet part of the approved architecture and does not authorize implementation. The consumer-facing API must remain unchanged.
