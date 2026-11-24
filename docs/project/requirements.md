# Project requirements

**Status:** Define draft (Cycle 1). These requirements describe approved intent, not implemented behavior.

## Purpose
eventlog is an embedded Go library for append-only, disk-backed event occurrence history, ordered replay and transition into live tailing.

## Ownership and concurrency
One process owns storage. Concurrent goroutines are supported with internal coordination. Multi-process/distributed writers are out of scope. Consumers must synchronize their own concurrent Event/Channel field mutations; polling does not make unsynchronized writes race-safe.

## Event, Channel and Fields
Consumer types implement library-owned `Fields()`. Only explicitly selected fields persist. Supported primitives are string, bool, signed/unsigned integer widths (including byte/rune aliases), float32/64 and named primitive types; uintptr and complex are excluded. Nullable primitive pointers are supported. Invalid names, duplicates, unsupported/nested fields, out-of-range integers and non-finite floats fail validation before writing. Names are case-sensitive, not trimmed; canonical order is bytewise UTF-8. Selectors use `FieldOf`/`FieldOfNullable` and must resolve uniquely to direct fields.

Channel identity is canonical from type, exposed field schema and designated identity field name/value, with UUID v5. Channels are created atomically with their first committed Event; channel-only records are not allowed. Channel-only changes do not create Event occurrences; subsequent occurrences use the latest Channel snapshot while prior associations remain fixed. Identical physical records are deduplicated.

Logical Event identity is global and independent of channel. It uses concrete Event type, exposed field-name schema and consumer-designated identity field name/value, canonical encoding and UUID v5. Non-identity changes create immutable new occurrences under the same logical ID; history is retained and latest occurrence determines HEAD. Identity-field mutation is an integrity case; exact response is open.

## Public API and configuration
Only approved preparation call:

```go
event, channel, err := eventlog.PrepareLog[MyEvent, MyChannel](ctx)
```

The internal Log owns append/read/recovery/lifecycle; consumers receive values, not a Log handle or operational methods. Exact Go-valid signatures, selectors, constraints and reconstruction conventions remain open.

Mandatory one-time `Config(options ...Option) error` requires `WithStorageDirectory(path)`. Nil, duplicate, missing and invalid options fail; failed validation remains retryable. Successful config is immutable; repeated/late calls match `ErrConfigAlreadySet`. PrepareLog before Config matches `ErrConfigNotInitialized`. Relative paths resolve and lexically clean to absolute during Config; symlinks are not resolved. Config does not create storage; PrepareLog creates it and rejects non-directory targets/creation errors. Concurrent PrepareLog shares initialization; each waiter honors context cancellation. Failed initialization can be retried using frozen config.

## Persistence and typed values
Strict versioned NDJSON channel/event/commit records (schema version 1); initial log is zero bytes without header. JSON Schema Draft 2020-12 is canonical and embedded via `go:embed`. Strings/bools are native JSON, integers decimal strings, floats IEEE bit-pattern hex (float32 8 chars, float64 16), null type-only. Global sequence includes commit markers; transaction ID is the first transaction record sequence. Count/checksum exclude commit marker. SHA-256 covers exact transaction record bytes including newline; commit is written last. Complete malformed records and unsupported schemas fail; incomplete trailing transaction truncates to its start.

## Durability, storage and recovery
Automatic sync every 1 second or 1 MiB pending writes. Append before sync is not guaranteed power-loss durable; Flush/Close are sync boundaries.

Manifest selects active segment generation. Segment IDs increase and are not reused; finalized segments are immutable. Rotate only between transactions; oversized transaction may occupy a segment alone. Index is derived, generation-bound and rebuildable, storing IDs, associations, HEAD, sequence, segment and offsets. Open validates manifest, segments, records, sequences and transactions; rolls back incomplete tail; rebuilds invalid index. Ambiguous recovery fails rather than guessing. Compaction validates/syncs replacements, atomically switches manifest/index, and retains old files while readers pin their generation.

## Reading and notifications
Reader is channel-scoped: committed history transitions gap-free and duplicate-free to live tailing. Cursor/sequence/offset remain internal. Typed reconstruction filters nonmatching event types while advancing; corruption and matching-type decode failures are explicit errors. Context cancellation is required.

Channel notification accessor is approved but its signature is open. Success notifications carry immutable concrete Event snapshots, only after persistence. Every persisted occurrence must eventually be visible; buffer capacity cannot drop data. Persistence/scheduler must not block on slow consumers. Recovered backlog is delivered first in persistence order. Orderly shutdown drains before closing; crash recovery resumes pending delivery. Temporary persistence errors retry with backoff; no success notification before commit; persistent failure is surfaced as a typed error and occurrences are never silently abandoned. Retry termination, error ordering and drain failure details remain open.

## Platforms and non-goals
Standard library only, local filesystem; Linux and Windows equal priority, macOS excluded. No distributed coordination, broker/database, custom codecs or domain semantics. Correctness, recovery, bounded memory and parity precede numeric performance targets.

## Open details
- Final Go-valid API, constraints/selectors, pointer/value reconstruction and occurrence shape.
- Internal lifecycle/value association, reader cancellation/resource cleanup, terminal errors.
- Identity-field mutation response and precise Channel change behavior.
- Notification accessor/error shape, backlog durability/replay boundary, retry termination, ordering and shutdown failure.
- Snapshot consistency and consumer synchronization contract.
- Benchmark and acceptance matrix, memory/performance targets.
