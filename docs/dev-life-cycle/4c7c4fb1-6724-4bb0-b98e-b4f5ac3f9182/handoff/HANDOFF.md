# Cycle 1 — Session handoff checkpoint (2026-11-24)

## Latest session checkpoint — execution lock reclaimed

- The previous released lock was structurally invalid: it retained non-null owner fields despite `status: released`. Repaired it to the schema-valid released shape in lock-only commit `9ab19931586cca2be42fcabe4b77d6fb59143656`.
- Claimed Cycle 1 with a separate lock-only commit `33603d1e87fad469555837a30e6b146f849d6cd6`.
- Re-fetched shared `main` and confirmed that exact claim commit is the remote head and `execution-lock.json` is active for this cycle, workflow `standard`, worker `ChatGPT-GPT-6`.
- The approved configuration error contract has since been recorded in requirements, architecture, plan, and this handoff; see the latest session checkpoint below.
- Registry remains `in-progress`, phase `define`, matching the active lock.
- Documentation commits are on shared `main`; latest confirmed head is `377fe94bcf2389f07d25e42bc6ca39013d6cf127`.
- Steering validation was queued for intermediate documentation commit `ca51f64a27c462c1aa448bd20ffe389d53195747`; no validation run for the latest head was found at the time of this checkpoint.
- No implementation, build, or Go tests have started.

**Cycle ID:** `4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182`
**Workflow:** Standard
**Phase:** Define
**Cycle status:** In progress
**Execution lock:** Active and confirmed on shared `main` at claim commit `33603d1e87fad469555837a30e6b146f849d6cd6`.

## Current approved public API

The only approved preparation entry point remains:

```go
event, channel, err := eventlog.PrepareLog[MyEvent, MyChannel](ctx)
```

The generic arguments select consumer-defined event and channel types. The package owns the internal Log and all persistence, append/read, recovery, synchronization, and lifecycle machinery. Consumers do not receive a Log handle or manage its lifecycle.

## Approved configuration contract

The user has approved the following direction and rules:

- Configuration is process-level initialization state, not context-bound state or mutable runtime state.
- A separate package-level `eventlog.Config(...)` call is required before `PrepareLog`.
- Proposed public shape: `Config(options ...Option) error`, with `WithStorageDirectory(path string) Option`. Option application is package-controlled through an unexported method.
- Storage-directory option is mandatory. Calling `Config()` without it fails.
- Nil options and duplicate settings fail. There is no last-option-wins behavior.
- Failed configuration validation leaves the runtime unconfigured and allows retry.
- A successful Config validates, captures, and freezes configuration exactly once. A second Config call, or one after initialization has begun, fails and cannot mutate the active configuration.
- `PrepareLog` before successful Config fails with a configuration-not-initialized error. There is no default directory and no environment-variable fallback.
- Relative storage paths are accepted, resolved to absolute paths during Config, lexically cleaned using `filepath.Clean` semantics, and frozen. Symlinks are not resolved.
- Config does not require the directory to exist and does not create it. PrepareLog creates a missing directory. An existing non-directory target or a directory-creation failure is an error.
- Concurrent PrepareLog calls wait for the first shared initialization attempt. Each waiter can cancel its own wait through its context. If initialization fails, frozen configuration remains set and a later PrepareLog call may retry.
- `context.Context` is used for cancellation/deadlines, not as a configuration carrier.

## Latest approved error contract

The exported sentinels and semantics are approved and recorded below in the latest session checkpoint. This is no longer an open decision.

## Remaining Define work

1. Continue specifying the internal initialization lifecycle and concurrency contract.
2. Finalize remaining API, cancellation, reader/resource lifecycle, and terminal error semantics.
3. Review benchmark scenarios, memory/performance targets, and the acceptance test matrix.
4. Reconcile all active requirements, architecture, plan, and handoff text; resolve stale or contradictory historical material.
5. Complete the Define consistency review and obtain explicit user approval before implementation.
3. Reconcile all active requirements, architecture, plan, and handoff text; older contradictory checkpoint notes below are historical and superseded by the current approved decisions above.
4. Finish the remaining API, lifecycle, cancellation, resource, benchmark, and acceptance criteria.
5. Complete the Define consistency review and obtain explicit user approval before implementation.

## Repository and verification state

