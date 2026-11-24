# Architecture

**Status:** Define-phase design baseline (Cycle 1). This architecture reflects approved decisions; unresolved details are called out explicitly. It describes the intended design, not implemented or verified behavior.

## System boundary

eventlog is an embedded Go library using local filesystem storage. A single process owns a log. It supports concurrent goroutine use but does not coordinate multiple processes or distributed writers. Consumer event/channel semantics remain outside the library; the consumer explicitly selects the persisted fields.

## Logical components

- **Public API:** The only approved public entry point is package-level generic `PrepareLog[TEvent, TChannel](ctx)`, returning `(TEvent, TChannel, error)`. Consumers receive event and channel values, not a Log handle. The package owns the internal `Log` instance and all append/read/next/lifecycle operations; none of those operations are exposed to consumers at this stage. Consumer event and channel types implement `Fields() Fields`; consumers set or change their own field values. A single process owns a log at a time, with concurrent goroutine use coordinated internally.
- **Field builder and validator:** Append's selector callback supplies typed field selectors; those selectors resolve direct fields on a zero-valued consumer probe. Field constructors support nullable/non-nullable primitives. Append-time validation rejects invalid field names, duplicate names, unsupported types, invalid bounds and non-finite floats before writing.
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
- **Logical Event identity:** global and independent of channel. Derive the UUID v5 from the concrete Go Event type name, exposed `Fields()` field-name schema, and consumer-designated identity field name/value, canonically encoded. Non-identity field changes retain the logical ID but create new occurrences.
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


## Current public reader API direction — channel-scoped

Every reader targets exactly one consumer-provided `Channel` value; no all-channel reader is exposed. Consumers do not pass channel UUIDs or global sequence cursors. The library owns channel identity resolution, sequence/cursor tracking, and offsets.

The intended public flow is `log.Append[TChannel,TEvent](selector)` and `log.Read[TChannel,TEvent]()` followed by `reader.Next()`, with replay-to-live handoff handled internally. The Append selector callback explicitly identifies the fields to persist. The reader reconstructs the requested concrete Event type internally and filters other event types in the same channel. The consumer supplies only the concrete generic type; eventlog performs allocation, decoding, field assignment, and persisted-type verification. A channel can contain other concrete event types; those are filtered out as the reader advances. No registration, prototype, Channel factory, or custom decoder is exposed. Exact generic constraints, pointer/value convention, occurrence wrapper, reader close behavior, and detailed cancellation/error semantics remain open.


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

## Define decision — serialized one-time configuration and shared initialization (2026-11-24)

Package configuration and first Log initialization are coordinated as one atomic state machine. A successful `Config(...)` publishes a validated immutable configuration. The first `PrepareLog` transitions the runtime into initialization; concurrent `PrepareLog` callers wait for that same attempt rather than opening storage independently or receiving an initialization-in-progress error. Each waiter observes its own context cancellation/deadline while waiting. Successful initialization publishes the single internal Log as ready. Failed initialization leaves the immutable configuration intact and permits a later `PrepareLog` attempt to retry. A `Config` attempt after initialization has begun is rejected and cannot replace the configuration. The synchronization mechanism must prevent races between Config, initialization, and concurrent callers.

## Define decision — storage path resolution and capture (2026-11-24)

The storage-directory option accepts a non-empty path, including a relative path. A successful `Config(...)` resolves the path against the process working directory and stores the resulting absolute path in the immutable runtime configuration. All subsequent initialization uses that captured absolute path, so a later working-directory change cannot alter the storage target. Empty or whitespace-only paths are rejected. Exact path cleaning, symlink behavior, filesystem existence/permission validation, and whether validation may inspect the filesystem remain open. Path resolution itself does not create or initialize storage; storage initialization remains part of `PrepareLog`.

## Define decision — Config option application and validation (2026-11-24)

The public configuration shape is `Config(options ...Option) error`, with exported option constructors and an unexported option application method so consumers cannot implement arbitrary mutations of internal configuration. `WithStorageDirectory(path)` is the required storage setting. Config accepts multiple distinct options, rejects nil options, rejects duplicate settings rather than applying last-option-wins, and fails if the storage-directory option is absent. Validation is completed before immutable configuration is published; a failed attempt leaves the runtime unconfigured and retryable.

## Define decision — storage directory validation (approved 2026-11-24)

`Config` rejects empty or whitespace-only storage paths. It accepts relative paths, resolves them to an absolute path at configuration time, lexically cleans the absolute result, and freezes that cleaned absolute path in immutable runtime configuration. Cleaning is equivalent to `filepath.Clean`; it removes redundant path elements but does not resolve symlinks. Config does not require the directory to exist and does not create it. During `PrepareLog`, initialization creates the directory when absent. If the target exists but is not a directory, or creation fails, initialization returns an error. These rules keep path selection stable across later working-directory changes while leaving actual storage initialization to PrepareLog.


## Define decision — exported configuration errors (approved 2026-11-24)

The package exports `ErrConfigNotInitialized` and `ErrConfigAlreadySet` sentinel errors. `PrepareLog` before successful configuration returns an error matching the former. Repeated or late `Config` calls return an error matching the latter. Invalid options produce descriptive validation errors. Filesystem failures wrap underlying causes for `errors.Is`/`errors.As`; context cancellation and deadline errors remain discoverable. Validation failure does not publish configuration and remains retryable.


