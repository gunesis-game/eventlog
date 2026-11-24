# Architecture

**Status:** Current technical design baseline; remaining open details are listed below.

This document owns technical design: components, responsibilities, boundaries, interfaces, algorithms,
data/storage representations, lifecycle, and trade-offs. Requirements are owned by
[requirements.md](requirements.md); this design must satisfy them rather than redefine product behavior.
It describes intended design, not implementation or verification evidence.


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

## Preparation and registration

Both generic arguments to PrepareLog must be concrete pointer types satisfying Event and Channel.
PrepareLog checks this constraint at runtime and returns an error before exposing prepared values if
either argument is not a pointer. The factory and heterogeneous notification stream are defined in the
System boundary section; the rules below specify internal registration and runtime resolution.

Compatible repeated preparations reuse the same live Event and Channel pointer instances. Event instance
reuse follows canonical Event identity. Channel runtime reuse follows canonical Channel identity, not
merely the concrete Channel Go type. Type/schema registration, Event instance identity, and Channel
runtime identity are separate registry concerns. A concrete Channel type may represent multiple logical
Channels; different concrete Event types may share one logical Channel runtime.

Preparation invokes the supplied Channel factory to create a candidate and derives its canonical Channel
identity from the candidate's exposed fields. Runtime lookup and candidate adoption are one atomic
operation: an existing logical Channel reuses its existing pointer, runtime, and single Reader progression;
otherwise the candidate is adopted and the runtime is created. The factory is a constructor only. It must
not start goroutines, open external resources, or perform other external side effects, because its
candidate may be discarded when an existing runtime is reused.

Concrete Event decoder registration is derived internally from the concrete TEvent generic argument.
A successful registration remains available for the Log runtime lifetime and is keyed by concrete Event
type plus exposed field-name schema, not by Channel. Re-registering the same type/schema reuses the
registration. Re-registering the same concrete type with a different schema fails with a typed
schema-conflict error and does not replace or mutate the existing registration or Readers using it.

Historical replay for a logical Channel waits asynchronously until every decoder required by its committed
history is registered. A missing decoder causes an indefinite wait rather than an arbitrary timeout or a
fatal error solely due to absence; replay proceeds when the decoder registers. The first preparation must
not block subsequent Event-type preparations. Unknown or undecodable persisted Event types fail closed.
No additional consumer-facing registration, activation, acknowledgement, start, stop, or close operation
is part of the API.

## Internal components


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
- **Reader/tailer:** One unified Reader progression serves each runtime Channel and continuously follows the committed global sequence, with no explicit replay-to-live boundary or separate Reader modes. It reads committed history in sequence order and, when caught up, waits for a wake-up hint indicating possible new commits. Wake-ups are not authoritative: after every wake (and after entering the wait), the Reader rechecks the committed append source so a missed or coalesced signal cannot lose an occurrence. A commit during catch-up is naturally consumed by the same progression. Global sequence/cursor/offset management is internal; consumers do not provide or receive storage cursors.
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
- **Notification dispatcher/backlog:** The Channel interface exposes `Notifications() <-chan Event` and `Errors() <-chan error`. One Channel owns one heterogeneous notification stream and one shared error stream. No `Notify` callback/sink method, public helper struct, or consumer embedding requirement is permitted. eventlog delivers immutable concrete Event snapshots after commit without blocking persistence/scheduler on slow consumers. The in-memory pending-delivery buffer is finite; a full buffer applies backpressure to the Channel Reader, which waits and resumes from its held position when capacity becomes available. This bounds notification-backlog memory without blocking persistence or discarding committed history. The authoritative committed append log remains the source of pending history; no separate durable queue is introduced. The error stream has capacity one, retains the first fatal error internally, and uses best-effort non-blocking delivery. Recoverable delivery errors may be retried only while the pending occurrence is retained and order/completeness are preserved; a failed occurrence is never silently skipped to advance the Reader. When retry policy declares a failure terminal, the Channel Reader terminates, reports the error through `Errors()`, and closes its streams after fatal-error reporting. Committed pending history remains recoverable from the authoritative log for a future runtime. Retry policy is internal: temporary delivery failures retry the same pending occurrence using bounded backoff and a finite retry limit; retry does not advance the Reader or permit later occurrences to overtake. On retry exhaustion, the Channel Reader terminates and reports through Errors(); committed pending history remains recoverable from the authoritative log. Other Channel Readers continue independently. Numeric buffer capacity, limits, delay values, and classification details are implementation/test policy, not public configuration. Exactly-once consumer processing is not guaranteed without consumer acknowledgment or transactional integration.

