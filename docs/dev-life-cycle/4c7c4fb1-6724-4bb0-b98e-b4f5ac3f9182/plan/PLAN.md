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
- **Last updated:** 2026-11-22T15:36:55.080Z

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
