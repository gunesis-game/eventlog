# Cycle 1 — Current Handoff

**Cycle ID:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182  
**Workflow:** Standard  
**Phase:** Define  
**Status:** Paused  
**Execution lock:** Released for session handoff.

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
