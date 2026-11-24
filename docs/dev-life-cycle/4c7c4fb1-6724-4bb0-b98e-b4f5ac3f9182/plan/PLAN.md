# Cycle 1 — Define plan

## Identity and status
- **Cycle ID:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182
- **Sequence / execution order:** 1 / 1
- **Workflow:** standard
- **Status / phase:** paused / define
- **Created:** 2026-11-22
- **Updated:** 2026-11-24

## Objective
Define eventlog's requirements, public API, persistence guarantees, internal lifecycle and acceptance criteria before implementation.

## Context and scope
eventlog is an embedded, disk-backed append-only Go event log. This cycle defines the product, contracts, identity, persistence, recovery, reading, lifecycle and quality criteria. Implementation, builds, Go tests, dependencies and multi-process/distributed writers are out of scope. No implementation/build/tests have started.

The complete pre-cleanup plan and handoff checkpoints are preserved verbatim in [plan history](../history/PLAN-history-2026-11-24.md) and [handoff history](../history/HANDOFF-history-2026-11-24.md). These archives are provenance only; current approved decisions below govern.

## Current approved baseline
Authoritative durable decisions are consolidated in `docs/project/requirements.md` and `docs/project/architecture.md`.

- **Public API:** sole approved entry point is `PrepareLog[TEvent, TChannel](ctx) (TEvent, TChannel, error)`. Consumers receive values, not a Log handle; internal Log owns append/read, persistence, recovery, scheduler and lifecycle.
- **Configuration:** mandatory, immutable, one-time `Config(options ...Option) error` with required `WithStorageDirectory(path)`. Nil, duplicate, missing, invalid options fail. Relative paths are resolved and lexically cleaned to absolute during Config; Config does not create storage. PrepareLog creates storage. `ErrConfigNotInitialized` and `ErrConfigAlreadySet` are approved.
- **Fields:** typed explicit primitive selectors; nullable primitive pointers; no nested fields; strict names/types/ranges and validation before writes.
- **Identity:** deterministic UUID v5 with canonical encoding. Event identity is concrete type + exposed field-name schema + designated identity field name/value, global across channels. Non-identity changes create immutable occurrences under the same logical ID; latest occurrence promotes HEAD. Channel identity is similarly canonical. Identical physical records are deduplicated. Identity-field mutation handling remains open.
- **Persistence:** strict versioned NDJSON channel/event/commit records; global sequence; commit markers; transaction count and SHA-256 framing over exact transaction bytes including newline; incomplete trailing transaction rolls back; complete malformed records fail.
- **Storage:** manifest-selected immutable segments; monotonic non-reused segment IDs; rebuildable generation-bound index; rotation at transaction boundaries; reader-safe generation-pinned compaction.
- **Durability:** sync every 1 second or 1 MiB pending writes. Flush and Close are durability boundaries; Append before sync is not power-loss durable.
- **Reader:** internal channel-scoped replay/live tail with gap-free, duplicate-free handoff, internal cursors, concrete reconstruction and type filtering.
- **Change manager:** one internal scheduler at 100 ms; no goroutine per field. Consumer synchronizes its own field writes.
- **Notifications:** channel accessor (signature TBD); post-persistence immutable Event snapshots; guaranteed eventual delivery without blocking persistence/scheduler; recovered backlog first, orderly shutdown drains, crash recovery restores pending deliveries; persistent failures become typed errors. Retry/termination and delivery error details remain open.
- **Platforms:** standard library and local filesystem; Linux/Windows parity; macOS excluded.

## Open Define decisions
1. Final Go-valid API signatures, constraints, selectors, pointer/value convention and occurrence result shape while preserving the approved PrepareLog call.
2. Prepared-value association, internal lifecycle, cancellation, reader/resource cleanup and terminal errors.
3. Identity-field mutation response and precise Channel change semantics.
4. Notification accessor/message/error shape, backlog durability/replay boundary, retry termination, delivery/error ordering and shutdown failure.
5. Snapshot consistency and consumer synchronization requirements.
6. Benchmark scenarios, memory/performance targets and complete acceptance matrix.

## Tasks and exit criteria
- [x] Purpose, ownership, scope, constraints and platform requirements.
- [x] Fields model, selector validation and canonical identity.
- [x] Typed persistence, transaction markers, checksum and recovery.
- [x] Durability, reader replay/live contract.
- [x] Segments, manifest, index and compaction.
- [x] Configuration contract and errors.
- [x] Poll cadence, Event identity/occurrence distinction, notification direction.
- [ ] Finalize remaining API/lifecycle/error details.
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