- Latest lock release is a lock-only commit: `87da7743911d31420eee2a203c71ae27b05b0a59`.
- The lock was released at `2026-11-24T00:56:00Z` for the next session to claim.
- Cycle registry remains `in-progress`, current phase `define`; the cycle is paused, not completed or cancelled.
- No source implementation, build, or Go tests have started.
- Next session: re-fetch shared `main`, verify lock is released, claim it with a lock-only commit, then continue Define from the error-contract decision above.

---

# Cycle 1 — Current Handoff

**Cycle ID:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182  
**Workflow:** Standard  
**Phase:** Define  
**Status:** Paused for session handoff  
**Execution lock:** Released for next session; lock-only release commit `87da7743911d31420eee2a203c71ae27b05b0a59`.

## Latest checkpoint — 2026-11-24 — configuration direction

- User confirmed generic arguments identify the consumer Event and Channel types; runtime settings such as storage location must be configured separately.
- A separate `eventlog.Config(...)`-style API is acceptable. Environment-variable configuration is not required.
- Keep `PrepareLog[TEvent, TChannel](ctx)` unchanged. A Config call may explicitly bind configuration to the context while preserving cancellation/deadline semantics; do not infer configuration from arbitrary context values.
- Exact Config signature, whether configuration is context-bound or represented explicitly, storage-directory option/default, and missing-config behavior remain open.
- Execution lock remains active; no implementation, build, or Go tests have started.

## Latest checkpoint — 2026-11-24 — resumed session

- Re-read the released execution lock first, then resumed this same Cycle 1 from Define.
- Claimed the lock with a lock-only commit: `7e2d3eebfd0c706ca19dd37e565ebec3bfe5693b`.
- Fetched `docs/dev-life-cycle/execution-lock.json` from `main` and confirmed it remains active for cycle `4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182`, workflow `standard`, worker `ChatGPT-GPT-6`.
- Updated the registry to `in-progress` and reconciled the plan's active API statement.
- The sole approved public API remains `PrepareLog[TEvent, TChannel](ctx) (TEvent, TChannel, error)`; the internal `Log` owns all operations and lifecycle.
- Next decision: storage-location selection and the role of `context.Context` for the parameterless storage configuration.
- No implementation, build, or Go tests have started.

## Latest checkpoint — 2026-11-24 — session handoff

### Approved public API

The user explicitly approved exactly one public API at this stage:

```go
event, channel, err := eventlog.PrepareLog[MyEvent, MyChannel](ctx)
```

Declaration:

```go
func PrepareLog[TEvent EventType, TChannel ChannelType](ctx context.Context) (TEvent, TChannel, error)
```

Constraints:

```go
type ChannelType interface { Fields() Fields }
type EventType interface { Fields() Fields }
```

### Agreed architecture and constraints

- `Log` is an internal struct owned by `eventlog`; consumers never receive or manage a Log handle.
- The internal Log manages append, read, next, synchronization, recovery, and the complete lifecycle. No other public append/read/next/close/flush/compact API is approved at this stage.
- Consumers receive typed event and channel values and set/change their fields. Internal routines use `Fields()` to inspect field changes.
- A single process owns a log at a time. Multiple goroutines within that process may use the API concurrently. This does not impose one global log across all processes or storage locations.
- Existing persistence, transaction, recovery, segmented storage, manifest, rebuildable index, reader-safe compaction, and Linux/Windows constraints remain in force unless separately changed.

### Work completed

- Updated `docs/project/requirements.md` and `docs/project/architecture.md` to record the approved single public API and internal Log ownership.
- Updated the Cycle 1 plan and handoff with the same decision.
- Updated the cycle registry from blocked to in-progress when Define resumed.
- No source implementation, build, or Go tests have started.

### Outstanding work

- Specify how `PrepareLog` identifies its storage location and how context participates in initialization/recovery. The prior design used `Open(path)`, but that API is superseded; no replacement mechanism has been approved.
- Specify how the returned event/channel values are associated with the internally owned Log.
- Finish internal lifecycle, validation, cancellation, resource, error, benchmark, and acceptance criteria.
- Complete the Define consistency review and obtain explicit approval before implementation.

### Verification state

- Documentation commits `27b72129f26c3d4f3c871dd6782053dca97c3e99` and `a905772aeaa52d17c602969d8bed0545ddef2e1b` are on `main`.
- No implementation or test verification has been performed.
- This session handoff pauses Cycle 1 in Define; it does not complete or cancel the cycle.

### Resume instructions

Resume this same standard cycle (ID `4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182`) from Define. Follow `docs/dev-life-cycle/README.md`: confirm the execution lock is released, then claim the lock for this cycle with a lock-only commit on shared `main` before changing the registry or project files. The next design task is to resolve storage-location selection for the sole `PrepareLog(ctx)` API without introducing another public API unless the user explicitly approves it.