## Field selection, reconstruction, and identity integrity

Consumers select direct fields using typed selectors. FieldOf accepts func(*T) *V for a non-nullable
field; FieldOfNullable accepts func(*T) **V for a nullable field. Field names are inferred, selectors
must resolve to exactly one direct field, and named primitive types are supported. Nil remains distinct
from a pointer to the zero value.

Historical occurrences are reconstructed into fresh concrete Event pointers and never overwrite the
prepared live Event pointer. Notification snapshots are immutable and independent of live values,
persistence storage, and other notification snapshots. Historical Channel snapshots are likewise
separate from the live Channel pointer. Concrete primitive types, widths, and nullability are preserved.

After a prepared Event instance is registered under its canonical logical identity, its designated
identity field is immutable for that instance's lifetime. Change detection rejects an identity-value
mutation and reports a descriptive asynchronous error through the existing Channel error stream. It does
not create another logical Event, rewrite identity, or alter existing occurrences or HEAD. A different
logical Event requires a separate prepared Event instance.

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
Null, non-nil uses primitive tag. UUID identity uses its dedicated permanent tag `0x03` followed by exactly 16 canonical UUID bytes. Unknown or unassigned tags fail recovery; tags are permanent and reserved ranges
remain unused.

UUID v5 namespace is `26612f1d-f648-483a-9cfe-59c18f113891`. Event domain prefix is
`eventlog\\x00event\\x00v1\\x00`; Channel domain prefix is `eventlog\\x00channel\\x00v1\\x00`.

Canonical identity encoding is separate from JSON: fixed encoding-format version, domain prefix,
UTF-8 package path and runtime-provided concrete type name, exposed field-name schema, and designated
identity field name/value. The concrete type is identified using the reflected type's `PkgPath()` and
`Name()`. Go aliases do not create distinct identities: reflection resolves an alias to the aliased type,
so its package path and name are those of the target type. For an instantiated generic named type, the
runtime-provided instantiated name is used as an opaque UTF-8 component; eventlog does not parse,
reconstruct, or separately encode generic type arguments. Non-identity values are excluded from logical
identity. Schema field names sort bytewise UTF-8; variable component lengths/counts use uint32 big-endian
framing. Exact field-schema/identity-value byte composition is approved; see the approved layout section
below.

This naming rule relies on Go reflection's runtime type naming, not a separately specified stable
serialization format. Go does not guarantee that the reflected name of an instantiated generic type is
byte-for-byte stable across Go toolchain versions. Consequently, v1 identity compatibility across
toolchain versions is not guaranteed where the runtime-provided package path or name changes; such a
change can prevent existing persisted type identities from matching.

## UUID identity representation

Event and Channel identity fields must have Go underlying type [16]byte. The unnamed [16]byte type and
named types with that underlying type are accepted. The canonical UUID payload is the 16 array elements
in index order (value[0] through value[15]). No string parsing, textual normalization, integer endianness
conversion, or external UUID dependency is involved. Nullable UUID identity fields and all other
underlying types are rejected. The UUID identity marker is permanently assigned tag `0x03` in the v1 identity type-tag namespace. It is distinct from Null (`0x00`), String (`0x01`), and Bool (`0x02`); the UUID payload is exactly the following 16 canonical bytes.

## Canonical identity byte composition

The exact canonical identity payload layout after the existing domain prefix is approved:
1. Format version byte `0x01`.
2. Type package path: UTF-8 bytes framed by u32 BE byte length.
3. Concrete type name: UTF-8 bytes framed by u32 BE byte length.
4. Schema field count: u32 BE, followed by each field name sorted bytewise by UTF-8 bytes and framed by u32 BE byte length.
5. Identity field name: UTF-8 bytes framed by u32 BE byte length.
6. Identity value: the permanent UUID identity type tag `0x03`, followed by the UUID's canonical 16 bytes. The identity field must be part of the exposed schema and UUID-typed; nullable and arbitrary primitive identity values are forbidden.

The already-approved namespace, Event/Channel domain prefixes, primitive field tags, big-endian framing, UUID v5 derivation, and non-identity value exclusion remain unchanged. UUID identity uses the permanent dedicated tag `0x03`; the accepted Go representation and byte extraction order are specified in the UUID identity contract. Format byte is intentionally redundant with the domain prefix's v1 marker.

