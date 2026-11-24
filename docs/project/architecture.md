# Architecture

**Status:** Define-phase design baseline (Cycle 1). This active document owns approved technical design:
components, responsibilities, boundaries, interfaces, algorithms, data/storage representations, lifecycle,
and design trade-offs. Requirements are owned by [requirements.md](requirements.md); this document must
trace to and satisfy them rather than redefine product behavior. Approved decisions are integrated by
topic; historical checkpoints are provenance only. Proposals and unresolved alternatives must be clearly
labeled and never presented as approved. It describes intended design, not implementation or verification
evidence.

## System boundary

eventlog is an embedded Go library using local filesystem storage. A single process owns a log. It
supports concurrent goroutine use but does not coordinate multiple processes or distributed writers.
Consumer Event/Channel domain semantics remain outside the library; the consumer explicitly selects
persisted fields.

The public preparation entry point is `PrepareLog[TEvent, TChannel]`. `TEvent` is a concrete Event type constrained by the common `Event` interface; `TChannel` is a concrete Channel type constrained by `Channel`. The required factory is `func(chan Event) TChannel`, so the single Channel notification stream can carry heterogeneous concrete Event values. The Channel interface exposes `Notifications() <-chan Event` and `Errors() <-chan error`. These approved contracts must not be expanded or changed without explicit user approval.
Consumers receive values, not a Log handle; internal Log owns append/read/next, persistence, recovery,
change manager, synchronization and lifecycle. No public Log handle or
Append/Read/Next/Close/Flush/Compact API is approved.

Runtime configuration is mandatory and independent of generics/Context: `Config(options ...Option) error`
with required `WithStorageDirectory(path)`. Config validates before publishing; nil, duplicate, missing
and invalid options fail, and validation failure is retryable. Successful config is immutable and
one-time; repeated or late calls match `ErrConfigAlreadySet`. PrepareLog before Config matches
`ErrConfigNotInitialized`. Relative paths become absolute, lexically cleaned at Config time (no symlink
resolution); Config does not create storage, PrepareLog does. Config and first initialization are
coordinated: concurrent PrepareLog callers await the same attempt and honor their own cancellation;
failed initialization may retry with frozen config. Context carries cancellation/deadline only;
configuration is not inferred from Context or environment.

## Logical components

- **Public API:** The only approved public entry point is package-level generic `PrepareLog[TEvent,
  TChannel](ctx, newChannel)`, returning `(TEvent, TChannel, error)`. `TEvent` is a concrete type constrained
  by `Event`; `TChannel` is a concrete type constrained by `Channel`; `newChannel` is the required
  `func(chan Event) TChannel` factory. The Channel interface exposes `Notifications() <-chan Event` and
  `Errors() <-chan error`; the notification stream carries heterogeneous concrete Event values.
  Consumers receive event and channel values, not a Log handle. The package owns the internal `Log`
  instance and all append/read/next/lifecycle operations; none of those operations are exposed to consumers
  at this stage. Consumer event and channel types implement `Fields() Fields`; consumers set or change their
  own field values. A single process owns a log at a time, with concurrent goroutine use coordinated
  internally.
- **Field builder and validator:** Selector callback supplies typed field selectors for direct fields.
  `FieldOf` selects non-nullable primitives; `FieldOfNullable` selects nullable primitive pointers and
  returns pointer-to-pointer (nil is explicit Null). Field names are inferred. Resolve a selector by
  invoking it on a zero-value probe and comparing its returned pointer with candidate direct-field
  addresses; exactly one match must be found. This does not inspect source and may miss conditional
  selectors that resolve on the probe. Validate names, uniqueness, type, nilability, integer range and
  finite floats before writing; invalid input is not omitted or coerced.
- **Identity encoder:** Canonically encodes type identity, exposed field-name schema, and designated
  identity field name/value; non-identity field values are excluded from logical ID so updates retain
  identity. UTF-8 names sort bytewise, variable components/counts use uint32 big-endian framing,
  primitive tags and exact widths/bits are explicit. UUID v5 uses fixed namespace
  `26612f1d-f648-483a-9cfe-59c18f113891` and separate Event/Channel domain prefixes
  (`eventlog\\x00event\\x00v1\\x00`, `eventlog\\x00channel\\x00v1\\x00`).
- **Record serializer/schema validator:** Encodes versioned strict JSON records as NDJSON and validates
  each record against the schema matching its `schema_version`. Standalone schemas are canonical; runtime
  embeds those files using `go:embed`.
