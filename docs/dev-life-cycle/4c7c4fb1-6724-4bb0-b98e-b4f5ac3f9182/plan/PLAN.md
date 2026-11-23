# Development Cycle Plan

## Registry identity

- **Cycle UUID:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182
- **Sequence:** 1
- **Execution order:** 1
- **Status:** in-progress
- **Status reason:** null
- **Current phase:** define
- **Created:** 2026-11-22T16:00:00+02:00

## Objective

Define eventlog's initial product requirements and core design decisions before implementation.

## Problem and context

The repository describes eventlog as a lightweight disk-backed append-only Go log intended for event-oriented applications. The scaffold proposes persistence, ordered replay, and transition into live tailing. Requirements, architecture, commands, and README content remain placeholders. No implementation or tests were found during initial assessment.

## Confirmed product direction

- **Primary scope:** General-purpose embedded event storage; independent of domain semantics and event-sourcing frameworks.
- **Ownership:** One process owns a log at a time. Multiple goroutines may concurrently use the library; writes are coordinated internally.
- **Public event API:** Consumers implement a library-owned Event interface returning eventlog.Fields.
- **Persistence allowlist:** Consumer event structs remain unrestricted. Only the explicitly returned Fields are persisted; eventlog does not inspect, serialize, or redact the full event struct.
- **Fields:** Heterogeneous primitive values.
- **Field construction:** Generic FieldOf selector constrains selected values to supported primitive types and derives the field name from a typed selector rather than a manually supplied string.
- **Named primitive types:** Supported when their underlying type is in the primitive allowlist (e.g. type UserID string, type AccountBalance int64, type IsActive bool). Exact allowlist remains to be finalized.
- **API direction:**
  ```go
  type Event interface {
      Fields() Fields
  }

  type Fields []Field

  func FieldOf[T any, V Primitive](
      selector func(*T) *V,
      event *T,
  ) Field
  ```
  This is the agreed API direction; implementation details, including selector name resolution, remain to be validated.
- **Serialization:** Built-in JSON persistence for v1; no custom codecs.
- **Record framing:** Newline-delimited JSON (NDJSON), one JSON event record per line, terminated by a newline.
- **Incomplete final append recovery:** If the file ends with bytes after the last newline, treat that unterminated suffix as an incomplete final record and truncate it to the last complete newline before continuing.
- **Complete-record corruption:** If a newline-terminated record contains invalid JSON or an invalid record structure, stop with a corruption error. Do not silently skip it or continue with later records.
- **Redaction:** The earlier redaction feature is superseded by the explicit Event/Fields allowlist contract. Consumers decide which fields to expose; eventlog persists only those fields.

- **Channel model:** A channel is a first-class persisted entity, represented by a consumer-defined struct and a library-owned `Channel` interface analogous to `Event`. The interface exposes `Fields() Fields`; channel fields use the same primitive-only Fields mechanism as event fields. A channel record has its own generated ID, consumer type identity, and explicitly exposed fields.
- **Channel lifecycle:** Channel creation is an explicit operation that persists the channel record and returns its generated ID. A channel must exist before events can be appended to it; appending an event never implicitly creates a channel.
- **Mandatory event association:** Every event belongs to exactly one existing channel and must persist a reference to that channel's ID. Channel-less events are not allowed.
- **Record direction (conceptual, not finalized schema):** Channel records contain `id`, `type`, and `fields`. Event records contain their own `id`, a `channel.id` reference, `type`, and `fields`. Exact JSON envelope and versioning remain open.

## Scope

### In scope
- Establish product goals, workload assumptions, guarantees, and non-goals.
- Define append, replay/read, recovery, and tailing semantics.
- Define record representation, durability, concurrency, and API behaviour.
- Record approved requirements and design decisions in maintained project documentation.

### Out of scope
- Implementing the Go library in this cycle.
- Distributed or multi-process writer coordination.
- Adding dependencies or tooling without clear need.
- Full event-sourcing frameworks, databases, or brokers.
- Introducing domain-specific assumptions into the storage layer.

## Constraints, dependencies, and risks

- **Constraints:** Follow steering.md and docs/dev-life-cycle/README.md. Work on main, serialized, cycle-tracked, and verified. No project-specific Go commands are yet verified.
- **Dependencies:** User confirmation of critical design trade-offs.
- **Risks:** Overdesign; unclear durability promises; unsafe recovery; concurrency/tailing contract ambiguity; portability and performance targets.