The UUID identity representation is specified in the UUID identity section below.

Metadata keys and values must be valid UTF-8. Reject invalid UTF-8 before persistence and during recovery; do not replace, normalize, or otherwise transform metadata bytes. Canonically equivalent but byte-distinct Unicode strings remain distinct.

### Canonical identity framing limits (v1)

The canonical identity encoding has explicit limits, measured in UTF-8 bytes for strings:
- Type package path: at most 4 KiB.
- Concrete type name: at most 4 KiB.
- Each exposed schema field name: at most 4 KiB.
- Exposed schema field count: at most 1,024.
- Complete canonical identity input: at most 1 MiB, including the Event/Channel domain prefix, format byte, all framing bytes, and identity value.

The 1 MiB aggregate limit applies in addition to the individual component and field-count limits; a payload that satisfies every individual limit but exceeds the aggregate limit is invalid. Enforce all limits when deriving an identity before persistence and when validating/recovering persisted identity data. Exceeding a limit produces an explicit error; values are never truncated. These limits govern canonical Event/Channel identity encoding only and do not establish limits for unrelated append-log records or metadata values.

The canonical identity framing limits are settled by D-038.

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

## Change observation and persistence scheduling

Detected selected-field changes are captured as immutable snapshots and submitted to one bounded,
Log-owned ingress queue. A Log-internal dispatcher schedules snapshots by canonical logical Event
identity. Persistence for one logical Event is serial and preserves FIFO enqueue order; different logical
Events may persist concurrently. Global sequence numbers are assigned centrally at commit.

Only consecutive unchanged observed states are suppressed. A later transition back to an earlier value
is a new occurrence. Enqueued snapshots are retained and are not silently dropped or replaced. Periodic
observation cannot guarantee capture of intermediate states that change and revert between observations.

When queue capacity is exhausted, eventlog pauses or delays internal monitoring/queueing work rather than
blocking consumer assignments to ordinary Go struct fields. Consumers may continue mutating their
structs while monitoring is paused; when monitoring resumes, the latest state it observes is captured,
so intermediate unobserved states may be missed. Queue capacity, dispatch scheduling, and worker count
are internal implementation details. Notifications become eligible only after the corresponding
persistence transaction commits.

## Channel mutation and snapshot consistency

Channel change observation uses the same identity-keyed bounded FIFO ingress/dispatch mechanism as Event change persistence, keyed by canonical logical Channel identity. Each logical Channel's snapshots are processed serially in FIFO order; different logical Channels may be processed concurrently. Snapshot capture and validation are Channel-specific, but there is no separate queue/persistence pipeline.

Each captured Channel snapshot is immutable and isolated from the live consumer-owned Channel and persisted data. Consecutive unchanged observed snapshots are suppressed; a later transition back to an earlier state is a new change. Queue capacity is bounded; under pressure, monitoring/queueing is delayed rather than blocking consumer mutations, and already-enqueued snapshots are retained.

FIFO guarantees order only after snapshots have been captured. eventlog does not guarantee an atomic snapshot while consumers concurrently mutate multiple ordinary Go fields; consumers remain responsible for synchronizing their own mutations. Channel identity is immutable after preparation: detected identity mutation is rejected and reported through the existing Channel error stream, without creating a new logical Channel or altering prior associations. Channel-only changes create no Event occurrence; the next Event occurrence uses the latest persisted Channel snapshot, while earlier Event occurrences retain their historical Channel association.

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

The internal unified Reader progression is mandatory and channel-scoped: exactly one progression serves each runtime Channel, never all Channels. It follows one continuous committed-sequence path from the earliest committed occurrence still pending delivery through current commits, without a replay/live mode transition, mode-specific cursor, or handoff boundary. When caught up it waits on wake-up hints and rechecks the authoritative committed append source after waiting; hints may coalesce or be missed and never determine completeness. Commits arriving during catch-up are consumed by the same progression. Occurrences are ordered by internal global sequence. Do not create separate historical and live Readers or combine independent typed Readers to construct the heterogeneous notification stream.

Consumers do not provide or receive Channel UUIDs, sequence numbers, cursors or offsets. These remain
library-owned. Context cancellation and explicit errors are required. Re-appended logical Events are
separate occurrences. A Channel may contain multiple concrete Event types. The unified notification Reader reconstructs each
committed Event through the existing internal type registration/codec mechanism. Typed Readers may filter
for a requested concrete type while advancing, but they are not separate notification sources. Corruption,
unknown persisted Event types, and decode failures are errors, never silently skipped.

