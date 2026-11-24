# Development Cycle Plan

## Registry identity

- **Cycle UUID:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182
- **Sequence:** 1
- **Execution order:** 1
- **Status:** in-progress
- **Current phase:** define
- **Created:** 2026-11-22T16:00:00+02:00

## Objective

Define eventlog's initial product requirements and core design decisions before implementation.

## Problem and context

The repository describes eventlog as a lightweight disk-backed append-only Go log intended for event-oriented applications. The scaffold proposes persistence, ordered replay, and transition into live tailing. Requirements, architecture, commands, and README content remain placeholders. No implementation or tests were found during initial assessment.

## Current approved product and design baseline (authoritative)

**API decision supersession (2026-11-24):** The sole approved public entry point is
`eventlog.PrepareLog[TEvent, TChannel](ctx) (TEvent, TChannel, error)`.
The `Log` implementation and all append/read/next/sync/recovery/close/compaction
machinery are internal to the package. Consumers receive only their typed event
and channel values; they do not receive a Log handle or manage its lifecycle.
Earlier public Append/Read/Next/Close APIs and generic Log-handle proposals below
are historical and are not current public API requirements.


- **Purpose:** General-purpose embedded, append-only event storage; no domain semantics or event-sourcing-framework assumptions.
- **Ownership/concurrency:** Single process owns the log; multiple goroutines may use the API, with internally coordinated writes. Multi-process/distributed writers are out of scope.
- **Consumer contracts:** Consumer-defined Channel and Event structs are supplied as generic type arguments to the public API. They implement the library-owned `Channel` and `Event` interfaces. Field selection is supplied as a selector callback to `Append`; selectors are part of the Append call contract, not hidden solely inside `Fields()` or inferred by serializing the whole struct. The exact selector callback body and interface method signatures must be transcribed from the agreed contract before implementation.
- **Append/channel lifecycle:** the public contract is generic over consumer Channel and Event types, with field selection supplied through the Append selector callback. The earlier `Append(channel, event)` call shape is superseded. Append validates the selected channel/event fields before writing; a new channel is created and persisted atomically with its first event. No standalone channel-creation API or channel-only records exist. Exact callback/value-input shape remains open.
- **Event identity/history (current approved rule):** Event identity is global and independent of channel. Derive UUID v5 from concrete Go Event type name, all field names exposed through `Fields()`, and the consumer-designated identity field name/value, canonically encoded. Non-identity data changes keep the same logical ID but create new immutable occurrences. One occurrence belongs to one channel; history is retained; latest occurrence defines HEAD/current channel and ordering.
- **Fields/API direction:** `Fields` is heterogeneous and primitive-only. `FieldOf` handles non-nullable fields; `FieldOfNullable` handles nullable primitives. Selectors are supplied through the third callback argument of Append; selector reflection resolves direct field pointers on a zero-valued typed probe and requires a unique match. Validation is at Append and atomic. Duplicate/empty names are rejected; names are case-sensitive and not trimmed; canonical order is bytewise UTF-8. The exact callback body and consolidated public signatures/error contract remain to be captured precisely.
- **Primitive allowlist:** string, bool, signed/unsigned integer types (including byte/rune aliases), float32/float64, named types over supported primitives; excludes uintptr and complex. Nullable primitive pointers are supported. Non-finite floats are rejected. Integer bounds are checked by Go exactly.
- **Identity:** UUID v5 with fixed namespace and separate event/channel domain prefixes. Canonical encoding is order-independent: fields sorted bytewise UTF-8, explicit type tags, uint32 big-endian framing, exact primitive widths/bits, and package path/type name. Channel identity includes channel type and all channel fields. Event identity includes concrete Event type name, exposed field-name schema, and consumer-designated identity field name/value (not all data values).
- **Persistence/framing:** Built-in JSON, NDJSON, zero-byte initial log, no header. Each channel/event/commit record has `schema_version: 1`; schema is selected per record. Versioned JSON Schema Draft 2020-12 files are canonical and embedded via `go:embed`.
- **Typed field JSON:** Strings/bools native JSON values; integers decimal strings; floats raw IEEE bit-pattern hex (float32 8 lowercase hex digits, float64 16) preserving width and signed zero; null is type-only. Strict typed variants, no extra properties. JSON Schema checks representation; Go checks integer bounds and rejects non-finite floats.
- **Transactions/sequence:** Global sequence applies to every persisted record, including commit markers. Transaction ID equals the sequence of the first transaction record. Records are ordered channel (if new), event, commit. Count includes channel/event records only; commit marker is excluded from count and checksum, but consumes its own sequence. Commit is a first-class envelope kind alongside channel and event.
- **Checksum:** SHA-256 covers exact serialized transaction-record bytes including each record's terminating newline. Commit record and its newline are excluded. Recovery applies the same framing.
- **Atomicity vs durability:** Recovery exposes only committed transactions. Interrupted trailing transaction (including partial commit) is discarded by truncating from transaction start. This recoverable atomicity is distinct from crash durability: Append is not durable until sync. Complete newline-terminated malformed records are corruption and must not be silently skipped.
- **Recovery/open:** `Open()` creates missing storage as a zero-byte log and recovers existing files. Unsupported schema versions are rejected. Incomplete trailing transaction data is rolled back; only validated committed transactions are exposed.
- **Sync:** Automatic sync every 1 second or 1 MiB, whichever comes first. `Flush()` is a durability boundary; `Close()` flushes and syncs. Append does not promise power-loss durability before sync.
- **Reader:** One replay/live stream is mandatory channel-scoped, starts with committed history, catches concurrent appends and transitions gap-free/duplicate-free to live tailing. Global sequence/cursor/offset management is internal; consumers do not supply or receive storage cursors. Context cancellation, ordered delivery, and explicit errors are required. Consumer-facing reads reconstruct the concrete consumer-defined Event struct selected by the caller's generic type argument. The caller supplies the concrete struct type to the typed read API; eventlog owns allocation, decoding, field assignment, and type verification. No registration API, prototype list, Channel factory, or consumer decoder is required. A channel may contain multiple concrete Event struct types. `Read[T]` filters the selected channel, returning matching T occurrences in channel order while advancing past occurrences of other concrete event types; nonmatching types are not errors. Malformed records and failures decoding a record identified as T remain explicit errors. User-corrected public API direction: `log.Append[TChannel, TEvent](selector)` and `log.Read[TChannel, TEvent]()` returning a reader whose `Next()` yields the reconstructed event. Append's selector callback is part of the public call. The exact callback parameter shape, selector expressions, interface method signatures, and `Next` result shape must be copied from the agreed contract, not guessed. The exact generic constraint, pointer/value convention, occurrence wrapper, and error semantics remain to be finalized.
- **Segments/index:** Segment files are authoritative; index is rebuildable. Index stores event IDs, channel associations, latest occurrence/HEAD, sequence positions, and offsets. Open validates and rebuilds absent/inconsistent index; index replacement is atomic. Fixed target segment size; rotate only at transaction boundaries, with oversized transaction allowed in its own segment.
- **Compaction:** Write temporary replacement segments, validate/checksum, sync, rename to final, atomically replace index, then remove old segments. Temporary artifacts are ignored during recovery and only cleaned after finalized source segments validate.
- **Platforms/dependencies:** Standard library only, local filesystem, Linux and Windows equal-priority supported; macOS excluded. Core behavior must have parity across both. Performance targets remain qualitative until benchmarks.
- **Initial implementation scope:** Do not begin implementation/build/tests until Define is complete and explicitly approved.

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

