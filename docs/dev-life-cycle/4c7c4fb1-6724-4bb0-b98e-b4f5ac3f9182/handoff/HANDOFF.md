# Cycle 1 — Current handoff (2026-11-24)

## State
- **Cycle:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182 (sequence 1, execution order 1)
- **Workflow / phase:** standard / define
- **Registry:** in-progress at Define
- **Execution lock:** active for ChatGPT (claimed 2026-11-24T16:30:00Z)
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
- Cycle 1's execution lock remains active and matches the registry's in-progress Cycle 1 claim; no release/reclaim was needed. Work continued under the existing claim.
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