## Open decisions

- Exact implementation of the generic selector and field-name discovery (Go has no built-in nameof; likely reflection-based selector resolution).
- Exact primitive allowlist and handling of nullable values.
- Public API error behaviour and validation of Fields.
- Exact record envelope/schema metadata and format versioning, including channel record representation.
- Sequence numbering and ordering guarantees across channel and event records.
- Durability/sync defaults and guarantees.
- Reader/tailer interface, cancellation, and replay-to-live transition semantics.
- Log rotation/segmentation and size limits (whether required initially).
- Portability expectations and performance goals.

## Proposed baseline (not yet approved)

- Library-owned Event contract returning Fields; FieldOf uses generic primitive constraints and selector-derived names. JSON is built-in; no custom codec in initial scope.
- NDJSON framing, one complete JSON record per newline-terminated line.
- Strict sequence ordering.
- Sequential replay and a tail mode.
- Configurable durability/sync policy with documented guarantees.
- Standard-library-only dependencies.
- Cross-platform Go where practical.

## Design and implementation approach

Confirm product requirements and trade-offs in discussion before documenting them as approved. Keep initial implementation scope minimal. Do not begin implementation until critical choices are resolved and recorded.

## Ordered tasks

- [x] Establish primary workload direction and ownership model.
- [x] Select typed generic events as public API direction.
- [x] Define Event interface and Fields/FieldOf API direction.
- [x] Replace redaction design with explicit allowlist Event contract.
- [x] Select built-in JSON serialization and NDJSON record framing.
- [x] Define incomplete-final-record and complete-record-corruption recovery policy.
- [ ] Finalize primitive allowlist and API edge cases.
- [ ] Finalize record envelope/versioning, durability, concurrency and tailing behaviour.
- [ ] Update requirements and architecture docs with confirmed decisions.
- [ ] Review consistency; close this cycle and prepare implementation-ready scope.

## Acceptance criteria

- [ ] Confirmed requirements are separated from proposals and open questions.
- [ ] Core design decisions are explicit, justified, and captured in architecture documentation.
- [ ] Initial API and operational guarantees are understandable and testable.
- [ ] No implementation begins while critical design choices remain unresolved.
- [ ] Focused documentation review completed; cycle checkpoint and lifecycle state reconciled.

## Verification strategy

- **Local checks:** Inspect available project validation and documentation format; select appropriate checks before using them.
- **CI/integration checks:** Existing lifecycle validator workflow is available; no Go implementation/test CI established yet.
- **Manual review:** Check requirements, architecture, coding standards, README and commands docs for consistency and avoid stating proposals as facts.

## Persisted phase checkpoint

- **Current phase:** define
- **Assess exit evidence:** Repository scaffold and governance were inspected; purpose, current state, risks, unknowns, and decision areas identified sufficiently to define. User confirmed general-purpose embedded storage, single-process ownership, and an explicit Event/Fields persistence allowlist contract.
- **Next concrete action:** Resolve the remaining primitive, record envelope, durability, and reader/tailer decisions before promoting the approved design to project documentation.
- **Last updated:** 2026-11-23T16:00:00+02:00

## Progress and decisions

- Cycle 1 resumed; execution lock claim confirmed on shared main.
- Confirmed general-purpose embedded event storage and single-process ownership, with concurrent goroutine access coordinated internally.
- Confirmed consumer-defined Event structs expose an explicit Fields allowlist; only those fields are persisted.
- Confirmed generic FieldOf selector API direction, primitive-only selected values, and named primitive types when their underlying type is allowed.
- Confirmed built-in JSON serialization and NDJSON framing (one JSON record per newline-terminated line).
- Confirmed recovery: truncate only an unterminated final suffix; fail on invalid complete records; never skip corruption.
- The earlier redaction design is superseded; no full-struct inspection or automatic redaction is required.
- No implementation, build, or Go tests have started.
- User requested a new session; cycle paused and lock to be released after checkpoint.

## Completion review

- [ ] Acceptance criteria verified or exceptions explicitly accepted
- [ ] Final diff reviewed
- [ ] Required verification results recorded
- [ ] Documentation reconciled
- [ ] Handoff updated
- [ ] Registry status, phase, and completion timestamp updated consistently