---

## Historical handoff (superseded API proposals retained for traceability)

## Latest checkpoint

## Resume checkpoint — 2026-11-24

- User instructed continuation of Cycle 1 in Define. The execution lock was claimed in a lock-only commit, `22cecdebbf12095c7e26b678c5dc9276b6a1044c`, and the remote `main` branch was fetched back to confirm that exact head and active lock owner.
- GitHub Actions `Steering state validation` run `45294380997` completed successfully for that exact commit. The `Validate cycle registry and execution lock` step passed.
- The prior release commit's validation run `45294221825` failed; the subsequent active-lock claim restored registry/lock consistency and the new validation passed.
- No implementation, build, or Go tests have started.

## Session handoff — public generic Log contract (2026-11-24)

The user has explicitly established the public API interface as the contract that all internal implementations must support:

```go
type Log[TChannel Channel, TEvent Event] interface {
    // Public operations are to be finalized against this generic interface.
}
```

The intended call shapes are:

```go
log.Append[TChannel, TEvent](selector)
reader := log.Read[TChannel, TEvent]()
reader.Next()
```

Important constraints from the user's latest clarification:

- This is the public API contract, not an illustrative internal design. Internal storage, reconstruction, indexes, readers, and implementation choices must conform to it.
- Consumer Event structs implement the library's generic Event interface in relation to their Channel type. Example: `OrderApproved[TOrderChannel]`.
- The Log interface must enforce the Channel/Event type relationship; the user specifically proposed `Log[TChannel Channel, TEvent Event]`.
- Append's selector callback is part of the public Append call. Do not replace it with `Append(channel, event)`, and do not relocate the selector contract solely into `Fields()`.
- Channels may contain multiple concrete Event struct types.
- Read is channel-scoped and typed by both TChannel and TEvent; the reader advances through the channel and yields the requested event type. The exact filtering/typing semantics should be walked through with examples in the next session rather than inferred further.
- The exact declarations of `Channel`, `Event[TChannel]`, `Log[TChannel,TEvent]`, the selector callback, and the method signatures are not yet fully written down. Next session should start from concrete Go examples and derive the interfaces together. The interface sketch above is a candidate shape, not an approved signature.
- Do not make further API substitutions or start implementation before that discussion and explicit Define approval.
- Go language constraint to discuss carefully: ordinary methods on a non-generic receiver cannot introduce their own type parameters. The user has nevertheless stated the public interface shape above as the required contract; explore a valid Go encoding that preserves the intended consumer-facing API rather than changing the contract unilaterally.

The previous handoff text suggesting `Read[T Event](ctx, log, channel)` or treating only TEvent as the read type is superseded by this latest user decision.


- The user explicitly instructed continuation of Cycle 1. The registry is `in-progress`; the current phase remains `define`.
- The current approved baseline is consolidated in `plan/PLAN.md`: append-only local event log; one process owner; concurrent goroutine access; typed primitive `Fields()`; deterministic UUID v5 identities; versioned NDJSON records; commit/checksum/recovery rules; periodic sync; segmented storage, rebuildable index, manifest, and reader-pinned compaction; standard-library-only Linux/Windows support.
- Every reader is scoped to exactly one consumer-provided `Channel` value. Consumers do not supply or receive channel UUIDs, global sequence numbers, cursors, or storage offsets. Those remain internal.
- Reads reconstruct the original concrete consumer-defined Event type internally. A separate/global `Register[T]()` API is rejected. User corrected the public API direction to `log.Append[TChannel,TEvent](selector)` and `log.Read[TChannel,TEvent]()` followed by `reader.Next()`. Append's selector callback is part of the public call and explicitly selects fields; do not reduce Append to `Append(channel,event)` or move selectors exclusively into `Fields()`.
- **User's clarified reconstruction requirement:** the consumer supplies the concrete generic struct type to the read API; eventlog owns allocation, decoding, field assignment, and persisted-type verification. No separate registration API, prototype list, Channel factory, or consumer decoder is wanted. A channel may contain multiple concrete Event struct types. `Read[T]` is a typed filter over the selected channel: it returns matching T occurrences in order and advances past occurrences of other concrete types; those nonmatching types are not errors. Malformed records and failures decoding a record identified as T are explicit errors. Proposed shape: `Read[T Event](ctx, log, channel) (*Reader[T], error)` with `Next() (T, error)`. Exact generic constraint, pointer/value convention, and occurrence wrapper remain open.
- Persisted records of a different concrete event type are skipped by a typed reader, not treated as unknown-type errors. Structural corruption and malformed fields for a matching event type remain explicit errors.
- The requirements and architecture docs were reconciled to the approved channel-value/internal-cursor contract. No implementation, build, or Go tests have started.

