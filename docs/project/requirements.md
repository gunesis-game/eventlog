# Project requirements

**Status:** Define-phase draft of confirmed requirements (Cycle 1). This document captures approved durable product requirements; unresolved design details are listed at the end and must not be inferred.

## Purpose

eventlog is a lightweight, embedded, disk-backed append-only event log for general-purpose event-oriented applications. It provides persistent event occurrences, ordered history replay, and a unified transition from replay into live tailing. It is not tied to an event-sourcing framework or a specific domain.

## Functional requirements

### Ownership and concurrency
- A single process owns a log at a time; multi-process and distributed writer coordination are out of scope.
- Multiple goroutines may concurrently use the API. The library coordinates writes and preserves ordering/transaction integrity.

### Event and channel model
- Consumers provide event and channel types implementing library-owned interfaces that expose `Fields() Fields`.
- Only explicitly returned fields are persisted. The library must not inspect or persist the consumer's full struct, and no automatic redaction layer is required.
- Every event occurrence is associated with exactly one channel.
- Channel identity is based on its consumer type and exposed fields. Channels are created implicitly and atomically with the first successful append that uses them. A channel without an event must not be persisted; no standalone channel-creation API is provided.
- Logical event identity is global and based on event type and exposed fields, not channel. Repeated appends create distinct occurrences and retain associations/history; the latest occurrence determines the event's current channel/HEAD and ordering.

### Fields and validation
- Fields are heterogeneous, typed primitive values.
- Supported primitive kinds: string, bool, all signed and unsigned integer widths (including byte/rune aliases), float32, float64, and named types over supported primitives. uintptr and complex types are excluded.
- Nullable primitive pointers are supported. Non-finite floats are rejected. Integer values must fit the declared Go primitive type exactly.
- Empty and duplicate field names are invalid. Names are case-sensitive and are not trimmed. Canonical field ordering is bytewise UTF-8.
- Field construction uses typed selectors (`FieldOf` for non-nullable and `FieldOfNullable` for nullable). Selectors must identify direct primitive fields; nested structs are not selectable. Reflection-based field-name resolution must be unique. Validation occurs during append and fails atomically before persistence.

### Append and transaction guarantees
- Public API direction: `Append(channel, event) error`; exact signatures and error types remain open.
- An append validates channel and event before writing. If the channel is new, channel and event records are one transaction; otherwise the transaction contains the event occurrence. A commit marker is written last.
- Transaction records are not exposed until their commit marker is validated. Recovery discards incomplete trailing transactions by truncating back to their start.
- Every persisted record has a global sequence, including commit markers. Transaction ID is the sequence of its first transaction record. Transaction record count excludes the commit marker.
- Commit checksum is SHA-256 over exact serialized transaction record bytes, including each record's terminating newline; commit marker bytes and its newline are excluded.
- Recoverable transaction atomicity is not the same as durability against power loss: Append may be buffered until the sync policy runs.

### Persistence and recovery
- v1 persistence uses built-in JSON in newline-delimited JSON (NDJSON). New logs start as zero-byte files; no header record is written.
- Every channel, event, and commit record carries `schema_version: 1`. Unsupported schema versions are rejected.
- Versioned JSON Schema Draft 2020-12 documents the record/value format. Schema files are standalone and canonical and embedded at build time via `go:embed`.
- Persisted field values use explicit typed JSON representations: strings and bools are native JSON values, integers are decimal strings, float32/float64 values are raw IEEE bit-pattern hex strings, and null is type-only. Float hex preserves width and signed zero; non-finite values are rejected.
- A complete newline-terminated invalid record is corruption; the log must not silently skip it. Incomplete trailing transaction data is rolled back to transaction start. `Open()` creates a missing zero-byte log and recovers existing storage.

### Reading
- One unified reader supports replay and live tailing. It delivers committed history in order, catches concurrent appends during replay, and transitions to live tailing without gaps or duplicates.
- Every reader is scoped to exactly one consumer-provided `Channel` value; there is no all-channel reader.
- Global sequence, cursor, segment offset, and tailing coordination are internal; consumers neither provide nor receive storage cursors or sequence numbers.
- Context cancellation and explicit errors are required.
- Reads are typed by a consumer-supplied generic concrete Event struct type. eventlog owns allocation, decoding, and field reconstruction; the consumer does not register types, supply prototypes, implement factories, or provide decoders. A channel may contain multiple concrete Event struct types. `Read[T]` returns only occurrences whose persisted concrete type matches T, in channel order, while advancing past nonmatching event types; nonmatching types are not errors. Malformed records and failures decoding a matching T record are explicit errors.
- Re-appended logical events are delivered as separate occurrences.

### Durability and maintenance
- Automatic synchronization occurs every 1 second or 1 MiB of pending writes, whichever threshold is reached first.
- Explicit `Flush()` is a durability boundary; `Close()` flushes and syncs.
- Segment files are the source of truth; index metadata is derived and rebuildable.
- Segment rotation occurs only at transaction boundaries against a fixed target size. An oversized transaction may occupy its own segment.
- Compaction creates and validates replacement segments, syncs them, finalizes them, atomically replaces the index, and only then removes old segments. Recovery may clean temporary compaction files only after finalized source segments validate.

## Constraints and non-goals
- Standard-library-only Go; local filesystem storage.
- Supported platforms: Linux and Windows at equal priority. macOS is excluded. Core features must not be platform-exclusive.
- No distributed coordination, multi-process writers, broker/database functionality, custom codecs, or domain-specific behavior.
- Correctness, recovery, predictable memory use, and cross-platform parity are prioritized. Numeric performance targets await benchmarks.

## Open details (not approved yet)
- Exact public signatures, reader interface shape, stable error categories, and detailed context/cancellation/close semantics.
- Whether a public Occurrence wrapper is needed; eventlog owns the concrete Event reconstruction mechanism.
- Benchmark scenarios and measurable performance/memory acceptance targets.
- Detailed acceptance test matrix.


## Current reader decision — channel value and internal cursor

Every reader is scoped to exactly one consumer-provided `Channel` value; there is no all-channel reader. The consumer does not supply a channel UUID or global sequence cursor. The library resolves channel identity and manages sequence/cursor/offset state internally. The unified reader starts with committed history and transitions gap-free into live tailing. A channel may contain multiple concrete Event types; `Read[T]` filters to T while scanning past nonmatching event types. eventlog allocates and reconstructs matching values internally. Exact public signatures, pointer/value convention, occurrence shape, and detailed cancellation/error semantics remain open.


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