## Session handoff

- **Paused at:** 2026-11-22T15:36:55.080Z
- **Reason:** User is moving development to a new session and requested the project be unlocked.
- **Current phase:** define
- **Last question:** Choose durability policy: sync every append, or buffer writes and sync periodically. No choice has been made.
- **Resume instruction:** New session must claim the execution lock, read this plan and the handoff, and continue with the durability question.


## Resume checkpoint — 2026-11-22T15:37:30Z

- Cycle 1 resumed in the `define` phase in a new session.
- The execution lock claim was committed to `main` and fetched back successfully before the registry was changed to `in-progress`.
- Additional user clarification from the prior session: keep the field-selection API simple; selectors target primitive fields only. A selector returning a pointer to a nested struct (for example `*Address`) is not supported and must not be made to compile through extra generic abstractions. Keep selected values within the primitive allowlist.
- No implementation has started.
- The previous handoff's durability question remains the next unresolved question unless the user's intervening agreement messages are clarified.


## Design checkpoint — channel concept and durability — 2026-11-22

- **Durability decision confirmed:** buffered/periodic synchronization. Each append is not required to synchronously force the record to disk. The exact sync trigger/interval, flush API, close semantics, and crash-loss window remain open design details.
- **New product concept introduced:** every event belongs to a channel. A channel groups related events even when those events are represented by different consumer-defined Go struct types. Channel identity and lifecycle/API semantics are not yet finalized.
- Example direction discussed: `UserCreated`, `UserEmailChanged`, and `UserDeleted` may all belong to a logical `users` channel.
- This is a newly introduced requirement and must be considered in the event API, persisted record envelope, replay/filtering, and documentation. Do not assume the channel is necessarily a Go type or infer a concrete channel API before discussion.
- The primitive-only selector constraint remains unchanged.
- No implementation has started.


## Design checkpoint — first-class channels — 2026-11-22

- User confirmed channels are first-class persisted entities represented by consumer-defined structs, with a library-owned `Channel` interface analogous to `Event`; both expose `Fields() Fields` and use the same primitive-only field mechanism.
- Creating a channel is explicit, persists a channel record, and returns its generated ID.
- A channel must exist before an event is appended. Every event has a mandatory reference to an existing channel ID; channel-less events and implicit channel creation are not allowed.
- Conceptual record shapes: channel record `{id, type, fields}`; event record `{id, channel: {id}, type, fields}`. These are design examples, not a finalized JSON schema.
- Exact operation names/signatures, type identity encoding, validation, record envelope/versioning, and channel read/query semantics remain open.
- No implementation has started.


## Session handoff — 2026-11-22

- User requested the project be unlocked to move development to a new session.
- Cycle remains in the `define` phase; implementation, builds, and Go tests have not started.
- Latest confirmed design: Channel is a first-class persisted entity represented by a consumer-defined struct implementing a library-owned `Channel` interface analogous to `Event`; both expose `Fields() Fields` and use primitive-only fields.
- Channel creation is explicit, persists a channel record, and returns its generated ID. Every event must reference exactly one existing channel; channel-less events and implicit channel creation are disallowed.
- Conceptual records: channel `{id, type, fields}`; event `{id, channel: {id}, type, fields}`. Exact API signatures and final schema remain open.
- Buffered/periodic sync is confirmed; cadence, flush/close semantics, and crash-loss window remain open.
- **Next discussion:** decide how the generated channel ID is passed into event append, then continue record envelope/versioning, channel/event API, primitive allowlist, and replay/tailing decisions.
- **Resume:** the next session must claim the execution lock before continuing.


## Resume checkpoint — 2026-11-22T16:25:00Z

- The execution lock was claimed in lock-only commit c8020efbb2bdeaa63cfbb2fd40fae6e3ce2b824c and fetched back from `main`; the active lock names ChatGPT and this cycle.
- The registry was resumed to `in-progress` in commit 3b45d73a949d6a0c0311390e54c42aff90d286bb. The cycle remains in `define`.
- No local checkout or working-tree state is available in this session; repository reads and writes are being performed against the remote `main` branch through GitHub.
- No implementation, build, or Go tests have started.
- Latest user decisions remain: Channel is a struct/interface analogous to Event; channel creation is explicit and returns its generated ID; every event requires an existing channel; selectors remain primitive-only.
- **Next discussion:** settle how the generated channel ID is supplied to the event append operation before finalizing record envelope/versioning and replay/tailing semantics.


