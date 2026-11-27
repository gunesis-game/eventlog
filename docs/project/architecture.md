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

The public preparation entry point is `PrepareLog[TEvent, TChannel]`. `TEvent` is a concrete Event type constrained by the common `Event` interface; `TChannel` is a concrete Channel type constrained by `Channel`. `PrepareLog(ctx, newEvent, newChannel)` takes two constructor factories (D-049): `newEvent func(chan<- func() error) TEvent` and `newChannel func(chan Event, chan *ChannelError) TChannel`. eventlog creates every channel those factories receive; the consumer stores them and returns them from `Event.Transaction()`, `Channel.Notifications()`, and `Channel.Errors()`. The Event interface is `Fields() Fields` plus `Transaction() chan<- func() error`; the Channel interface is `Fields() Fields`, `Notifications() <-chan Event`, and `Errors() <-chan *ChannelError` (D-052). These approved contracts must not be expanded or changed without explicit user approval.
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

After the Channel runtime is resolved, preparation invokes the Event factory with that Channel's shared
transaction channel, so the consumer can set the Event's identity and initial fields in its constructor
(D-049). The candidate's `Transaction()` must return the channel it received. Its canonical Event identity
selects the live instance: an existing logical Event on that Channel reuses its pointer and the candidate is
discarded; otherwise the candidate is adopted. A Channel may therefore hold many live Events of one concrete
type, one per logical Event identity. Preparation persists nothing; only transactions create occurrences.

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

## Package layout

The module is one public package plus internal packages that consumers cannot import (D-053). The public
API is defined only in the root package `eventlog`; internal packages never define public types.

| Package | Responsibility |
|---|---|
| `eventlog` (root) | Public API (`Config`, `PrepareLog`, field selectors, `Event`, `Channel`, `ChannelError`) and the process runtime: Channel runtimes, Readers, transaction processors, and decoder registry |
| `internal/identity` | UUID representation and canonical v1 identity encoding with UUID v5 derivation |
| `internal/fieldschema` | Resolving field selections into schemas; typed value capture, encoding, decoding, rollback, and copies |
| `internal/record` | Record types, framing, encoding, embedded canonical JSON Schemas (`internal/record/schemas/`), checksums, and transaction and stream validation |
| `internal/storage` | Segment store, manifest, derived index, recovery, and compaction, expressed as file operations through `internal/filesystem` |
| `internal/filesystem` | The file-system boundary (D-054): an HTTP client used by `internal/storage`, a localhost HTTP server started by `storage.Open`, and a single `Backend` interface the server delegates to. The OS backend uses the local file system and the ownership lock |

Dependencies point one way: `identity` ← `fieldschema` ← `record` ← `storage` ← root, with `storage` →
`filesystem`. Tests live beside the code they test.

### File-system boundary (D-054)

`internal/storage` performs no operating-system calls. It calls the `internal/filesystem` client, which
speaks HTTP to a server bound to `127.0.0.1` on a random port and authenticated by a per-process random
bearer token. Client and server speak only HTTP/2 over cleartext TCP with prior knowledge (RFC 9113
section 3.3), and the client holds at most one connection, so every concurrent request is a stream on a
single long-lived connection (D-055). That avoids ephemeral-port exhaustion from TIME_WAIT churn and keeps
descriptor use constant as Channels grow; the deprecated `Upgrade: h2c` mechanism is not used. The
research behind this choice is in [docs/research/os-connection-limits.md](../research/os-connection-limits.md); the server is started and stopped with the store and is invisible above `internal/storage`.
Paths are slash-separated and relative to the storage root; the server rejects absolute paths and `..`.

| Operation | Request | Contract |
|---|---|---|
| Read a byte range or whole file | `GET /files/{path}` with optional `Range: bytes=a-b` | Missing file is 404 |
| List a folder | `GET /dirs/{path}` | JSON array of `{name, dir}` |
| Create a folder | `PUT /dirs/{path}` | Idempotent |
| Create an empty file only if absent | `PUT /files/{path}` with `If-None-Match: *` and no body | Existing file is 412; durable on success |
| Atomically replace a file | `PUT /files/{path}` with the content | Readers see the old or new content, never a mix; durable on success |
| Append | `POST /files/{path}?op=append` with the bytes | Returns the new size; appends are ordered |
| Truncate | `POST /files/{path}?op=truncate&size=N` | |
| Sync | `POST /files/{path}?op=sync` | Earlier appends are durable on success |
| Rename | `POST /files/{path}?op=rename&to={path}` | Atomic; replaces the target; durable on success |
| Delete | `DELETE /files/{path}` | Missing file is 404 |
| Acquire / release ownership | `POST /locks/{name}`, `DELETE /locks/{name}` | Held lock is 409; the OS backend's lock is released by the operating system when the process exits |