## Define decision — internal polling cadence and scheduler (approved 2026-11-24)

The internal change manager uses a 100 ms default polling interval. A single managed scheduler per prepared Log/change manager scans all selected event and channel field bindings; it does not spawn a goroutine per field. The cadence may be adjusted through internal implementation/test configuration only, not through a new public option or method. Each scan compares the selected-field state with the last observed/persisted snapshots and routes detected changes through internal persistence. Polling does not make unsynchronized writes to consumer-owned Go struct fields race-safe. Caller synchronization and snapshot consistency, event/channel change semantics, scheduler termination/retry, and internal persistence-error handling remain unresolved and must be specified before implementation.


## Define decision — event-field mutation creates a new occurrence (approved 2026-11-24)

When polling observes a changed selected Event field snapshot, the change manager persists a new physical event occurrence. Persisted history is append-only: the prior occurrence is immutable and retained; no in-place rewrite is permitted. This decision does not resolve whether a changed field snapshot maps to the same logical event ID or a new ID. The current identity rule derives event ID from event type plus exposed fields, so the interaction between changed field values, logical identity, HEAD promotion, and ordering must be explicitly reconciled in Define before implementation.


## Define decision — stable Event ID from initial snapshot (superseded 2026-11-24)

This provisional rule was incorrect: Event identity is not based on an initial full-field snapshot or on retaining an ID for a prepared struct instance. The consumer-designated identity field defines the logical entity identity, together with the Event type and exposed field-name schema. Non-identity data changes create new occurrences under the same logical ID.


## Define decision — identity and occurrence are distinct (approved 2026-11-24)

Logical Event ID identifies the entity, derived from the concrete Go Event type name, the exposed `Fields()` field-name schema, and the consumer-designated identity field name/value. Canonically encode these inputs and derive deterministic UUID v5. A change in non-identity data (e.g., Order 501 status from created to paid) keeps the same logical Event ID but constitutes a new event occurrence. Each new occurrence is persisted immutably; previous occurrences remain in history and the newest occurrence becomes HEAD/current state. Physical Event/Channel record data must be deduplicated so identical physical data is stored once and reused.


## Define decision — internal-only synchronization (approved 2026-11-24)

- Preserve the existing public API; do not introduce any lock, mutex, synchronization interface, or configuration option for consumer-field mutation.
- eventlog's synchronization remains internal and covers its own scheduler, change detection, persistence, and lifecycle coordination.
- Consumers are responsible for synchronizing their own mutations of returned Event and Channel fields when concurrent access occurs. The 100 ms polling mechanism does not make unsynchronized ordinary Go struct field writes race-safe and must not be documented as doing so.
- This decision rejects the proposed consumer-shared-lock configuration option and introduces no public API.

## Define decision — canonical Channel identity and mutation (approved 2026-11-24)

Channel logical identity follows the same canonical determination as Event identity: concrete Go type, exposed `Fields()` schema, consumer-designated identity field name/value, canonical encoding, and deterministic UUID v5. Non-identity field changes do not alter logical Channel identity, but update the data snapshot. A Channel-only mutation does not create an Event occurrence; a detected Event change creates a new occurrence using the latest Channel snapshot. Prior occurrences keep their original Channel association. Canonically identical Channel records are deduplicated and created/reused atomically with the first successful occurrence referencing them.

## Define topic — consumer mutation of designated identity fields (principle approved; mechanics open)

When consumers mutate the designated identity field on Event or Channel instances returned from `PrepareLog`, eventlog must detect and manage the identity transition internally, without requiring a public coordination API. Define the exact detection, snapshot, occurrence, HEAD/index, Channel-association, and persistence behavior before implementation.

## Define decision — Channel notification direction (approved 2026-11-24)

Channel's public capability will include a notification Go channel, exposed by an accessor method (exact signature TBD; Go interfaces cannot declare fields). Occurrence notifications carry the actual concrete consumer Event struct instance rather than identifier-only metadata or a generic payload intended to represent many event types. Notification errors are typed and support `errors.Is`/`errors.As`. Slow or unread notifications must not block persistence or the polling scheduler; buffering/delivery details remain open. The notification channel closes on associated lifecycle shutdown, including failed initialization if allocated. Released-API backward compatibility is not a constraint during this early design phase. Identity-violation response remains open.


## Define decision — notification payload snapshots approved (2026-11-24)

- Approved Option A: successful occurrence notifications carry a snapshot of the concrete consumer Event instance as captured at the persistence boundary, not a live mutable pointer.
- The notification is emitted only after the occurrence has been successfully persisted.
- This establishes payload immutability/history correspondence; delivery buffering, overflow/backpressure policy, exact accessor/message signatures, and typed persistence-error notification behavior remain open Define decisions.
- No implementation/build/tests have started; implementation still requires Define completion and explicit approval.


## Define decision — notification overflow policy approved (2026-11-24)

Notification delivery is bounded and non-blocking. When the buffer is full, the notification is dropped rather than blocking persistence or the internal scheduler. Persisted occurrences remain recoverable through internal replay, and notification loss must be detectable rather than silent. Buffer capacity, the loss-detection mechanism, replay contract/API, and exact delivery details remain open Define decisions. No implementation/build/tests have started; implementation still requires Define completion and explicit approval.
