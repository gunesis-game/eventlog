# Architecture

**Status:** Define-phase design baseline (Cycle 1). This is the active curated architecture specification. Approved decisions are integrated by topic; historical checkpoints are provenance only. It describes intended design, not implementation or verification evidence.

## System boundary

eventlog is an embedded Go library using local filesystem storage. A single process owns a log. It supports concurrent goroutine use but does not coordinate multiple processes or distributed writers. Consumer event/channel semantics remain outside the library; the consumer explicitly selects the persisted fields.

## Logical components

- **Public API:** The only approved public entry point is package-level generic `PrepareLog[TEvent, TChannel](ctx)`, returning `(TEvent, TChannel, error)`. Consumers receive event and channel values, not a Log handle. The package owns the internal `Log` instance and all append/read/next/lifecycle operations; none of those operations are exposed to consumers at this stage. Consumer event and channel types implement `Fields() Fields`; consumers set or change their own field values. A single process owns a log at a time, with concurrent goroutine use coordinated internally.
- **Field builder and validator:** Append's selector callback supplies typed field selectors; those selectors resolve direct fields on a zero-valued consumer probe. Field constructors support nullable/non-nullable primitives. Append-time validation rejects invalid field names, duplicate names, unsupported types, invalid bounds and non-finite floats before writing.
- **Identity encoder:** Produces canonical bytes independent of input field ordering, using bytewise UTF-8 field sort, explicit primitive tags, uint32 big-endian framing, exact widths/IEEE bits, concrete Go type, exposed field-name schema, and designated identity field name/value. UUID v5 uses a fixed namespace and separate Event/Channel domain prefixes.
- **Record serializer/schema validator:** Encodes versioned strict JSON records as NDJSON and validates each record against the schema matching its `schema_version`. Standalone schemas are canonical; runtime embeds those files using `go:embed`.
- **Append transaction writer:** Coordinates concurrent appends, sequence assignment, transaction records and commit marker. A new channel is included in the same transaction as its first event. Writes are recoverable as all-or-nothing at transaction boundaries.
- **Segment manager:** Writes authoritative segment files, rotates at transaction boundaries, and maintains ordered global record sequence. A transaction exceeding target segment size may be placed alone in an oversized segment.
- **Recovery/index manager:** On Open, validates committed transactions and record sequences, rolls back incomplete trailing transactions, validates derived index, and rebuilds it from segment source data when missing or inconsistent. Index replacement is atomic.
- **Reader/tailer:** Streams committed historical occurrences for exactly one consumer-provided Channel value, catches up with concurrent appends, and transitions to live tailing without gaps or duplicates. Global sequence/cursor/offset management is internal; consumers do not provide or receive storage cursors. Cancellation and errors are explicit. Reads are generic over the consumer's concrete Event struct type; eventlog allocates and reconstructs that type internally without a registration/factory/prototype API. A channel may contain multiple concrete Event types; a `Reader[T]` returns matching occurrences in order and advances past other event types, which are filtered rather than treated as errors.
- **Compactor:** Produces temporary replacement segments, validates/checksums and syncs them, renames to final segments, atomically replaces derived index, and removes old segments only after replacement is finalized and validated.

## Record model and identity

The record envelope supports three kinds: `channel`, `event`, and `commit`. Each record carries `schema_version: 1` and a global sequence number. The envelope is versioned JSON; no log header exists.

- **Channel identity:** concrete Channel type, exposed field-name schema, and designated identity field name/value, canonically encoded and UUID v5. Non-identity changes retain logical identity but update the Channel snapshot. A Channel-only mutation does not create an Event occurrence; the next Event occurrence uses the latest Channel snapshot. Earlier occurrences keep their original Channel association. Identical physical Channel records are deduplicated.
- **Event identity:** global and independent of Channel. It is based on concrete Go Event type, exposed field-name schema, and designated identity field name/value, canonical encoding and UUID v5. Non-identity data changes keep logical ID and create new physical occurrences.
- **Occurrence/HEAD:** each append or detected selected Event-field change creates an immutable physical occurrence associated with one Channel. Older occurrences remain in history. Highest-sequence occurrence determines HEAD/current Channel association and ordering. Logical promotion is not a separate record.
- **Transaction ID:** the global sequence of the first transaction record. Transaction order is Channel record when new, Event occurrence, then Commit; existing-channel transactions contain Event then Commit.
- **Sequence:** global across records and segments; Commit consumes a sequence but is excluded from transaction record count and checksum.