The server enforces these contracts for every backend, so recovery's guarantees (exclusive create, atomic
replace and rename, truncation, sync before success, offset reads) do not depend on the backend.

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
- **Transaction processor:** One worker per logical Channel runtime receives transaction functions from
  that Channel's shared, bounded transaction channel, applies them in send order, validates the result,
  rolls back rejected changes, and group-commits accepted changes (see Transactions).
- **Notification dispatcher/backlog:** The Channel interface exposes `Notifications() <-chan Event` and `Errors() <-chan *ChannelError`. One Channel owns one heterogeneous notification stream and one shared error stream. No `Notify` callback/sink method, public helper struct, or consumer embedding requirement is permitted. eventlog delivers immutable concrete Event snapshots after commit without blocking persistence/scheduler on slow consumers. The in-memory pending-delivery buffer is finite; a full buffer applies backpressure to the Channel Reader, which waits and resumes from its held position when capacity becomes available. This bounds notification-backlog memory without blocking persistence or discarding committed history. The authoritative committed append log remains the source of pending history; no separate durable queue is introduced. The error stream carries `*ChannelError` values (D-052); its delivery rules are specified under Transactions. Recoverable delivery errors may be retried only while the pending occurrence is retained and order/completeness are preserved; a failed occurrence is never silently skipped to advance the Reader. When retry policy declares a failure terminal, the Channel Reader terminates, reports the error through `Errors()`, and closes its streams after fatal-error reporting. Committed pending history remains recoverable from the authoritative log for a future runtime. Retry policy is internal: temporary delivery failures retry the same pending occurrence using bounded backoff and a finite retry limit; retry does not advance the Reader or permit later occurrences to overtake. On retry exhaustion, the Channel Reader terminates and reports through Errors(); committed pending history remains recoverable from the authoritative log. Other Channel Readers continue independently. Numeric buffer capacity, limits, delay values, and classification details are implementation/test policy, not public configuration. Exactly-once consumer processing is not guaranteed without consumer acknowledgment or transactional integration.

### Registry key implementation boundary

The registry-key representation is an internal implementation detail and is not a persisted format or
consumer-facing contract. Implementations may choose maps, composite keys, or another representation,
provided they preserve these observable rules:

- Event decoder registration is keyed by concrete Event Go type plus exposed field-name schema.
  Re-registering the same combination reuses the registration; registering the same concrete type with a
  different schema fails with the specified typed schema-conflict error without replacing the existing
  registration.
- Event instance reuse follows canonical logical Event identity.
- Channel runtime reuse follows canonical logical Channel identity, not concrete Channel Go type alone.
  Distinct logical Channels of the same concrete Go type have separate live pointers, runtimes, and Reader
  progressions; repeated preparation of the same logical Channel reuses its existing pointer/runtime/Reader.
- Event decoder registration, Event instance identity, and Channel runtime identity remain separate
  registry concerns.

No internal registry key is serialized or exposed through the public API. Changing the internal
representation is permitted as long as these behaviors remain unchanged. This implementation boundary is approved by D-039.

## Field selection, reconstruction, and identity integrity

Consumers implement `Fields() Fields` on their Event and Channel pointer types and select direct fields
using typed selectors. FieldOf accepts func(*T) *V for a non-nullable field; FieldOfNullable accepts
func(*T) **V for a nullable field; IdentityOf accepts func(*T) *U, where U has underlying type [16]byte,
and designates the UUID identity field (D-044). Exactly one IdentityOf selector is required per Fields()
declaration; the identity field is part of the exposed schema. Field names are inferred, selectors
must resolve to exactly one direct field, and named primitive types are supported. Nil remains distinct
from a pointer to the zero value.