## Current API direction (not finalized)

```go
func Open(path string) (*Log, error)
func (l *Log) Append[TChannel, TEvent](selector /* exact callback contract to transcribe */)
func (l *Log) Read[TChannel, TEvent]() /* reader */
func (r *Reader[...]) Next() /* reconstructed event */
func (r *Reader[...]) Close() error
```

The generic API direction above preserves the user's proposed generic Log contract as a candidate. Go does not permit methods to declare their own type parameters; the generic handle approach keeps the Channel/Event relationship at compile time but changes the call to `log.Append(selector)` / `log.Read()` after generic construction. A package-level generic API is another option. Resolve the call-site trade-off explicitly before finalizing the public contract. Do not substitute the former non-generic `Append(channel, event)` contract. The selector callback's exact type and body must be recovered from the agreed selector design rather than guessed.

The generic read shape is illustrative; whether `Read` is a package function or a `Log` method depends on Go's generic-method constraints and the final API layout. The consumer supplies the concrete event type as T; eventlog creates and decodes values internally. Finalize whether T is the pointer type (likely, if `Fields()` has a pointer receiver), how the persisted type key is matched, and the occurrence wrapper. A channel may contain other event types; the reader skips those while scanning. Also finalize nil validation, context cancellation, terminal errors, close semantics, Flush/Compact, and the stable error taxonomy. Do not implement until Define is complete and approved.

## Remaining Define work

1. Finalize the generic read contract, pointer/value convention, and whether Next returns T directly or an Occurrence wrapper. Heterogeneous channels and filtering of non-T occurrences are approved.
2. Finalize Event/Channel/Fields contracts and public Append/Read/Next/Close signatures.
3. Define stable errors, cancellation, reader termination, and resource lifecycle.
4. Define benchmark scenarios and memory/performance acceptance criteria.
5. Review and approve the proposed acceptance matrix and benchmark scenarios now drafted in `plan/PLAN.md`; refine them as the API decisions are finalized.
6. Perform the final consistency review and obtain explicit approval to exit Define before implementation.

## Verification and repository state

- Execution lock is active and remote-confirmed; Cycle 1 is in-progress in the registry, phase `define`.
- Requirements and architecture docs now state channel-value reader selection and internal cursor ownership; the superseded public UUID/cursor reader proposal was removed from those active sections.
- The plan now contains a proposed acceptance matrix and benchmark scenarios; these are planning artifacts, not executed tests or numeric performance commitments.
- GitHub Actions `Steering state validation` completed successfully for commit `7baf39324989e4ab45b495474ee888e9fb0fbad3` (run `45279936340`). The validator step passed; this checks lifecycle registry/lock consistency only.
- No source implementation, build, or Go tests have been performed.

## Blocker and next action

Cycle 1 is blocked in Define pending a Go-valid public API call shape. Go does not allow methods to declare their own type parameters, so `log.Append[TChannel,TEvent](selector)` cannot compile. The generic `Log[TChannel,TEvent]` interface direction can be retained by binding type arguments when constructing/opening the handle, after which methods are called as `log.Append(selector)` and `log.Read()`. Package-level generic functions are another option but change the call shape.

**User decision required:** confirm whether to use the generic Log handle with type arguments fixed at construction, or package-level generic functions. After that, specify one concrete consumer append example to settle how actual Channel and Event values enter through the selector callback.

The execution lock has been released. Resume this same cycle through the normal claim protocol after the user decision. No implementation, build, or Go tests have started.


## Configuration decision — 2026-11-24

Generic arguments to `PrepareLog[TEvent, TChannel](ctx)` select the consumer-defined types, not runtime storage configuration. A separate package-level `eventlog.Config(...)`-style facility is acceptable. Avoid requiring environment variables. Keep context semantics focused on cancellation/deadlines rather than using context as a hidden configuration store. The precise Config signature and scope/defaults need a concrete example and confirmation before implementation.


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

## Latest checkpoint — concurrent initialization behavior (2026-11-24)