## Remaining open decisions (current only)

1. **Internal lifecycle and consumer value association:** Specify how the returned event/channel values from PrepareLog are associated with the internally owned Log, including how polling is stopped/retried, persistence errors are handled internally, and event/channel updates are translated into persistence without exposing a Log handle or additional public lifecycle APIs. The 100 ms default cadence and single manager-owned scheduler are approved.
2. **Public API and errors:** Consolidate exact signatures and stable error categories (validation, corruption/recovery, unsupported schema, I/O, closed state), including channel selection, context and close semantics.
3. **Benchmark scenarios and acceptance matrix:** Define measurable scenarios and qualitative limits; convert approved guarantees into verification cases.
4. **Performance/memory expectations:** Define measurable benchmark scenarios and acceptable qualitative limits without prematurely fixing numeric targets.
5. **Acceptance/verification matrix:** Convert all approved guarantees into test cases, including interrupted writes, checksum failures, sequence accounting, typed reconstruction failures, reader handoff races, index rebuild and compaction crash windows.

Items already approved (primitive allowlist, channel creation, transaction markers/checksum, versioned schema, sync policy, reader replay/live behavior, segment rotation, rebuildable index, portability) are not open decisions.

## Approved baseline (supersedes the historical proposal)

The approved design baseline is consolidated above. Earlier proposals and decision checkpoints below are retained as history, not as current instructions. If a historical statement conflicts with this baseline or a later approved checkpoint, the newer approval and this consolidated section prevail.

## Design and reconciliation approach

Reconcile the cycle artifacts first: keep one current specification and one current handoff; label dated decision checkpoints as historical evidence. Then complete the remaining Define questions, promote durable decisions into project requirements/architecture docs, and verify consistency. Implementation, build, and tests remain prohibited until Define is complete and explicitly approved.

## Ordered tasks

- [x] Establish product workload, ownership, and scope.
- [x] Define Event/Fields and persistence allowlist direction.
- [x] Approve primitive allowlist, nullable fields, validation and selector resolution.
- [x] Approve deterministic UUID v5 identity and canonical field encoding.
- [x] Approve versioned record envelope, three record kinds, typed value encoding and JSON Schema.
- [x] Approve Append transaction/commit markers, transaction IDs, checksums and recovery.
- [x] Approve sync/durability policy and zero-byte initialization.
- [x] Approve unified replay/live reader contract.
- [x] Approve segmentation, rebuildable index, compaction protocol, and supported platforms.
- [x] Reconcile stale plan/handoff descriptions and lifecycle status.
- [ ] Resolve public API signatures/error model and reader details.
- [x] Approve segment/index/manifest layout, recovery order and reader-safe compaction.
- [ ] Define performance/benchmark scenarios and test matrix.
- [x] Update requirements/architecture docs with approved decisions.
- [ ] Complete Define acceptance review; obtain explicit approval before implementation.

## Acceptance criteria