Historical occurrences are reconstructed into fresh concrete Event pointers and never overwrite the
prepared live Event pointer. Notification snapshots are immutable and independent of live values,
persistence storage, and other notification snapshots. Historical Channel snapshots are likewise
separate from the live Channel pointer. Concrete primitive types, widths, and nullability are preserved.

The zero UUID is never a valid identity. PrepareLog fails when either factory's candidate has a zero
identity (D-045, as revised by D-049). A prepared Event or Channel identity is immutable: a transaction
that changes it is rejected as invalid state and rolled back (D-051). It does not create another logical
Event, rewrite identity, or alter existing occurrences or HEAD.

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
- **Occurrence/HEAD:** each accepted transaction change to an Event's selected fields creates an immutable physical
  occurrence associated with one Channel. Older occurrences remain in history. Highest-sequence
  occurrence determines HEAD/current Channel association and ordering. Logical promotion is not a
  separate record.
- **Transaction ID:** the global sequence of the first transaction record. A persisted transaction
  belongs to one Channel and contains one or more Event occurrences, each optionally preceded by a
  Channel record carrying the Channel snapshot that occurrence was made with, then one Commit (D-050).
  A Channel record is written only when the Channel is new or its snapshot differs from the last one
  written.
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
(8/16 digits) preserving width and signed zero; null is type-only. The designated identity field is
persisted as a `uuid`-tagged value holding the lowercase canonical hyphenated text, so reconstruction
restores it and recovery can re-derive and verify the record's logical identity (D-044). JSON Schema uses strict variants,
required keys, fixed type tags, and no extra properties to validate representation; Go validates exact
integer bounds and non-finite float rejection. Schemas are versioned canonical standalone files under
`internal/record/schemas/` and embedded from those same files using go:embed. Named primitive types use underlying
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
2.  For each occurrence in the batch, determine whether its Channel snapshot differs from the last Channel record written.
3.  Assign consecutive global record sequence numbers to transaction records and Commit.
4.  Serialize each needed Channel record followed by its Event occurrence as newline-terminated NDJSON.
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

## Transactions

Consumers change Events and Channels only through transactions (D-049). A transaction is a
`func() error` sent on an Event's `Transaction()` channel. All Events of one logical Channel share that
Channel's transaction channel, so every transaction for the Channel is applied strictly in send order;
different Channels proceed independently. The channel is buffered with a fixed internal capacity; senders
block only when it is full, which bounds memory (D-050). Reading live fields safely also requires a
transaction.

The Channel's transaction processor runs each function on its own goroutine, so only eventlog writes the
live instances while a transaction runs. For each transaction it snapshots the Channel and every live Event
of that Channel, runs the function, then validates the result. A transaction is rejected when the function
returns an error (`TransactionRefused`), panics (`TransactionPanicked`), or leaves invalid state
(`InvalidState`): a changed identity, an invalid field value, or a Channel change without any Event change
(D-051). Channel fields change only inside a transaction that also changes an Event. A transaction that
changes nothing is accepted and persists nothing. Each changed Event in an accepted transaction becomes one
occurrence carrying the Channel snapshot after that transaction.

Accepted transactions are group-committed (D-050): the processor keeps applying queued transactions and
commits them as one atomic persisted transaction when the queue is empty or the batch reaches a fixed
internal size. Notifications follow only after the commit.

Failures keep the prefix and reject the rest (D-051). When a transaction is rejected, its changes to the
selected Channel and Event fields are rolled back; the accepted transactions before it in the batch are
committed; and the failure is reported. If a commit fails (`CommitFailed`), every transaction in the batch
is rolled back. Rollback restores only selected fields; unselected consumer fields are not tracked.

A rejection pauses the Channel until the consumer receives the error (D-052). The error channel is
unbuffered, so receipt is the acknowledgement. After receipt, every transaction still queued, which was sent
before the consumer knew of the failure, is rejected without being run. Processing then resumes with later
sends. If the Channel's streams have already closed after a fatal Reader error, rejected transactions are
rolled back without pausing.

