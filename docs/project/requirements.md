# Project requirements

**Status:** Define-phase requirements (Cycle 1). This active document is the curated source of current
requirements. Later approved decisions are integrated below; archived checkpoints are provenance only.
Intended behavior is not implementation evidence.

## Purpose

eventlog is a lightweight, embedded, disk-backed append-only event log for general-purpose event-oriented
applications. It provides persistent event occurrences, ordered history replay, and a unified transition
from replay into live tailing. It is not tied to an event-sourcing framework or a specific domain.

## Functional requirements

### Ownership and concurrency
- A single process owns a log at a time; multi-process and distributed writer coordination are out of
  scope.
- Multiple goroutines may concurrently use the API. The library coordinates writes and preserves
  ordering/transaction integrity.

### Event and channel model
- Consumers provide event and channel types implementing library-owned interfaces that expose `Fields()
  Fields`.
- Only explicitly returned fields are persisted. The library must not inspect or persist the consumer's
  full struct, and no automatic redaction layer is required.
- Every event occurrence is associated with exactly one channel.
- Channel logical identity is derived canonically from concrete Channel type, exposed field-name schema,
  and consumer-designated identity field name/value using deterministic UUID v5. Channels are created
  implicitly and atomically with the first successful append that uses them. A channel without an event
  must not be persisted; no standalone channel-creation API is provided. Non-identity Channel changes
  update its snapshot without changing identity; Channel-only changes do not create Event occurrences.
  New Event occurrences use the latest Channel snapshot, and earlier occurrences keep their original
  association.
- Logical Event identity is global and independent of channel. Derive it from the concrete Go Event type
  name, all field names explicitly exposed through `Fields()`, and the consumer-designated identity field
  name/value. The selected identity field **must contain a UUID**; arbitrary primitive, string, numeric,  nullable, or non-UUID identity values are forbidden. Canonically encode these identity inputs and derive
  a deterministic UUID v5. Changes to
  non-identity fields do not change the logical Event ID, but each detected data change is a new
  immutable occurrence. Repeated occurrences retain history and channel associations; the latest
  occurrence determines HEAD/current state.

### Runtime configuration

Configuration is mandatory, separate from generic type selection and Context. The public shape is
`Config(options ...Option) error`, requiring `WithStorageDirectory(path)`. Config validates the complete
option set before publishing immutable runtime state: nil options, duplicate settings, missing
storage-directory option, empty/whitespace path, and invalid values fail. Failed validation is retryable.
Successful Config is one-time; repeated or late calls match `ErrConfigAlreadySet` and cannot replace
settings. `ErrConfigNotInitialized` is exported and PrepareLog before successful Config matches it.
Relative paths are resolved against the working directory, made absolute and lexically cleaned at Config
time; symlinks are not resolved. Config does not create or require the storage directory; PrepareLog
creates it and errors if an existing target is not a directory or creation fails. Config and first
initialization are coordinated as a state machine: concurrent PrepareLog callers wait on one
initialization attempt and each waiter honors its Context; failure permits retry with frozen settings.
Context is for cancellation/deadline only; no configuration is inferred from it or environment variables.

### Fields and validation
- Fields are heterogeneous, typed primitive values.
- Supported primitive kinds: string, bool, all signed and unsigned integer widths (including byte/rune
  aliases), float32, float64, and named types over supported primitives. uintptr and complex types are
  excluded.
- Nullable primitive pointers are supported. Non-finite floats are rejected. Integer values must fit the
  declared Go primitive type exactly.
- Empty and duplicate field names are invalid. Names are case-sensitive and are not trimmed. Canonical
  field ordering is bytewise UTF-8.
