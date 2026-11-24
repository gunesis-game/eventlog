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

The public preparation entry point remains `PrepareLog[TEvent, TChannel]`; its approved consumer-facing contract must not be changed without explicit approval. In this approved API, `TEvent` denotes the common `Event` interface implemented by concrete Event types, not one individual concrete Event type. The Channel interface defines `Notifications() <-chan Event`. A single consumer-defined Channel notification stream is heterogeneous and may carry multiple concrete Event types (for example, `OrderCreated` and `OrderCancelled`) in commit order. Accordingly, a factory `func(chan TEvent) TChannel` is compatible because its channel element is the common Event interface. The earlier concrete-event factory example is superseded. Event and Channel constraints require `Fields() Fields`.
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
  TChannel](ctx)`, returning `(TEvent, TChannel, error)`. Consumers receive event and channel values, not
  a Log handle. The package owns the internal `Log` instance and all append/read/next/lifecycle
  operations; none of those operations are exposed to consumers at this stage. Consumer event and channel
  types implement `Fields() Fields`; consumers set or change their own field values. A single process
  owns a log at a time, with concurrent goroutine use coordinated internally.
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
- **Compactor:** Produces replacement segments/index without modifying reader-pinned files,
  validates/checksums and syncs them, finalizes, atomically switches manifest/index, and deletes old
  generations only after the last reader releases its pin.
- **Change manager/scheduler:** One managed scheduler per prepared Log/change manager polls selected
  Event/Channel fields every 100 ms; no per-field goroutines. Cadence is internal/test-only. Consumer
  synchronizes its own mutations. Detected changes route to internal persistence.
- **Notification dispatcher/backlog:** The Channel interface exposes `Notifications() <-chan Event`, where `Event` is the common interface implemented by all concrete Event types. One Channel owns one heterogeneous stream and may carry multiple concrete Event implementations in commit order. No `Notify` callback/sink method, public helper struct, or consumer embedding requirement is permitted. eventlog delivers immutable concrete Event snapshots after commit without blocking persistence/scheduler on slow consumers. Internal durable backlog/replay and error handling remain required. The internal writable-channel construction path preserves this exact receive-only consumer API. `TEvent` in the approved generic preparation API is the common Event interface, so the typed factory channel can carry heterogeneous concrete Event implementations. Retry, recovery, drain, and lifecycle failure semantics must be specified honestly; exactly-once consumer processing is not guaranteed without consumer acknowledgment or transactional integration.

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

The already-approved namespace, Event/Channel domain prefixes, primitive field tags, big-endian framing, UUID v5 derivation, and non-identity value exclusion remain unchanged. Identity payload is UUID-only; the UUID tag and accepted Go representation are still unresolved. Format byte is intentionally redundant with the domain prefix's v1 marker.

Validation/test questions remain open: accepted UUID type/representation; canonical UUID byte order and UUID tag; metadata UTF-8 policy; canonical naming of aliases/generic instantiated types; and maximum framing sizes. Nullable and primitive identities are excluded. These are not silently decided by this approval.

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

The reader stays internal under the currently approved public API. The common `Event` interface and `Notifications() <-chan Event` accessor are settled; pointer/value convention, occurrence wrapper, and close/resource semantics remain open.

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

- Exact public Event/Channel interface, field selector signatures, generic constraints, pointer/value
  reconstruction convention and occurrence shape.
- UUID Go type/representation, canonical byte order and identity-tag assignment; metadata UTF-8 policy;
  canonical naming of aliases/generic instantiated types; and maximum framing sizes. The field-schema
  framing layout is approved, but these validation details remain open.
- Internal prepared-value association, Log lifecycle, cancellation, reader close/resource cleanup,
  terminal-error semantics.
- Identity-field mutation detection and transition mechanics, snapshot consistency,
  occurrence/HEAD/index and Channel association effects.
- Internal writable-channel construction/ownership, backlog durability/retention and replay boundary,
  retry termination, delivery/error ordering, scheduler interaction and shutdown drain failure.
- Consumer synchronization/snapshot consistency details.
- Benchmarks, memory/performance targets, and measurable acceptance criteria.


## Approved notification public API decision — 2026-11-24

- The notification stream element type is the common `Event` interface, implemented by each concrete Event type. A Channel stream may therefore carry heterogeneous concrete Event values.
- The Channel interface accessor is `Notifications() <-chan Event`. No public `Notification[T]` wrapper, Event ID, Occurrence ID, delivery token, or consumer-facing Ack API.
- Notifications are channel-scoped. Consumer-defined Channel types implement the Channel interface; the accessor signature is settled.
- Delivery identity, occurrence identity, durable backlog, retries, replay bookkeeping, and acknowledgment of internal dispatch are private implementation details.
- Since the consumer provides no acknowledgment, eventlog cannot claim exactly-once consumer processing. Delivery/recovery semantics must be stated honestly; duplicates may occur in a crash window if internal dispatch is durably recorded after sending.
- The consumer-facing notification accessor is implemented by consumer-defined Channel types as `Notifications() <-chan Event`; this is settled and must not be reopened.