- [ ] Current approved baseline is consolidated and free of contradicting active statements.
- [ ] Historical decisions are retained but clearly subordinate to the current baseline.
- [ ] Remaining open decisions are accurate and actionable.
- [ ] Atomic recovery guarantees are distinct from unsynced durability guarantees.
- [ ] Public API, operational guarantees and test matrix are implementation-ready.
- [ ] Requirements and architecture docs capture durable approved decisions.
- [ ] Lifecycle lock, registry, plan and handoff are mutually consistent.
- [ ] No implementation/build/test begins before explicit Define approval.

## Verification strategy

- **Local checks:** Inspect available project validation and documentation format; select appropriate checks before using them.
- **CI/integration checks:** Existing lifecycle validator workflow is available; no Go implementation/test CI established yet.
- **Manual review:** Check requirements, architecture, coding standards, README and commands docs for consistency and avoid stating proposals as facts.

## Persisted phase checkpoint — reconciliation complete

- **Current phase:** define
- **Assess exit evidence:** Repo structure, steering/lifecycle rules, plan, handoff and relevant placeholder documentation inspected. Cycle is the sole in-progress standard cycle.
- **Reconciliation work:** Lock claimed and remotely confirmed. Plan contradictions consolidated, current handoff rewritten, durable requirements/architecture docs updated, and registry/lock coherence verified.
- **Next action:** Continue Define by resolving the public API/error contract and reader interface, then storage/recovery details. Reconciliation validator runs passed for the plan and handoff revisions.
- **Last updated:** 2026-11-23 (SAST).

## Current progress

- Cycle 1 is blocked in Define pending the user's decision on the Go-valid generic API surface; the execution lock has been released.
- The prior handoff's “released” checkpoint was correct for the prior session but is now superseded by the current claim.
- Approved architecture decisions are consolidated in the current baseline above.
- Reconciliation is focused on stale wording and lifecycle consistency; no implementation, build or Go tests have begun.

## Latest resume checkpoint — 2026-11-24 (blocked)

- The Cycle 1 lock was successfully claimed and confirmed on `main` in commit `22cecdebbf12095c7e26b678c5dc9276b6a1044c`.
- Requirements, architecture, plan, and handoff were reconciled to remove active `Append(channel, event)` wording and record the user's generic Log direction plus the Go language constraint.
- The lifecycle validator passed for the lock claim and the subsequent plan/handoff documentation checkpoints. The final documentation checkpoint is commit `603fe20bc6caa9c85d81907c2774288bedfbce19`; its workflow result is being checked.
- **Blocker:** Go methods cannot declare type parameters. The literal requested `log.Append[TChannel,TEvent](selector)` call is not valid Go. A generic Log handle instantiated with `TChannel` and `TEvent` is a viable candidate, exposing `log.Append(selector)` and `log.Read()`; package-level generic functions are an alternative. The user must confirm the desired call-site trade-off.
- The Append selector's actual channel/event value-input shape remains open and must be specified with a concrete consumer example.
- The cycle remains in Define; no implementation, build, or Go tests have started. The registry is now blocked and the execution lock is released pending the user's decision.

## Proposed acceptance matrix and benchmark scenarios (Define draft)

The following scenarios translate approved guarantees into observable verification cases. They are a test-plan proposal, not evidence that tests exist or have run.

### Acceptance matrix

| Area | Scenario | Expected result |
|---|---|---|
| Field validation | Empty/duplicate names, nested selectors, unsupported kinds, non-finite floats, invalid nullable values | Append rejects before any bytes or channel record are persisted |
| Primitive fidelity | Signed/unsigned bounds, named primitives, null, float32/float64 signed zero and exact bit patterns | Round-trip preserves declared primitive type and exact approved representation |
| Identity | Reorder Fields output; vary event/channel type or values | Identity is stable under field order and changes only for the approved type/value inputs |
| Append transactions | First append to a channel; later append to existing channel; repeated logical event | New channel and first event commit together; later appends omit channel record; each append is a distinct occurrence |
| Concurrent append | Many goroutines append concurrently | No interleaved transactions; global record sequence is unique and continuous |
| Validation atomicity | Invalid channel or event in a new/existing channel | No partial records, index changes, or channel creation |
| Commit/recovery | Truncate at every byte boundary in trailing channel/event/commit records | Only complete committed transactions are exposed; incomplete trailing transaction rolls back to its start |
| Corruption detection | Complete malformed NDJSON, wrong count, transaction ID, checksum, or sequence | Open reports corruption; it does not skip or silently repair complete invalid records |
| Schema versioning | Valid v1 and unsupported future record versions | v1 validates; unsupported versions fail explicitly |
| Sync/durability | Trigger 1-second and 1-MiB sync thresholds; Flush; Close | Sync occurs at either threshold; Flush/Close are durability boundaries |
| Reader replay | Existing channel history | Only committed occurrences for the requested channel are delivered in order |
| Replay/live handoff | Append during replay and exactly at the transition to waiting | Every matching occurrence is delivered once, with no gap or duplicate |
| Reader cancellation | Cancel context while replaying and while waiting | Next returns the context error promptly and releases reader resources |
| Reader lifecycle | Close reader during replay/tailing; close log with readers active | Behavior matches the approved close/wait policy; no leaked descriptors or goroutines |
| Typed reconstruction | Mixed concrete event types in one channel; matching and nonmatching records; malformed matching payload | `Read[T]` returns matching T occurrences in order, skips other concrete types while advancing the channel cursor, and reports malformed records/decoding failures rather than silently skipping corruption |
| Index rebuild | Missing, stale, malformed, or inconsistent index | Index is rebuilt from authoritative committed segments and atomically replaced |
| Manifest recovery | Missing/corrupt manifest with one versus multiple plausible complete generations | Recover only when unambiguous; otherwise fail rather than guess |
| Segment rotation | Transaction crosses target size; one transaction exceeds target | Rotate only between transactions; oversized transaction occupies its own segment |
| Compaction | Inject failure before/after each finalize/switch/delete step | Old authoritative generation remains recoverable until atomic manifest switch; temporary files are safely handled |
| Reader-safe compaction | Existing reader pins old generation while compaction switches manifest | Existing reader continues on old files; new readers use new generation; old files are deleted only after last pin releases |
| Portability | Run storage, lock, rename, sync, recovery, and compaction cases on Linux and Windows | Same core guarantees on both supported platforms |