- Field construction uses typed selectors (`FieldOf` for non-nullable and `FieldOfNullable` for
  nullable). Nullable selector returns pointer-to-pointer (nil means explicit Null, not zero/empty).
  Field names are inferred automatically. Selector resolution invokes the selector on a zero-value probe
  and compares the returned pointer with candidate direct-field addresses; exactly one direct field
  pointer must match. This does not inspect selector source and may not catch conditional selectors that
  resolve successfully on the zero-value probe. Nested structs are not selectable. Validation occurs
  during append and fails atomically before persistence.

### Canonical identity encoding and permanent type tags

Identity encoding is separate from JSON and architecture-independent. It begins with a fixed
encoding-format version and identity-domain prefix. Type identity is UTF-8 package path plus concrete
struct type name. Inputs include the exposed field-name schema (not non-identity field values) and
designated identity field name/value. Schema fields sort bytewise by UTF-8 name. Every variable-length
component and count uses unsigned 32-bit big-endian framing. The schema records names; the designated
identity name/value is the identity input, and non-identity field values are excluded. Exact byte composition is approved in the following section. Oversized components fail before encoding. Event and Channel use
separate UUID v5 domains.

Permanent v1 one-byte primitive tags: Null `0x00` (no payload); String `0x01` (UTF-8); Bool `0x02` (one
byte 0/1); int `0x10` (8-byte signed BE); int8 `0x11` (1-byte signed); int16 `0x12` (2-byte signed BE);
int32 `0x13` (4-byte signed BE); int64 `0x14` (8-byte signed BE); uint `0x20` (8-byte unsigned BE); uint8
`0x21` (1-byte unsigned); uint16 `0x22` (2-byte unsigned BE); uint32 `0x23` (4-byte unsigned BE); uint64
`0x24` (8-byte unsigned BE); float32 `0x30` (4 raw IEEE bytes); float64 `0x31` (8 raw IEEE bytes). Go
int/uint identity payloads are fixed 64-bit independent of architecture. Named primitives use underlying
tags. Nullable nil uses Null, non-nil uses primitive tag. Unknown tags fail recovery; tag assignments are
permanent and reserved ranges remain unused.

UUID v5 namespace is `26612f1d-f648-483a-9cfe-59c18f113891`; Event prefix is UTF-8
`eventlog\\x00event\\x00v1\\x00`, Channel prefix `eventlog\\x00channel\\x00v1\\x00`. Logical identity is
type + exposed field-name schema + designated identity field name/value, and remains stable across
non-identity changes. Canonical physical-record deduplication is separate from logical identity.

## Approved canonical identity byte composition — 2026-11-24

The exact canonical identity payload layout after the existing domain prefix is approved:
1. Format version byte `0x01`.
2. Type package path: UTF-8 bytes framed by u32 BE byte length.
3. Concrete type name: UTF-8 bytes framed by u32 BE byte length.
4. Schema field count: u32 BE, followed by each field name sorted bytewise by UTF-8 bytes and framed by u32 BE byte length.
5. Identity field name: UTF-8 bytes framed by u32 BE byte length.
6. Identity value: UUID-specific encoding (UUID tag value TBD) followed by the UUID's canonical 16 bytes. Identity field must be part of the exposed schema and UUID-typed; nullable or arbitrary primitive identity values are forbidden.

The already-approved namespace, Event/Channel domain prefixes, permanent primitive tags, big-endian framing, UUID v5 derivation, and non-identity value exclusion remain unchanged. Format byte is intentionally redundant with the domain prefix's v1 marker.

Validation/test questions remain open: accepted UUID Go type/representation; canonical UUID byte order and UUID tag assignment; metadata UTF-8 policy; canonical naming of aliases/generic instantiated types; and maximum framing sizes. Nullable and primitive identities are excluded.

### Append and transaction guarantees
- The only approved public API is `eventlog.PrepareLog[TEvent, TChannel](ctx) (TEvent, TChannel,
  error)`. The internal Log owns append and read operations; consumers receive only event and channel
  values. No consumer-facing Append, Read, Next, or Log handle is exposed at this stage.
