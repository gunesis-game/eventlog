# Cycle 1 — Define plan

## Identity and status
- **Cycle ID:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182
- **Sequence / execution order:** 1 / 1
- **Workflow:** standard
- **Status / phase:** in-progress / define
- **Created:** 2026-11-22
- **Updated:** 2026-11-24

## Objective
Define eventlog's requirements, public API, persistence guarantees, internal lifecycle and acceptance criteria before implementation.

## Context and scope
eventlog is an embedded, disk-backed append-only Go event log. This cycle defines the product, contracts, identity, persistence, recovery, reading, lifecycle and quality criteria. Implementation, builds, Go tests, dependencies and multi-process/distributed writers are out of scope. No implementation/build/tests have started.

The complete pre-cleanup plan and handoff checkpoints are preserved verbatim in [plan history](../history/PLAN-history-2026-11-24.md) and [handoff history](../history/HANDOFF-history-2026-11-24.md). These archives are provenance only; current approved decisions below govern.

## Current approved baseline
Authoritative durable decisions are consolidated in `docs/project/requirements.md` and `docs/project/architecture.md`.

- **Public API:** sole approved entry point is `PrepareLog[TEvent, TChannel](ctx, newChannel) (TEvent, TChannel, error)`, where `TEvent` is a concrete pointer type constrained by `Event`, `TChannel` is a concrete pointer type constrained by `Channel`, and `newChannel` is `func(chan Event) TChannel`. Runtime reflection rejects either non-pointer type argument before exposing prepared values. Compatible repeated preparation for the same concrete Event/Channel type pair and exposed schemas returns the same live pointer pair; incompatible reuse fails with a typed conflict. The common `Event` interface is the heterogeneous notification payload type. Channel exposes `Notifications() <-chan Event` and `Errors() <-chan error`. The earlier statement that `TEvent` itself is the common interface and that the factory accepts `chan TEvent` is superseded. Consumers receive values, not a Log handle; internal Log owns append/read, persistence, recovery, scheduler and lifecycle.
- **Configuration:** mandatory, immutable, one-time `Config(options ...Option) error` with required `WithStorageDirectory(path)`. Nil, duplicate, missing, invalid options fail. Relative paths are resolved and lexically cleaned to absolute during Config; Config does not create storage. PrepareLog creates storage. `ErrConfigNotInitialized` and `ErrConfigAlreadySet` are approved.
- **Fields:** typed explicit primitive selectors; nullable primitive pointers; no nested fields; strict names/types/ranges and validation before writes.
- **Identity:** deterministic UUID v5 with canonical encoding. Event identity is concrete type + exposed field-name schema + designated identity field name/value, global across channels. Non-identity changes create immutable occurrences under the same logical ID; latest occurrence promotes HEAD. Channel identity is similarly canonical. Identical physical records are deduplicated. Identity-field mutation handling remains open.
- **Persistence:** strict versioned NDJSON channel/event/commit records; global sequence; commit markers; transaction count and SHA-256 framing over exact transaction bytes including newline; incomplete trailing transaction rolls back; complete malformed records fail.
- **Storage:** manifest-selected immutable segments; monotonic non-reused segment IDs; rebuildable generation-bound index; rotation at transaction boundaries; reader-safe generation-pinned compaction.
- **Durability:** sync every 1 second or 1 MiB pending writes. Flush and Close are durability boundaries; Append before sync is not power-loss durable.
- **Reader:** internal channel-scoped replay/live tail with gap-free, duplicate-free handoff, internal cursors, concrete reconstruction and type filtering.
- **Change manager:** one internal scheduler at 100 ms; no goroutine per field. Consumer synchronizes its own field writes.
- **Notifications:** settled Channel accessor `Notifications() <-chan Event` using the common Event interface; post-persistence immutable concrete Event snapshots; delivery ordering follows the unified Channel Reader; slow consumers do not block persistence; recovered backlog precedes newer deliveries; orderly shutdown drains; crash recovery restores pending deliveries. Retry/termination and delivery error details remain open.
- **Platforms:** standard library and local filesystem; Linux/Windows parity; macOS excluded.

## Open Define decisions
1. Field selector signatures, pointer/value convention and occurrence result shape; the `PrepareLog` generic constraints, factory type and Channel notification/error accessors are approved.
2. Internal lifecycle, cancellation, reader/resource cleanup and terminal errors; the compatible prepared-instance reuse contract is approved, while registry-key details and distinct logical Channels of one concrete type remain open.
3. Identity-field mutation response and precise Channel change semantics.
4. Backlog durability/replay boundary, recoverable retry termination, and shutdown drain failure.
5. Snapshot consistency and consumer synchronization requirements.
6. Benchmark scenarios, memory/performance targets and complete acceptance matrix.

### Approved D-006 — missing required decoder (2026-11-24)

When committed Channel history requires a concrete Event decoder that has not yet registered, the
Channel Reader waits asynchronously and indefinitely. There is no arbitrary timeout, and decoder
absence alone does not produce a fatal Channel error. Replay proceeds when the missing decoder registers.
Unknown or undecodable persisted Event types still fail closed; they are not silently skipped. This
decision does not add or change any consumer-facing API.