## Session handoff checkpoint — 2026-11-22T19:26:00+02:00

- User confirmed Go package path and struct type name may define the type identity used for deterministic IDs. Renaming/moving a type therefore changes its identity; document this as a persisted compatibility consideration.
- **Channel ID:** deterministic UUID derived from channel type identity plus canonical representation of all fields returned by `Channel.Fields()`. Channel fields participate in identity; channel ID is independent of event ID.
- **Event ID:** deterministic UUID derived from event type identity plus canonical representation of all fields returned by `Event.Fields()`. Channel ID is explicitly excluded from event ID calculation.
- **Append API direction:** retain `log.Append(channel, event)`; channel and event are supplied together. The channel's deterministic ID is resolved internally.
- **Duplicate event semantics:** event identity is global across channels. Re-appending an existing event promotes that one logical event to the HEAD of the entire log, rather than creating a second logical event. The event may be associated with multiple channels; the latest example explicitly chose association with both channels.
- **Ordering/storage distinction:** logical order is global and independent of physical segment placement. User proposed segment splitting/reorganization followed by scavenging to remove obsolete physical copies. A logical promotion operation persisted in segments, followed by asynchronous physical compaction/scavenging, was discussed as a possible implementation; not yet finalized.
- **Durable source of truth:** segment files alone must reconstruct the complete logical log, including event promotions and channel associations. An index must be rebuildable and must not be authoritative.
- **Initial index:** use a local index file for the first draft, not SQLite. Keep the index implementation replaceable so a database-backed index can be considered later.
- **Segment metadata/checksums:** file references and checksums were discussed as useful index/segment metadata; exact checksum scope and format remain open.
- **Implementation state:** no implementation, build, or Go tests have started. Work remains in `define`.
- **Next-session instructions:** claim the execution lock; read this plan and handoff; continue defining canonical identity encoding/UUID derivation, global ordering/promotion records, segment recovery/compaction, and the rebuildable index format. Do not start implementation until the cycle's critical design and acceptance criteria are complete.


## Design checkpoint — floating-point fields — 2026-11-22

- **User decision confirmed:** the primitive field allowlist must include floating-point values; float32 and float64 are in scope. Named types whose underlying type is one of these floating-point types are included under the existing named-primitive rule.
- The primitive allowlist is therefore not limited to strings, booleans, and integer types.
- **Still open:** define the handling of NaN and positive/negative infinity, negative zero, and canonical floating-point encoding for deterministic event/channel IDs. JSON has no standard representation for non-finite numbers, so the persistence and validation contract must address these explicitly before implementation.
- No implementation, build, or Go tests have started.


## Design checkpoint — floating-point canonicalization — 2026-11-22

- **User approved:** reject NaN, positive infinity, and negative infinity. Only finite float32 and float64 values are valid persisted primitive fields.
- **Precision:** preserve the source floating-point width and IEEE-754 value; serialization and deterministic identity encoding must not round through a lower-precision representation.
- **Signed zero:** positive zero and negative zero remain distinct values for deterministic identity (their IEEE-754 bit patterns differ); the persisted encoding must preserve this distinction even though ordinary JSON number text may not reliably do so.
- Validation must reject non-finite values before persistence and before deterministic ID derivation. This is a confirmed v1 requirement.
- No implementation, build, or Go tests have started.


## Design checkpoint — v1 primitive field allowlist — 2026-11-22

- **User approved the v1 primitive allowlist:** `string`, `bool`; all signed integer types (`int`, `int8`, `int16`, `int32`, `int64`); all unsigned integer types (`uint`, `uint8`, `uint16`, `uint32`, `uint64`); and `float32`, `float64`.
- Named types whose underlying type is one of the supported primitives are supported. `byte` and `rune` are included as aliases of `uint8` and `int32` respectively.
- `uintptr`, `complex64`, and `complex128` are excluded.
- Floating-point constraints remain: reject NaN and infinities; preserve IEEE-754 width and value; preserve the distinction between positive and negative zero in canonical identity and persistence.
- The `Fields` API remains primitive-only; selectors for nested structs are unsupported.
- No implementation, build, or Go tests have started.