- An append validates channel and event before writing. If the channel is new, channel and event records
  are one transaction; otherwise the transaction contains the event occurrence. A commit marker is
  written last.
- Transaction records are not exposed until their commit marker is validated. Recovery discards
  incomplete trailing transactions by truncating back to their start.
- Every persisted record has a global sequence, including commit markers. Transaction ID is the sequence
  of its first transaction record. Transaction record count excludes the commit marker.
- Commit checksum is SHA-256 over exact serialized transaction record bytes, including each record's
  terminating newline; commit marker bytes and its newline are excluded.
- Recoverable transaction atomicity is not the same as durability against power loss: Append may be
  buffered until the sync policy runs.

### Persistence and recovery
- v1 persistence uses built-in JSON in newline-delimited JSON (NDJSON). New logs start as zero-byte
  files; no header record is written.
- Every channel, event, and commit record carries `schema_version: 1`. Unsupported schema versions are
  rejected.
- Versioned JSON Schema Draft 2020-12 documents the record/value format. Schema files are standalone and
  canonical and embedded at build time via `go:embed`.
- Persisted field values use explicit typed JSON representations: strings and bools are native JSON
  values, integers are decimal strings, float32/float64 values are raw IEEE bit-pattern hex strings, and
  null is type-only. Float hex preserves width and signed zero; non-finite values are rejected.
- A complete newline-terminated invalid record is corruption; the log must not silently skip it.
  Incomplete trailing transaction data is rolled back to transaction start. `Open()` creates a missing
  zero-byte log and recovers existing storage.

### Reading
- One unified reader supports replay and live tailing. It delivers committed history in order, catches
  concurrent appends during replay, and transitions to live tailing without gaps or duplicates.
- Every reader is scoped to exactly one consumer-provided `Channel` value; there is no all-channel
  reader.
- Global sequence, cursor, segment offset, and tailing coordination are internal; consumers neither
  provide nor receive storage cursors or sequence numbers.
- Context cancellation and explicit errors are required.
- Read/replay and live tailing are internal responsibilities of the Log instance. Consumers do not
  obtain readers or call Next. The internal reader remains channel-scoped, reconstructs typed consumer
  events, and handles nonmatching event types internally.
- Re-appended logical events are delivered as separate occurrences.

### Change detection and notifications

The internal change manager polls selected Event and Channel fields at 100 ms using one managed scheduler
per prepared Log/change manager—not a goroutine per field. Cadence is internal/test-only. Polling does
not make consumer-owned field mutations race-safe; consumers synchronize their own concurrent accesses.

Consumer-defined Channel types implement the notification method required by the public Channel interface; eventlog uses that method to deliver notifications. No public helper struct or embedding requirement is permitted. The exact method signature is TBD. Occurrence messages
carry the actual concrete Event value as an immutable snapshot captured at the persistence boundary.
Messages are emitted only after persistence succeeds. Every persisted occurrence must eventually be
visible; buffer capacity cannot cause loss. Persistence and scheduler work must not block on slow/unread
consumers, so an internal backlog/replay mechanism is required. Recovered backlog is delivered first in
persistence order and new notifications cannot overtake it. Orderly shutdown drains before closing the
channel; the notification channel closes when its associated lifecycle ends, including initialization
failure if allocated; interrupted delivery is recovered on next initialization. Temporary persistence
failures retry with backoff; persistent failures surface as typed errors and occurrences are not silently
abandoned. Retry limits, error shapes/order, backlog persistence/retention, replay boundary and drain
failure remain open. The older drop-on-full proposal is superseded.

### Storage layout, manifest, index and compaction

The storage directory contains `manifest.json`, rebuildable `index.json`, and `segments/` with monotonic
segment IDs that are never reused (including compaction). Only the active segment is writable; finalized
segments are immutable. Record sequence is global across segments. Rotation is transaction-boundary-only;
an oversized transaction may occupy a segment alone.

