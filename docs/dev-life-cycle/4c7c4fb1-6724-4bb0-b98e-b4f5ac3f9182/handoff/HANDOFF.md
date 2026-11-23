# Development Cycle Handoff

## Cycle objective and status

- **Cycle UUID:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182
- **Objective:** Define eventlog's initial product requirements and core design decisions before implementation.
- **Registry status:** in-progress
- **Persisted phase:** define
- **Last updated:** 2026-11-22T16:25:00Z

## Current repository state

- **Branch/revision:** main; sequential remote commits through GitHub contents API.
- **Working tree:** No local checkout used; no working-tree status available.
- **Collaboration state:** Cycle 1 resumed. Execution lock active and verified on shared main for ChatGPT.
- **Relevant environment:** No Go source or test suite discovered; project-specific commands remain unverified.

## Completed work

- Inspected repository metadata and tree, README, steering policy, lifecycle README, lock and registry schemas/state, plan/handoff templates, requirements, architecture, coding standards, commands documentation, and workflows.
- Initial assessment checkpoint persisted; cycle paused while awaiting product direction.
- User confirmed general-purpose embedded event storage.
- User confirmed single-process ownership; concurrent goroutines may be coordinated inside the library.
- Confirmed typed generic event API direction: consumers implement a library-owned Event interface returning eventlog.Fields.
- Confirmed consumer event structs remain unrestricted and may contain runtime-only or sensitive fields, but eventlog persists only the explicitly returned Fields.
- Confirmed Fields holds heterogeneous primitive values and FieldOf constrains selected values to the approved primitive set.
- Confirmed field identity is derived from a typed selector rather than a hand-written string; reflection may be needed internally.
- Confirmed named types are supported when their underlying type remains within the primitive allowlist (e.g. UserID string, AccountBalance int64, IsActive bool). Exact allowlist still needs final confirmation.
- Confirmed built-in JSON serialization for v1; custom codecs/encoders/decoders are not in v1 scope.
- Confirmed newline-delimited JSON (NDJSON): one JSON record per line, terminated by a newline.
- User approved recovery semantics: truncate an unterminated trailing suffix back to the last complete newline; fail with a corruption error on invalid newline-terminated records; never skip a damaged record to continue replay.
- The previous redaction design is superseded by the explicit Event/Fields allowlist contract; eventlog does not inspect or redact source event structs.
- Cycle 1 is paused for session handoff on main in define phase; no implementation started.

## Confirmed decisions and constraints

- General-purpose embedded storage, independent of event/domain semantics.
- Single process owns a log at a time; library may coordinate concurrent goroutines.
- Consumer-defined event structs remain unconstrained; only their Event method's returned Fields are persisted.
- Library Event interface and Fields container form the persistence allowlist.
- Generic FieldOf constructor limits each selected value to primitive types; named primitive types are supported when their underlying type is in the allowlist.
- Avoid manually supplied string field names; derive names from typed selectors.
- Built-in JSON serialization; no custom codec in initial scope.
- NDJSON record framing: one JSON record followed by a newline.
- Recovery: truncate an unterminated final suffix to the last newline; a malformed newline-terminated record is a corruption error; do not skip corruption.
- Earlier redaction feature is superseded; consumers explicitly expose only intended fields.

## Proposed / open

- Exact primitive allowlist and nullable value behaviour.
- Exact Channel interface/API and explicit create operation signatures; channel type identity encoding and channel read/query semantics.
- Exact Go selector implementation, field-name resolution, and API error behaviour.
- Record envelope/schema metadata and format-version strategy, including channel records and mandatory event channel references.
- Sequence numbering, record identity, and ordering.
- Durability/sync defaults and guarantees.
- Replay/tailing API, cancellation, and replay-to-live behaviour.
- Segmentation/rotation, record limits, portability, and performance goals.

## Verification evidence

- Cycle lock claim and resume previously fetched back and confirmed.
- Previous plan checkpoint was persisted; the latest plan update records the confirmed Event/Fields/FieldOf, JSON, and NDJSON decisions.
- No implementation, build, or Go tests were run because the cycle remains in define.
- The lifecycle validator has not yet been run in this checkpoint.

## Documentation state

- Cycle plan updated with confirmed API and NDJSON decisions; stale redaction wording removed.
- Handoff updated to reflect current decisions and remaining open questions.
- Requirements and architecture documents remain placeholders and will be reconciled after the operational contract is finalized.
- No implementation or Go validation performed.

## Persisted phase checkpoint