## Typed field encoding

Persisted values use strict type-tagged JSON objects. Strings and bools are native JSON; integers are decimal strings to avoid parser precision loss; float32/float64 are lowercase raw IEEE bit-pattern hex (8/16 digits) preserving width and signed zero; null is type-only. JSON Schema validates representation; Go validates exact integer bounds and non-finite float rejection. Named primitive types use underlying kinds.

Canonical identity encoding is separate from JSON and architecture-independent: fixed encoding-format version, domain prefix, UTF-8 package path and concrete type name, field schema and designated identity input, canonical field-name ordering, explicit primitive tags, exact widths/bits, and uint32 big-endian framing. The namespace/domain and tag protocol are detailed below.

## Append transaction flow

1. Resolve selector bindings, Event/Channel field values, schema, designated identity, and validate all fields before writing.
2. Determine whether Channel record is new/reusable.
3. Assign consecutive global sequence numbers to transaction records and Commit.
4. Serialize Channel record (when new) and Event occurrence as exact newline-terminated NDJSON.
5. Compute SHA-256 over exact transaction record bytes including each newline; exclude Commit bytes and its newline.
6. Write Commit marker last. It contains transaction ID (first transaction record sequence), transaction-record count (Channel/Event records only), and checksum.
7. Persist transaction on segment source; atomically update derived index metadata/HEAD/associations. Notify dispatcher only after successful commit.

Validation failure leaves the log unchanged. New Channel and Event are fully atomic: no partial Channel or Event is persisted on invalid fields. Write interruption/crash must not expose partial append; recovery truncates incomplete transaction from its start. Partial Commit is treated as incomplete transaction. Append returns an internal error; no consumer-facing Append API is approved. Recoverable atomicity is distinct from power-loss durability before sync.

## Open and recovery flow

A missing storage structure is initialized automatically during PrepareLog; a new active segment starts as a zero-byte NDJSON file, with no header record. Existing storage is recovered before it becomes available.

- Validate the manifest and selected generation; validate complete records/schema, segment sequence continuity, transaction boundaries, Commit IDs/count/checksums. A complete newline-terminated malformed record, unknown kind/tag, unsupported schema, duplicate/invalid sequence, missing Channel reference, or invalid fields is corruption; never silently skip it.
- Expose only fully committed transactions. An incomplete trailing transaction or partial Commit marker is truncated from transaction start.
- Segments are authoritative. Validate the index; if missing, stale, or inconsistent, rebuild from validated committed segment records and atomically replace it. Index never overrides segments.
- Existing storage with missing/corrupt manifest can be recovered only if exactly one complete generation is unambiguous; otherwise fail closed. Do not choose a generation by filename.
- Recovery ignores temporary compaction files. Cleanup waits until finalized authoritative source segments validate.

Duplicate reconciliation: identical physical copies at the same sequence are deduplicated. Same sequence with different content is corruption. Channel records with same ID and identical data may deduplicate; conflicting data for one ID is corruption. Same logical Event ID at distinct sequences represents valid history and is retained; highest sequence determines HEAD, latest association and ordering. Compaction preserves original sequences and must not promote older occurrences because duplicate copies.

## Reader behavior

The internal unified reader is mandatory and channel-scoped: each reader targets exactly one consumer-provided Channel value, never all channels. It begins with committed history, catches concurrent appends during replay, and transitions to live tailing without gaps or duplicates. Occurrences are ordered by internal global sequence.

Consumers do not provide or receive Channel UUIDs, sequence numbers, cursors or offsets. These remain library-owned. Context cancellation and explicit errors are required. Re-appended logical Events are separate occurrences. A Channel may contain multiple concrete Event types; an internal typed reader reconstructs matching Event values and filters nonmatching types while advancing. Corruption and matching-type decode failures remain errors. Eventlog allocates/reconstructs values without consumer registration, prototype or custom decoder.

The reader stays internal under the currently approved public API; public signatures, pointer/value convention, occurrence wrapper, close and resource semantics remain open.

## Durability, segment management and compaction

Automatic sync occurs every 1 second or 1 MiB of pending writes, whichever first. Flush and Close are durability boundaries; Close flushes and syncs. Append before sync is not promised to survive power loss.