The versioned manifest selects the active finalized segment set/generation and next segment ID, and
contains no payloads. Manifest replacement is the atomic generation-switch point: write temporary,
validate, sync, atomically replace; retain old files until finalized. Unreferenced segments are
candidates for safe recovery, not automatically authoritative. Ambiguous/multiple plausible generations
cause recovery failure rather than filename-based guessing.

The index is derived and generation-bound. It records Event IDs, Channel associations, latest
occurrence/HEAD, global sequence, segment and byte offsets. Missing/stale/inconsistent index is rebuilt
from validated authoritative segments and atomically replaced; index never overrides segments.

Open/recovery order: exclusive ownership; validate manifest and generation; validate segments, schema and
sequences; validate commit boundaries/count/checksums; truncate incomplete trailing transaction from
transaction start; rebuild/replace index as needed; clean temporary/orphan artifacts only after
authoritative sources validate; then open for append and internal readers. Existing directory with
missing/corrupt manifest may be recovered only if exactly one complete generation is unambiguous;
otherwise fail closed.

Compaction is reader-safe and generation-pinned: build replacement segments/index without altering pinned
data; validate and sync; finalize; atomically switch manifest/index; new readers use the new generation
while existing readers continue on pinned generation; delete old files only after last pin releases.

### Durability and maintenance
- Automatic synchronization occurs every 1 second or 1 MiB of pending writes, whichever threshold is
  reached first.
- Internal flush is a durability boundary; internal lifecycle shutdown flushes and syncs. No consumer-facing Flush/Close API is approved.
- Segment files are the source of truth; index metadata is derived and rebuildable.
- Segment rotation occurs only at transaction boundaries against a fixed target size. An oversized
  transaction may occupy its own segment.
- Compaction creates and validates replacement segments, syncs them, finalizes them, atomically replaces
  the index, and only then removes old segments. Recovery may clean temporary compaction files only after
  finalized source segments validate.

## Constraints and non-goals
- Standard-library-only Go; local filesystem storage.
- Supported platforms: Linux and Windows at equal priority. macOS is excluded. Core features must not be
  platform-exclusive.
- No distributed coordination, multi-process writers, broker/database functionality, custom codecs, or
  domain-specific behavior.
- Correctness, recovery, predictable memory use, and cross-platform parity are prioritized. Numeric
  performance targets await benchmarks.

## Define details still open

- Final Go-valid public interface/selector signatures, generic constraints, pointer/value conventions,
  reconstruction behavior, and occurrence representation.
- Exact canonical byte composition of schema metadata and designated identity value (framing/tag/version
  rules are approved, final byte layout remains to be specified).
- Internal prepared-value association, lifecycle, cancellation, reader/resource close, and
  terminal-error semantics.
- Identity-field mutation transition details, snapshot consistency, and persistence/index effects.
- Notification accessor/message/error API, backlog durability/retention/replay boundary, retry
  termination, ordering, and drain-failure behavior.
- Consumer synchronization and snapshot consistency details.
- Benchmarks, performance/memory acceptance targets, and detailed acceptance test matrix.


## Approved notification public API decision — 2026-11-24

- The consumer-facing notification payload is the concrete Event type `T` only. No public `Notification[T]` wrapper, Event ID, Occurrence ID, delivery token, or consumer-facing Ack API.
- Notifications are channel-scoped, accessed through the Channel abstraction. Public accessor shape remains to be reconciled with the initialization/ownership mechanism.
- Delivery identity, occurrence identity, durable backlog, retries, replay bookkeeping, and acknowledgment of internal dispatch are private implementation details.
- Since the consumer provides no acknowledgment, eventlog cannot claim exactly-once consumer processing. Delivery/recovery semantics must be stated honestly; duplicates may occur in a crash window if internal dispatch is durably recorded after sending.
- API example: `for event := range channel.Notifications() { handle(event) }`.