`ChannelError` has `Kind` (an `ErrorKind`: `TransactionRefused`, `TransactionPanicked`, `InvalidState`,
`CommitFailed`, `ReaderFailed`, `DeliveryFailed`), `Cause` (the consumer's error, the panic value, the
field problem, or the I/O error, also returned by `Unwrap`), and `Events` (isolated copies of the attempted
values of each Event the failed transaction changed, or of every Event in the batch for `CommitFailed`). Its
message names the Channel identity, the kind, the Event types and identities, and the cause. `ReaderFailed`
and `DeliveryFailed` are fatal for that Channel's Reader: the error is offered to the consumer, then the
notification and error streams close.

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

A fatal Channel Reader error still terminates only that Reader, reports the first fatal error through its existing `Errors()` stream, and then closes that Channel's notification and error streams. It must not silently discard recoverable committed history. This per-Channel failure behavior does not shut down the process-scoped runtime. Stream closure is a consequence of a terminal per-Channel Reader failure, not a process-exit cleanup action; eventlog does not close streams merely because the process is terminating.

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

Compaction is triggered internally (D-048): after a segment rotation, when the number of finalized segments
reaches a fixed internal threshold, eventlog consolidates all finalized segments into one replacement
segment. Every record, sequence number, and transaction is preserved byte-for-byte; compaction reduces file
count and never removes history. The threshold is internal test/implementation policy, not public
configuration.

Compaction protocol: build replacement segments/index without modifying reader-pinned files →
validate/checksum → sync → finalize/rename → atomically switch manifest/index → retain old generation
while readers pin it → delete after last pin releases. Temporary files are ignored during recovery and
cleaned only after finalized source segments validate. New readers bind to the switched generation;
existing readers continue on their pinned generation.

## Implementation notes

The current implementation realizes this design as one Go package. Internal policy values (notification
buffer 64, transaction buffer 64, batch limit 64 transactions, five delivery attempts with doubling backoff
from 10 ms, target segment size 64 MiB, compaction at 8 finalized segments, sync every 1 s or 1 MiB) are
implementation/test policy, not public configuration. Single-process ownership uses an OS-held lock file
(`flock` on Linux, an unshared handle on Windows), taken through the file-system boundary's lock route and
released by the operating system when the process exits. The derived
index is held in memory and persisted to `index.json` at sync points; recovery always revalidates every
segment and replaces a stored index that does not match. The OS backend keeps one open handle per file and
closes it before the file is renamed or removed, which Windows requires. The file-system client multiplexes every
request over one HTTP/2 connection (D-055). A Reader fetches up to 64 consecutive occurrences of its Channel
from one segment, at most 1 MiB, with a single ranged read rather than one request per occurrence. Every process start replays a Channel's full committed
history, because delivery has no acknowledgement.

## Portability and dependencies

Use only Go standard library dependencies and local filesystem storage, with one approved exception
(D-046): `github.com/santhosh-tekuri/jsonschema/v6` validates records against the embedded canonical JSON
Schemas. Linux and Windows are
equal-priority supported platforms; macOS is excluded. If platform behavior differs or is unreliable,
core functionality must use a strategy that works on both platforms. Correctness, recovery, predictable
memory use and platform parity precede numeric performance targets.

## Multi-device operation (Proposed, D-060; not implemented)

**Status: Proposed, awaiting approval.** This section designs the response to D-056 to D-059 and applies
D-061 to D-064. The current implementation is single-device and local-only. Research:
[causal ordering and conflicts](../research/causal-ordering-and-conflicts.md) and
[cross-device notifications](../research/cross-device-notifications.md).

### Configuration and identity

- `WithStorageEndpoint(GoogleDrive{Account, DeviceStateDirectory, OAuthClientFile})` selects Google Drive
  (D-061, D-065):
  - `Account` is the **log owner's** Google account, whose Drive holds the master store. It is the same on
    every device.
  - `DeviceStateDirectory` is a directory on this device only, never shared, holding this device's
    identity, its one-process lock, its Google refresh token, and the cached folder IDs. It must be on
    persistent storage; losing it makes the machine a new device.
  - `OAuthClientFile` is the Desktop-app OAuth client created in the owner's free Google Cloud project.
- Each device signs in with **its own** Google account, through Google OAuth with the restricted `drive`
  scope, approved in a browser, with the browser returning to the local file-system server (D-065). Every
  account must be listed as a test user while the OAuth app stays in Testing status.
