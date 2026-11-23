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
- **Reconstruction investigation:** a persisted package/type name is not enough for Go to instantiate an arbitrary concrete type after process restart. The leading proposal, not yet approved, is to pass a reader-scoped set of typed zero-value Event prototypes to `Read`. The library can use each prototype's concrete pointer type to create fresh instances, call `Fields()` on those instances, and populate the selected primitive fields. This avoids a global registration API, but the consumer must provide prototypes for every event type expected on that channel when opening a reader.
- Alternatives: a Channel factory avoids a separate Read argument but couples each Channel implementation to every event type it may contain; a factory/decoder method on each Event adds consumer boilerplate. The prototype approach appears to be the smallest change to the existing `Event{ Fields() Fields }` contract.
- Proposed unknown-type behavior: return an explicit typed unknown-event-type error; never silently skip an occurrence. Whether the reader becomes terminal after this error remains open.
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

1. Obtain the user's decision on reader-scoped Event prototypes versus another reconstruction mechanism.
2. Finalize Event/Channel/Fields contracts and public Append/Read/Next/Close signatures.
3. Define stable errors, cancellation, reader termination, and resource lifecycle.
4. Define benchmark scenarios and memory/performance acceptance criteria.
5. Build an acceptance-test matrix for validation, append atomicity, interrupted writes, checksums/sequences, replay/live races, unknown event types, index rebuild, compaction crash windows, and Linux/Windows parity.
6. Perform the final consistency review and obtain explicit approval to exit Define before implementation.

## Verification and repository state

- Execution lock is active and remote-confirmed; Cycle 1 is in-progress in the registry, phase `define`.
- Requirements and architecture docs now state channel-value reader selection and internal cursor ownership; the superseded public UUID/cursor reader proposal was removed from those active sections.
- No source implementation, build, or Go tests performed.
- Lifecycle validator/CI result has not yet been checked for the latest documentation and registry commits.

## Next action

Present the reader-scoped prototype proposal for user approval, then continue finalizing the API/error contract and acceptance matrix. Do not begin implementation until Define is complete and the user explicitly approves the design.
