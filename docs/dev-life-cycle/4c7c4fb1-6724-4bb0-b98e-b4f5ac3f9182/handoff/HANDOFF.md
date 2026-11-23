# Cycle 1 — Current Handoff

**Cycle ID:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182  
**Workflow:** Standard  
**Phase:** Define  
**Status:** Blocked pending user design approval  
**Execution lock:** Claimed and verified in commit 51b1080b821d8d2e1deda541a5eed04f2fcfd914; release commit follows this checkpoint.

## Checkpoint
- Resumed Cycle 1 from Define and verified lock was released.
- Claimed execution lock and verified remote main claim commit.
- Re-read current handoff and authoritative plan.
- Reconciled the plan's stale reader API baseline: no consumer-supplied channel UUID or cursor; typed reconstruction is the current direction.
- User explicitly decided: all sequence/cursor management stays internal to the log. Consumers do not pass sequence cursors or manage storage positions.
- User explicitly selected **typed reconstruction**: consumers should receive their original concrete Go event structs when reading, rather than generic persisted fields.
- This choice supersedes the earlier generic Occurrence proposal containing EventID, ChannelID, EventType, and Fields as consumer-facing values.
- No source implementation, build or tests performed.

## Approved design baseline
Preserve consolidated baseline in PLAN.md: append-only disk-backed Go log; single-process ownership; concurrent goroutines supported; Event/Channel Fields() contracts; primitive allowlist and nullable primitives; deterministic UUID v5 identity; typed versioned NDJSON records channel/event/commit; transaction commits and SHA-256 framing; recovery/rollback; periodic sync; channel-scoped replay-to-live reader; segment rotation; rebuildable index; manifest and reader-pinned compaction; stdlib-only Linux/Windows support.

## Current API direction (not fully finalized)
- `Open(path string) (*Log, error)`
- `(*Log).Append(channel Channel, event Event) error`
- Channel-based `Read(ctx, ...)` with no consumer-supplied sequence/cursor.
- Reader replays matching channel history then tails live events.
- Consumer-facing occurrences reconstruct the original concrete event struct (typed reconstruction), rather than exposing generic persisted fields as the primary event value.
- Global sequence, transaction IDs, storage offsets and cursors remain internal.
- Flush, Compact, Close and sentinel error model remain proposed.

## Typed reconstruction: current proposal and decisions still required

Recommended API direction for discussion:
- `Open(path string, registrations ...Registration) (*Log, error)`
- `Register[T Event]() Registration` explicitly registers a concrete event type for reconstruction.
- `(*Log).Read(ctx context.Context, channel Channel) (*Reader, error)` selects a channel using the consumer's channel struct; no public ChannelID lookup is needed.
- `(*Reader).Next() (Event, error)` returns the reconstructed concrete event value through the Event interface. Since the reader already targets one channel, a public Occurrence wrapper and Channel field may be unnecessary.
- `(*Log).Append(channel Channel, event Event) error` remains the write operation.

This is a proposal, not an approved API. Resolve:
1. Whether explicit `Register[T Event]()` at Open is the chosen registration mechanism.
2. Whether registration supports struct values only or also pointers to structs.
3. What happens when Next encounters an unregistered persisted event type (typed error and reader behavior).
4. How type renames/moves and schema/type evolution are handled; existing identity rules make Go package path/type name part of persisted type identity.
5. Ensure reconstruction restores only explicitly selected Fields and preserves nullable primitive values and exact primitive types.

## Remaining Define work
1. Finalize typed registration and reconstruction contract.
2. Finalize Occurrence and Reader semantics, cancellation, and close behavior.
3. Finalize API/error model.
4. Define benchmark scenarios and memory/performance criteria.
5. Create acceptance-test matrix for transaction failures, recovery, checksums/sequences, replay/live races, index rebuild, typed reconstruction failures, and compaction crash windows.
6. Final consistency review and explicit approval to exit Define.

## Next action
Ask the user to approve or revise the proposed registration and typed Read/Next API direction, then finalize unknown-type behavior. Do not implement before Define is complete and explicitly approved.

## Verification
- Lock claim was verified against remote main.
- This checkpoint records the user's typed-reconstruction decision.
- No project source changes; no build or tests run.