### Benchmark scenarios

- Append throughput and latency with one/many goroutines, varying field count and event payload size.
- Append behavior across channel cardinalities and repeated logical event identities.
- Replay throughput and tailing latency for small/large histories and sparse/dense channel matches.
- Peak memory during long replay and concurrent tailing; verify memory remains bounded by reader/buffer strategy rather than total log size.
- Open/recovery time as segment count and total bytes grow, with valid index versus index rebuild.
- Compaction throughput, temporary disk-space amplification, and peak memory across representative segment sizes.

Record environment, Go version, OS/filesystem, dataset size, concurrency, and sync mode with every benchmark. Establish baselines before setting numeric performance thresholds; do not invent targets in advance.

## Completion review (not yet performed)

- [ ] Acceptance criteria verified or exceptions explicitly accepted
- [ ] Final diff reviewed
- [ ] Required verification results recorded
- [ ] Requirements and architecture documentation reconciled
- [ ] Current handoff updated
- [ ] Registry status, phase and lock updated consistently

## Historical session notes (superseded checkpoints)

The following dated notes are retained for decision history. They do not override the current approved baseline, remaining-open-decisions list, persisted phase checkpoint, or current handoff.

### Previous session handoff

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
- **Event ID (superseded historical rule):** older notes derived the UUID from the current Event type and fields. The current approved rule derives the initial UUID from Event type plus the initial exposed-field snapshot, then persists and reuses that ID for all later occurrences of the same prepared Event value, even as its fields change. Channel remains excluded.
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
- Initial identity uses package path, type name, and canonical initial fields; variable-length components must be framed length-prefixed to avoid ambiguity. Once assigned, the Event ID is stable across later field changes.
- Channel IDs derive from channel type + canonical fields. Initial Event IDs derive from event type + canonical initial fields, excluding channel association; the assigned ID is thereafter stable across field mutations for that prepared Event value.
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


## Design checkpoint — transaction checksum algorithm approved — 2026-11-23

- User approved **SHA-256** for transaction commit-marker verification.
- Compute the checksum over the exact serialized transaction record bytes, excluding the commit marker itself. The checksum is persisted in the marker and validated during recovery.
- This uses Go standard library `crypto/sha256`; no dependency added. Exact transaction record byte framing is still to be specified.
- Cycle 1 remains in Define; no implementation, build, or Go tests have started.


## Design checkpoint — sequential transaction IDs approved — 2026-11-23

- User approved **Option A: sequential transaction IDs**. Each transaction uses a monotonically increasing transaction number, aligned with the log's global sequence ordering/single-writer model.
- Transaction ID format details and precise relationship to record sequence numbers remain to be specified; IDs must not be reused.
- Cycle 1 remains in Define; no implementation, build, or Go tests have started.


## Design checkpoint — transaction IDs derived from first record sequence — 2026-11-23

- User approved **Option A**: a transaction ID equals the global sequence number of the transaction's first record.
- This avoids a separate counter, provides unique ordered transaction IDs, and allows recovery to derive transaction identity from existing sequence numbers. Example: channel record seq 41, event record seq 42, commit marker seq 43 => transaction ID 41.
- Exact transaction marker framing/schema remains open. Cycle 1 remains in Define; no implementation/build/tests have started.


## Design checkpoint — transaction record order approved — 2026-11-23

- User approved transaction record order: **channel record (if new), event record, commit marker**. Commit marker is written last. For transactions where channel already exists, only event record then commit marker are written.
- This order ensures any event's channel reference is available in transaction context before event and recovery can validate before accepting commit.
- Cycle 1 remains in Define; no implementation/build/tests have started.


## Design checkpoint — partial commit marker recovery approved — 2026-11-23

- If a commit marker is partially written, recovery treats it as an incomplete trailing transaction and truncates the log back to that transaction's start, discarding all records in the transaction (including any channel record), preserving atomicity.
- Cycle 1 remains in Define; no implementation/build/tests have started.


## Design checkpoint — Open missing log behavior approved — 2026-11-23

- User approved: `Open()` creates a new empty log and initializes its storage structure automatically if the log does not exist. Existing logs are recovered on open. No separate initialization command is required.


## Design checkpoint — empty-log initialization approved — 2026-11-23

- User approved Option A: a newly created log starts as a zero-byte file. No metadata/header record is written; the first transaction supplies versioned records. This avoids a special header record and header-specific recovery logic.

