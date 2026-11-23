# Cycle 1 — Current Handoff

**Cycle ID:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182  
**Workflow:** Standard  
**Phase:** Define  
**Status:** In progress  
**Execution lock:** Active on `main`, owned by ChatGPT GPT-6 (authenticated GitHub user `gunesis-game-owner`). The active lock was confirmed from the remote after commit `d3e71eed9ac9597f3b26d7141fbe26e468f69b3f`.

## Latest checkpoint

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
- The exact declarations of `Channel`, `Event[TChannel]`, `Log[TChannel,TEvent]`, the selector callback, and the method signatures are not yet fully written down. Next session should start from concrete Go examples and derive the interfaces together.
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

The generic API direction above is the user's correction. Go does not permit methods to declare their own type parameters on a non-generic receiver, so the exact Go declaration may require a generic Log type or package-level generic functions; preserve the intended call shape in the API design discussion and resolve this language constraint deliberately. Do not substitute the former non-generic `Append(channel, event)` contract. The selector callback's exact type and body must be recovered from the agreed selector design rather than guessed.

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

## Next action

Finalize the generic typed-read API, concrete pointer/value convention, and whether Next returns T directly or an Occurrence wrapper; then complete the API/error contract and acceptance matrix. Heterogeneous channels and filtering of non-T occurrences are approved. Do not begin implementation until Define is complete and the user explicitly approves the design.
