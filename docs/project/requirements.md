# Project requirements

**Status:** Product requirements baseline; design is not yet implementation-authorized. This document owns product purpose, required capabilities,
observable behavior, domain rules, constraints, quality expectations, and acceptance outcomes. It does not
prescribe internal components, algorithms, storage layouts, encoding formats, or implementation steps.
Approved technical design belongs in [architecture.md](architecture.md). The design there must satisfy these
requirements; neither document is implementation or verification evidence.

## Purpose

eventlog is a lightweight, embedded, disk-backed append-only event log for general-purpose event-oriented
applications. It provides persistent event occurrences, ordered history replay, and a unified transition
from replay into live tailing. It is not tied to an event-sourcing framework or a specific domain.

## Functional requirements

### Ownership, concurrency, and scope
- A single process owns a log at a time. Multi-process and distributed writer coordination are out of scope.
- Multiple goroutines may use the library concurrently. The library must coordinate operations and preserve
  ordering and integrity.
- Consumers own their Event and Channel domain behavior and explicitly select the data eventlog stores.
  eventlog must not inspect or persist unselected consumer struct data. No automatic redaction layer is
  required.

### Events and Channels
- Every Event occurrence belongs to exactly one Channel.
- Event logical identity is global and independent of Channel. Identity is derived deterministically from
  concrete Event type, exposed field schema, and a consumer-designated identity field name/value.
- The designated identity field for both Events and Channels must contain a UUID represented by an
  unnamed [16]byte or a named type with that underlying type. Nullable and other underlying types
  are not permitted.
- Changes to non-identity Event fields create new immutable occurrences under the same logical Event
  identity. Repeated occurrences remain in history, including their Channel associations. The latest
  occurrence determines current/HEAD state.
- Channel identity is stable for the same concrete Channel type, exposed schema, and designated UUID
  identity. Non-identity Channel changes update the current Channel snapshot but do not independently
  create Event occurrences. New Event occurrences use the latest Channel snapshot; prior occurrences
  retain their original association.
- Channels are created implicitly and atomically with the first successful append that uses them. Empty
  Channels are not persisted, and no standalone Channel-creation API is provided.

### Configuration and initialization
- Configuration is mandatory and separate from generic type selection and Context. The public configuration
  API is `Config(options ...Option) error` and requires a storage-directory option.
- Configuration validation rejects nil options, duplicate settings, missing required settings, empty or
  whitespace paths, and invalid values. Failed validation is retryable.
- Successful configuration is immutable and one-time. Attempts to configure again or after initialization
  return the established configuration error. Preparation before successful configuration returns the
  established not-initialized error.
- Relative storage paths resolve against the working directory and are normalized at configuration time.
  Configuration does not create the storage directory; preparation creates it and reports failures.
- Concurrent preparation shares initialization safely. Each caller's Context controls its own wait or
  operation; configuration is not inferred from Context or environment variables.
- The generic `TEvent` and `TChannel` arguments to `PrepareLog[TEvent, TChannel]` must be pointer types;
  non-pointer arguments are rejected during preparation. Repeating a compatible preparation for the same
  concrete Event/Channel type pair and exposed field schemas returns the same live Event and Channel
  pointer instances. Incompatible reuse of an already prepared type/schema registration fails explicitly
  rather than creating competing live instances.
- Existing event-log storage is loaded and recovered as the system of record before eventlog resolves or
  creates any runtime Channel. A Channel is resolved by its stable Channel identity; event-specific
  preparation must reuse that Channel runtime rather than create a parallel Channel stream.

### Fields and validation
- Consumers expose a heterogeneous set of typed primitive fields. Supported values include strings,
  booleans, signed/unsigned integer widths, float32/float64, and named types over supported primitives;
  uintptr and complex values are excluded. Nullable primitive pointers are supported.
- Field names are inferred, case-sensitive, and must be non-empty and unique; names are not trimmed.
- Typed selectors distinguish nullable and non-nullable direct fields. Nested struct fields are not selectable.
  A nil nullable field remains distinct from a pointer to its zero value.
- Invalid selectors, types, nullability, integer ranges, non-finite floats, or field schemas must fail
  before persistence; values must not be silently omitted, coerced, or partially stored.

### Append, persistence, and recovery
- The public preparation API returns the consumer's Event and Channel values; internal persistence and
  append/read lifecycle remain library-owned. No consumer-facing Log handle or Append/Read/Next/Close/
  Flush API is approved at this stage.