## Tasks and exit criteria
- [x] Purpose, ownership, scope, constraints and platform requirements.
- [x] Fields model, selector validation and canonical identity.
- [x] Typed persistence, transaction markers, checksum and recovery.
- [x] Durability, reader replay/live contract.
- [x] Segments, manifest, index and compaction.
- [x] Configuration contract and errors.
- [x] Poll cadence, Event identity/occurrence distinction, notification direction.
- [ ] Finalize remaining lifecycle and notification error details.
- [ ] Complete acceptance matrix and benchmark plan.
- [ ] Reconcile current documentation and obtain explicit approval to exit Define.

**Acceptance:** approved baseline is unambiguous; open questions are actionable; requirements/architecture/plan/handoff/registry agree; history is subordinate to current decisions; no implementation begins before explicit Define approval.

## Documentation correction checkpoint — 2026-11-24
- Recurated the full active requirements and architecture, restoring substantive design rather than leaving it in history.
- Current active project docs contain the design baseline and integrate latest decisions. Archived pre-cleanup versions remain unchanged for provenance.
- Reviewed original requirements/architecture headings and the complete decision checkpoint sequence from the plan archive to ensure the current topics are represented.

## Verification and next action
- Manually inspected active requirements/architecture headings and sections for API, configuration, fields/selectors, identity/tag/UUID protocol, transaction/commit, schema, recovery, reader, scheduler/notifications, storage/manifest/index, compaction and portability.
- Lifecycle validator passed on commit `7243ec1bc8e3950c5376882333849e27b662c624` (run 45316732733). This run validated registry/lock only; subsequent project-doc-only commits did not trigger the workflow because its path filter excludes `docs/project/**`. No manual dispatch capability was available.
- An earlier claim-only commit had a failed validator because the lock was active while the cycle registry was still paused; the error was `must match exactly one in-progress registry cycle`. The registry was corrected in commit 7243ec1 and CI passed thereafter.
- No implementation, build or Go tests were started.

**Next:** continue Define on remaining API/lifecycle/identity-mutation/notification details, acceptance matrix and benchmarks; do not start implementation until Define is reconciled and explicitly approved.
## Resume design proposal — not yet approved (2026-11-24)

This section records recommendations to discuss and validate. It does not change the approved requirements or architecture baseline.

### API and mutability
- Prefer Event and Channel values returned as pointers to consumer structs, even though the public generic return values remain `TEvent` and `TChannel`. Change detection requires stable mutable instances; pointer receiver `Fields()` methods are natural.
- Require each concrete type's `Fields()` definition to be deterministic and stable for its lifetime. Field schema changes after preparation should be rejected, not dynamically reinterpreted.
- Keep identity designation explicit and exactly one identity field per Event/Channel. Selectors must resolve to direct fields; no reflection-based nested traversal or silent fallback.
- Avoid exposing internal `Log`, cursors, append/read methods, or storage sequence metadata. Keep the currently approved `PrepareLog` entry point.

### Lifecycle
- Treat preparation as process-wide, single-owner state tied to the configured directory. Repeated preparation of the same concrete Event/Channel types should be idempotent; conflicting type/schema combinations should return a typed error.
- Context cancellation should cancel the caller's wait/operation, but not leave shared initialization half-complete. Shared initialization should have library-owned cancellation and a terminal state; the precise retry rule needs a testable state table.
- Define typed errors for initialization, validation, identity mutation, storage corruption, and shutdown/delivery failure. Use `errors.Is` for stable sentinel categories and `errors.As` for detail types.

### Identity mutation
- Recommend identity fields be immutable after preparation. If polling detects a changed identity value, do not silently change the logical UUID or append under a new ID; report a typed identity-mutation error and retain the last valid persisted snapshot.
- Non-identity Event field changes create new immutable occurrences. Non-identity Channel changes update the latest Channel snapshot but do not independently create Event occurrences. These existing semantics remain.

### Notifications and memory
- Keep notification messages as typed occurrence snapshots, captured after successful commit; snapshots should be independent of consumer-owned mutable structs.
- Delivery should be ordered and lossless, but do not rely on an unbounded in-memory queue. Prefer a disk-backed durable backlog with a defined acknowledgment/replay boundary.
- The accessor should be read-only from the consumer perspective (receive-only channel). The consumer may not close or send on it.
- Retry policy should distinguish transient errors from permanent errors; permanent delivery errors must be surfaced through a lifecycle error channel/status, not silently discard pending occurrences. Exact API and termination policy remain open.

### Acceptance-test outline
- API compile-time contract and pointer receiver/value reconstruction behavior.
- Config validation matrix (nil, duplicate, missing, invalid path, repeated call, concurrent Config/PrepareLog).
- Field-selector resolution (wrong type, nullable/nonnullable mismatch, duplicate names/selectors, conditional selectors, nested fields, bounds, NaN/Inf).
- UUID golden vectors for every primitive tag, named types, nullable Null vs zero, identity stable across non-identity changes, Event/Channel domains separated.
- Append transaction fault injection at each write/commit/sync point; crash recovery truncation and malformed complete record fail-closed.
- Reader replay/live race test proving no gap/duplicate, channel isolation, mixed Event-type filtering, cancellation, and ordering.
- Change detection timing with deterministic test clock; concurrent consumer mutation synchronization documented/tested.
- Notification backlog/restart replay, slow consumer, order, no-loss, shutdown drain, and permanent-error handling.
- Segment rotation, manifest atomic switch, index rebuild, duplicate reconciliation, compaction with pinned readers, and generation cleanup.
- Linux/Windows CI, race detector, fuzz tests for record/schema/recovery parsing, and benchmarks (append, replay, tail latency, compaction, memory under stalled consumer). Numeric performance thresholds should be set only after baseline measurements.

