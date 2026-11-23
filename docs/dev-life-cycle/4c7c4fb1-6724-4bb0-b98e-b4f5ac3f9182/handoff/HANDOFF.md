# Cycle 1 — Current Handoff

**Cycle ID:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182  
**Workflow:** Standard  
**Phase:** Define  
**Status:** In progress  
**Execution lock:** Active on `main`, owned by ChatGPT GPT-6 (authenticated GitHub user `gunesis-game-owner`). The active lock was confirmed from the remote after commit `d3e71eed9ac9597f3b26d7141fbe26e468f69b3f`.

## Latest checkpoint

- The user explicitly instructed continuation of Cycle 1. The registry is `in-progress`; the current phase remains `define`.
- The current approved baseline is consolidated in `plan/PLAN.md`: append-only local event log; one process owner; concurrent goroutine access; typed primitive `Fields()`; deterministic UUID v5 identities; versioned NDJSON records; commit/checksum/recovery rules; periodic sync; segmented storage, rebuildable index, manifest, and reader-pinned compaction; standard-library-only Linux/Windows support.
- Every reader is scoped to exactly one consumer-provided `Channel` value. Consumers do not supply or receive channel UUIDs, global sequence numbers, cursors, or storage offsets. Those remain internal.
- Reads must reconstruct the original concrete consumer-defined Event type. A separate/global `Register[T]()` API is rejected. `Append(channel, event)` remains the point where the concrete event type is first supplied.
- **User's clarified reconstruction requirement:** the consumer supplies the concrete generic struct type to the read API; eventlog owns allocation, decoding, field assignment, and persisted-type verification. No separate registration API, reader-scoped prototypes, Channel factory, or consumer decoder is wanted. Proposed shape: `Read[T Event](ctx, channel) (*Reader[T], error)` with `Next() (T, error)`. Exact generic constraint, pointer/value convention, and mismatch behavior remain open.
- Proposed unknown-type behavior: return an explicit typed unknown-event-type error; never silently skip an occurrence. Whether that error is terminal or recoverable remains open.
- The requirements and architecture docs were reconciled to the approved channel-value/internal-cursor contract. No implementation, build, or Go tests have started.

## Current API direction (not finalized)

```go
func Open(path string) (*Log, error)
func (l *Log) Append(channel Channel, event Event) error
func (l *Log) Read(ctx context.Context, channel Channel, prototypes ...Event) (*Reader, error)
func (r *Reader) Next() (Event, error)
func (r *Reader) Close() error
```

The `Read` prototype argument is a proposal only. Exact signatures, nil/pointer validation, unknown-type behavior, context cancellation, terminal errors, close semantics, Flush/Compact, and the stable error taxonomy remain to be finalized. Do not implement these signatures until the user approves the design.

## Remaining Define work

1. Finalize the generic read contract, pointer/value convention, and behavior when a channel contains a persisted event type different from T.
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

Present the reader-scoped prototype proposal for user approval, then continue finalizing the API/error contract and acceptance matrix. Do not begin implementation until Define is complete and the user explicitly approves the design.