### Notification delivery: explicit acknowledgment and at-least-once
**Recommendation:** make the contract at-least-once, not exactly-once. A consumer-visible channel send is not evidence that application processing completed.
- Notification message carries an immutable Event snapshot, logical Event ID, occurrence ID, and opaque delivery token. Do not expose storage sequence, segment, or file offset.
- Provide receive-only `Notifications() <-chan Notification` plus a message `Ack() error` (or equivalent library-owned acknowledgment function). Ack commits the acknowledgment in the durable delivery ledger. Consumers must Ack only after their own handling succeeds.
- Persist pending delivery before exposing notification. Replay unacknowledged deliveries first in original order after restart. A crash after send but before Ack may redeliver; consumers must be idempotent. Once Ack is durably recorded, never redeliver that token.
- Delivery dispatcher uses a bounded channel; slow consumers cannot block append/polling. Durable backlog supplies backpressure. Define one in-flight notification per channel to simplify order and acknowledgment.
- A receive-only diagnostics/errors stream reports typed permanent or repeated delivery errors and quarantined-object faults. Error stream must itself be bounded with coalescing/status semantics so it cannot deadlock the log.
- Shutdown drains only while consumers Ack pending items. If the drain deadline expires, return a typed shutdown error, retain unacked durable backlog for restart, and close notification channels only after dispatcher stop; do not claim drain success. A channel send without Ack is unconfirmed delivery.
- Delivery/ack ordering is commit order for the channel. Event persistence and delivery enqueue are atomic; notification acknowledgments are separate durable records. The sync policy applies to both occurrence/enqueue and acknowledgment ledger records.

### Explicit trade-off
This proposal adds Ack/status API surface and delivery-ledger writes, but avoids claiming exactly-once consumer processing (which cannot be guaranteed across the send/process/ack crash window without consumer-side transactional integration). A consumer may see duplicates; loss of unacknowledged delivery is prohibited. Durable acknowledgment may add disk I/O and latency; batching acknowledgments is a possible future optimization, but not part of the initial contract.


## Approved notification public API decision — 2026-11-24

- The consumer-facing notification payload is the concrete Event type `T` only. No public `Notification[T]` wrapper, Event ID, Occurrence ID, delivery token, or consumer-facing Ack API.
- Notifications are channel-scoped, accessed through the Channel abstraction. Public accessor shape remains to be reconciled with the initialization/ownership mechanism.
- Delivery identity, occurrence identity, durable backlog, retries, replay bookkeeping, and acknowledgment of internal dispatch are private implementation details.
- Since the consumer provides no acknowledgment, eventlog cannot claim exactly-once consumer processing. Delivery/recovery semantics must be stated honestly; duplicates may occur in a crash window if internal dispatch is durably recorded after sending.
- API example: `for event := range channel.Notifications() { handle(event) }`.


## Approval checkpoint — notification public payload (2026-11-24)

**Approved:** The public notification payload is the concrete Event type `T` only. Remove the proposed public `Notification[T]` wrapper and public identity/occurrence/token metadata, and do not expose consumer-facing Ack. Notifications remain Channel-scoped. Delivery IDs, durable backlog, retry, replay, and dispatch acknowledgment are internal only. With no consumer Ack, eventlog cannot guarantee exactly-once consumer processing; public delivery/recovery guarantees must reflect the crash window and possible duplicates. The accessor and channel initialization/ownership mechanism still require design. This approval resolves the payload-shape open item, not the full notification API.


## Follow-on Define proposal — API, identity faults, notification source (not approved)

This proposal resolves remaining design questions with minimal public surface while retaining the approved `PrepareLog[TEvent,TChannel](ctx)` call shape and the approved Event-only notification payload.

### Type and preparation contract
- Require pointer type arguments (`PrepareLog[*MyEvent,*MyChannel]`). Pointer receiver `Fields()` is supported; value type arguments are rejected at preparation with a typed contract error.
- Each type/schema pair is prepared once per process and returns the same live Event/Channel pointer pair on repeated calls; incompatible pair/schema reuse is a typed conflict.
- Context cancellation only stops that caller's wait. Shared initialization is library-owned and proceeds independently; failed attempts clean partial resources and may retry under frozen Config. No object is returned until initialization succeeds.
- Field schemas and identity selectors are fixed at preparation. Drift is a typed error; consumer writes must synchronize themselves.

### Identity mutation
- Quarantine is object-scoped and latched. Event identity mutation preserves last committed snapshot, emits one typed fault, and stops tracking that Event. Channel identity mutation preserves its last committed snapshot, faults once, quarantines that Channel, and prevents further writes through it. Other Channels continue. No reset/rebind API in v1.
- Channel non-identity changes update the current snapshot only; future Event occurrences bind to it. Historical occurrences retain their Channel snapshot.

### Notification channel construction and ownership — corrected
- Public payload remains concrete Event `T`, with no public IDs, token, wrapper, or Ack.
- No public notification-helper struct and no consumer embedding requirement. The consumer defines its own Channel struct.
- The Channel interface must require a notification function/method that consumer-defined Channel types implement. eventlog uses that method for notification delivery.
- Exact method signature, channel construction/access contract, bounded buffering/backpressure, dispatch ownership, persistence/recovery and shutdown semantics remain open Define questions. Do not revive the helper struct proposal.
- Delivery guarantees must account for the absence of public consumer Ack; the exact semantics need to be finalized after the required consumer method is defined.