- In Testing status, Google expires each device's authorization seven days after consent. eventlog detects
  the expired sign-in and asks for re-approval; publishing the app later requires Google's restricted-scope
  verification with a yearly security assessment.
- Access is controlled by Drive sharing (D-065):
  - The owner shares the folder: Editors read and write; Viewers can replay and tail but not write.
  - A device whose account cannot see the owner's folder is rejected at start with a clear "not shared"
    error. Only the owner's devices may create the folder.
- Files a device writes are owned by that device's account and use its storage; the owner's folder holds
  them. This is accepted (D-065).
- A device generates a random device identity on first start and keeps it in its device state directory.
- `WithStorageDirectory(path)` keeps its state inside that directory and serves one device.

### Storage layout

- Every backend uses a folder with the fixed name `eventlog` (D-062). On Drive it is found by tag and
  ownership, not by ID: an `appProperties` tag marking it as eventlog's root, owned by the configured owner
  account, with the oldest winning if two were created at once (D-065). Found IDs are cached in the device
  state directory, so later starts cost one lookup instead of a search. Locally the folder is
  `<storage directory>/eventlog`.
- The folder holds `log.json` with a permanent log identity generated once, so a log keeps its identity
  across backends (D-062).
- Each device writes only its own log under `devices/{deviceID}/` (manifest, index, segments), in the
  single-writer format. No file has two writers, so storage without append or compare-and-swap stays safe.
- On Drive, an append re-uploads the segment, so Drive segments are small (1 MiB), and every upload is
  durable when it returns.

### Causal metadata (D-064)

- Each Event record adds the writing device's identity, the Channel's version vector (for each device that
  has written to the Channel, how many of its Channel occurrences this writer had delivered), and a hybrid
  logical clock timestamp. Vectors are scoped per Channel.
- The v1 record format is changed in place; no released log exists.

### Change notifications (D-063)

- Notifications are generic file-change events of the file-system layer. Nothing above `internal/storage`
  knows of them.
- The Google Drive backend keeps a "latest position per device" board in Firebase Realtime Database:
  `/eventlog/{logID}/devices/{deviceID} = {"path": "...", "size": N}`. Each device writes only its own
  record, after its own successful Drive writes, at most once per file per second.
- Every device streams `/eventlog/{logID}/devices` with server-sent events, over one outbound connection. The
  stream starts with the current board, which gives a catch-up on connect, then delivers each change.
- The file-system server forwards board changes as file-change events on its `/events` SSE stream to
  `internal/storage`, which wakes the affected Channel Readers. A Reader expecting size N retries its read
  briefly if Drive does not yet show N.
- Authentication: the Google ID token from each device's sign-in is exchanged through Firebase
  Authentication (`accounts:signInWithIdp`, `providerId=google.com`) for a Firebase ID token and refresh
  token; database requests pass it as `auth=`, and security rules restrict the board to the accounts the
  owner allows.
- The board holds no data of value. If Firebase is unavailable or switched off for exceeding the free plan,
  devices still commit, and see each other's changes at their next start.
- The OS backend has no other writers and sends no change events.

### Delivery, conflicts, and remote changes

- **Causal delivery:** a Channel's Reader delivers a remote occurrence only once every occurrence in its
  version vector has been delivered. Unrelated occurrences are not delayed.
- **Conflicts:** an occurrence of a logical Event whose vector is concurrent with that Event's current state
  is reported on the Channel's error stream, naming both occurrences. The current state is the greatest
  hybrid logical clock timestamp, with device identity as the tie-break. Conflicts are informational: they
  neither reject nor pause.
- **Remote changes:** the Reader submits each delivered remote occurrence to the Channel's transaction
  processor, which sets the live instance's selected fields to the converged current state without
  persisting anything.
- **Recovery:** a device fully recovers and may truncate only its own log; other devices' logs are read
  read-only, and an incomplete trailing transaction there is treated as not yet committed.

### Sign-in, Firebase access, and free-plan bounds (D-066)

Approved on 2026-11-27; these settle the open points of this design.