The reader stays internal. The common `Event` notification payload and `Notifications() <-chan Event` accessor, the concrete `TEvent` generic constraint, and `Errors() <-chan error` accessor are approved. Historical reconstruction and notification delivery use fresh concrete Event pointers; no occurrence wrapper is part of the approved consumer API.

## Runtime lifecycle and shutdown

The Log runtime is process-scoped and owns storage, append workers, Channel runtimes, Readers, and delivery workers. The Context passed to `PrepareLog` governs only that caller's preparation and waiting: canceling it must not shut down the shared Log or cancel other consumers. Internal append and delivery workers are not children of any one preparation caller's Context.

eventlog owns the runtime lifecycle; consumers do not receive public `Start`, `Activate`, `Close`, or other lifecycle controls. Successful append Commit is the authoritative ordering point. The Channel's single unified Reader progression observes only committed records and drives notification delivery in global sequence order. No notification is emitted before its append Commit succeeds.

Slow consumers may apply backpressure to the notification delivery worker, but must not stall append/persistence or cause committed Events to be reordered or skipped. A delivery failure never rolls back or deletes a committed Event.

Reader advancement is permitted only after the committed occurrence is safely transferred into the internal pending-delivery mechanism. Delivery backpressure holds the delivery worker, not persistence. Pending notifications are reconstructed from the authoritative committed append log; no second durable queue is introduced. Recovery resumes pending committed notifications before newer occurrences, preserving order. Without consumer acknowledgment or transactional integration, duplicates may occur across a crash boundary; exactly-once consumer processing is not guaranteed. Delivery failures retry the same occurrence using internal bounded backoff and a finite retry limit; Reader advancement and later delivery remain blocked until success. Retry exhaustion terminates that Channel Reader and is reported through `Errors()`; committed pending history remains recoverable. Other Channels continue independently.

The Log runtime is process-lifetime scoped. Once initialized, it owns storage, append workers, Channel runtimes, Readers, and delivery workers until the operating system terminates the process. There is no library-managed runtime shutdown path: eventlog does not register process-exit hooks, install signal handlers, intercept process termination, or expose a public shutdown/flush/close operation. Runtime workers are not tied to any caller's Context and remain active for the process lifetime.

Process termination is not a graceful-shutdown boundary. eventlog makes no guarantee that pending notifications are drained, notification/error streams are closed, or final writes are synchronously flushed at process exit. An interrupted append transaction is handled by the existing recovery rule and is discarded from its beginning if incomplete. Fully committed occurrences present in the authoritative append log, including occurrences whose notifications were not delivered, are recovered and delivered before newer occurrences on the next process start. Because delivery has no consumer acknowledgment, a crash around delivery may produce duplicates; exactly-once consumer processing is not guaranteed. The existing periodic sync policy remains the durability boundary for power-loss guarantees.

A fatal Channel Reader error still terminates only that Reader and is reported through its existing `Errors()` stream; it must not silently discard recoverable committed history. This per-Channel failure behavior does not shut down the process-scoped runtime.

D-030 and D-031 recorded a bounded graceful-drain shutdown design before the process-lifetime decision. D-034 supersedes that design for process termination: no graceful drain, timeout abort, shutdown-failure report, sender join, stream closure, or resource-release sequence is initiated by eventlog at process exit. Their recoverable-pending-history principle remains applicable.

## Durability, segment management and compaction

Automatic sync occurs every 1 second or 1 MiB of pending writes, whichever comes first. There is no
public Flush or Close operation and no final library-managed sync at process exit. Writes not yet synced
are not promised to survive power loss; process termination recovery applies to the committed records
that remain present in the authoritative append log.

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

- Maximum framing sizes. Canonical type naming is settled by D-037: reflected package path and runtime-provided name, with aliases resolving to their target and instantiated generic names treated as opaque text.
- Exact internal registry-key composition for prepared instances and the precise behavior for distinct
  logical Channels represented by the same concrete Channel Go type.
- Numeric benchmark targets, memory targets, representative measurement environments, repetitions,
  warm-up policy, and statistical acceptance criteria.

Channel identity immutability and snapshot consistency, Reader wake-up/recheck behavior, retry ordering and
termination, and bounded notification backpressure are specified and approved; their numeric queue/retry
parameters remain internal implementation/test policy.

