# Project requirements

**Status:** Define-phase requirements (Cycle 1). This document owns product purpose, required capabilities,
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
- The designated identity field for both Events and Channels must contain a UUID. Nullable or arbitrary
  primitive/string/numeric identity values are not permitted.
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
- Existing event-log storage is loaded and recovered as the system of record before eventlog resolves or
  creates any runtime Channel. A Channel is resolved by its stable Channel identity; event-specific
  preparation must reuse that Channel runtime rather than create a parallel Channel stream.

### Fields and validation
- Consumers expose a heterogeneous set of typed primitive fields. Supported values include strings,
  booleans, signed/unsigned integer widths, float32/float64, and named types over supported primitives;
  uintptr and complex values are excluded. Nullable primitive pointers are supported.
- Field names are inferred, case-sensitive, and must be non-empty and unique; names are not trimmed.
- Typed selectors distinguish nullable and non-nullable fields. Nested struct fields are not selectable.
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
- All concrete Event types and their decoders for a logical Channel must be prepared/registered before
  heterogeneous historical notification replay begins. Unknown persisted Event types must never be
  silently discarded. Sequential event-specific `PrepareLog` calls alone do not establish that all
  registrations are complete; the completion boundary is an internal Define item and must not be exposed
  through new consumer-facing `Activate`, `Start`, `RegisterEventType`, or `Close` methods.
- Each reader is scoped to exactly one consumer-provided Channel. No all-channel reader is provided.
- Storage sequences, offsets, cursors, and tailing coordination remain internal.
- Context cancellation and explicit errors are required. Re-appended logical Events are separate
  occurrences. Consumers do not directly control internal readers.

### Change detection and notifications
- eventlog detects changes to selected Event and Channel fields on a managed cadence. Consumer-owned
  concurrent mutations still require consumer synchronization.
- The Channel interface defines the receive-only notification accessor `Notifications() <-chan Event`, where `Event` is the common interface implemented by concrete Event types. In the approved generic preparation API, `TEvent` denotes this common Event interface, not an individual concrete Event type. A single consumer-defined Channel may receive heterogeneous concrete Event values (for example, `OrderCreated` and `OrderCancelled`) through this one stream. Notifications carry concrete Event values as immutable snapshots after successful persistence. There is no `Notify` callback/sink method. The typed channel-construction factory therefore accepts the common Event interface and supports this heterogeneous stream without changing the approved consumer-facing Channel API.
- Notifications are channel-scoped, ordered, and lossless across buffer pressure or recovery. Slow
  consumers must not stall persistence. Pending deliveries are recovered and delivered before newer ones.
- Shutdown drains pending notifications before closing the consumer channel; lifecycle failure is reported.
  Delivery/recovery must be described honestly: without consumer acknowledgment or transactional
  integration, exactly-once consumer processing is not guaranteed and duplicates may occur.

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

Technical open questions and approved low-level decisions are tracked in
[architecture.md](architecture.md) and the active Cycle 1 plan. They include final Go API/interface and
selector signatures, pointer/value reconstruction conventions, initialization/lifecycle and cancellation
details, identity mutation behavior, notification method/delivery/backlog/error/shutdown semantics,
snapshot consistency, complete acceptance tests, and measurable performance/memory targets. Do not treat
proposals as approved decisions or begin implementation before explicit Define approval.