- **Append transaction writer:** Coordinates concurrent appends, sequence assignment, transaction
  records and first-class Commit marker. New Channel and Event records are one transaction; validation
  failure writes nothing. Recoverable atomicity uses Commit and rollback/truncation for interrupted
  writes.
- **Segment manager:** Writes authoritative segment files, rotates at transaction boundaries, and
  maintains ordered global record sequence. A transaction exceeding target segment size may be placed
  alone in an oversized segment.
- **Recovery/index manager:** Validates manifest, segment generation, records, sequences, transaction
  Commit IDs/count/checksums and derived index. Incomplete trailing transaction truncates to its start;
  index is rebuilt from segment source if missing/inconsistent and atomically replaced.
- **Reader/tailer:** One unified Reader progression serves each runtime Channel, streams committed
  history, catches concurrent appends, and transitions to live tailing without gaps or duplicates. Global
  sequence/cursor/offset management is internal; consumers do not provide or receive storage cursors.
  Cancellation and errors are explicit. Concrete Event values are reconstructed through the project's
  existing internal event-type registration/codec mechanisms. Unknown persisted Event types and decode
  failures are errors, never silently skipped. Typed Readers may filter concrete Event types for typed
  reads, but independent typed Readers do not compose the heterogeneous notification stream.
- **Preparation barrier:** `PrepareLog` is event-specific, while Channel resolution, notification delivery,
  and runtime lifecycle are Channel-specific. Concrete Event registration is derived internally from the
  concrete `TEvent` generic argument. Historical notification replay waits asynchronously until every
  decoder required by committed history for that logical Channel is available; the first `PrepareLog` call
  must not block in a way that prevents subsequent registrations. If a required decoder has not registered,
  the Channel Reader waits indefinitely: there is no arbitrary timeout, and absence alone does not produce
  a fatal Channel error. Replay proceeds when the decoder registers. No consumer-facing registration or
  activation method is added.
- **Compactor:** Produces replacement segments/index without modifying reader-pinned files,
  validates/checksums and syncs them, finalizes, atomically switches manifest/index, and deletes old
  generations only after the last reader releases its pin.
- **Change manager/scheduler:** One managed scheduler per prepared Log/change manager polls selected
  Event/Channel fields every 100 ms; no per-field goroutines. Cadence is internal/test-only. Consumer
  synchronizes its own mutations. Detected changes route to internal persistence.
- **Notification dispatcher/backlog:** The Channel interface exposes `Notifications() <-chan Event` and `Errors() <-chan error`. One Channel owns one heterogeneous notification stream and one shared error stream. No `Notify` callback/sink method, public helper struct, or consumer embedding requirement is permitted. eventlog delivers immutable concrete Event snapshots after commit without blocking persistence/scheduler on slow consumers. Internal durable backlog/replay remains required. The error stream has capacity one, retains the first fatal error internally, and uses best-effort non-blocking delivery. A fatal Reader error terminates that Channel's Reader, then eventlog closes the notification stream and error stream after the buffered fatal error; safe recoverable errors may be retried/handled without terminating only when order and completeness are preserved. Exactly-once consumer processing is not guaranteed without consumer acknowledgment or transactional integration.

## Record model and identity

The record envelope supports three kinds: `channel`, `event`, and `commit` (the later Commit-marker
decision supersedes the earlier two-kind proposal). Each record carries `schema_version: 1` and a global
sequence number. Envelope includes version, sequence, kind and type-specific data; Channel payload
contains ID/type/fields, Event payload contains ID/channel_id/type/fields, and Commit payload contains
transaction ID/count/checksum. Versioned JSON is stored as NDJSON; there is no log header.

- **Channel identity:** concrete Channel type, exposed field-name schema, and designated identity field
  name/value, canonically encoded and UUID v5. The selected identity field must contain a UUID;
  nullable or arbitrary primitive identity types are forbidden. Non-identity changes retain logical identity but update
  the Channel snapshot. A Channel-only mutation does not create an Event occurrence; the next Event
  occurrence uses the latest Channel snapshot. Earlier occurrences keep their original Channel
  association. Identical physical Channel records are deduplicated.
- **Event identity:** global and independent of Channel. It is based on concrete Go Event type, exposed
  field-name schema, and designated identity field name/value, canonical encoding and UUID v5. The
  consumer-selected identity field must contain a UUID, not an arbitrary primitive or nullable value.
  Non-identity data changes keep logical ID and create new physical occurrences.
- **Occurrence/HEAD:** each append or detected selected Event-field change creates an immutable physical
  occurrence associated with one Channel. Older occurrences remain in history. Highest-sequence
  occurrence determines HEAD/current Channel association and ordering. Logical promotion is not a
  separate record.