**Next:** review this proposal with the user; accept/reject each recommendation explicitly before promoting any to active requirements/architecture.

## Define follow-up — concrete API and lifecycle recommendation (not approved)

### Proposed pointer-type contract
Use the existing generic return contract with pointer instantiations, e.g. `PrepareLog[*MyEvent, *MyChannel](ctx)`. Both type arguments must implement `Fields() Fields`; pointer receiver methods are supported. Reject non-pointer type arguments at preparation with a typed contract error rather than silently accepting values that cannot be monitored for mutation. Allocate fresh non-nil instances internally before exposing them. A nil/typed-nil input should never be accepted as a live object. The final selector/identity metadata mechanism still needs an API sketch and compile-time review before approval.

### Proposed lifecycle rules
- One process-wide configured directory and one initialized Log owner. Preparation for the same concrete Event/Channel pair/schema returns the same prepared live objects; incompatible pair/schema attempts fail with a typed conflict.
- Shared initialization is owned by eventlog, not by the first caller's Context. Each caller can stop waiting on its own context; cancellation of one caller does not cancel initialization needed by other waiters.
- A failed initialization releases partial resources and allows retry using the already-frozen Config. No prepared values are returned until initialization is fully successful.
- Keep lifecycle state/errors private except for documented notification delivery failures; make failure details wrapped and errors.Is/As-compatible.
- Consumer writes to returned Event/Channel fields must be externally synchronized. Polling is not a substitute for race-safe writes.

### Identity mutation decision recommendation
Event and Channel identity fields should be treated as immutable after preparation. Detect a changed identity before snapshotting; record a typed error, preserve the last persisted snapshot and identity, and stop change processing for the invalid object rather than repeatedly appending. Whether this should stop the entire Log or only that object is an explicit unresolved severity decision; recommend object-scoped quarantine to avoid taking down unrelated channels.

### Notification API recommendations
A read-only accessor should expose a receive-only typed channel. Delivery order follows commit order. Occurrence snapshots must be detached copies (not references to live consumer structs). A durable pending-delivery record should be committed alongside occurrence persistence so restart cannot lose a notification. Mark delivery complete only after the consumer-facing send succeeds; document that this is in-process handoff, not proof that consumer business processing completed. Use a separate typed error/status channel or lifecycle accessor for delivery failures; avoid closing a notification stream early while backlog remains.

### Decision table (for review)
| Topic | Recommendation | Still needs decision |
|---|---|---|
| Generic types | Instantiate with pointers | Enforce pointer-only at runtime vs compile time |
| Prepared instance | One shared live instance per type/schema pair | Whether repeated PrepareLog returns identical pointer pair |
| Context | Per-caller cancellation, shared initialization continues | Initialization timeout/ownership |
| Identity mutation | Immutable; typed error and quarantine | Object-level vs Log-wide failure |
| Notification | Receive-only channel, ordered durable backlog | Ack semantics, backlog policy, error accessor |
| Schema | Stable after preparation | How to detect drift and how errors are surfaced |

These are recommendations for Define, not active requirements. No implementation should begin until these are reviewed and promoted explicitly.

## Identity mutation and notification delivery — concrete proposal (not approved)

### Identity-field mutation: object-scoped quarantine
**Recommendation:** treat identity mutation as a terminal fault for the affected live object, not for the whole Log.
- Event identity changes: detect before snapshot/commit; do not create a new logical Event ID or occurrence. Preserve the last committed Event snapshot. Quarantine that Event object; unrelated Events/Channels continue.
- Channel identity changes: detect before snapshot/commit; do not create a new Channel ID or Channel record. Preserve last committed Channel snapshot and quarantine that Channel object. Events bound to that Channel are blocked from further persistence; unrelated Channels continue.
- Quarantine is latched until the live object lifecycle is replaced (no implicit auto-clear). No rebind/reset public API is proposed now. Emit one typed fault when quarantined, then avoid repeated error spam; expose ongoing status in diagnostics.
- Non-identity mutations remain supported: Channel snapshot updates are applied to future occurrences only; prior Event occurrences retain their committed Channel snapshot association.

### Notification delivery (superseded proposal)
This prior Ack-based proposal is not active. The notification stream element is the common `Event` interface; concrete Event values are delivered through `Notifications() <-chan Event`. No public wrapper,
Ack, delivery token, or callback/sink method is part of the API. Delivery bookkeeping remains internal.


## Approved notification public API decision — 2026-11-24

- The notification stream exposes concrete Event values through the common `Event` interface. No public `Notification[T]` wrapper, Event ID, Occurrence ID, delivery token, or consumer-facing Ack API.
- Notifications are channel-scoped, accessed through the Channel abstraction. Public accessor shape remains to be reconciled with the initialization/ownership mechanism.
- Delivery identity, occurrence identity, durable backlog, retries, replay bookkeeping, and acknowledgment of internal dispatch are private implementation details.
- Since the consumer provides no acknowledgment, eventlog cannot claim exactly-once consumer processing. Delivery/recovery semantics must be stated honestly; duplicates may occur in a crash window if internal dispatch is durably recorded after sending.
- API example: `for event := range channel.Notifications() { handle(event) }`.