- User decided that concurrent `PrepareLog` calls wait for the first shared initialization attempt; they do not fail merely because initialization is in progress.
- Each waiting caller may cancel its own wait through its context. If initialization fails, the immutable configuration remains set and a later `PrepareLog` may retry.
- This decision does not approve a specific exported error taxonomy or exact Config option signatures; those remain Define work.
- Execution lock remains active. No source implementation, build, or Go tests have started.

## Latest checkpoint — storage path resolution decision (2026-11-24)

- User confirmed the configured storage path must be absolute at configuration time.
- Relative inputs such as `./data/events` are accepted and resolved against the process working directory during successful `Config(...)`; the resulting absolute path is frozen in immutable runtime configuration.
- A later working-directory change cannot redirect storage. Empty or whitespace-only paths are invalid.
- Exact path cleaning, symlink behavior, filesystem existence/permission validation, and whether Config may inspect the filesystem remain open. Path resolution does not create or initialize storage; that remains part of `PrepareLog`.
- Execution lock remains active. No source implementation, build, or Go tests have started.

## Latest checkpoint — Config option handling approved (2026-11-24)

- User approved rejecting duplicate options; no last-option-wins behavior.
- `Config(options ...Option) error` accepts multiple distinct options; `WithStorageDirectory(path)` is mandatory.
- Nil options and missing storage-directory configuration fail validation.
- A failed Config attempt leaves runtime configuration uninitialized and retryable.
- Option application is package-controlled through an unexported method; consumers use exported option constructors.
- Storage path cleaning and directory existence/creation behavior were subsequently approved; see the latest storage directory validation checkpoint. Exported error names remain open.
- Execution lock remains active; no implementation, build, or Go tests have started.

## Latest checkpoint — storage directory validation approved (2026-11-24)

- User approved all six proposed storage-directory validation rules.
- Reject empty/whitespace-only paths; accept relative paths and resolve them to absolute paths during Config.
- Lexically clean the resolved absolute path (`filepath.Clean` semantics); do not resolve symlinks.
- Config does not require the directory to exist and does not create it.
- PrepareLog creates a missing directory during initialization; an existing non-directory target or directory-creation failure returns an error.
- The resolved, cleaned absolute path is frozen in immutable runtime configuration.
- Execution lock remains active for Cycle 1. No source implementation, build, or Go tests have started.


## Latest session checkpoint — exported error contract approved

- User approved `ErrConfigNotInitialized` and `ErrConfigAlreadySet` sentinel errors, with the semantics and error-wrapping contract recorded in the requirements and architecture.
- The error taxonomy is no longer an open Define decision. Invalid options remain descriptive validation errors; filesystem and context errors must preserve discoverable causes.
- The lock remains active for Cycle 1. Continue the Define review with the remaining lifecycle/API, cancellation/resource, benchmark, acceptance-matrix, and documentation consistency decisions. No implementation, build, or Go tests have started.


## Latest session handoff — 2026-11-24 — Fields contract and polling approved

- User reaffirmed the **only public API** is `Config(options ...Option) error`, the `PrepareLog[TEvent, TChannel](ctx)` entry point, and the consumer-implemented `Event` / `Channel` interfaces with their `Fields() Fields` declarations, plus the approved exported errors. Do not introduce public `Append`, `Read`, `Reader`, `Next`, `Close`, `Save`, or other operational APIs.
- `PrepareLog` creates and owns the consumer's Event and Channel instances and returns pointers to those same instances. The package's internal Log retains ownership and manages persistence, replay, synchronization, change tracking, and lifecycle.
- Consumers implement `Fields() Fields` to declare which fields they want persisted. The package must honor that declaration and build the internal machinery. Preserve the documented `FieldOf` and `FieldOfNullable` selectors; do not invent struct tags or a different field-selection scheme.
- `PrepareLog` reads the declarations, validates and resolves the selected fields, and builds internal bindings to the actual selected fields on the owned instances. The declaration is the sole field-selection point; do not require consumers to pass selectors to a public append call.
- User approved **polling-based internal change detection**. The internal manager periodically inspects selected fields, compares them with the last observed/persisted snapshots, and performs the required internal persistence. No public notification or operational methods are to be added.
- Go cannot intercept ordinary struct field assignments. Polling is therefore the agreed mechanism; its interval, scheduling, synchronization/race guarantees, event-vs-channel change semantics, and shutdown/resource behavior remain to be specified during Define. Do not claim polling itself makes unsynchronized concurrent field writes race-safe.
- Previously suggested package-level public `Append`/`Read` APIs and a public typed Reader were explicitly rejected by the user and are not part of the contract.
- The prior handoff's historical notes proposing a public generic Log interface, public Append/Read, selector arguments to Append, and alternative reader APIs are superseded by this latest decision and must not be revived.
- Lock released for the next session in lock-only commit `5d532af3a2c65a5d23208f30a896317301aa723d`; shared `main` and `execution-lock.json` were re-fetched and confirmed at that commit with `status: released`.
- Cycle 1 remains `in-progress`, phase `define`. No implementation, build, or Go tests have started.