- **Transaction ID:** the global sequence of the first transaction record. Transaction order is Channel
  record when new, Event occurrence, then Commit; existing-channel transactions contain Event then
  Commit.
- **Sequence:** global across records and segments; Commit consumes a sequence but is excluded from
  transaction record count and checksum.

Duplicate reconciliation: recovery deduplicates same-sequence identical physical copies. Same sequence
with different content is corruption. Identical Channel ID/data may deduplicate; conflicting Channel data
for one ID is corruption. Same logical Event ID at different sequences is valid history; the highest
sequence defines HEAD, latest Channel association and ordering. Compaction preserves original sequence
numbers and must not promote older occurrences due to duplicate copies. Temporary compaction artifacts
are not authoritative recovery input and are cleaned only after finalized source segments validate.

## Typed field encoding

Persisted values use strict type-tagged JSON objects. Strings and bools are native JSON; integers are
decimal strings to avoid parser precision loss; float32/float64 are lowercase raw IEEE bit-pattern hex
(8/16 digits) preserving width and signed zero; null is type-only. JSON Schema uses strict variants,
required keys, fixed type tags, and no extra properties to validate representation; Go validates exact
integer bounds and non-finite float rejection. Schemas are versioned canonical standalone files under the
schema directory and embedded from those same files using go:embed. Named primitive types use underlying
kinds.

Permanent v1 identity type tags: Null `0x00` (no payload), String `0x01` (UTF-8), Bool `0x02` (one byte
0/1), int `0x10` (8-byte signed BE), int8 `0x11` (1-byte signed), int16 `0x12` (2-byte signed BE), int32
`0x13` (4-byte signed BE), int64 `0x14` (8-byte signed BE), uint `0x20` (8-byte unsigned BE), uint8
`0x21` (1-byte unsigned), uint16 `0x22` (2-byte unsigned BE), uint32 `0x23` (4-byte unsigned BE), uint64
`0x24` (8-byte unsigned BE), float32 `0x30` (4 raw IEEE bytes), float64 `0x31` (8 raw IEEE bytes). Go
int/uint are fixed 64-bit regardless of architecture; named types use underlying tags. Nullable nil uses
Null, non-nil uses primitive tag. Unknown tags fail recovery; tags are permanent and reserved ranges
remain unused.

UUID v5 namespace is `26612f1d-f648-483a-9cfe-59c18f113891`. Event domain prefix is
`eventlog\\x00event\\x00v1\\x00`; Channel domain prefix is `eventlog\\x00channel\\x00v1\\x00`.

Canonical identity encoding is separate from JSON and architecture-independent: fixed encoding-format
version, domain prefix, UTF-8 package path and concrete type name, exposed field-name schema, and
designated identity field name/value. Non-identity values are excluded from logical identity. Schema
field names sort bytewise UTF-8; variable component lengths/counts use uint32 big-endian framing. Exact field-schema/identity-value byte composition is approved; see the approved layout section below and remaining validation questions.

## Approved canonical identity byte composition — 2026-11-24

The exact canonical identity payload layout after the existing domain prefix is approved:
1. Format version byte `0x01`.
2. Type package path: UTF-8 bytes framed by u32 BE byte length.
3. Concrete type name: UTF-8 bytes framed by u32 BE byte length.
4. Schema field count: u32 BE, followed by each field name sorted bytewise by UTF-8 bytes and framed by u32 BE byte length.
5. Identity field name: UTF-8 bytes framed by u32 BE byte length.
6. Identity value: a UUID-specific encoding marker (tag assignment remains open) followed by the UUID's canonical 16 bytes. The identity field must be part of the exposed schema and UUID-typed; nullable and arbitrary primitive identity values are forbidden.

The already-approved namespace, Event/Channel domain prefixes, primitive field tags, big-endian framing, UUID v5 derivation, and non-identity value exclusion remain unchanged. Identity payload is UUID-only; the UUID tag remains unresolved. The accepted Go representation and byte extraction order are settled by D-024 below. Format byte is intentionally redundant with the domain prefix's v1 marker.

**D-024 — UUID identity representation (approved 2026-11-24):** A UUID identity field MUST have Go underlying type `[16]byte`. The unnamed `[16]byte` type and named types whose underlying type is `[16]byte` are accepted. Type aliases are accepted insofar as their Go underlying type is `[16]byte`. The canonical UUID payload is the 16 array elements in index order (`value[0]` through `value[15]`); no string parsing, textual normalization, integer endianness conversion, or external UUID dependency is involved. Nullable UUID identity fields and all other underlying types are rejected. This applies equally to Event and Channel identity fields.