1. **Firebase project details.** The `GoogleDrive` descriptor gains two fields, `FirebaseDatabaseURL` and
   `FirebaseAPIKey`, copied from the Firebase console. Neither can be derived: the database URL depends on
   the database's region, and the web API key is not in the OAuth client file. The web API key identifies
   the project and is not a secret. The Firebase project is the owner's, in the same Google Cloud project as
   the OAuth client, because Firebase Authentication accepts Google ID tokens from that project's clients.
   Like the Drive folder, it exists once, under the owner's account. Other accounts never sign up for
   Firebase or create anything: eventlog exchanges each device's Google sign-in for a token from the owner's
   project, which records the account there automatically.
2. **Security rules.** The rules are fixed and installed once by the owner, pasted from eventlog's
   documentation into the Firebase console, with the owner's email filled in; they never change as the folder
   is shared with more people. Access to the board follows the Drive folder's sharing, as access to the data
   does:
   - **Members list:** `/eventlog/{logID}/members/{emailKey} = "writer" | "reader"`, where `emailKey` is the
     lowercased email with `.` replaced by `,` (keys cannot hold `.`). Drive Editors and the owner are
     writers; Viewers are readers. Only the owner's account may write this list.
   - **Sync from Drive:** an owner's device reads the folder's Drive permissions and rewrites the members
     list at its start and whenever a membership request arrives. Nobody maintains the list by hand.
   - **Membership requests:** a device whose account Drive admits but the members list does not yet name
     writes `/eventlog/{logID}/requests/{emailKey}`, the only place a non-member may write, and only under its
     own key. Owner devices stream the requests and re-sync from Drive on each one, so a newly shared account
     gets live notifications as soon as an owner's device is online, without polling. Until then the device
     works without live notifications and reports `NotificationsUnavailable`.
   - **Board:** `/eventlog/{logID}/devices/{uid}/{deviceID}`, where `uid` is the signed-in Firebase user.
     Writers may write only under their own `uid`; writers and readers may read the board; anyone else is
     refused.
   - **Revocation:** when the owner stops sharing the folder, Drive refuses the data at once; the members
     list drops the account at an owner's device's next sync. Until then the account can only see positions.
   - Sign-in itself is also limited by the project's test-user list while the OAuth app is in Testing, so
     the owner both adds each account as a test user and shares the folder with it.
3. **Sign-in and re-approval.**
   - First start: when the device has no valid sign-in, `PrepareLog` returns a `*SignInRequired` error with
     `URL` (the local file-system server's sign-in address, which redirects to Google) and `Completed`, a
     channel closed when the sign-in finishes. The application shows the URL, waits on `Completed`, and calls
     `PrepareLog` again. No polling, and no new public function.
   - While running: when Google expires the sign-in after seven days, each Channel reports one ChannelError
     of a new kind `SignInRequired`, whose `Cause` is the `*SignInRequired` error. Transactions sent while
     signed out are rejected with that kind under the prefix rule; Readers wait and resume by themselves
     once sign-in completes.
   - A device without a local browser: Google's device flow for limited-input devices does not allow the
     `drive` scope, so the loopback sign-in stays. The user forwards the port from a machine with a browser
     (`ssh -L 8765:127.0.0.1:8765 <device>`) and opens the URL there; this is documented, not coded.
4. **Free-plan bounds.** Accepted as limits, documented with a budget:
   - At most 100 devices online at once, one stream each; a device beyond that has no live notifications.
   - Downloads: each board change reaches every online device, about 200 bytes each. With N devices each
     writing continuously, coalesced to once per second, that is about N² × 200 bytes per second, so 10 GB a
     month allows about 5 devices writing nonstop, or many more writing in bursts.
   - Over either limit, or when Firebase is switched off for the month, devices keep committing and see each
     other's changes at their next start. Each Channel reports this once with a new ChannelError kind
     `NotificationsUnavailable`, so the loss of live updates is not silent.

## Unresolved architectural details

- Canonical type naming is settled by D-037: reflected package path and runtime-provided name, with aliases resolving to their target and instantiated generic names treated as opaque text; v1 framing limits are settled by D-038.
- Numeric product performance and memory acceptance targets, to be proposed only after representative baselines are measured. The approved workload, environment-recording, repetition, warm-up, and statistical reporting protocol is specified in [benchmarks.md](benchmarks.md).

Channel identity immutability and snapshot consistency, Reader wake-up/recheck behavior, retry ordering and
termination, and bounded notification backpressure are specified and approved; their numeric queue/retry
parameters remain internal implementation/test policy.