- **Current phase:** define
- **Assess exit evidence:** Repository governance and initial state reviewed; general-purpose scope, single-process ownership, explicit typed persistence allowlist, built-in JSON, NDJSON framing, and recovery semantics confirmed.
- **Next concrete action:** Resolve the primitive allowlist and operational guarantees: record envelope/versioning, durability, and replay/tailing semantics.

## Uncommitted or unpushed work

- None known; remote contents API used.

## Continuation instructions

Cycle 1 is active in the define phase. The execution lock is claimed and verified on remote main for ChatGPT. Continue the design discussion; do not begin implementation until design decisions and acceptance criteria are ready.

## Unexpected-worker termination recovery checkpoint

- **Checkpointed at:** 2026-11-22T15:23:21.602Z
- **Reason:** The prior worker did not release the active execution lock. The user explicitly confirmed that the prior worker has terminated and authorized lock recovery.
- **Persisted phase:** define (unchanged).
- **Work completed at the last durable checkpoint:** General-purpose embedded storage scope, single-process ownership, typed Event/Fields allowlist persistence, generic field-selector direction, and built-in JSON for v1 were recorded as confirmed direction. The plan's checked tasks remain the authoritative progress record.
- **Implementation state:** No implementation was started. No build or tests were run; this is consistent with the define phase.
- **Outstanding work:** Finalize Event/Fields/FieldOf API and supported primitive representation; decide framing, durability, recovery, concurrency and tailing behavior; reconcile approved requirements and architecture documentation.
- **Next action:** Resume cycle 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182 in the define phase after a fresh lock claim is published and confirmed on shared main.
- **Recovery disposition:** Cycle temporarily marked paused while the terminated worker's lock is released; cycle identity, sequence, execution order, timestamps, and phase are preserved.

## Recovery completed and cycle resumed

- **Resumed at:** 2026-11-22T15:24:15.040Z
- The user explicitly confirmed the previous worker had terminated and authorized recovery.
- The previous cycle state was checkpointed; the registry was temporarily set to paused; the stale lock was released and confirmed on shared main.
- A fresh active lock for this same cycle was committed and fetched back from shared main before the registry was returned to in-progress.
- **Current owner:** ChatGPT; **current phase:** define; **implementation:** not started.
- Continue by resolving the outstanding API and storage design questions listed above. Do not begin implementation until the critical decisions and acceptance criteria are ready.

## Latest design checkpoint

- **Updated:** 2026-11-22T16:00:00Z
- User approved NDJSON framing: each event is one JSON record followed by a newline.
- The cycle plan now records the confirmed Event/Fields/FieldOf API direction, named primitive type support, built-in JSON, and NDJSON.
- User approved recovery semantics: truncate an incomplete unterminated final suffix; fail on malformed complete records without skipping them.
- The exact primitive allowlist, record envelope/versioning, durability, and replay/tailing behaviour remain undecided.


## Session handoff checkpoint

- **Paused at:** 2026-11-22T15:36:55.080Z
- User requested to continue development in a new session and explicitly asked to unlock the project.
- Current durable decisions: general-purpose embedded event log; single-process ownership with goroutine concurrency; Event/Fields allowlist; primitive-only selected values including named primitive types; built-in JSON; NDJSON framing; truncate incomplete unterminated final suffix; fail on malformed complete records without skipping.
- No implementation, build, or Go tests have started.
- **Unresolved next discussion:** durability policy. The previous session was asked whether to sync every append or buffer and sync periodically; the user has not answered yet.
- Other open topics: exact primitive allowlist and selector behaviour, record envelope/versioning, ordering, replay/tailing API, portability/performance, and docs reconciliation.
- **Next action:** new session claims execution lock, reads this handoff and plan, then continues with the durability decision.


## New-session resume checkpoint — 2026-11-22T15:37:30Z

- The released lock was claimed on `main` by ChatGPT in lock-only commit `c9b6f4e803ad0cb01f2e6585d3ec3f7630dc4c2b`; the active lock was fetched back and confirmed.
- The cycle registry was then changed from `paused` to `in-progress` in commit `fdb815fcb587cd1303a562f6e64c4881162c29c9`. The cycle remains in `define`.
- User's latest API clarification: retain the simple selector approach; selector values must be primitive. A selector for a nested struct such as `*Address` is unsupported; do not add complexity to support it.
- No implementation, build, or Go tests have started.
- The previous checkpoint says the next unresolved decision is durability: sync each append versus buffered/periodic sync. The intervening short agreement replies are not mapped to a specific proposal in the persisted record, so do not infer a durability choice from them.
- Continue the define phase by confirming durability, then resolve remaining record envelope/versioning and replay/tailing guarantees before updating product requirements and architecture.