## Design checkpoint — canonical field ordering — 2026-11-22

- **User approved:** canonical identity encoding ignores the order in which fields are returned or declared. Fields are sorted by field name before canonical encoding.
- Therefore, equal field-name/value/type sets produce the same canonical field representation regardless of input order.
- The encoding must still define deterministic ordering for field names (bytewise UTF-8 ordering is the proposed rule) and reject duplicate field names rather than silently choosing one.
- No implementation, build, or Go tests have started.


## Design checkpoint — field-name validation — 2026-11-22

- **User approved:** field names are case-sensitive; empty names are invalid; duplicate names are rejected; names are not trimmed or normalized. Thus names differing only by case are distinct (for example, `userID` and `userid`).
- Canonical sorting remains deterministic bytewise UTF-8 ordering by field name.
- No implementation, build, or Go tests have started.


## Design checkpoint — nullable primitive fields — 2026-11-22

- **User approved:** nullable primitive fields are supported in v1. A nil pointer represents null and is distinct from zero, false, and empty string.
- Canonical identity encoding must include an explicit null marker, distinct from any concrete value encoding.
- Nullable fields follow the same primitive allowlist, named-type, float validation, and deterministic field-name rules.
- No implementation, build, or Go tests have started.


## Design checkpoint — strict validation errors — 2026-11-22

- **User approved:** validation is strict and failures return errors. Invalid fields, unsupported types, duplicate names, non-finite floats, and references to nonexistent channels must fail explicitly.
- No silent omission, coercion, or automatic correction of invalid input is permitted.
- No implementation, build, or Go tests have started.


## Design checkpoint — record versioning — 2026-11-22

- **User approved:** every persisted record carries an explicit schema version, starting at version 1.
- Readers must reject unsupported future versions with a clear error instead of attempting to interpret them.
- Versioning applies consistently to persisted record types, including event data and structural records; exact record taxonomy and version migration policy remain to be specified.
- No implementation, build, or Go tests have started.


## Design checkpoint — canonical identity encoding — 2026-11-22

- **User approved:** canonical identity encoding is independent of field ordering; fields are sorted by bytewise UTF-8 field name.
- Each field encoding includes a type tag and length-prefixed field name/value to avoid ambiguous concatenation.
- Preserve exact integer and floating-point widths and IEEE-754 bit patterns, including the distinction between positive and negative zero.
- Include Go package path and struct type name in the persisted type identity.
- Deterministic hash-based UUID derivation uses the canonical encoded bytes.
- This encoding makes identity stable across field-order changes, but identity intentionally changes with field values, primitive types, or consumer type identity.
- Exact hash and UUID algorithm, canonical byte layout details, and compatibility/versioning policy remain to be decided.
- No implementation, build, or Go tests have started.


## Design checkpoint — deterministic UUID algorithm — 2026-11-22

- **User approved UUID v5** (namespace-based SHA-1) for deterministic IDs.
- Use a fixed eventlog namespace, with distinct domain prefixes for channel IDs and event IDs so the two ID classes cannot collide by kind.
- UUID v5 derivation uses the approved canonical identity bytes as its name input; details of exact byte composition and the concrete namespace UUID remain to be recorded.
- No implementation, build, or Go tests have started.


## Design checkpoint — segment-first, rebuildable index — 2026-11-22

- **User approved:** write the segment record first, then update the index. Segment data is the source of truth; the index is a replaceable performance cache and must be rebuildable from segments.
- Re-appending an existing event writes another physical occurrence; index logic promotes the event to global HEAD without duplicating the logical event, retaining channel associations.
- Recovery scans segment records to reconstruct index state and logical order; latest occurrence provides promotion/order evidence, so no separate promotion record is required for this design.
- Index file updates should use atomic replacement to avoid partial index state after a crash.
- Sync/durability policy remains buffered/periodic; detailed sync and atomic-replacement protocol are still open.
- No implementation, build, or Go tests have started.


## Design checkpoint — global sequence numbers — 2026-11-22

