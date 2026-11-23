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
- Reading uses a global sequence cursor, supports an optional channel filter and context cancellation, and reports errors explicitly.
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
- Exact public signatures, reader interface shape, cursor/filter/context/close behavior, and stable error categories.
- Segment filenames, manifest requirements/format, index schema, and Open recovery/rebuild ordering.
- Reader-safe compaction and concurrent-reader resource lifecycle.
- Benchmark scenarios and measurable performance/memory acceptance targets.
- Detailed acceptance test matrix.