The storage directory contains versioned `manifest.json`, generation-bound rebuildable `index.json`, and `segments/`. Segment IDs monotonically increase and are never reused (including compaction), and are distinct from global record sequences. Finalized segments are immutable; only active segment is writable. Rotate only at transaction boundaries against a fixed target size; oversized transaction may occupy its own segment.

The manifest identifies active finalized segment set/generation and next segment ID, and contains no event payloads. Manifest replacement is the atomic generation-switch/compaction commit point: write temporary manifest, validate, sync, atomically replace. Old files remain until switch is finalized. Orphans are recovery candidates, not authoritative. Ambiguous complete generations fail rather than guessing.

The index is derived, generation-bound and rebuildable. It contains Event IDs, Channel associations, HEAD/latest occurrence, global sequence, segment ID and byte offsets. Missing/stale/inconsistent index rebuilds from validated authoritative segments and is atomically replaced.

Compaction protocol: build replacement segments and index without modifying reader-pinned files → validate/checksum → sync → finalize/rename → atomically switch manifest/index → retain old generation while readers pin it → delete after last pin releases. Temporary files are ignored during recovery and cleaned only after finalized source segments validate.

## Portability and dependencies

Use only Go standard library dependencies and local filesystem storage. Linux and Windows are equal-priority supported platforms; macOS is excluded. If platform behavior differs or is unreliable, core functionality must use a strategy that works on both platforms. Correctness, recovery, predictable memory use and platform parity precede numeric performance targets.
## Current public preparation/configuration decision

The only approved public preparation call is `event, channel, err := eventlog.PrepareLog[MyEvent, MyChannel](ctx)`, with declaration shape `func PrepareLog[TEvent EventType, TChannel ChannelType](ctx context.Context) (TEvent, TChannel, error)`. Consumers receive values, not a Log handle. Internal Log owns append/read/next, persistence, recovery, change manager and lifecycle. No public Log, append/read/next/close/flush/compact API is approved. Event/Channel constraints currently require `Fields() Fields`.

Mandatory configuration is a package-level `Config(options ...Option) error`, requiring `WithStorageDirectory(path)`. Nil/duplicate/missing/invalid options fail before publishing. Successful config is immutable and one-time; repeated/late attempts match `ErrConfigAlreadySet`. PrepareLog before Config matches `ErrConfigNotInitialized`. Relative path becomes absolute and lexically cleaned at Config; symlinks are not resolved. Config does not create storage; PrepareLog does. Config/initialization are coordinated; concurrent PrepareLog calls wait for one initialization, each with its own cancellation. Failed initialization may retry using frozen configuration. Context carries cancellation/deadline only.

## Integrated change-manager and notification design

One internal scheduler polls at 100 ms across selected field bindings; no per-field goroutines. The cadence is internal/test-only. Consumer synchronizes its own field mutations. Channel notification accessor exposes a Go channel (exact signature open); successful occurrence payloads are concrete Event snapshots captured at persistence boundary. Notification only follows commit. Every committed occurrence eventually becomes visible; finite buffers cannot drop items. Persistence and scheduler cannot block on slow consumers, requiring an internal backlog/dispatcher. Recovered backlog is first and persistence-ordered; live items do not overtake. Orderly shutdown drains before channel close; crash recovers undelivered backlog. Temporary persistence failures retry with backoff; persistent failures are typed notifications, never silent loss. Retry termination, backlog durability/retention, replay boundary, typed error and delivery order, and drain failure semantics remain unresolved. Drop-on-full was superseded by guaranteed delivery.

## Integrated manifest, index, recovery and compaction protocol

Storage uses manifest, rebuildable generation-bound index and segment files. Segment IDs increase and are never reused; global record sequence is independent. Finalized segments are immutable; only active segment is writable. Rotate at transaction boundaries; oversized transaction may occupy a segment alone. Manifest is versioned and authoritative for generation/active finalized segment set/next ID; it has no payloads. Manifest replacement is the atomic generation-switch point after temporary write, validation and sync. Orphan files are not automatically authoritative; ambiguous recovery fails closed.

The index contains Event IDs, Channel associations, occurrence/HEAD, global sequence, segment and offsets. It is derived and rebuildable from validated segments, generation-bound and atomically replaced. It never overrides source segments.