## Approval checkpoint — notification public payload (2026-11-24)

**Approved:** The public notification payload is the concrete Event value itself, exposed through the common `Event` interface on one Channel-scoped stream. Remove the proposed public `Notification[T]` wrapper and public identity/occurrence/token metadata, and do not expose consumer-facing Ack. Delivery IDs, durable backlog, retry, replay, and dispatch acknowledgment are internal only. With no consumer Ack, eventlog cannot guarantee exactly-once consumer processing; public delivery/recovery guarantees must reflect the crash window and possible duplicates. The accessor and channel initialization/ownership mechanism still require design. This approval resolves the payload-shape open item, not the full notification API.


## Follow-on Define proposal — API, identity faults, notification source (not approved)

This proposal explores remaining design questions while retaining the approved `PrepareLog[TEvent,TChannel](ctx, newChannel)` call shape and the common-Event notification stream.

### Type and preparation contract
- Require pointer type arguments (`PrepareLog[*MyEvent,*MyChannel]`). Pointer receiver `Fields()` is supported; value type arguments are rejected at preparation with a typed contract error.
- Each type/schema pair is prepared once per process and returns the same live Event/Channel pointer pair on repeated calls; incompatible pair/schema reuse is a typed conflict.
- Context cancellation only stops that caller's wait. Shared initialization is library-owned and proceeds independently; failed attempts clean partial resources and may retry under frozen Config. No object is returned until initialization succeeds.
- Field schemas and identity selectors are fixed at preparation. Drift is a typed error; consumer writes must synchronize themselves.

### Identity mutation
- Quarantine is object-scoped and latched. Event identity mutation preserves last committed snapshot, emits one typed fault, and stops tracking that Event. Channel identity mutation preserves its last committed snapshot, faults once, quarantines that Channel, and prevents further writes through it. Other Channels continue. No reset/rebind API in v1.
- Channel non-identity changes update the current snapshot only; future Event occurrences bind to it. Historical occurrences retain their Channel snapshot.

### Notification channel construction and ownership
- **Settled public contract:** consumer-defined Channel types expose `Notifications() <-chan Event`.
  Consumer code receives concrete Event values from this stream.
- No public helper struct, embedding requirement, wrapper payload, Ack, or callback/sink method is permitted.
- The receive-only accessor is not itself writable by eventlog. The approved construction direction is the
  required `func(chan TEvent) TChannel` factory, with `TEvent` as the common Event interface; eventlog
  creates and owns the writable channel and supplies it to the consumer-defined Channel constructor.
- Backlog durability, replay/order, bounded memory, retry, shutdown and delivery failure reporting
  remain design topics. Without consumer Ack, exactly-once processing is not promised.


### Acceptance matrix and benchmark proposal
- API/type contract: compile examples; pointer-only enforcement; stable repeated preparation; incompatible schema/pair conflict; cancellation and shared initialization tests.
- Fields/identity: all primitive golden vectors; null vs zero; named types; selector failures; schema drift; Event and Channel identity mutation/quarantine isolation.
- Persistence: transaction fault injection at every write/commit/sync boundary; incomplete-tail rollback; malformed complete record fail-closed; checksum/count/sequence validation.
- Reader: replay/live handoff race, no gap/duplicate, channel isolation, mixed type filtering, cancellation and order.
- Notifications: recovered backlog precedence, ordering, slow consumer bounded memory, restart duplicates allowed only before internal dispatch confirmation, no loss of unconfirmed items, shutdown drain deadline/error, diagnostics coalescing.
- Storage: segment rotation, manifest atomic replacement, index rebuild, duplicate reconciliation, pinned-reader compaction and generation cleanup.
- CI/quality: Linux and Windows; race detector; parser/recovery fuzzing. Benchmark append throughput, replay throughput, tail latency, compaction cost, and resident memory with stalled consumers. Set numeric thresholds only after baselines.

### Approval needed before promotion
Remaining approvals/decisions include pointer-only preparation, shared-instance reuse, identity-mutation handling, diagnostics/shutdown details, and this acceptance matrix. The common Event notification accessor and factory element type are already settled; implementation remains forbidden until explicit Define exit approval.


### Superseded notification helper sketch
The embedded helper proposal was rejected by the user. It is retained in historical context only and
must not be treated as a viable current design or reintroduced.


## Superseded notification-sink checkpoint
Earlier sink-method discussion is superseded. The settled public accessor is `Notifications() <-chan Event`;
do not re-open or re-propose the sink-method decision. The remaining problem is internal send-channel
ownership/construction, not the consumer-facing API signature.


## Approved canonical identity byte composition — 2026-11-24
The permanent UUID v5 namespace, Event/Channel domain prefixes, primitive tags, big-endian framing and schema sorting are already approved. The exact byte composition below was explicitly approved by the user and has been promoted to active requirements/architecture.