## Reconciliation audit — 2026-11-23 (current status)

The earlier audit identified ten documentation inconsistencies. Current resolution:

1. Channel API/lifecycle — **resolved**: no standalone creation; first append atomically creates channel+event.
2. Record taxonomy — **resolved**: versioned shared envelope kinds are `channel`, `event`, `commit`.
3. Float representation — **resolved**: typed value objects; float32/float64 IEEE hex preserve width and signed zero.
4. Stale open decisions/proposals — **reconciled** in the current baseline and remaining decisions list.
5. Stale checkpoints/handoff — **partially reconciled**: old checkpoints retained as historical; current handoff rewritten as the single current state.
6. Atomicity vs durability — **reconciled**: commit/recovery atomicity is separate from the periodic sync durability window.
7. Channel association — **resolved**: one channel per occurrence; global logical event identity can retain associations, and latest occurrence sets HEAD/current channel.
8. Sequence/count framing — **resolved**: every record has a global sequence; transaction ID is first transaction-record sequence; commit marker consumes sequence but is excluded from record count/checksum.
9. Compaction temporary cleanup — **resolved**: cleanup only after finalized source segments validate.
10. API drift — **partially reconciled**: constructors/validation behavior is current; exact public signatures/error model remain open.

Remaining work is substantive Define completion, not a reason to repeat already-approved decisions. No implementation/build/tests were started during this audit.

## Design checkpoint — commit markers are first-class records approved — 2026-11-23

- User approved Option A: transaction commit markers are persisted as first-class records in the shared versioned envelope, with `kind: commit` alongside `channel` and `event`. This resolves the record-taxonomy conflict: the envelope's supported kinds are now three, not two.
- Commit marker data contains the transaction ID, transaction-record count, and SHA-256 checksum of the exact serialized transaction record bytes excluding the marker. The marker itself has its own global sequence number and is not included in the transaction record count or checksum.
- The transaction ID remains the global sequence number of the first transaction record. For a new channel transaction, channel record then event record then commit marker; for an existing channel, event then commit marker. The count includes channel/event records only, not the commit marker.
- Recovery exposes transaction records only after validating the corresponding commit marker. An incomplete trailing transaction (including a partial commit marker) is truncated back to the transaction start. This decision specifies marker taxonomy/sequence accounting; precise JSON marker payload/framing remains to be finalized.
- Cycle 1 remains in Define. No implementation, build, or Go tests have started.


## Design checkpoint — JSON Schema adopted for persistence validation — 2026-11-23

- User approved adopting JSON Schema as a core design tool for the persisted JSON record format.
- Use versioned JSON Schemas (Draft 2020-12 unless subsequently revised) to define and validate the shared record envelope and the `channel`, `event`, and `commit` record structures, including required properties, types, nullability, and allowed-property constraints.
- Schema validation replaces hand-written structural validation where schema can express the constraints; it does not replace log-engine responsibilities such as global sequencing, transaction atomicity/commit-marker validation, checksums, recovery, segment/index management, concurrency, or durability.
- JSON Schema does not by itself preserve Go numeric primitive widths or exact IEEE-754 bit patterns (including signed zero). Existing approved requirements for deterministic identity and float fidelity are not silently revoked. Persisted primitive value encoding, including float representation, remains to be reconciled with the new schema approach before implementation.
- Cycle 1 remains in Define. No implementation, build, or Go tests have started.


## Design checkpoint — float persistence uses schema-validated IEEE hex strings — 2026-11-23

- Research confirmed that JSON Schema's standard numeric types are only `number` and `integer`; it cannot directly validate or guarantee Go float width or exact IEEE-754 bit preservation, including negative zero.
- User selected exact bit preservation (Option A). Persist float32 as an 8-character lowercase hexadecimal string of its raw 4 IEEE-754 bytes, and float64 as a 16-character lowercase hexadecimal string of its raw 8 IEEE-754 bytes. The byte order for this display encoding is the canonical big-endian byte order already approved for numeric encoding.
- JSON Schema validates the representation structurally: tagged float type and string value constrained to the appropriate hex pattern/length. Decoding restores exact width and bits. Hex is chosen over Base64 for inspectability.
- Example float32 negative zero: `{"type":"float32","value":"80000000"}`; float64 negative zero: `{"type":"float64","value":"8000000000000000"}`.
- This keeps JSON Schema as validation while using a lossless encoded-string representation where JSON numeric semantics are insufficient. Reject non-finite float values as already approved.
- Cycle 1 remains in Define. No implementation, build, or Go tests have started.


## Design checkpoint — uniform typed JSON field values approved — 2026-11-23

- User approved Option A: encode every persisted primitive field value using a uniform explicit typed-value object, validated through JSON Schema.
- Each value has a `type` discriminator and typed `value` representation (or type-only `null` variant). String values remain JSON strings; booleans remain JSON booleans; integer values are decimal strings to avoid consumer/parser precision loss; float32/float64 values are raw IEEE bit-pattern hex strings (8/16 lowercase hex digits) to preserve width and signed zero.
- The typed variants align with the approved primitive tag set: string, bool, int/int8/int16/int32/int64, uint/uint8/uint16/uint32/uint64, float32/float64, and null. Named primitive Go types persist using their underlying type variant.
- JSON Schema will use strict variant definitions (e.g. `oneOf`, discriminator-like `const` tags, required keys, and `additionalProperties: false`) to validate representation. Exact schema file layout and whether schemas are embedded or stored as standalone files remain open.
- This JSON representation is separate from the already-approved canonical byte encoding used for deterministic IDs; those identity tags and bytes remain unchanged.
- Cycle 1 remains in Define. No implementation, build, or Go tests have started.