Still open: the dedicated UUID identity marker/tag assignment; metadata UTF-8 policy; canonical naming of aliases/generic instantiated types; and maximum framing sizes. The accepted representation and byte extraction order are no longer open.

## Append transaction flow

1.  Resolve selector bindings on direct fields; infer field names and require exactly one field-pointer
   match. Validate all Event and Channel fields, nullability, types, ranges and finite floats before any
   persistence. Validation failure leaves log unchanged.
2.  Determine whether the Channel record is new or physically reusable.
3.  Assign consecutive global record sequence numbers to transaction records and Commit.
4.  Serialize Channel (when new) and Event occurrence as newline-terminated NDJSON.
5.  Compute SHA-256 over exact serialized transaction-record bytes including terminating newlines;
   exclude Commit bytes and newline.
6.  Write Commit marker last. It contains transaction ID (first transaction record sequence),
   transaction-record count (Channel/Event records only), and checksum. Commit is first-class and has its
   own global sequence.
7.  Persist transaction on segment source; atomically update derived index metadata/HEAD/associations.
   Notify dispatcher only after successful commit.

Validation failure leaves the log unchanged. New Channel and Event are fully atomic: no partial Channel
or Event is persisted on invalid fields. Write interruption/crash must not expose partial append;
recovery truncates incomplete transaction from its start. Partial Commit is treated as incomplete
transaction. Append returns an internal error; no consumer-facing Append API is approved. Recoverable
atomicity is distinct from power-loss durability before sync.

## Open and recovery flow

A missing storage structure is initialized automatically during PrepareLog. The first active segment is a
zero-byte NDJSON file; no header record is written. Existing storage is recovered before it becomes
available. Runtime Channel resolution or creation is not attempted until this recovery has completed successfully.

Recovery order:
1.  Acquire exclusive single-process ownership.
2.  Validate the manifest and identify its active generation/segment set.
3.  Validate active segment files, record schemas and sequence continuity.
4.  Validate transaction boundaries, Commit IDs, record counts and checksums.
5.  Truncate an incomplete trailing transaction (including partial Commit) from its transaction start.
6.  Validate or rebuild the generation-bound index from validated authoritative segments; atomically
   replace it if needed.
7.  Clean orphan/temp artifacts only after finalized source segments validate and cleanup is safe.
8.  Open for append and initialize the mandatory channel-scoped readers/change manager.

A complete newline-terminated malformed record, unknown kind/tag, unsupported schema, duplicate/invalid
sequence, missing Channel reference, or invalid fields is corruption; never silently skip it. Expose only
fully committed transactions. Segments are authoritative; index never overrides them. Existing storage
with missing/corrupt manifest can recover only if exactly one complete generation is unambiguous;
otherwise fail closed. Never choose a generation by filename. Recovery ignores temporary compaction files
and cleans them only after finalized source segments validate.

Duplicate reconciliation: identical physical copies at the same sequence are deduplicated. Same sequence
with different content is corruption. Channel records with same ID and identical data may deduplicate;
conflicting data for one ID is corruption. Same logical Event ID at distinct sequences represents valid
history and is retained; highest sequence determines HEAD, latest association and ordering. Compaction
preserves original sequences and must not promote older occurrences because duplicate copies.

## Reader behavior

The internal unified Reader progression is mandatory and channel-scoped: exactly one progression serves each runtime Channel, never all Channels. It begins with committed history, catches concurrent appends during replay, and transitions to live tailing through that same progression without gaps, duplicates introduced by the handoff, or ordering breaks. Occurrences are ordered by internal global sequence. Do not create separate historical and live Readers or combine independent typed Readers to construct the heterogeneous notification stream.

Consumers do not provide or receive Channel UUIDs, sequence numbers, cursors or offsets. These remain
library-owned. Context cancellation and explicit errors are required. Re-appended logical Events are
separate occurrences. A Channel may contain multiple concrete Event types. The unified notification Reader reconstructs each
committed Event through the existing internal type registration/codec mechanism. Typed Readers may filter
for a requested concrete type while advancing, but they are not separate notification sources. Corruption,
unknown persisted Event types, and decode failures are errors, never silently skipped.

The reader stays internal. The common `Event` notification payload and `Notifications() <-chan Event` accessor, the concrete `TEvent` generic constraint, and `Errors() <-chan error` accessor are approved. Pointer/value convention, occurrence wrapper, and remaining resource-close details remain open.

