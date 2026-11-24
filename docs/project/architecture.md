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

The persisted envelope has three kinds: `channel`, `event`, and `commit`. Each record has a `schema_version: 1` and a global sequence number; there is no log header.

- **Channel identity:** concrete Channel type, exposed field-name schema, and consumer-designated identity field name/value, canonically encoded and UUID v5. Non-identity changes retain logical identity but update snapshots; Channel-only changes do not create Event occurrences. New occurrences use latest Channel snapshot; previous associations remain immutable. Identical physical Channel records deduplicate.
- **Logical Event identity:** global and independent of Channel. Derive UUID v5 from concrete Event type, exposed field-name schema, and consumer-designated identity field name/value, canonically encoded. Non-identity changes retain logical ID but create new immutable occurrences.
- **Occurrence:** every append or detected selected-field change produces a new immutable physical occurrence associated with one channel. Field changes retain the Event's original logical ID; earlier occurrences remain in history. Latest occurrence defines HEAD/current channel and ordering.
- **Transaction ID:** sequence of the first transaction record.
- **Transaction order:** channel record if new, then event occurrence, then commit marker. Existing-channel append writes event then commit.
- **Sequence accounting:** commit marker consumes a global sequence but is excluded from transaction-record count and checksum.

## Typed field encoding

Persisted values use explicit type-tagged JSON objects with strict allowed properties. Strings and bools use native JSON values; integers use decimal strings to avoid precision loss; float32 and float64 use lowercase hex of their raw IEEE bit patterns (8 and 16 digits respectively), preserving width and signed zero; null is type-only. Schema validates shapes; Go validates exact integer bounds. Non-finite floats are rejected. Named primitive types persist according to their underlying supported primitive.

Canonical identity encoding is separate from JSON serialization: fields are sorted bytewise UTF-8, encoded with explicit type tags and uint32 big-endian framing, and values retain exact primitive widths/bits. Event and channel identity use distinct UUID v5 domain prefixes.

## Append transaction flow

1. Resolve and validate channel/event fields and stable identities before writing.
2. Determine whether the channel record is needed.
3. Assign consecutive global record sequence numbers to transaction records and commit marker.
4. Serialize channel (when new) and event occurrence as NDJSON records, each newline-terminated.
5. Compute SHA-256 over the exact serialized transaction-record bytes including newline terminators; exclude commit marker and its newline.
6. Persist the commit marker last. It contains transaction ID (first transaction-record sequence), transaction-record count (excluding marker), and checksum.
7. Update the derived index using segment-first persistence and atomic index replacement. Append is recoverably atomic, but not guaranteed power-loss durable until sync.

## Open and recovery flow

- A missing log/storage structure is initialized automatically; the initial log is zero bytes and has no header.
- Validate complete records and schema version. A complete malformed record is corruption; do not skip it.
- Expose only transactions whose commit marker validates transaction ID, count and checksum.
- An incomplete trailing transaction or partial commit marker is truncated from transaction start.
- Treat segments as authoritative. Validate the index and rebuild it from validated committed segment records if absent/inconsistent; atomically replace the rebuilt index.
- Unsupported schema versions are explicitly rejected.
- Recovery cleanup of compaction temp artifacts must wait until finalized source segments validate.

## Reader behavior

The unified reader is mandatory channel-scoped: each reader targets exactly one consumer-provided `Channel` value. It starts with committed history, catches concurrent appends while replaying, then switches to waiting for new occurrences without a gap or duplicate. Delivery is ordered by the internal global sequence. Consumers neither provide nor receive storage cursors, sequence numbers, or offsets; those remain library-owned. Context cancellation and explicit errors are required. Re-appends are distinct occurrences. A channel may contain multiple concrete Event types. `Read[T]` filters the channel stream, returning only T occurrences in order and advancing past nonmatching types; malformed records and failures decoding matching T values remain errors. eventlog allocates and reconstructs matching values internally.

## Durability, segment management and compaction

Automatic sync is every 1 second or 1 MiB, whichever comes first. Flush and Close are explicit sync boundaries. A successful Append before sync is not promised to survive power loss.

Segments are fixed-target-size and rotated only at transaction boundaries. Oversized transactions may use their own segment. Segment data is authoritative; the local index is rebuildable and contains event IDs, channel associations, latest occurrence/HEAD, global sequence positions and segment offsets.

Compaction ordering: write temporary replacements → validate/checksum → sync → rename/finalize replacements → atomically replace index → remove old segments. Temporary compaction artifacts must not be cleaned until finalized source segments validate. Reader-safe compaction is generation-pinned: existing readers hold the generation they opened; new readers use the manifest-switched generation. Old generation files are removed only after the last reader releases them.

## Portability and dependencies

Use only Go standard library dependencies and local filesystem storage. Linux and Windows are equal-priority supported platforms; macOS is excluded. If platform behavior differs or is unreliable, core functionality must use a strategy that works on both platforms. Correctness, recovery, predictable memory use and platform parity precede numeric performance targets.

## Unresolved architectural details

- Exact public API signatures and detailed error semantics beyond the approved configuration sentinels.
- Whether a public Occurrence wrapper is needed; eventlog owns concrete Event reconstruction.
- Reader cancellation, close, and resource-lifecycle semantics.
- Benchmarks and memory/performance targets.

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