## Design checkpoint — standalone and embedded JSON Schemas approved — 2026-11-23

- User approved Option C: JSON Schema files will be stored as standalone, versioned repository files and embedded into the Go package at build time.
- Standalone schema files are the canonical source; the embedded copies are derived from those same files using Go `//go:embed`, avoiding duplicate schema definitions and drift.
- Schema layout is versioned (v1), with record/value schemas organized under a dedicated schemas directory. Tooling can inspect and validate persisted records without compiling the Go library; runtime/package code can access the same schema bytes through embedding.
- Cycle 1 remains in Define. No implementation, build, or Go tests have started.


## Design checkpoint — integer range validation approved — 2026-11-23

- User approved Option A for integer validation: JSON Schema validates the decimal-string representation; Go validates exact signed/unsigned bounds for each declared primitive integer type before persistence.
- This avoids complex regex-based range schemas while preserving JSON parser interoperability and preventing out-of-range values from being persisted.
- JSON Schema checks typed integer tag, decimal string shape, required fields, and no extra properties; Go validation checks the type-specific range (including unsigned vs signed and width).
- Cycle 1 remains in Define. No implementation, build, or Go tests have started.


## Design checkpoint — JSON Schema version on every record approved — 2026-11-23

- User approved Option A: every persisted record (`channel`, `event`, and `commit`) carries `schema_version: 1` in its envelope.
- Schema selection is record-local: the version identifies which embedded/versioned JSON Schema validates that record. Unsupported future versions are rejected explicitly.
- This preserves the approved zero-byte initial log; no file header or metadata record is introduced.
- No implementation, build, or Go tests have started.

## Design checkpoint — standalone channel creation API rejected — 2026-11-23

- User approved Option A: there is no standalone `CreateChannel` operation. A channel is persisted atomically with the first successful `Append(channel, event)` that uses it; channel-only records/log entities are not created.
- `Append(channel, event)` may create a new channel implicitly as part of its transaction, or append to an already known channel. Validation of both channel and event occurs before any write. A failed append leaves no partial channel/event transaction.
- This supersedes the earlier explicit channel-creation API/lifecycle wording and resolves that conflict for the current design.
- No implementation, build, or Go tests have started.


## Design checkpoint — transaction checksum newline framing approved — 2026-11-23

- User approved Option A: the SHA-256 checksum covers the exact serialized NDJSON bytes of all transaction records, including each record's terminating newline. The commit marker and its newline are excluded from the checksum.
- For a new-channel transaction, checksum input is the channel record bytes + newline, followed by event record bytes + newline. For an existing-channel transaction, it is the event record bytes + newline.
- Recovery recomputes the checksum over the original transaction bytes using the same framing rule.
- Cycle 1 remains in Define; no implementation, build, or Go tests have started.


## Design checkpoint — unified replay/live reader approved — 2026-11-23

- User approved Option A: one unified reader stream handles historical replay and live tailing.
- The reader starts with committed history, catches up with appends during replay, then transitions into live waiting without gaps or duplicates. It uses a global sequence cursor, supports optional channel filtering, context cancellation, and explicit error reporting.
- Each appended occurrence is delivered as a distinct stream occurrence, including re-appends of a logical event. Reader delivery is ordered.
- Replay-to-live race handling is library-owned; consumers do not coordinate separate replay and subscribe APIs.
- Cycle 1 remains in Define; no implementation, build, or Go tests have started.


## Design checkpoint — transaction-boundary segment rotation approved — 2026-11-23

- User approved Option A: segment files have a fixed target size, with rotation only at transaction boundaries. Transactions are never split across segments.
- If a single transaction exceeds the target segment size, it is permitted to occupy its own oversized segment rather than being split.
- Cycle 1 remains in Define; no implementation, build, or Go tests have started.


## Design checkpoint — rebuildable index approved — 2026-11-23

- User approved Option A: segment files are the authoritative source of truth; the local index is replaceable/rebuildable and stores derived lookup metadata (event IDs, channel associations, latest occurrence/HEAD, sequence positions, segment offsets).
- `Open()` validates the index and automatically rebuilds it from validated committed segment transactions if missing or inconsistent. Index replacement is atomic; index corruption does not imply event loss.
- This complements the approved segment-first write and atomic index replacement ordering. Index/manifest exact file layout and recovery order still need definition.
- Cycle 1 remains in Define; no implementation, build, or Go tests have started.


## Design checkpoint — portability and performance assumptions — 2026-11-23

- User approved standard-library-only Go and local filesystem storage, with supported platforms restricted to Linux and Windows (not macOS).
- Linux and Windows are equal-priority supported targets at all times. Any platform-specific capability or filesystem behavior that is unsupported, unreliable, or materially different on either must use an alternative strategy that works on both. Do not accept a Linux-only or Windows-only implementation for core functionality.
- Performance targets remain qualitative until benchmarks are established. Prioritize correctness, crash recovery, predictable memory use, and cross-platform parity.
- Cycle 1 remains in Define; no implementation, build, or Go tests have started.