## Runtime lifecycle and shutdown

eventlog owns the runtime lifecycle; consumers do not receive public `Start`, `Activate`, `Close`, or other lifecycle controls. Successful append Commit is the authoritative ordering point. The Channel's single unified Reader progression observes only committed records and drives notification delivery in global sequence order. No notification is emitted before its append Commit succeeds.

Slow consumers may apply backpressure to the notification delivery worker, but must not stall append/persistence or cause committed Events to be reordered or skipped. A delivery failure never rolls back or deletes a committed Event. The Reader/delivery progression and pending delivery state remain recoverable; without consumer acknowledgment, exactly-once consumer processing is not guaranteed and duplicates may occur in the documented crash window.

Shutdown is ordered:
1. Stop accepting new preparations and stop initiating new change-detection appends.
2. Coordinate all outstanding append transactions until each reaches a committed or safely aborted state.
3. Cancel the Channel Reader and delivery workers after the append boundary is settled.
4. Drain pending notifications according to the defined shutdown policy and wait for all senders to stop. If drain cannot complete, report the lifecycle failure and preserve recoverable pending state.
5. Close notification streams only after no sender can still write to them.
6. Release storage and process-ownership resources.

## Durability, segment management and compaction

Automatic sync occurs every 1 second or 1 MiB of pending writes, whichever first. Flush and Close are
durability boundaries; Close flushes and syncs. Append before sync is not promised to survive power loss.

Storage layout:

```text
eventlog/
├── manifest.json
├── index.json
└── segments/
    ├── segment-000001.ndjson
    ├── segment-000002.ndjson
    └── ...
```

Segment IDs are monotonic and never reused (including compaction), distinct from global record sequences.
Finalized segments are immutable; only the active segment is writable. Rotate only at transaction
boundaries against a fixed target size; oversized transaction may occupy its own segment.

The versioned manifest identifies the active finalized segment set/generation and next segment ID; it
contains no event payloads. Example:

```json
{
  "format_version": 1,
  "generation": 3,
  "segments": ["segment-000004.ndjson", "segment-000005.ndjson"],
  "next_segment_id": 6
}
```

Manifest replacement is the atomic generation-switch/compaction commit point: write temporary manifest,
validate, sync, atomically replace. Old files remain until switch is finalized. Orphans are recovery
candidates, not authoritative. Ambiguous complete generations fail rather than guessing.

The index is derived, generation-bound and rebuildable. It contains Event IDs, Channel associations,
HEAD/latest occurrence, global sequence, segment ID and byte offsets. Missing/stale/inconsistent index
rebuilds from validated authoritative segments and is atomically replaced.

Compaction protocol: build replacement segments/index without modifying reader-pinned files →
validate/checksum → sync → finalize/rename → atomically switch manifest/index → retain old generation
while readers pin it → delete after last pin releases. Temporary files are ignored during recovery and
cleaned only after finalized source segments validate. New readers bind to the switched generation;
existing readers continue on their pinned generation.

## Portability and dependencies

Use only Go standard library dependencies and local filesystem storage. Linux and Windows are
equal-priority supported platforms; macOS is excluded. If platform behavior differs or is unreliable,
core functionality must use a strategy that works on both platforms. Correctness, recovery, predictable
memory use and platform parity precede numeric performance targets.

## Unresolved architectural details

- Field selector signatures, pointer/value reconstruction convention and occurrence shape; the generic constraints, factory type and notification/error accessors are approved.
- UUID Go type/representation, canonical byte order and identity-tag assignment; metadata UTF-8 policy;
  canonical naming of aliases/generic instantiated types; and maximum framing sizes. The field-schema
  framing layout is approved, but these validation details remain open.
- Internal prepared-value association, Log lifecycle, cancellation, reader close/resource cleanup,
  terminal-error semantics.
- Identity-field mutation detection and transition mechanics, snapshot consistency,
  occurrence/HEAD/index and Channel association effects.
- Internal writable-channel ownership; backlog durability/retention and replay boundary; retry termination for recoverable failures; scheduler interaction and shutdown drain failure.
- Consumer synchronization/snapshot consistency details.
- Benchmarks, memory/performance targets, and measurable acceptance criteria.


## Approved notification public API decision — 2026-11-24