## Latest session decisions — 2026-11-22

- User selected **buffered/periodic sync** rather than syncing every append. The sync cadence/trigger and durability guarantees still need definition.
- User introduced **channels** as a new core concept: each event needs a channel, and a channel groups related events even if their Go struct types differ. Example: different user lifecycle event structs can belong to one logical users channel.
- Treat channel as an explicit event/logging concept, not as a Go event struct type. Its identifier representation, whether it is supplied per append or by an event, filtering/replay semantics, and relationship to record ordering remain undecided.
- Revisit the record envelope and reader API with channels in scope.
- Existing decisions remain: primitive-only FieldOf selectors; Event/Fields allowlist; JSON/NDJSON; truncate only an incomplete unterminated suffix and fail on malformed complete records.
- No implementation, build, or Go tests have started.
- **Next discussion:** clarify what a channel represents operationally and how callers associate an event with one.


## First-class channel decision — 2026-11-22

- A channel is a persisted entity, not merely a string label or append-time grouping parameter.
- Consumers represent channels with their own structs implementing a library-owned `Channel` interface analogous to `Event`; both expose `Fields() Fields` and use the same primitive-only field-selection mechanism.
- Channel creation is explicit and returns the generated channel ID after persisting its channel record.
- Events require an existing channel. Every event record references the channel ID; channel-less events and implicit channel creation are disallowed.
- Conceptual record shapes: channel `{id, type, fields}`; event `{id, channel: {id}, type, fields}`. Exact schema and API signatures remain open.
- Next: settle record envelope/versioning and channel/event API details, then reconcile requirements and architecture docs. No implementation has started.


## Current session handoff — 2026-11-22

- User requested the project be unlocked to move development to a new session.
- Cycle 1 remains in `define`; no implementation, build, or Go tests have started.
- Latest confirmed channel design:
  - A channel is a first-class persisted entity represented by a consumer-defined struct implementing a library-owned `Channel` interface analogous to `Event`.
  - Both `Channel` and `Event` expose `Fields() Fields`; both use the same primitive-only field mechanism.
  - Channel creation is explicit, persists a channel record, and returns its generated ID.
  - Every event must reference exactly one existing channel ID. Channel-less events and implicit channel creation are forbidden.
  - Conceptual record shapes are channel `{id, type, fields}` and event `{id, channel: {id}, type, fields}`; exact schema and API signatures remain open.
- Buffered/periodic sync is confirmed. Exact cadence/trigger, flush API, close semantics, and crash-loss window remain open.
- Other open decisions include the primitive allowlist, selector edge cases, record envelope/versioning and ordering, replay/tailing semantics, rotation/limits, portability, and performance goals.
- **Next discussion:** decide how the generated channel ID is passed to the event append operation.
- **Resume instruction:** claim the execution lock before continuing this cycle.


## Resume checkpoint — 2026-11-22T16:25:00Z

- The execution lock was claimed in lock-only commit c8020efbb2bdeaa63cfbb2fd40fae6e3ce2b824c and fetched back from `main`; the active lock names ChatGPT and this cycle.
- The registry was resumed to `in-progress` in commit 3b45d73a949d6a0c0311390e54c42aff90d286bb. The cycle remains in `define`.
- No local checkout or working-tree state is available in this session; repository reads and writes are being performed against the remote `main` branch through GitHub.
- No implementation, build, or Go tests have started.
- Latest user decisions remain: Channel is a struct/interface analogous to Event; channel creation is explicit and returns its generated ID; every event requires an existing channel; selectors remain primitive-only.
- **Next discussion:** settle how the generated channel ID is supplied to the event append operation before finalizing record envelope/versioning and replay/tailing semantics.


## Session handoff — 2026-11-22T19:26:00+02:00