- Appending an Event and any newly required Channel data is atomic. A failed validation or interrupted
  append must not expose a partial transaction.
- Only fully committed transactions are visible. Recovery discards an incomplete trailing transaction
  from its beginning. Complete malformed records, unsupported formats, invalid sequence/order data,
  and corruption must fail rather than be silently skipped.
- Persistence is versioned and portable across supported platforms. Event and Channel values preserve
  their declared types, nullability, and valid value range.
- The append log is authoritative. Derived metadata can be reconstructed from validated persisted
  records, and recovery must fail closed when storage is ambiguous rather than guess.

### Reading
- One unified Reader progression per runtime Channel provides committed history and live updates in
  order, without gaps or duplicates when transitioning from replay to tailing. The same progression
  advances through replay and live tailing; separate typed Readers must not be combined to synthesize
  the heterogeneous notification stream.
- All concrete Event types and their decoders required by a logical Channel's committed history must
  be prepared/registered before heterogeneous historical notification replay begins. If a required
  decoder has not yet registered, the Channel Reader waits asynchronously and indefinitely; there is no
  arbitrary timeout, and absence alone does not produce a fatal Channel error. Replay can proceed when
  the decoder registers. Unknown or undecodable persisted Event types must never be silently discarded.
  Sequential event-specific `PrepareLog` calls alone do not establish that all registrations are complete;
  the completion boundary is internal and must not be exposed through new consumer-facing `Activate`,
  `Start`, `RegisterEventType`, or `Close` methods.
- Each reader is scoped to exactly one consumer-provided Channel. No all-channel reader is provided.
- Storage sequences, offsets, cursors, and tailing coordination remain internal.
- Context cancellation and explicit errors are required. Re-appended logical Events are separate
  occurrences. Consumers do not directly control internal readers.

### Change detection and notifications
- eventlog detects changes to selected Event and Channel fields on a managed cadence. Consumer-owned
  concurrent mutations still require consumer synchronization.
- The Channel interface defines `Notifications() <-chan Event`, where `Event` is the common interface implemented by concrete Event types. `TEvent` in `PrepareLog` is instead a concrete Event type constrained by `Event`; the required factory accepts `func(chan Event) TChannel` so one Channel can receive heterogeneous concrete Event values. Notifications carry concrete Event values as immutable snapshots after successful persistence. There is no `Notify` callback/sink method.
- Notifications are channel-scoped, ordered, and lossless across buffer pressure or recovery. Slow
  consumers must not stall persistence. Pending deliveries are recovered and delivered before newer ones.
- Shutdown stops new preparation/appends, coordinates outstanding commits, cancels Reader/delivery workers,
  waits for senders to stop, drains pending notifications according to the shutdown policy, closes the
  notification stream only after sending has stopped, and then releases storage resources. Lifecycle
  failure is reported and recoverable pending state is retained. Without consumer acknowledgment or
  transactional integration, exactly-once consumer processing is not guaranteed and duplicates may occur.

## Quality expectations and constraints
- Correctness, recovery safety, predictable memory use, and Linux/Windows parity are prioritized ahead
  of numeric performance claims.
- Go standard library only; local filesystem storage. Linux and Windows are equal-priority supported
  platforms. macOS is excluded. Core behavior must not rely on platform-exclusive features.
- No distributed coordination, multi-process writers, broker/database functionality, custom codecs, or
  domain-specific behavior.

## Acceptance expectations
- Public API and consumer responsibilities are unambiguous and compile-valid once Define is approved.
- Invalid configuration, fields, identities, and records fail without partial persistence.
- Committed occurrences replay in order and transition to live tailing without gaps or duplicates.
- Channel isolation and Event identity/occurrence semantics hold across updates and recovery.
- Crash/interruption recovery, derived metadata rebuild, and compaction preserve committed history.
- Notifications meet ordering, losslessness, non-blocking persistence, and honest delivery guarantees.
- Automated tests and benchmarks establish correctness, recovery behavior, platform parity, and agreed
  performance/memory targets before implementation is considered complete.

## Define details still open

Remaining design questions are maintained in [architecture.md](architecture.md), with decision provenance
in [decisions.md](decisions.md). They include the UUID identity marker/tag, metadata UTF-8 policy,
canonical naming of aliases and instantiated generic types, maximum framing sizes, internal lifecycle and
cancellation details, Channel mutation/snapshot consistency, backlog and retry behavior, shutdown failure
semantics, and measurable acceptance/performance targets. Approved outcomes are integrated into the
relevant specification sections rather than appended as dated decision or checkpoint sections.