- **User approved:** every persisted record (including channel creation and event append/promotion occurrences) receives a globally monotonically increasing sequence number.
- Sequence numbers are stored in segment records, not solely in the index; they are the authoritative ordering evidence and are independent of physical segment placement.
- Recovery derives global logical event order from the latest occurrence of each event ID, ordered by sequence descending. Channel records also occupy sequence numbers in the global record stream.
- No implementation, build, or Go tests have started.


## Design checkpoint — sync policy accepted — 2026-11-23T02:16:10+02:00

- User accepted the proposed buffered/periodic sync policy: automatic sync every 1 second or when the buffer reaches 1 MiB, whichever occurs first.
- Explicit `Flush()` is proposed/accepted as a durability boundary that flushes buffered bytes and forces sync.
- `Close()` attempts final flush and sync before closing.
- `Append()` does not promise crash durability before a successful sync; recent appended records may be lost on crash within the unsynced window.
- No implementation or tests have started. The current cycle remains in define.
- Next: finalize record taxonomy/envelope, UUID and canonical encoding details, and promotion/association record semantics before implementation.


## Design checkpoint — crash-safe compaction protocol approved — 2026-11-23

- User approved this compaction order: (1) write replacement segment files under temporary names, (2) validate their contents/checksums, (3) sync replacement files, (4) rename them to final segment names, (5) atomically replace the index so it references the new segment set, and (6) delete obsolete segments only after index replacement.
- Recovery contract: before index replacement, old segments remain authoritative/intact; after index replacement but before cleanup, old and new segments may coexist and recovery must reconcile duplicate physical occurrences; after obsolete-segment deletion, compacted segments must reconstruct the complete logical log.
- Interrupted compaction may leave temporary files or duplicate physical segment copies. Recovery should tolerate and reconcile these safely; temp-file cleanup policy remains to be defined.
- This protocol is now a confirmed design decision. Exact index/manifest contents, duplicate reconciliation details, and cleanup rules remain open.
- No implementation, build, or tests have started; cycle remains in define.


## Design checkpoint — record taxonomy and persisted envelope (approved)

Approved by the user on 2026-11-23:

- Persist exactly two record kinds: `channel` (channel creation) and `event` (event append occurrence). Promotion is represented by a later event occurrence with a larger sequence; no separate promotion record.
- Shared envelope: `version` (starting at 1), `seq` (global monotonic sequence), `kind` (`channel` or `event`), and `data` (type-specific payload).
- Channel payload: `id`, `type`, `fields`. Event payload: `id`, `channel_id`, `type`, `fields`.
- Channel creation is logically idempotent for deterministic channel identity.
- Recovery scans valid segment records, validates envelope/content, reconstructs channels, resolves event occurrences/associations, retains the latest occurrence per event ID as logical HEAD, sorts logical events by latest sequence descending, and rebuilds the index.
- Unknown kind or unsupported version, duplicate/invalid sequence, missing channel references, or invalid fields are corruption/errors. Only an incomplete trailing NDJSON suffix may be truncated; malformed complete records fail recovery.
- Persisted encoding must preserve float width and IEEE bit patterns, including signed zero, as previously approved.


## Design checkpoint — canonical byte encoding structure (approved)

Approved by the user on 2026-11-23:

- Canonical identity encoding uses distinct domain prefixes for channel IDs and event IDs.
- Type identity is package path plus struct type name, encoded as UTF-8.
- Explicit field count; fields sorted bytewise by UTF-8 field name.
- Each field encodes name length/name bytes, type tag, and value length/value bytes.
- Integers use fixed-width big-endian encoding.
- Floating point values encode raw IEEE-754 bits and preserve float32/float64 width and signed zero.
- Nullable values have an explicit null type marker distinct from zero values.
- Strings are raw UTF-8 bytes with length prefixes.
- Canonical identity bytes begin with a fixed encoding-format version.
- Exact type-tag table, namespace UUID, and byte-composition details remain undecided.


## Design checkpoint — primitive type-tag table (approved)

Approved by the user on 2026-11-23. Persisted v1 one-byte tags:

- `0x00`: Null (no payload)
- `0x01`: String (UTF-8 bytes)
- `0x02`: Bool (one byte, 0 or 1)
- `0x10`: int (8-byte signed)
- `0x11`: int8 (1-byte signed)
- `0x12`: int16 (2-byte signed)
- `0x13`: int32 (4-byte signed)
- `0x14`: int64 (8-byte signed)
- `0x20`: uint (8-byte unsigned)
- `0x21`: uint8 (1-byte unsigned)
- `0x22`: uint16 (2-byte unsigned)
- `0x23`: uint32 (4-byte unsigned)
- `0x24`: uint64 (8-byte unsigned)
- `0x30`: float32 (4 raw IEEE-754 bytes)
- `0x31`: float64 (8 raw IEEE-754 bytes)

