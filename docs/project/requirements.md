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
- The only approved public API is `eventlog.PrepareLog[TEvent, TChannel](ctx) (TEvent, TChannel, error)`. The internal Log owns append and read operations; consumers receive only event and channel values. No consumer-facing Append, Read, Next, or Log handle is exposed at this stage.
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
- Read/replay and live tailing are internal responsibilities of the Log instance. Consumers do not obtain readers or call Next. The internal reader remains channel-scoped, reconstructs typed consumer events, and handles nonmatching event types internally.
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
- Internal lifecycle algorithms and the exact way PrepareLog associates returned event/channel values with the internally owned Log.
- Field validation details, stable error categories, and detailed context/cancellation/resource semantics.
- Internal typed-reader filtering and pointer/value reconstruction conventions.
- Benchmark scenarios, performance/memory acceptance targets, and the detailed acceptance test matrix.


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


## Define decision — single public preparation API (approved 2026-11-24)

The only public API currently approved is:

```go
event, channel, err := eventlog.PrepareLog[MyEvent, MyChannel](ctx)
```

Declaration:

```go
func PrepareLog[TEvent EventType, TChannel ChannelType](ctx context.Context) (TEvent, TChannel, error)
```

The public constraints remain:

```go
type ChannelType interface { Fields() Fields }
type EventType interface { Fields() Fields }
```

The returned event and channel values are consumer-owned data values whose exposed fields consumers may set or change. The package owns the internal `Log` struct and all append, read, next, synchronization, recovery, and lifecycle machinery. Consumers do not receive a Log handle and do not call lifecycle methods. No other public append/read/next/close/flush/compact API is approved at this stage. A single process owns a log at a time; multiple goroutines may use the API concurrently. This replaces earlier generic Log-handle and consumer-facing Append/Read proposals.


## Define decision — separate runtime configuration from generic type selection (2026-11-24)

The generic arguments in `PrepareLog[TEvent, TChannel](ctx)` identify the consumer-defined event and channel types. They do not supply runtime configuration such as the storage directory. Configuration must be provided separately through a package configuration facility (a public `eventlog.Config(...)`-style API is acceptable); do not infer a storage path from the generic types and do not require environment variables as the configuration mechanism. Keep the approved `PrepareLog[TEvent, TChannel](ctx)` call shape unchanged. A Config call may explicitly bind configuration to the context while preserving its cancellation/deadline semantics; configuration must not be inferred from arbitrary context values. The exact Config signature, whether it produces a context-bound configuration or another explicit configuration value, storage-directory option name/default, and behavior when configuration is absent remain open for a concrete API example and confirmation.


## Latest Define decision — one-time immutable runtime configuration (2026-11-24)

Configuration is process/runtime initialization state, not mutable runtime state and not context-bound state. A package-level `eventlog.Config(...)`-style call configures the runtime exactly once, before the first `PrepareLog` initialization. The configuration is validated, captured, and frozen; all later operations use that same immutable configuration. A second configuration attempt, or an attempt after runtime initialization has begun, must return an error and must never replace or mutate the active configuration. Consumers cannot change settings midway through execution by supplying another context or calling Config again.

The approved preparation call remains unchanged:

```go
if err := eventlog.Config(
    eventlog.WithStorageDirectory("./data/events"),
); err != nil {
    return err
}

event, channel, err := eventlog.PrepareLog[MyEvent, MyChannel](ctx)
```

Generic arguments select consumer Event and Channel types only. `context.Context` continues to provide cancellation/deadline semantics; it does not carry configuration. Environment variables are not required. The exact option names/defaults, behavior if Config is omitted, and synchronization/error details for concurrent Config versus first PrepareLog remain open for final API specification. No implementation, build, or Go tests have started.


## Latest Define decision — configuration is mandatory (2026-11-24)

`Config(...)` is mandatory. If no successful `Config(...)` call has completed, `PrepareLog[TEvent, TChannel](ctx)` must fail with an explicit configuration-not-initialized error. It must not select a default storage directory, infer configuration from the environment, or silently initialize with implicit settings. A failed Config call does not count as successful initialization; only a successful one-time Config establishes the immutable runtime configuration. This decision resolves the previously open behavior when Config is omitted.

## Latest Define decision — concurrent preparation waits for initialization (2026-11-24)

Concurrent calls to `PrepareLog[TEvent, TChannel](ctx)` during the first storage initialization wait for that single initialization attempt to finish; they do not fail merely because initialization is in progress and must not initialize storage independently. Each waiting caller may stop waiting when its own context is cancelled or its deadline expires. If the initialization attempt fails, the immutable successful configuration remains set and a later `PrepareLog` call may retry initialization. Configuration remains one-time and cannot be changed after initialization has begun.

## Latest Define decision — absolute storage path captured at configuration time (2026-11-24)

`WithStorageDirectory(path)` accepts a non-empty path. During a successful `Config(...)` call, eventlog resolves that path against the process working directory and captures the resulting absolute path in the immutable runtime configuration. This means a relative input such as `./data/events` is accepted, but the resolved absolute path—not the original relative string—is retained and used for the process lifetime. A later working-directory change cannot redirect storage. An empty or whitespace-only path is invalid. The exact path cleaning, symlink, filesystem existence, and permission-validation rules remain open; `Config` must not silently create or initialize the log storage as part of path resolution.

## Latest Define decision — Config option validation (2026-11-24)

`Config(options ...Option)` accepts multiple distinct options. The storage-directory option is mandatory; calling `Config()` without it fails. A nil option fails. Supplying the same setting more than once fails; options never use last-option-wins behavior. A failed Config validation leaves runtime configuration uninitialized and may be retried. The package controls option implementations through an unexported application method; consumers configure settings using exported option constructors such as `WithStorageDirectory(path)`.