### Next Define tasks

1. Specify the polling interval and scheduler strategy, including how polling is stopped/retried and how errors surface internally without expanding the public API.
2. Define event-field changes versus channel-field changes: occurrence creation, channel identity/association, ordering, and persistence semantics.
3. Define concurrency guarantees for consumer mutation while polling. Plain unsynchronized writes to returned structs cannot be made race-free by the package; decide and document the caller synchronization contract or a supported internal snapshot strategy.
4. Finalize initialization, cancellation, reader/resource, terminal error, benchmark, and acceptance criteria.
5. Reconcile requirements, architecture, plan, and handoff; remove stale active contradictions while retaining historical decisions only when clearly marked superseded.
6. Complete Define consistency review and obtain explicit user approval before implementation.

### Resume instructions

The next session must re-fetch shared `main` and verify `docs/dev-life-cycle/execution-lock.json` is still released. It must then claim Cycle 1 with a lock-only commit before editing project documents. Continue from the polling and Fields decisions above. Do not start implementation until Define is complete and the user explicitly approves implementation.


## Latest session checkpoint — polling cadence and scheduler approved (2026-11-24)

- User agreed to the proposed **100 ms default polling interval**.
- Use one managed polling scheduler owned by the internal Log/change manager to inspect all selected event and channel fields; do not create one goroutine per field.
- The interval may be adjusted internally for implementation and focused tests. Do not add a public polling configuration option or operational API.
- The scheduler compares selected-field snapshots and sends detected changes through the internal persistence path.
- This approval does not make unsynchronized concurrent writes to ordinary consumer-owned Go struct fields race-safe.
- Remaining Define decisions: caller synchronization/snapshot consistency, event-field versus channel-field change semantics, scheduler shutdown/retry, and internal persistence-error handling.
- Cycle 1 remains in-progress, phase define. The execution lock is active under ChatGPT-GPT-6. No implementation, build, or Go tests have started.

### Next Define tasks

1. Define caller synchronization and snapshot consistency while polling consumer-owned values; explicitly prohibit claims that polling makes data races safe.
2. Define event-field changes versus channel-field changes: occurrence creation, channel identity/association, ordering, and persistence semantics.
3. Specify scheduler shutdown/retry and how persistence errors are retained or surfaced internally without expanding the public API.
4. Finalize initialization, cancellation, reader/resource, terminal error, benchmark, and acceptance criteria.
5. Reconcile requirements, architecture, plan, and handoff; remove stale active contradictions while retaining historical decisions only when clearly marked superseded.
6. Complete Define consistency review and obtain explicit user approval before implementation.


## Latest session checkpoint — event-field changes append occurrences (2026-11-24)

- User approved that a detected change to selected Event fields creates a new persisted event occurrence.
- Earlier occurrences remain immutable and retained in history. The internal polling manager must not overwrite an earlier persisted occurrence.
- Important unresolved interaction: current logical Event identity is derived from Event type plus exposed fields. A changed field set therefore appears to derive a different ID under the existing rule, while the new decision speaks in terms of a changed occurrence. Do not assume continuity of logical identity or HEAD promotion; explicitly reconcile event identity, occurrence history, and index ordering in Define before implementation.
- The 100 ms default polling interval and one manager-owned scheduler remain approved.
- Execution lock was re-fetched and is active for Cycle 1 under ChatGPT-GPT-6. No implementation, build, or Go tests have started.

### Next Define tasks

1. Resolve how changed Event field values interact with logical Event identity, occurrence association, HEAD, and ordering.
2. Define Channel-field changes and whether they create a new channel identity, move current associations, or require another internal persistence behavior.
3. Define caller synchronization and snapshot consistency; polling does not make unsynchronized struct writes race-safe.
4. Specify scheduler shutdown/retry and internal persistence-error handling without adding public operational APIs.
5. Complete initialization/cancellation/resource, benchmark, acceptance, and cross-document consistency review.
6. Obtain explicit user approval before implementation.