Multi-byte numerics are big-endian. int/uint are fixed 64-bit for architecture-independent encoding. Named primitive types use underlying-type tags. Nullable values encode nil with Null and non-nil with the primitive tag. Unknown tags fail recovery. Reserved ranges are not allocated. Tags are permanent and cannot be reassigned.


## Design checkpoint — UUID v5 namespace and domain prefixes (approved)

Approved by the user on 2026-11-23:

- Fixed UUID v5 namespace: `26612f1d-f648-483a-9cfe-59c18f113891`.
- Event identity domain prefix: UTF-8 `eventlog\\x00event\\x00v1\\x00`.
- Channel identity domain prefix: UTF-8 `eventlog\\x00channel\\x00v1\\x00`.
- Identity uses package path, type name, and canonical fields; variable-length components must be framed length-prefixed to avoid ambiguity.
- Channel IDs derive from channel type + canonical fields. Event IDs derive from event type + canonical fields, excluding channel association.
- UUID v5 (SHA-1) yields standard UUID IDs; domain prefixes keep event/channel identity domains separate.
- Exact framing details for variable components (including length-width choice) remain to be specified; the namespace and prefixes are fixed.


## Design checkpoint — uint32 big-endian framing approved

Approved by the user on 2026-11-23:

- Every variable-length byte component length and count uses unsigned 32-bit big-endian (`uint32 BE`).
- This applies to package path, type name, field count, field name, field payload, and string lengths.
- Canonical identity layout: domain prefix; length-prefixed package path; length-prefixed type name; uint32 BE field count; canonical sorted field entries. Each field uses length-prefixed name, one-byte type tag, and length-prefixed value payload.
- Deterministic framing across architectures; values/components exceeding uint32 limits are rejected before encoding.


## Design checkpoint — global event/channel associations and promotion (approved)

Approved by the user on 2026-11-23:

- Event IDs are global and independent of channel association.
- Re-appending an existing event creates a new physical occurrence.
- An event may retain associations with multiple channels.
- The newest occurrence determines the event's logical HEAD and current channel.
- Logical events are ordered by each event's latest occurrence sequence, descending.
- Occurrence history is retained in segment records.


## Design checkpoint — duplicate reconciliation for recovery and compaction (approved)

Approved by the user on 2026-11-23:

- The sequence number identifies a physical record occurrence; the event ID identifies the logical event. These identities must not be conflated.
- A repeated record with the same sequence and identical content is treated as a physical duplicate copy and deduplicated during recovery.
- Repeated occurrences of the same event ID at different sequence numbers are legitimate history and must be preserved. The highest sequence occurrence determines the logical HEAD, current channel, and ordering; older occurrences remain history.
- Two records with the same sequence but different content are corruption and fail recovery; do not guess which wins.
- Duplicate channel records with the same channel ID and identical channel data may be deduplicated; conflicting data for one channel ID is corruption.
- Incomplete temporary compaction artifacts are ignored and cleaned up. Valid finalized replacement segments are recovered normally.
- Compaction copies retain original sequence numbers. Reconciliation must not promote an older event due to duplicate copies.


## Design checkpoint — interrupted compaction temporary files (approved)

Approved by the user on 2026-11-23 (Option B): recovery ignores temporary compaction files. Temporary files are cleaned up only after finalized source segments have been validated. Temporary artifacts must not be treated as authoritative log input. This keeps recovery anchored on validated finalized segments and prevents incomplete temporary compaction output from affecting logical event history.


## Resume checkpoint — Option B approved and cycle resumed — 2026-11-23T16:00:00+02:00