- User requested the project be unlocked to continue in a new session. The execution lock has been released on `main`; the cycle registry is to be marked paused for handoff. The new session must claim the lock before continuing.
- Cycle remains in `define`. No implementation, build, or Go tests have started.
- User confirmed Go package path and struct type name may define type identity for deterministic IDs. Renaming/moving a type changes its identity and is a persisted compatibility concern.
- **Deterministic Channel ID:** UUID from channel type identity plus canonical representation of all `Channel.Fields()` values.
- **Deterministic Event ID:** UUID from event type identity plus canonical representation of all `Event.Fields()` values. Channel ID is not part of Event ID.
- **API direction:** keep `log.Append(channel, event)`; pass channel and event together. Channel ID is calculated internally.
- **Duplicate handling:** Event ID is global across channels. Re-appending an existing event does not create another logical event; it promotes the existing event to HEAD of the entire log. The event can be associated with multiple channels; user explicitly chose association with both channels when the same event is appended under another channel.
- **Storage direction:** logical global ordering is separate from physical segment placement. Segment files alone must reconstruct the complete log, including promotions and channel associations. The index is derived/rebuildable, not authoritative.
- **Initial index:** use a local index file rather than SQLite. Keep the index replaceable so SQLite or another database can be considered later.
- **Segmentation:** user suggested splitting the segment at the existing event, moving the suffix into a new active segment, and scavenging obsolete copies from the prior segment. Alternative discussed: persist a logical promotion operation first, then compact/scavenge asynchronously. Exact algorithm, crash protocol, and performance trade-offs remain open.
- **Checksums:** checksums and references to segment files were proposed; decide record-level versus segment-level coverage and metadata format later.
- **Next-session focus:** define canonical serialization/type identity and UUID derivation; specify promotion and channel-association records; specify crash-safe segment rotation/compaction/scavenging; define the rebuildable index file format and recovery procedure. Then finish primitive allowlist, envelope/versioning, durability, replay/tailing and acceptance criteria.
- **Operational note:** current work was performed through GitHub remote file operations on `main`; there is no local checkout/working-tree status or Go test evidence from this session.


## Lifecycle reconciliation checkpoint — 2026-11-22

- **Disposition:** The prior release request is honored; Cycle 1 is **paused** in `define` and the execution lock remains released.
- **Reason:** Remote registry had Cycle 1 marked `in-progress` while `execution-lock.json` was `unlocked`, violating the lifecycle invariant.
- **Repair:** Preserve the same cycle UUID, sequence, execution order, start/created timestamps, and phase. Update only the registry status to `paused`; status_reason remains null (pause context is recorded here).
- **Latest durable design context:** Continue from the newest decision checkpoints in the plan. Prior handoff sections contain superseded snapshots; the plan's latest decisions are the active design checkpoint. User-approved decisions include deterministic IDs, append(channel,event), cross-channel event association/promotion, local rebuildable index, finite floats with bit-preserving signed zero, nullable primitive fields, strict validation, and record schema version 1.
- **No implementation or tests:** No source code changes, builds, or Go tests have been performed.
- **Resume protocol:** Before ordinary development, read the lock first. It is released. Read lifecycle docs, validate the registry, then claim the lock on main and confirm the published claim before any ordinary project changes.


## Session handoff — 2026-11-22T18:51:00Z

- User requested unlock for transition to a new session.
- Cycle 1 remains in **define**. No implementation, local checkout, build, or Go tests have started.
- Decisions added in this session and persisted in the plan: canonical identity bytes (UTF-8 bytewise field sorting, type tags, length-prefix encoding, width preservation, IEEE float bit preservation incl signed zero, package path+struct name); UUID v5 namespace-based SHA-1 with separate domain prefixes for channel/event; segments are source of truth, local index rebuildable; duplicate physical event occurrences allowed; latest occurrence promotes logical HEAD and can associate the event with multiple channels without separate promotion records; segment-first then index update; atomic index replacement; global monotonic sequence on every persisted record, stored in segments.
- The compaction sequence is still only a proposal and **not approved**: write compacted temp segments, sync, rename, atomically update index, delete obsolete segments last.
- Outstanding define decisions: exact canonical byte layout and fixed namespace UUID, durable promotion/channel-association semantics and index format, crash-safe compaction/recovery protocol, sync cadence/flush/close guarantees, primitive allowlist, envelope/versioning, replay/tailing and acceptance criteria.
- Next session should read execution-lock first; lock will be released and Cycle 1 marked paused. Resume Cycle 1 in define through the required claim protocol.


## Resume checkpoint — sync policy accepted — 2026-11-23T02:16:10+02:00

- Execution lock claimed and cycle 1 resumed from paused to in-progress; commits were fetched back through GitHub API.
- User accepted buffered/periodic synchronization: sync every 1 second or when buffered data reaches 1 MiB, whichever occurs first.
- `Flush()` explicitly flushes and forces sync; `Close()` attempts final flush/sync; `Append()` is not crash-durable until sync completes, so recent data may be lost after a crash.
- No implementation, build, or Go tests started.
- Continue define phase: next settle record envelope/taxonomy and promotion/channel-association semantics. Existing plan/handoff latest checkpoints remain authoritative; old handoff entries are historical snapshots.