## Current reconciliation checkpoint — 2026-11-23 (completed)

- Execution lock claim commit: `23ccba45fd17e96b039a54d032c0fd25a6ab8621`; claim timestamp corrected to commit time in follow-up commit `41cecfbf49d0a0a62b6ce08670f4a063a3cf01a6`.
- Lock is active for Cycle 1 / standard workflow / ChatGPT GPT-6, and remote state was verified.
- Plan sections that described early APIs, two record kinds, explicit channel creation, unsynced durability as atomicity, and stale open questions were replaced or explicitly demoted to historical context.
- Current phase remains Define. No implementation, build or tests have started.
- Requirements and architecture docs updated in commits `3c87844abb9a60fc7c4b9386a498d6de208e493c` and `21db0b1b77ab2d8578e65afe7629622471d3a5a7`. Plan/handoff reconciliation is complete; validator checks passed. Next is the remaining Define design work.


## Define decision — channel-scoped reader and internal cursor (current user decisions)

- Reader filtering is mandatory: every reader targets exactly one channel. There is no all-channels or unfiltered reader.
- The consumer does not pass a channel UUID or global sequence cursor. All sequence/cursor/offset management is internal to the log.
- The user selected typed reconstruction: reads return the original concrete consumer-defined Event struct, not a generic map/Fields payload.
- The exact public signatures, registration mechanism, whether an Occurrence wrapper is necessary, and unknown-type behavior remain under Define discussion; the older UUID/cursor API below is superseded.
- Reader provides channel-scoped committed replay and gap-free transition to live tailing; context cancellation stops waiting. Detailed cancellation/error behavior remains to be finalized.


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


## Resume checkpoint — 2026-11-24

- The released execution lock was claimed in lock-only commit `7e2d3eebfd0c706ca19dd37e565ebec3bfe5693b` and fetched from shared `main`; owner is `ChatGPT-GPT-6`.
- Cycle 1 is resumed as `in-progress` in Define. No implementation, build, or Go tests have started.
- The latest approved public API and internal Log ownership are unchanged.
- User confirmed that generic type arguments identify consumer types only; runtime configuration may use a separate `eventlog.Config(...)`-style facility. Environment variables are not required. Context remains for cancellation/deadlines rather than hidden configuration.
- The exact Config call shape, configuration scope, storage directory option/default, and missing-configuration behavior remain to be designed with a concrete Go example.


## Define decision — separate runtime configuration from generic type selection (2026-11-24)

The generic arguments in `PrepareLog[TEvent, TChannel](ctx)` identify the consumer-defined event and channel types. They do not supply runtime configuration such as the storage directory. Configuration must be provided separately through a package configuration facility (a public `eventlog.Config(...)`-style API is acceptable); do not infer a storage path from the generic types and do not require environment variables as the configuration mechanism. Keep the approved `PrepareLog[TEvent, TChannel](ctx)` call shape unchanged. A Config call may explicitly bind configuration to the context while preserving its cancellation/deadline semantics; configuration must not be inferred from arbitrary context values. The exact Config signature, whether it produces a context-bound configuration or another explicit configuration value, storage-directory option name/default, and behavior when configuration is absent remain open for a concrete API example and confirmation.


## Latest resume checkpoint — 2026-11-24 — configuration direction

- User clarified that `PrepareLog[TEvent, TChannel](ctx)` already receives the consumer types through its generic arguments, and accepted a separate `eventlog.Config(...)`-style configuration facility for runtime settings.
- Do not use generic type information as a source of storage-path configuration; environment variables are not required. A Config call may explicitly bind configuration to the context while preserving cancellation/deadline semantics; do not infer configuration from arbitrary context values.
- Exact Config signature and configuration scope/defaults remain open. No implementation, build, or Go tests have started.


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

## Latest Define decision — concurrent PrepareLog callers wait (2026-11-24)

Concurrent `PrepareLog` calls wait for the first shared storage initialization attempt to complete; they do not fail with an initialization-in-progress error or initialize storage independently. Each waiting caller may cancel its own wait through its context. If initialization fails, the successful immutable configuration remains set and a later call may retry. Config remains unavailable for changes once initialization has begun.

## Define decision — storage path resolution and capture (2026-11-24)

The storage-directory option accepts a non-empty path, including a relative path. A successful `Config(...)` resolves the path against the process working directory and stores the resulting absolute path in the immutable runtime configuration. All subsequent initialization uses that captured absolute path, so a later working-directory change cannot alter the storage target. Empty or whitespace-only paths are rejected. Exact path cleaning, symlink behavior, filesystem existence/permission validation, and whether validation may inspect the filesystem remain open. Path resolution itself does not create or initialize storage; storage initialization remains part of `PrepareLog`.

## Define decision — Config option application and validation (2026-11-24)

The public configuration shape is `Config(options ...Option) error`, with exported option constructors and an unexported option application method so consumers cannot implement arbitrary mutations of internal configuration. `WithStorageDirectory(path)` is the required storage setting. Config accepts multiple distinct options, rejects nil options, rejects duplicate settings rather than applying last-option-wins, and fails if the storage-directory option is absent. Validation is completed before immutable configuration is published; a failed attempt leaves the runtime unconfigured and retryable.

## Define decision — storage directory validation (approved 2026-11-24)