- The notification stream element type is the common `Event` interface, implemented by each concrete Event type. A Channel stream may therefore carry heterogeneous concrete Event values.
- The Channel interface accessor is `Notifications() <-chan Event`. No public `Notification[T]` wrapper, Event ID, Occurrence ID, delivery token, or consumer-facing Ack API.
- Notifications are channel-scoped. Consumer-defined Channel types implement the Channel interface; the accessor signature is settled.
- Delivery identity, occurrence identity, durable backlog, retries, replay bookkeeping, and acknowledgment of internal dispatch are private implementation details.
- Since the consumer provides no acknowledgment, eventlog cannot claim exactly-once consumer processing. Delivery/recovery semantics must be stated honestly; duplicates may occur in a crash window if internal dispatch is durably recorded after sending.
- The consumer-facing notification accessor is implemented by consumer-defined Channel types as `Notifications() <-chan Event`; this is settled and must not be reopened.


## Approved PrepareLog generic and Channel error contract — 2026-11-24

The approved generic contract distinguishes the concrete Event type being prepared from the common Event interface used by the heterogeneous Channel notification stream:

```go
func PrepareLog[
    TEvent Event,
    TChannel Channel,
](
    ctx context.Context,
    newChannel func(chan Event) TChannel,
) (TEvent, TChannel, error)
```

Here `TEvent` is a concrete Event type satisfying the `Event` interface; it is not the common interface type itself. `TChannel` is a consumer-defined concrete Channel type satisfying the `Channel` interface. The factory receives `chan Event`, allowing one Channel to receive different concrete Event implementations. The generic `TEvent` identifies the concrete Event type for internal preparation/decoder registration. This generic relationship does not authorize a consumer-facing API expansion.

The Channel interface exposes:

```go
type Channel interface {
    Notifications() <-chan Event
    Errors() <-chan error
}
```

The error stream is one-way, shared by the logical Channel runtime, and has buffer capacity one. The first fatal error is retained internally and sent non-blockingly on a best-effort basis; error delivery must not block persistence or indefinitely block the Reader. Fatal Reader errors terminate that Channel's Reader, after which eventlog closes `Notifications()` and then `Errors()`. A buffered fatal error remains readable before the closed error stream is exhausted. Recoverable errors do not terminate the Reader when safe continuation is possible.

### Deferred concrete decoder resolution

Concrete Event type registration is derived internally from the concrete `TEvent` generic argument supplied to `PrepareLog`; consumers do not call a separate registration API. Decoder registrations are maintained in an internal registry and notify waiting Channel runtimes when available.

Historical notification replay must not begin until every concrete Event decoder required by the committed history for that logical Channel is available. The runtime waits asynchronously and indefinitely for a required decoder that has not yet registered; there is no arbitrary timeout, and decoder absence alone does not produce a fatal Channel error. The first `PrepareLog` call must not block in a way that prevents subsequent Event types from being prepared. Once the missing decoder registers, replay may proceed. Unknown or undecodable persisted Event types must fail closed and must never be silently skipped. The existing codec implementation's availability remains to be verified before implementation.

No public `RegisterEventType`, `Activate`, `Start`, `Close`, Ack, or checkpoint method is permitted. The public Channel surface remains the two accessors above.


## Approved pointer-only preparation and prepared-instance reuse — D-008 / D-009

Both generic arguments to `PrepareLog[TEvent, TChannel]` must be pointer types. `PrepareLog` validates both generic types at runtime using reflection and returns an error before exposing prepared values if either argument is not a pointer. The two-parameter generic signature and all consumer-facing method signatures remain unchanged.

For a compatible repeated preparation using the same concrete `TEvent` and `TChannel` types and unchanged exposed field schemas, eventlog returns the same live Event pointer and Channel pointer rather than allocating a second prepared pair. An incompatible registration for an already prepared type/schema combination fails with a typed conflict error; it must not silently create a competing prepared instance or Channel runtime.

This reuse is consistent with the Channel-scoped lifecycle: event-specific preparations that resolve to the same logical Channel reuse that Channel's existing live pointer, runtime and single Reader progression, including when another concrete Event type is prepared for that Channel. Each concrete Event type retains its own prepared Event instance. The precise internal registry key, typed conflict taxonomy, and behavior for distinct logical Channels of the same concrete Channel type remain Define details. No consumer-facing API is added or changed.


## Approved prepared-instance registry separation — D-010

The internal registry design separates type registration from logical live-instance identity:

- **Event type registry:** keyed by concrete Event type and exposed field-name schema; owns field metadata and decoder registration for that type/schema.
- **Event instance registry:** keyed by canonical Event identity; reuses the live Event pointer for the same logical Event.
- **Channel runtime registry:** keyed by canonical Channel identity; reuses the live Channel pointer, runtime and single Reader progression for the same logical Channel.