Approved byte layout after the existing domain prefix:
1. Format version byte: `0x01` (explicitly redundant with the prefix's v1 marker, to make payload decoding/versioning self-contained).
2. Type package path: UTF-8, framed with u32 BE byte length.
3. Concrete type name: UTF-8, framed with u32 BE byte length (separate from package path).
4. Schema field count: u32 BE; each field name UTF-8 framed with u32 BE length, sorted bytewise by UTF-8 bytes.
5. Identity field name: UTF-8 framed with u32 BE length.
6. Identity value: existing permanent primitive tag followed by canonical payload bytes. Nullable nil uses Null tag only. Named primitive types use underlying primitive tag. The identity field must be included in the exposed schema; the value must match its declared field type.

Edge cases to explicitly decide/test: identity nil-pointer validity; float negative-zero distinction (raw IEEE currently implies -0 and +0 distinct); string byte-length vs rune count; invalid UTF-8 (Go strings may contain arbitrary bytes, so should reject invalid UTF-8 or define byte-preserving semantics); package/type names for aliases and generic instantiated types; overflow/framing limits.

Required golden-vector suite: one vector per permanent primitive tag, nil and non-nil nullable values, named primitives, type/schema variation, Event-vs-Channel domain separation, stable ID when non-identity values change, and sensitivity to identity value/type/schema changes. **User approved this byte layout on 2026-11-24.** Remaining edge-case validation questions are listed in active requirements and are not yet decided.


## Identity edge-case proposal — updated after user correction (not approved)
- **UUID-only identity:** Consumer-selected Event and Channel identity fields must contain a UUID. Primitive, string, numeric, nullable, or otherwise non-UUID identity values are forbidden. This is a user-directed requirement, not an optional proposal.
- **UUID encoding:** Use the UUID's canonical 16-byte representation; UUID-specific tag value and accepted Go UUID type/representation remain open.
- **UTF-8 metadata:** Recommend rejecting invalid UTF-8 in type/field metadata. UUID identity payload is binary, not UTF-8.
- **Type names:** Use runtime concrete named type identity (package path and type name). Type aliases resolve to the aliased concrete type. Generic instantiations use runtime concrete type names; verify with golden tests.
- **Framing limits:** Enforce uint32 framing limits and a documented practical maximum before allocation; define the exact cap with benchmarks.

Do not re-propose nil or primitive identity values. Add UUID-focused golden tests and metadata/type/framing tests.


## Documentation ownership review checkpoint — 2026-11-24
- Corrected `requirements.md` to focus on product behavior, observable guarantees, constraints and acceptance expectations. Implementation/storage mechanisms were removed from its body as prescriptions; technical open questions now refer to architecture and Cycle 1.
- Clarified the owner boundary in `architecture.md`, `coding-standards.md`, and `steering.md`: architecture owns approved technical design; coding standards own implementation conventions; steering gives general documentation governance; plans/handoffs are temporary checkpoints.
- Preserved technical details in architecture rather than treating this as a size-reduction task.
- Reconciled outdated UUID-only identity encoding and notification accessor language in architecture with current approved decisions.
- No implementation or Go build/tests were started. Connector-only access means local branch/working-tree checks and schema-validator execution were unavailable.
- Next: fetch back all updated documents, check cross-document contradictions, and continue the remaining Define work. No Define exit or implementation authorization is implied.


## User correction — notification accessor finalized (2026-11-24)
- **Approved consumer-facing accessor:** `Notifications() <-chan Event` on the Channel interface; The consumer receives from a one-way, receive-only stream.
- Remove `Notify(event TEvent) error` from the proposed/active API. It is not part of the Channel interface and must not be reconsidered.
- No public helper struct or embedding requirement. Eventlog must not ask consumers to implement a callback/sink.
- **Remaining technical blocker:** a receive-only accessor does not expose a writable channel to eventlog. Define a send-side ownership/construction mechanism that preserves this accessor and consumer-owned Channel struct, without inventing a `Notify` callback or requiring public helper embedding. Do not begin implementation until the mechanism is resolved and Define is approved.


## Notification channel construction decision — 2026-11-24

The internal ownership gap is resolved with a required typed channel-construction factory parameter on public preparation. `TEvent` is the common Event interface, so the factory channel is heterogeneous. Go cannot require a named constructor function on a type via interface constraints, but it can compile-time enforce the exact factory signature at the API call site.

```go
func PrepareLog[TEvent EventType, TChannel ChannelType](
    ctx context.Context,
    newChannel func(chan TEvent) TChannel,
) (TEvent, TChannel, error)
```
Consumer passes their constructor, e.g. `NewOrdersChannel`, which must have the exact compatible signature:

```go
func NewOrdersChannel(send chan Event) *OrdersChannel {
    return &OrdersChannel{notifications: send}
}
```
`TEvent` in this API is the common `Event` interface, not `*OrderEvent`. The Channel stores a receive-only field and returns it through `Notifications() <-chan Event`; eventlog creates the bidirectional `chan Event`, retains send-side ownership, and supplies it to the constructor. The same stream can therefore carry `*OrderCreated`, `*OrderCancelled`, and other concrete Event implementations. The consumer does not send or close the channel. No `Notify` sink/callback, public helper/embedding, or notification wrapper payload is introduced. Delivery/backlog/recovery/shutdown semantics remain separately open.

## User correction — heterogeneous Channel notification stream (2026-11-24)

**Approved contract clarification:**
- `Notifications() <-chan Event` is defined by the Channel interface.
- `Event` is the common interface implemented by concrete Event types.
- One consumer-defined Channel may carry multiple concrete Event types (for example, `OrderCreated` and `OrderCancelled`) through the same notification stream.
- The consumer-facing Channel API must remain unchanged. There is no `Notify` callback/sink method, no public helper/embedding requirement, and no per-concrete-type notification stream.

**[SUPERSEDED] Clarification of the factory type:** This earlier checkpoint incorrectly treated `TEvent` as the common `Event` interface. The current approved contract at the top of this plan makes `TEvent` a concrete type constrained by `Event` and uses `func(chan Event) TChannel` for heterogeneous notifications.

**Next Define work:** establish how the approved event-registration barrier is completed without adding a consumer-facing lifecycle method, then finish internal lifecycle/error and identity-mutation details. The common Event type and factory relationship are settled. No implementation authorization is implied.

## Define catch-up — recovery, Channel convergence and Reader progression (2026-11-24)

These decisions are part of the current design baseline and must be reflected consistently in requirements and architecture:

1. **Recovery precedes Channel resolution.** Load and validate the existing internal event log/system of record first. Only after recovery succeeds may `PrepareLog` resolve or create a runtime Channel.
2. **Channel identity is Channel-scoped.** Event-specific preparation must resolve to the same logical runtime Channel when the Channel identity matches. It must not create a separate Channel runtime or notification stream for each concrete Event type.
3. **One heterogeneous notification stream.** The Channel interface accessor is `Notifications() <-chan Event`, where `Event` is the common interface implemented by concrete Event types. The same Channel may carry `OrderCreated`, `OrderCancelled`, and other concrete Event values.
4. **One Reader progression per runtime Channel.** Historical replay and live tailing are one continuous progression. Do not create separate historical/live Readers or combine independent typed Readers to synthesize notifications. Typed Readers may filter for typed reads but do not define notification ordering.
5. **Ordering authority.** Successful append Commit is authoritative. Notification delivery is driven by the Channel's unified Reader progression in committed global-sequence order. No notification may be emitted before successful Commit.
6. **Backpressure and failures.** Slow consumers may hold up notification delivery but must not stall persistence/append, reorder, or skip committed Events. A delivery failure cannot rollback or delete a committed Event; Reader/delivery progression must remain recoverable. Without consumer acknowledgment, exactly-once consumer processing is not promised.
7. **Ownership and shutdown.** eventlog owns the writable side, Reader, delivery worker and runtime lifecycle. Shutdown stops new preparation/appends, coordinates outstanding commits, cancels Reader/delivery workers, waits for senders to stop, closes notification streams, then releases storage resources. Consumers do not get a public lifecycle method.

Event type reconstruction uses the project's existing internal registration/codec mechanisms. Unknown persisted Event types are errors and are never silently skipped.

These decisions are recorded from the design discussion and are not permission to start implementation. Remaining Define work is to reconcile the precise `PrepareLog` registration/convergence behavior, internal lifecycle/error boundaries, identity mutation, and the acceptance/benchmark matrix, then obtain explicit approval to exit Define.

## Approved Define decision — Channel event-registration barrier (2026-11-24)

**Approved:** Every concrete Event type and decoder that may appear on a logical Channel must be prepared/registered before heterogeneous historical notification replay starts. This prevents the Reader from encountering a persisted concrete type that has not yet been registered. Unknown persisted Event types must not be silently skipped.

**API constraint:** The consumer-facing API remains unchanged. Do not add `Activate()`, `Start()`, `RegisterEventType()`, `Close()`, acknowledgments, or consumer checkpoints. Eventlog owns Reader activation and shutdown. `PrepareLog` is event-specific; Channel resolution, notification delivery, and runtime lifecycle are Channel-specific. All event-specific preparations for the same Channel must converge on the same runtime Channel and Reader progression.

**Unresolved mechanism:** Sequential `PrepareLog` calls do not provide a completion signal proving that every Event type for a Channel has been registered. No existing completion boundary has yet been established from repository evidence. Determine whether an existing internal boundary can serve or whether an internal coordination mechanism is required. Do not invent a consumer-facing completion method or start implementation until this is resolved.

## Proposed resolution — registry-driven replay barrier (not approved)

The only API-compatible candidate identified so far is to make the existing Event type/codec registry the authoritative decoder source before Channel replay:

1. Initialize/load the existing process-wide Event type/codec registry before the first `PrepareLog` can activate any Channel Reader.
2. Recover and validate the authoritative event log first. Enumerate concrete Event type identifiers present in committed history and require a decoder for every referenced type. Missing/unknown decoders fail closed; no Channel runtime is resolved and no historical replay starts.
3. Only after recovery and decoder validation succeed, resolve/create the Channel runtime and attach the event-specific `PrepareLog` registration to that Channel. All such registrations reuse the same Channel runtime and Reader progression.
4. Start the unified Reader only after the preceding checks. New Event types must be registered through the existing codec mechanism before an append of that type can commit.

This would provide a deterministic historical-replay barrier without a consumer-facing `Activate`, `Start`, or `RegisterEventType` method and without treating sequential `PrepareLog` calls as a completion signal. **It is not approved yet:** the repository currently contains no Go source or codec implementation, so it is not established that the existing registry is process-wide, available before recovery, or enumerable by persisted type ID. Verify those facts before promoting this proposal or implementing it.

## Blocked checkpoint — 2026-11-24T17:17:13Z

**[SUPERSEDED]** This earlier checkpoint marked Cycle 1 blocked pending a process-wide codec-registry confirmation. The user subsequently approved generic concrete-type-driven decoder registration and asynchronous waiting for all decoders required by Channel history. Cycle 1 is now in-progress at Define. The exact policy for a decoder that never registers remains open; no consumer-facing API additions are permitted.


## Governance reconciliation checkpoint — 2026-11-24T20:02+02:00

- Resumed the existing Cycle 1 at Define; cycle identity, sequence, execution order and phase were preserved.
- Reconciled approvals made after the previous handoff: `TEvent` is the concrete generic type constrained by `Event`; `TChannel` is constrained by `Channel`; the factory accepts `func(chan Event) TChannel`; the heterogeneous stream uses `Notifications() <-chan Event`; Channel also exposes `Errors() <-chan error`.
- Approved error behavior: shared Channel-scoped error stream, buffer capacity one, first fatal error retained internally, best-effort non-blocking publication, fatal Reader termination, then notification/error stream closure; recoverable errors may continue only when ordering and completeness remain safe.
- Approved decoder direction: concrete registration derives from generic `TEvent`; historical replay waits asynchronously until all concrete decoders required by committed Channel history are available; no consumer registration/activation API. Missing/never-registered decoder policy remains an implementation/design detail to verify; no source or codec registry has yet been found.
- Canonical requirements updated at `4b7bc62c6cf851bf62a86bd7a11286c761063f02`; architecture updated at `d4fe4e197b07385e95159d7090e8110f4805b68f`; both were fetched back and compared exactly.
- No implementation, build or Go tests were started.


## Supersession note — approved API and error decisions (2026-11-24)

The following earlier proposals/checkpoints in this plan are superseded by the current approved baseline above and the decision register in the current handoff:
- Any statement that `TEvent` is the common `Event` interface. `TEvent` is a concrete type constrained by `Event`.
- Any factory signature `func(chan TEvent) TChannel`. The approved factory is `func(chan Event) TChannel`.
- Any Channel interface limited to `Notifications()` alone. It also exposes `Errors() <-chan error`.
- Any proposal that consumers must call a registration, activation, start, stop, close, or acknowledgement method. No such public method is approved.
- Any earlier error-delivery proposal inconsistent with the approved capacity-one, retained-first-fatal-error, best-effort non-blocking behavior and fatal Reader termination contract.

These superseded statements remain in the plan only as historical discussion/provenance and must not be used as the current specification.


[Governance reconciliation note] Historical proposal/checkpoint wording that conflicts with the current approved API or current Cycle 1 status is superseded by the current approved baseline and decision register. Do not treat historical wording as operative.


## Approved D-008 / D-009 — preparation contract and reuse

- **D-008:** Both `TEvent` and `TChannel` generic arguments must be pointers. `PrepareLog[TEvent, TChannel]` enforces this using runtime reflection and returns an error for either non-pointer argument. The two-parameter generic API is unchanged.
- **D-009:** Repeating a compatible preparation for the same concrete Event/Channel type pair and unchanged exposed field schemas returns the same live Event and Channel pointer instances. Incompatible reuse of an already prepared type/schema registration returns a typed conflict rather than creating competing live instances.
- Event-specific preparations resolving to the same logical Channel reuse its existing Channel pointer, runtime and single Reader progression, including across different concrete Event types. Each concrete Event type retains its own prepared Event instance.
- The exact internal registry key, typed conflict taxonomy, and handling of distinct logical Channels of the same concrete Channel type remain open. No consumer-facing API is added or changed.
- Canonical requirements and architecture updates were published and fetched back before this plan update. No implementation, build or Go tests were started.


## Approved D-010 — separate type registration from logical instances

- Keep three internal concerns distinct: Event type/schema registration, Event live-instance reuse, and Channel runtime reuse.
- Event type registration is keyed by concrete Event type plus exposed field-name schema.
- Event live instances are keyed by canonical Event identity.
- Channel runtimes are keyed by canonical Channel identity, not just concrete Channel Go type.
- Different concrete Event types may share one logical Channel runtime; distinct logical Channels may use the same concrete Channel type without being merged.
- Type/schema conflicts are separate from logical identity conflicts.
- No consumer-facing API or Channel interface changes.
- Still open: the exact preparation ordering and mechanism for obtaining/validating candidate Channel identity fields before Channel runtime lookup.


## Approved D-011 — resolve Channel runtime from factory candidate

`PrepareLog[TEvent, TChannel]` invokes the supplied factory to create a candidate Channel, derives canonical Channel identity from `Fields()`, then looks up the runtime registry by that identity. A hit reuses the existing Channel pointer and runtime/Reader and discards the candidate; a miss adopts the candidate and creates the runtime. The factory may be called even on a registry hit. This internal sequence preserves the approved consumer-facing API.


## Approved D-012 — Channel factory is side-effect-free construction

The supplied Channel factory only creates and initializes a candidate Channel value. It does not start goroutines, open external resources, or perform other external side effects. A candidate can be discarded on a runtime-registry hit, so it must be safe to abandon. The library starts the Channel runtime only after adopting a candidate for a new logical Channel. No consumer-facing API change.


## Approved D-013 — atomic Channel runtime resolution

Runtime lookup and candidate adoption are atomic under canonical Channel identity. Concurrent preparations resolving to the same logical Channel result in one adopted Channel pointer and one runtime/Reader; competing preparations reuse the winner. No consumer-facing API change.


## Approved D-014 — concurrent Event-type preparation

Different concrete Event types may be prepared concurrently for the same logical Channel. Each Event type registers independently; preparations converge on the one Channel pointer/runtime/Reader selected by canonical Channel identity and atomic adoption. A preparation does not synchronously wait for another Event type registration. The shared Reader's historical replay barrier remains asynchronous and waits for all decoders required by committed history. The consumer-facing API remains unchanged.


## Approved D-015 — Event decoder registration lifetime

A successful concrete Event decoder registration remains available for the lifetime of the Log runtime. It is independent of the lifetime/completion of the `PrepareLog[TEvent, TChannel]` call that registered it and is available to any Channel Reader whose committed history requires it. Registration is keyed by concrete Event type and exposed field-name schema, not by Channel. No consumer-facing unregister operation or API change.


## Approved D-016 — conflicting Event decoder registrations

Decoder registration is keyed by concrete Event Go type plus exposed field-name schema. Re-registering that type with the same schema reuses the existing registration. Re-registering it with a different schema returns a typed schema-conflict error and cannot replace/mutate the existing registration or affect Readers already using it. No consumer-facing API change.


## Approved D-017 through D-022 — field selection and internal mutation pipeline

These approvals are part of the active Define baseline and are recorded in `docs/project/requirements.md` and `docs/project/architecture.md`:

- **D-017 — Typed field selectors:** `FieldOf` and `FieldOfNullable` use typed selector callbacks to select direct fields. Named primitive types are supported; nullable pointer state preserves nil separately from a pointer to zero. No consumer API change.
- **D-018 — Reconstruction and snapshot isolation:** Historical Event occurrences are reconstructed into fresh concrete Event pointers. Notification snapshots are immutable and isolated from the live prepared Event, persistence state and other notifications. Historical Channel snapshots are separate from the live Channel pointer.
- **D-019 — Immutable Event identity:** The designated identity of a prepared Event instance cannot change. A detected mutation is rejected and reported through the existing Channel error stream; it cannot create a new logical Event or alter history.
- **D-020 — Internal mutation queue:** Detected selected-field changes become immutable snapshots in an eventlog-owned internal queue. Persistence workers dequeue and persist them. Notifications are eligible only after commit. No public queue, callback, lock or method is introduced.
- **D-021 — Per-Event FIFO and duplicate suppression:** Preserve FIFO enqueue order for each logical Event and never silently drop or replace an enqueued snapshot. Suppress only consecutive unchanged observed states; A→B→A remains distinct when observed. Global sequence numbers are assigned centrally at commit.
- **D-022 — Bounded queue and backpressure:** The internal queue is bounded. Capacity pressure pauses/delays internal monitoring rather than blocking consumer field assignments. Already-enqueued snapshots remain retained; intermediate mutations while monitoring is paused may be missed.

All six decisions preserve the existing consumer-facing API, including `PrepareLog[TEvent, TChannel]`, `Fields() Fields`, `Notifications() <-chan Event`, and `Errors() <-chan error`.

## Governance adoption — lock-free template baseline 1.3.0

Eventlog adopts the lock-free governance baseline from `gunesis-game/project-steering-template` 1.3.0. There is no repository-wide execution lock, worker claim, exclusive ownership or single-worker restriction. Multiple contributors and cycles may proceed concurrently; the registry and handoff record progress, while contributors coordinate through ordinary Git collaboration, review and conflict resolution. The existing Cycle 1 ID, sequence, execution order and Define phase are preserved.



## Proposed D-023 — Log-wide queue ownership and persistence concurrency (not approved)

**Question:** Should eventlog use one internal queue owned by each Log, with multiple persistence workers processing different logical Events concurrently while preserving serial processing for each individual logical Event?

Proposed model:

1. Each Log owns one bounded internal ingress queue for immutable snapshots captured by its monitors.
2. A Log-internal dispatcher dequeues snapshots and schedules them by canonical logical Event identity.
3. At most one persistence operation for a given logical Event runs at a time. Its snapshots retain FIFO enqueue order; a later snapshot for that Event cannot overtake an earlier one.
4. Different logical Events may be persisted concurrently by multiple workers, avoiding unnecessary serialization between unrelated Events.
5. Global sequence numbers are assigned centrally at the commit/serialization point. The committed sequence defines the total order observed by Readers and notifications.
6. Queue capacity, dispatch lanes/scheduling and worker count remain internal implementation details. Backpressure follows D-022: pause/delay monitoring when bounded capacity is exhausted; never drop already-enqueued snapshots and never block consumer field assignments.

This proposal composes with approved D-020 through D-022. It introduces no consumer-facing queue, method, callback, configuration requirement or API change.

**Trade-off to confirm:** the shared ingress queue and scheduler add internal coordination complexity, but allow unrelated logical Events to make persistence progress concurrently while preserving each Event's own history. A simpler single persistence worker would serialize unrelated Events unnecessarily.

Status: Proposed only. Do not implement or update the canonical architecture until the user explicitly approves this decision.
