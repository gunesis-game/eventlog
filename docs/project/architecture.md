# Architecture

**Status:** Define-phase design baseline (Cycle 1). This architecture reflects approved decisions; unresolved details are called out explicitly. It describes the intended design, not implemented or verified behavior.

## System boundary

eventlog is an embedded Go library using local filesystem storage. A single process owns a log. It supports concurrent goroutine use but does not coordinate multiple processes or distributed writers. Consumer event/channel semantics remain outside the library; the consumer explicitly selects the persisted fields.

## Logical components

- **Public API:** Open/Append/Flush/Close direction plus a unified replay/live reader. Exact exported signatures remain to be finalized.
- **Field builder and validator:** Typed field selectors resolve direct fields on a zero-valued consumer probe; field constructors support nullable/non-nullable primitives. Append-time validation rejects invalid field names, duplicate names, unsupported types, invalid bounds and non-finite floats before writing.
- **Identity encoder:** Produces canonical bytes independent of input field ordering, using bytewise UTF-8 field sort, explicit primitive tags, uint32 big-endian framing, exact widths/IEEE bits, and consumer package/type identity. UUID v5 uses a fixed namespace and separate event/channel domain prefixes.
- **Record serializer/schema validator:** Encodes versioned strict JSON records as NDJSON and validates each record against the schema matching its `schema_version`. Standalone schemas are canonical; runtime embeds those files using `go:embed`.
- **Append transaction writer:** Coordinates concurrent appends, sequence assignment, transaction records and commit marker. A new channel is included in the same transaction as its first event. Writes are recoverable as all-or-nothing at transaction boundaries.
- **Segment manager:** Writes authoritative segment files, rotates at transaction boundaries, and maintains ordered global record sequence. A transaction exceeding target segment size may be placed alone in an oversized segment.
- **Recovery/index manager:** On Open, validates committed transactions and record sequences, rolls back incomplete trailing transactions, validates derived index, and rebuilds it from segment source data when missing or inconsistent. Index replacement is atomic.
- **Reader/tailer:** Streams committed historical occurrences for exactly one consumer-provided Channel value, catches up with concurrent appends, and transitions to live tailing without gaps or duplicates. Global sequence/cursor/offset management is internal; consumers do not provide or receive storage cursors. Cancellation and errors are explicit. Reads are generic over the consumer's concrete Event struct type; eventlog allocates and reconstructs that type internally without a registration/factory/prototype API. A channel may contain multiple concrete Event types; a `Reader[T]` returns matching occurrences in order and advances past other event types, which are filtered rather than treated as errors.
- **Compactor:** Produces temporary replacement segments, validates/checksums and syncs them, renames to final segments, atomically replaces derived index, and removes old segments only after replacement is finalized and validated.

## Record model and identity

The persisted envelope has three kinds: `channel`, `event`, and `commit`. Each record has a `schema_version: 1` and a global sequence number; there is no log header.

- **Channel identity:** channel type plus all exposed channel fields.
- **Logical event identity:** event type plus exposed event fields; channel is excluded from identity.
- **Occurrence:** every Append produces a new event occurrence associated with one channel. Logical event IDs are global; associations/history are retained across repeated appends. Latest occurrence defines HEAD/current channel.
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

- Exact public API signatures and stable error taxonomy.
- Whether a public Occurrence wrapper is needed; eventlog owns concrete Event reconstruction.
- Reader cancellation, close, and resource-lifecycle semantics.
- Benchmarks and memory/performance targets.


## Current public reader API direction — channel-scoped

Every reader targets exactly one consumer-provided `Channel` value; no all-channel reader is exposed. Consumers do not pass channel UUIDs or global sequence cursors. The library owns channel identity resolution, sequence/cursor tracking, and offsets.

The intended flow is a generic typed read, e.g. `Read[MyEvent](ctx, channel)`, followed by ordered `Next()` calls returning matching `MyEvent` occurrences (or a documented occurrence wrapper containing one), with replay-to-live handoff handled internally. The consumer supplies only the concrete generic type; eventlog performs allocation, decoding, field assignment, and persisted-type verification. A channel can contain other concrete event types; those are filtered out as the reader advances. No registration, prototype, Channel factory, or custom decoder is exposed. Exact generic constraints, pointer/value convention, occurrence wrapper, reader close behavior, and detailed cancellation/error semantics remain open.


## Define decision — storage layout, manifest, index, recovery, and compaction (approved)

### Storage layout and segment lifecycle

The storage directory contains a manifest, a rebuildable index, and a segments directory. Segment IDs are monotonically increasing integers and are never reused, including after compaction; they are not record sequence numbers. Finalized segment files are immutable. Only the active segment may be written. Record sequence numbers remain global across all segments.

```text
eventlog/
├── manifest.json
├── index.json
└── segments/
    ├── segment-000001.ndjson
    ├── segment-000002.ndjson
    └── ...
```
### Manifest

A minimal versioned manifest identifies the active finalized segment set and the next segment ID. It is authoritative for the active generation and contains no event payloads. Example shape:

```json
{
  "format_version": 1,
  "generation": 3,
  "segments": ["segment-000004.ndjson", "segment-000005.ndjson"],
  "next_segment_id": 6
}
```
Manifest replacement is the atomic generation-switch/compaction commit point: write temporary manifest, validate, sync, then atomically replace it. Old segments are retained until the new manifest is finalized. Unreferenced segment files are recovery candidates for safe cleanup, not automatically authoritative. If manifest corruption leaves multiple plausible complete generations, Open must fail with a recovery/corruption error rather than guessing.

### Rebuildable index

The index is derived metadata and is bound to the manifest generation. It includes event IDs, channel associations, latest occurrence/HEAD, global sequence, segment ID, and byte offsets. If absent, inconsistent, or stale for the active generation, Open rebuilds it from validated authoritative segments and atomically replaces it. Index metadata never overrides segment contents.

### Open recovery order

1. Acquire exclusive single-process ownership.
2. Validate the manifest and identify its active segment generation.
3. Validate active segment files, record schemas, and sequence continuity.
4. Validate transaction boundaries, commit markers, counts, and checksums.
5. Roll back an incomplete trailing transaction by truncating from its transaction start.
6. Validate or rebuild the generation-bound index and atomically replace it if needed.
7. Clean orphan/temp artifacts only after the active finalized source segments validate and cleanup is safe.
8. Open for appends and mandatory channel-scoped readers.

If the manifest is corrupt or missing in an existing storage directory, recovery may proceed only if exactly one complete generation can be determined unambiguously; otherwise report corruption/recovery failure. Do not select the newest-looking files by filename alone.

### Reader-safe compaction

Each reader pins the generation it started against. Compaction constructs replacement segments and index without modifying files used by current readers, validates and syncs them, then atomically switches the manifest. New readers use the new generation; existing readers continue using their pinned generation. Old generation files are deleted only after the last reader releases its pin. This policy applies equally on Linux and Windows, avoiding platform-specific reader semantics.