Canonical Event identity remains based on concrete Event type, exposed field-name schema, and designated UUID identity field name/value. Canonical Channel identity remains based on concrete Channel type, exposed field-name schema, and designated UUID identity field name/value. Therefore, one concrete Channel Go type may represent multiple distinct logical Channels, while different concrete Event types may converge on one logical Channel runtime.

Type/schema registration conflicts are distinct from logical-instance identity. A registry keyed only by concrete Channel Go type is insufficient and must not merge distinct logical Channels. This is an internal design only; the approved `PrepareLog[TEvent, TChannel]` signature and all consumer-facing interfaces remain unchanged.

The exact preparation ordering and mechanism for obtaining/validating candidate Channel identity fields before resolving the Channel runtime remain open.


## Approved Channel candidate resolution sequence — D-011

During `PrepareLog[TEvent, TChannel]` preparation, invoke the supplied Channel factory to create a candidate, derive its canonical Channel identity from the candidate's `Fields()` schema and designated identity field, then consult the Channel runtime registry by that canonical identity. If a runtime already exists, return and reuse its existing Channel pointer and runtime/Reader; the newly created candidate is discarded. If no runtime exists, adopt the candidate and create the Channel runtime. This is an internal preparation rule and does not change the consumer-facing API. The factory may therefore be invoked even when its candidate is discarded.


## Approved Channel factory contract — D-012

The supplied Channel factory is a constructor only: it creates and initializes the candidate Channel value but does not start goroutines, open external resources, or perform other external side effects. `PrepareLog[TEvent, TChannel]` may discard a candidate when its canonical Channel identity resolves to an existing runtime, so discarded candidates must be safe to abandon. The library starts the Channel runtime only after adopting a candidate for a previously unresolved logical Channel. This is an internal lifecycle contract; the consumer-facing API remains unchanged.


## Approved atomic Channel runtime resolution — D-013

Channel runtime lookup and adoption are one atomic registry operation keyed by canonical Channel identity. For concurrent `PrepareLog[TEvent, TChannel]` calls that resolve to the same logical Channel identity, at most one candidate is adopted and at most one runtime/Reader is created. A competing preparation reuses the winning Channel pointer and runtime. This synchronization is internal and does not change the consumer-facing API.


## Concurrent Event-type preparation for one logical Channel — D-014

When concurrent `PrepareLog[TEvent, TChannel]` calls register different concrete Event types that resolve to the same logical Channel, each concrete Event type is registered independently. Channel runtime resolution remains keyed by canonical Channel identity and uses the atomic lookup/adoption rule: all such preparations converge on the one adopted Channel pointer, runtime and Reader progression. A preparation does not synchronously wait for another Event type's registration to complete. Historical replay remains asynchronous and waits until every decoder required by the committed history for that logical Channel is registered; registration of a required decoder allows replay to proceed. This does not add a consumer-facing registration or activation API and does not change the approved consumer-facing API.


## Event decoder registration lifetime — D-015

A successful concrete Event decoder registration is retained for the lifetime of the Log runtime. Its availability is independent of the lifetime or completion of the individual `PrepareLog[TEvent, TChannel]` call that caused registration, and it remains available to any Channel Reader whose committed history requires that decoder. Registration is associated with the concrete Event type and its exposed field-name schema, not with an individual Channel. There is no consumer-facing unregister operation. This is internal lifecycle behavior and does not change the approved consumer-facing API.


## Conflicting concrete Event decoder registrations — D-016

Concrete Event decoder registration is keyed by the concrete Event Go type and its exposed field-name schema. If that concrete Event type is registered again with the same exposed field-name schema, the existing registration is reused. If it is presented with a different exposed field-name schema, registration fails with a typed schema-conflict error; it must not replace or mutate the existing registration or affect Readers already using it. This is an internal registration rule and does not change the approved consumer-facing API.



## Approved field selector contract — D-017

The consumer-facing field selector forms are approved:

```go
func FieldOf[T any, V Primitive](selector func(*T) *V) Field
func FieldOfNullable[T any, V Primitive](selector func(*T) **V) Field
```

Selectors receive a pointer to the owning struct. `FieldOf` selects a non-nullable primitive field; `FieldOfNullable` selects a nullable primitive pointer and preserves nil separately from a pointer to the zero value. The field name is inferred by resolving the selector against a zero-value probe and matching exactly one direct field address. Nested fields and selectors that do not resolve uniquely are rejected. Named primitive types remain supported. This does not change the approved `Fields() Fields` consumer method or `PrepareLog` signature.


