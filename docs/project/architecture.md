# Architecture

**Status:** Define baseline (Cycle 1). This is a design specification, not implementation evidence.

## Boundary and public API
Embedded Go library using local filesystem. One process owns storage; concurrent goroutines are coordinated internally. Distributed and multi-process writers are out of scope.

Only approved public preparation call:
```go
event, channel, err := eventlog.PrepareLog[MyEvent, MyChannel](ctx)
```
Consumer Event/Channel values are returned. Internal Log owns append/read, persistence, recovery, polling and lifecycle; no Log handle or consumer-facing operational API is approved. Exact Go-valid interface and selector signatures remain open.

Configuration is mandatory and immutable through one-time `Config(options ...Option) error`, requiring `WithStorageDirectory(path)`. Nil/duplicate/missing/invalid options fail. Relative paths resolve and lexically clean to absolute; symlinks are not resolved. Config does not create storage; PrepareLog creates it. `ErrConfigNotInitialized` and `ErrConfigAlreadySet` are approved. Concurrent preparation shares initialization; each waiter can cancel; failed initialization may retry with frozen configuration.

## Components
- **Fields/validation:** typed primitive selectors; nullable pointers; direct uniquely resolved fields; reject invalid names/types/ranges before persistence.
- **Identity:** canonical encoding (UTF-8 field-name sort, type tags, uint32 big-endian framing, exact widths/bits) and UUID v5 with Event/Channel domain prefixes. Event identity is global: type + exposed schema + designated identity name/value. Non-identity changes append immutable occurrences; latest promotes HEAD. Channel identity follows canonical type/schema/identity. Deduplicate identical physical records.
- **Serializer:** strict versioned NDJSON channel/event/commit records; JSON Schema v1 canonical and embedded. Integers are decimal strings; floats are IEEE bit-pattern hex; null type-only.
- **Transaction writer:** new Channel record when needed, Event occurrence, then commit. Global sequence includes marker; transaction ID is first record sequence. Count/checksum exclude marker; SHA-256 covers exact transaction bytes including newlines. Commit last. Recoverable atomicity differs from durability.
- **Storage:** manifest chooses active generation; finalized segments immutable; active segment writable. Segment IDs monotonic/non-reused; sequence global. Rotate only at transaction boundaries.
- **Recovery/index:** segments authoritative. Validate manifest, schemas, sequence and commit integrity; incomplete trailing transaction rolls back; complete malformed record fails. Index is generation-bound, derived and rebuildable. Ambiguous manifest recovery fails closed.
- **Reader:** channel-scoped committed replay transitions gap-free into live tailing, ordered and duplicate-free. Cursors internal. Typed reconstruction filters other event types. Cancellation/errors explicit.
- **Change manager/notifications:** one 100 ms internal scheduler scans selected snapshots. Consumer synchronizes own mutations. Persisted occurrence notifications carry immutable Event snapshots. Guaranteed eventual delivery, no persistence/scheduler blocking, recovered backlog first, orderly shutdown drains, crash replay, typed errors after persistent failure.

## Durability and compaction
Sync every 1 second or 1 MiB pending writes, whichever first. Append before sync is not power-loss durable; Flush/Close are sync boundaries. Index includes event/channel association, HEAD, sequence, segment and offsets.

Compaction builds, validates and syncs replacement segments, finalizes, atomically switches manifest/index, then retains old generation until pinned readers release. Temporary cleanup follows validation of finalized source segments.

## Platforms and dependencies
Go standard library and local filesystem. Linux and Windows equal priority; macOS excluded.

## Open architectural details
- Exact public signatures, generic constraints, selectors and reconstruction pointer/value convention.
- Prepared-value association, reader cancellation/close and resource lifecycle.
- Identity-field mutation/integrity response; Channel mutation semantics.
- Notification API/types, backlog durability/replay boundary, retry termination, ordering and shutdown drain failures.
- Snapshot consistency, consumer synchronization details.
- Benchmarks, memory/performance targets and measurable acceptance criteria.