- User approved Option B for interrupted compaction artifacts: recovery ignores temporary compaction files and cleans them up only after finalized source segments are validated. This is now a confirmed design decision.
- Cycle 1 was resumed from paused to in-progress in `define`. The execution lock was claimed in lock-only commit `e1c8c087c682c50685b176c236b05311510d1288` and fetched back from shared `main` to confirm the active claim. Registry resume commit: `a8074ce9353ac207acee39ea3067de1ac53d4410`.
- No implementation, build, or Go tests have started. No local checkout is available; updates are remote via GitHub contents API.
- Continue defining the remaining implementation-ready details: API/error behavior and selector resolution, index/manifest shape and recovery ordering, reader/replay/tailing semantics, segment rotation/size limits, portability/performance assumptions, and testable acceptance criteria.


## Design checkpoint — nullable field constructor API approved — 2026-11-23

User approved Option A: retain `FieldOf` for non-nullable primitive fields and introduce a separate `FieldOfNullable` constructor for nullable primitive fields. The nullable selector returns a pointer-to-pointer (e.g. `func(e *User) **string { return &e.Email }`); nil means explicit Null, distinct from empty string and zero values. Keep the API straightforward and strict; do not add a universal complicated generic constructor. No implementation, build, or Go tests have started.


## Design checkpoint — selector field-name resolution approved — 2026-11-23

User approved Option A: resolve the selected struct field name through reflection automatically. Selector resolution must identify exactly one field and return a clear error if it cannot be resolved uniquely. No manual field-name string is required. No implementation/build/tests have started.


## Design checkpoint — selector field pointer validation approved — 2026-11-23

User approved Option A: resolve the field by invoking the selector on a zero-valued instance and comparing its returned pointer with candidate struct-field addresses. This supports direct field selectors without requiring a field-name string. The runtime contract is that the selector must resolve to exactly one field pointer; selectors that do not yield a uniquely matched direct field pointer fail with a clear error. This method does not inspect selector source code and cannot guarantee detection of every conditional selector if it happens to return a field pointer on the probe instance. No implementation, build, or tests have started.


## Design checkpoint — Field validation timing approved — 2026-11-23

User approved validation at append time. Validate returned event/channel Fields when the append operation is performed; invalid data must return a clear error and must not be silently omitted or coerced. No implementation/build/tests have started.


## Design checkpoint — validation atomicity approved — 2026-11-23

User approved Option A for append validation failures: all-or-nothing. Validate all returned event/channel fields before writing any record; if any field is invalid, return a clear error and leave the log unchanged, with no partial event/channel record. No implementation, build, or tests have started.

## Design checkpoint — fully atomic channel/event append approved — 2026-11-23

User approved fully atomic `Append(channel, event)` behavior: validate both channel and event first; when the channel is new, persist the channel and event together only if the full append is valid. Any validation failure leaves the log unchanged and writes neither a new channel nor an event record. No implementation, build, or tests have started.

## Design checkpoint — recoverable atomic append on write failure approved — 2026-11-23

User approved a recoverable-atomic append guarantee: if a write fails or a crash interrupts an append, recovery must not expose a partial logical append. Any incomplete append is rolled back/discarded during recovery; the log recovers to a valid state. This includes a new channel and its event being committed together. The concrete on-disk transaction/commit-marker or rollback mechanism remains to be designed. No implementation, build, or tests have started.

## Design checkpoint — Append transaction commit markers approved — 2026-11-23

User approved Option A: every `Append(channel, event)` is persisted as an explicit transaction, with a commit marker written only after all transaction records are complete. Recovery exposes only committed transactions and discards incomplete trailing transactions. A newly created channel record and its event record belong to the same transaction. The exact transaction record encoding, commit marker contents, and recovery/truncation behavior remain to be specified. No implementation/build/tests have started.


## Design checkpoint — append return and commit verification approved — 2026-11-23

- User approved the idiomatic append API: `Append(channel, event) error`. Append returns an error directly; no separate result value.
- User approved commit-marker verification contents: each transaction's commit marker includes a transaction ID, record count, and checksum of the transaction records. Recovery validates these before exposing a transaction; invalid or incomplete trailing transactions are discarded according to the atomic recovery contract. Details of checksum algorithm, transaction-ID format, and precise marker framing remain open.
- Workflow housekeeping: `.github/workflows/initialize.yml` was deleted from `main` in commit `450c2bca54dfbddb782c69c01205ae0513a5d3c9`; deletion was verified (contents endpoint returned 404). Other workflows were untouched.
- Cycle 1 remains in Define. No implementation, build, or Go tests have started.