## Approved pointer reconstruction and snapshot isolation — D-018

Both generic preparation arguments remain pointers to concrete struct types. Historical Event occurrences are reconstructed into newly allocated pointers to their concrete Event types; historical reconstruction must not overwrite the consumer's prepared live Event pointer. Notification payloads are independent immutable snapshots and must not share mutable Event storage with the prepared live Event, persistence state, or another notification snapshot. Historical Channel snapshots are likewise separate from the live Channel pointer. Reconstruction preserves concrete named primitive types, widths, nullability, and nil-versus-zero distinctions. This is internal behavior and does not change the consumer-facing API.


## Approved Event identity immutability — D-019

After a prepared Event instance has been registered under its canonical logical identity, its designated identity field is immutable for that prepared instance's lifetime. Internal change detection must detect an identity-value change before persisting it, reject the change, and report a descriptive asynchronous error through the existing Channel `Errors() <-chan error` stream. It must not create a second logical Event, rewrite identity, or alter existing occurrences/HEAD. A consumer representing a different logical Event prepares a separate Event instance. No public API change is permitted.


## Approved internal change queue and snapshot handoff — D-020

Detected selected-field changes are captured as immutable snapshots and submitted to an internal eventlog-owned change queue. Persistence workers dequeue and persist those snapshots independently of the change-monitoring mechanism. A queued snapshot is immutable after enqueue. A notification is eligible only after the corresponding occurrence has committed. This queue is entirely internal: consumers continue mutating their prepared Event/Channel values through the existing API; no public queue, callback, lock, or method is introduced. Direct struct-field assignments cannot be intercepted, so monitoring detects changes on its managed cadence.


## Approved per-Event queue ordering and duplicate suppression — D-021

For each logical Event, snapshots are processed in FIFO enqueue order. An already-enqueued snapshot must not be silently discarded or replaced by a newer snapshot. Monitoring suppresses a snapshot only when the currently observed selected state is unchanged from the last observed state enqueued for that logical Event. A transition away from a value and later back to it is a distinct occurrence and must be retained when observed. Global sequence numbers are assigned centrally at commit time; per-Event processing must preserve enqueue order. Because consumers mutate ordinary Go fields directly and monitoring is periodic, intermediate states that change and revert between observations cannot be guaranteed to be recorded.


## Approved bounded queue and backpressure policy — D-022

The internal change queue is bounded to keep memory use predictable. When capacity is exhausted, eventlog applies backpressure to its internal monitoring/queueing work; it does not block or intercept consumer assignments to ordinary Go struct fields. Consumers may continue mutating their structs while monitoring is paused. When monitoring resumes, it captures the latest state it observes; intermediate states that occurred while monitoring was paused may be missed. Already-enqueued snapshots remain queued and are not dropped. Queue capacity and worker scheduling are internal configuration/implementation details, not consumer-facing API. Persistence failures are reported through the existing asynchronous error contract and must not be disguised as successful commits.


## Approved Log-wide queue ownership and persistence concurrency — D-023 (Decision 8)

Each Log owns one bounded internal ingress queue for immutable snapshots captured by its change monitors. A Log-internal dispatcher dequeues snapshots and schedules them by canonical logical Event identity. At most one persistence operation for a given logical Event may run at a time, and that Event's snapshots retain FIFO enqueue order; a later snapshot for that Event must not overtake an earlier one.

Different logical Events may be persisted concurrently by multiple workers. This avoids serializing unrelated Event histories while preserving each Event's ordered history. Global sequence numbers are assigned centrally at the commit/serialization point; committed sequence order defines the total order observed by Readers and notifications.

The queue, dispatcher, scheduling lanes and worker count are internal implementation details. The bounded-capacity backpressure policy remains D-022: monitoring/queueing work is paused or delayed when capacity is exhausted; consumer field assignments are never blocked; snapshots already enqueued are retained and never dropped. No consumer-facing queue, method, callback, configuration requirement or API change is introduced. The existing PrepareLog[TEvent, TChannel], Fields() Fields, Notifications() <-chan Event, and Errors() <-chan error contracts remain unchanged.


## D-024 — UUID identity representation

The UUID identity field must have underlying Go type `[16]byte`. Accept `[16]byte` and named types with that underlying type; reject nullable UUID fields and every other underlying type. Encode the UUID payload as the 16 bytes in array index order, without string parsing or integer byte-order conversion. This applies to both Event and Channel identity fields. The dedicated UUID identity marker/tag remains unresolved; D-024 does not assign a tag value or change the already-approved canonical identity framing.