`Config` rejects empty or whitespace-only storage paths. It accepts relative paths, resolves them to an absolute path at configuration time, lexically cleans the absolute result, and freezes that cleaned absolute path in immutable runtime configuration. Cleaning is equivalent to `filepath.Clean`; it removes redundant path elements but does not resolve symlinks. Config does not require the directory to exist and does not create it. During `PrepareLog`, initialization creates the directory when absent. If the target exists but is not a directory, or creation fails, initialization returns an error.


## Define decision — exported configuration errors (approved 2026-11-24)

The user approved the exported sentinel error contract: `ErrConfigNotInitialized` for PrepareLog before successful Config and `ErrConfigAlreadySet` for repeated Config or Config after initialization begins. Invalid options return descriptive validation errors; filesystem causes are wrapped and discoverable through `errors.Is`/`errors.As`; context cancellation/deadline causes remain discoverable. Failed validation leaves configuration unset and retryable. This decision closes the exported configuration error taxonomy item; implementation remains out of scope until Define is complete and explicitly approved.


## Latest Define decision — polling cadence and scheduler approved (2026-11-24)

- User approved a 100 ms default polling interval.
- One managed scheduler owned by the internal Log/change manager scans all selected event and channel fields; do not create one goroutine per field.
- The interval may be adjusted internally for implementation and focused tests. No public polling option or operational API is added.
- Polling compares selected-field snapshots and routes detected changes through internal persistence.
- This approval does not make unsynchronized concurrent struct-field writes race-safe.
- Remaining Define work: caller synchronization/snapshot consistency, event-versus-channel change semantics, scheduler shutdown/retry, and internal persistence-error handling.
- No implementation, build, or Go tests have started.


## Latest Define decision — event-field changes append occurrences (approved 2026-11-24)

- User approved that a detected change to selected Event fields creates a new persisted event occurrence.
- Previous occurrences are immutable and remain in history; polling must never overwrite an earlier occurrence.
- This approval settles occurrence creation/history retention, not logical identity. Existing identity is derived from Event type plus exposed fields, so whether changed values produce a new logical event ID or remain linked to the prior logical event, and the resulting HEAD/index behavior, still requires explicit reconciliation.
- No implementation, build, or Go tests have started.


## Define decision — stable Event ID from initial snapshot (superseded)

This earlier provisional decision is replaced by the approved consumer-designated identity-field rule. No Event-value-to-initial-snapshot ID association is needed.


## Latest Define decision — logical Event identity vs occurrence (approved 2026-11-24)

- The user approved that logical identity and event occurrence are distinct.
- Event ID derives from concrete Go Event type name, exposed `Fields()` field-name schema, and the consumer-designated identity field name/value; inputs are canonically encoded and yield deterministic UUID v5.
- The identity field identifies the entity (e.g., Order 501); changes in other exposed data (e.g., status created → paid) are new occurrences under the same logical Event ID.
- Occurrences are append-only and immutable; previous history is retained, newest occurrence becomes HEAD/current state.
- Physical Event and Channel records are deduplicated; identical physical data must be stored once and referenced/reused.
- No implementation/build/test has begun. This supersedes conflicting provisional stable-ID/full-field identity decisions above; reconcile active text and historical notes in Define.


## Define decision — internal-only synchronization (approved 2026-11-24)

- Preserve the existing public API; do not introduce any lock, mutex, synchronization interface, or configuration option for consumer-field mutation.
- eventlog's synchronization remains internal and covers its own scheduler, change detection, persistence, and lifecycle coordination.
- Consumers are responsible for synchronizing their own mutations of returned Event and Channel fields when concurrent access occurs. The 100 ms polling mechanism does not make unsynchronized ordinary Go struct field writes race-safe and must not be documented as doing so.
- This decision rejects the proposed consumer-shared-lock configuration option and introduces no public API.

## Define decision — canonical Channel identity and mutation (approved 2026-11-24)

- Channel identity uses the same canonical determination as Event identity: concrete type, exposed `Fields()` schema, designated identity field name/value, canonical encoding, and deterministic UUID v5.
- Non-identity Channel-field changes do not alter logical identity; Channel-only changes do not create Event occurrences. New Event occurrences use the latest Channel snapshot, while history associations remain unchanged.
- Deduplicate canonical Channel records and create/reuse them atomically with the first successful occurrence referencing them.

## Define topic — identity-field mutation on prepared instances (principle approved; details open)

Consumer changes to the designated identity field on Event or Channel instances returned by `PrepareLog` must be handled internally by eventlog. The exact detection and transition semantics (including occurrence creation, HEAD/index updates, channel association, and persistence) remain a Define task. No implementation/build/tests have started.

## Define decision — Channel notification direction (approved)

- Public Channel API will expose a notification Go channel through an accessor method (name/signature TBD).
- Successful occurrence notifications carry the concrete consumer Event struct instance, rather than IDs-only or a generic event-type payload.
- Errors are typed to support `errors.Is` and `errors.As`.
- Slow/unread notification consumers must not block persistence or the internal poll scheduler; delivery/buffering strategy remains to be specified.
- Close the notification channel when the relevant lifecycle shuts down; close it on initialization failure if it was allocated.
- API backward compatibility is not a constraint in this early Define phase.
- Identity-violation handling remains an open design discussion.