Recovery order: exclusive process ownership; validate manifest/generation; validate segments, schemas and sequences; validate transaction markers/count/checksum; truncate incomplete trailing transaction to its start; validate or rebuild index; safe cleanup only after authoritative source validation; then initialize append and readers. Missing/corrupt manifest may recover only if one complete generation is unambiguous; otherwise error.

Compaction is generation-pinned and reader-safe: build replacements without modifying pinned files; validate/checksum/sync/finalize; atomically switch manifest/index; new readers use new generation; existing readers keep old pinned generation; delete old files only when the final pin releases.

## Canonical encoding, type tags, and UUID protocol

Canonical identity encoding is distinct from JSON. It starts with an encoding-format version and domain prefix; UTF-8 package path and concrete type name; uint32 BE field count; then sorted field entries. Field names sort bytewise UTF-8. Each field is length-prefixed name, one-byte primitive tag, and length-prefixed payload. All variable component lengths/counts use uint32 BE; oversized components fail. Multi-byte numeric payloads use big-endian; int/uint are fixed 64-bit regardless of architecture; floats use raw IEEE bits preserving width and signed zero; nullable nil has Null tag; strings are UTF-8. Named primitives use underlying tags.

Permanent v1 tags: Null `0x00`, String `0x01`, Bool `0x02`, int `0x10`, int8 `0x11`, int16 `0x12`, int32 `0x13`, int64 `0x14`, uint `0x20`, uint8 `0x21`, uint16 `0x22`, uint32 `0x23`, uint64 `0x24`, float32 `0x30`, float64 `0x31`. Bool payload is one byte 0/1; signed/unsigned widths are fixed as indicated; int/uint use 64-bit. Unknown tags fail recovery. Tags are permanent; reserved ranges stay unused. UUID v5 namespace: `26612f1d-f648-483a-9cfe-59c18f113891`; Event domain prefix `eventlog\\x00event\\x00v1\\x00`; Channel domain prefix `eventlog\\x00channel\\x00v1\\x00`. Identity includes type, exposed field-name schema and designated identity name/value; non-identity mutation retains logical ID.

## Record taxonomy and duplicate recovery rules

Supported records are `channel`, `event`, and `commit` (the later commit-marker decision supersedes the earlier two-kind design). Envelope includes `schema_version`, global `seq`, `kind`, and payload. Channel payload has ID/type/fields; Event has ID/channel_id/type/fields; Commit has transaction ID/count/checksum. Logical promotion is a higher-sequence occurrence, not a separate record.

Recovery deduplicates same-sequence identical physical copies; same sequence with differing content is corruption. Identical Channel ID+data can deduplicate; conflicting Channel data under one ID is corruption. Different-sequence occurrences for the same Event ID are valid history; highest sequence is HEAD, current Channel association and ordering. Compaction retains original sequence numbers and must not promote older occurrences because of duplicate copies. Temporary compaction files are ignored as input and cleaned only after finalized source segments validate.

## Selector resolution and append atomicity

`FieldOf` selects non-nullable primitive fields. `FieldOfNullable` selects nullable primitives and returns pointer-to-pointer (e.g. `func(e *User) **string`); nil is explicit Null. Field names are inferred, not passed as strings. Resolve selectors by invoking them on a zero-valued probe and comparing returned pointers with candidate direct-field addresses; exactly one match is required. This does not inspect source and may not catch conditional selectors that resolve to a field on the probe. Validate all Event and Channel fields before writing; validation failure leaves log unchanged. New Channel and Event records commit together. Write interruption/partial Commit rolls back from transaction start during recovery.

## Commit marker specifics

Commit is a first-class versioned record with its own global sequence. Transaction ID equals the first transaction record sequence; count includes only Channel/Event transaction records, not Commit. SHA-256 covers exact serialized transaction-record bytes including terminating newlines; Commit bytes/newline are excluded. Write order is Channel (if new), Event, Commit. Recovery exposes records only after marker validation. Partial Commit rolls back whole transaction. Append has an internal error result; no consumer-facing Append API is approved.



## Commit marker specifics

Commit is a first-class versioned record with its own global sequence. Transaction ID equals the first transaction record sequence; count includes only Channel/Event transaction records, not Commit. SHA-256 covers exact serialized transaction-record bytes including terminating newlines; Commit bytes/newline are excluded. Write order is Channel (if new), Event, Commit. Recovery exposes records only after marker validation. Partial Commit rolls back whole transaction. Append has an internal error result; no consumer-facing Append API is approved.