### Acceptance matrix and benchmark proposal
- API/type contract: compile examples; pointer-only enforcement; stable repeated preparation; incompatible schema/pair conflict; cancellation and shared initialization tests.
- Fields/identity: all primitive golden vectors; null vs zero; named types; selector failures; schema drift; Event and Channel identity mutation/quarantine isolation.
- Persistence: transaction fault injection at every write/commit/sync boundary; incomplete-tail rollback; malformed complete record fail-closed; checksum/count/sequence validation.
- Reader: replay/live handoff race, no gap/duplicate, channel isolation, mixed type filtering, cancellation and order.
- Notifications: recovered backlog precedence, ordering, slow consumer bounded memory, restart duplicates allowed only before internal dispatch confirmation, no loss of unconfirmed items, shutdown drain deadline/error, diagnostics coalescing.
- Storage: segment rotation, manifest atomic replacement, index rebuild, duplicate reconciliation, pinned-reader compaction and generation cleanup.
- CI/quality: Linux and Windows; race detector; parser/recovery fuzzing. Benchmark append throughput, replay throughput, tail latency, compaction cost, and resident memory with stalled consumers. Set numeric thresholds only after baselines.

### Approval needed before promotion
Approve/reject pointer-only preparation, shared-instance reuse, object-scoped identity quarantine, consumer-defined Channel notification method, at-least-once duplicate semantics, diagnostics/shutdown shape, and this acceptance matrix. These are proposals only; active requirements/architecture remain unchanged and implementation remains forbidden until explicit Define exit approval.


### Go feasibility sketch for the embedded notification helper (proposal)
The helper approach is feasible with a type-parameterized Channel constraint, e.g. conceptually:
```go
type EventType interface { Fields() Fields }
type Notifications[T EventType] struct { ch chan T }
func (n *Notifications[T]) Notifications() <-chan T { return n.ch }
func (n *Notifications[T]) initNotifications() { /* library initializes once */ }
type ChannelType[T EventType] interface {
    Fields() Fields
    initNotifications()
}
func PrepareLog[TEvent EventType, TChannel ChannelType[TEvent]](ctx context.Context) (TEvent, TChannel, error)
```
The unexported initialization method is promoted from the embedded helper, so external consumers can satisfy the constraint by embedding `eventlog.Notifications[TEvent]`; they cannot forge the initialization method. The helper is zero-value-safe, initialized before PrepareLog returns, and its channel is never closed/sent by consumer code. Runtime pointer-kind and typed-nil checks are still required because Go constraints do not cleanly enforce pointer-only type arguments.
This is a sketch to validate during final API design, not committed implementation or an approved contract.


## User correction — consumer-defined Channel notification method (2026-11-24)

**Supersedes the embedded `eventlog.Notifications[T]` helper proposal above.** The public API must not introduce a library-provided notification helper struct or require consumers to embed one. The consumer defines their own Channel struct, and the Channel interface requires a notification method/function implemented by that consumer-defined type. eventlog uses that method for notification delivery/access. Exact method signature and internal delivery mechanics remain to be designed; do not assume or promote an embedded helper. This user-confirmed requirement takes precedence over the earlier proposal.


## Approved canonical identity byte composition — 2026-11-24
The permanent UUID v5 namespace, Event/Channel domain prefixes, primitive tags, big-endian framing and schema sorting are already approved. This proposal only fills the exact composition gap; it does not alter those constants.

Proposed byte layout after the existing domain prefix:
1. Format version byte: `0x01` (explicitly redundant with the prefix's v1 marker, to make payload decoding/versioning self-contained).
2. Type package path: UTF-8, framed with u32 BE byte length.
3. Concrete type name: UTF-8, framed with u32 BE byte length (separate from package path).
4. Schema field count: u32 BE; each field name UTF-8 framed with u32 BE length, sorted bytewise by UTF-8 bytes.
5. Identity field name: UTF-8 framed with u32 BE length.
6. Identity value: existing permanent primitive tag followed by canonical payload bytes. Nullable nil uses Null tag only. Named primitive types use underlying primitive tag. The identity field must be included in the exposed schema; the value must match its declared field type.

Edge cases to explicitly decide/test: identity nil-pointer validity; float negative-zero distinction (raw IEEE currently implies -0 and +0 distinct); string byte-length vs rune count; invalid UTF-8 (Go strings may contain arbitrary bytes, so should reject invalid UTF-8 or define byte-preserving semantics); package/type names for aliases and generic instantiated types; overflow/framing limits.

Suggested golden-vector suite: one vector per permanent primitive tag, nil and non-nil nullable values, named primitives, type/schema variation, Event-vs-Channel domain separation, stable ID when non-identity values change, and sensitivity to identity value/type/schema changes. **User approved this byte layout on 2026-11-24.** Remaining edge-case validation questions are listed in active requirements and are not yet decided.
