# Cycle 1 — Current Handoff

**Cycle ID:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182  
**Workflow:** Standard  
**Phase:** Define  
**Status:** Blocked pending user design approval  
**Execution lock:** Claimed and verified in commit 0ea33ca9124b986a172835e94bd35a0f53529fac; release commit follows this checkpoint.

## Checkpoint
- Resumed Cycle 1 from Define and verified lock was released.
- Claimed execution lock and verified remote main claim commit.
- Re-read current handoff.
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

## Typed reconstruction: decisions still required
1. Decide how concrete event and channel types are registered or otherwise made available when opening an existing log.
2. Define how a reader selects the concrete Go type from persisted type identity, including unknown/unregistered types.
3. Define whether Occurrence exposes the reconstructed Event only, or also a reconstructed Channel value; avoid exposing internal UUIDs unless a consumer use case requires them.
4. Define behavior for renamed/moved Go types and schema/type evolution across application versions.
5. Ensure reconstruction restores only the explicitly selected Fields and preserves nullable primitive values and exact primitive types.

A possible shape for discussion is `Occurrence{Event Event, Channel Channel}`, where both values are concrete consumer-defined implementations. This is illustrative, not approved API.

## Remaining Define work
1. Finalize typed registration and reconstruction contract.
2. Finalize Occurrence and Reader semantics, cancellation, and close behavior.
3. Finalize API/error model.
4. Define benchmark scenarios and memory/performance criteria.
5. Create acceptance-test matrix for transaction failures, recovery, checksums/sequences, replay/live races, index rebuild, typed reconstruction failures, and compaction crash windows.
6. Final consistency review and explicit approval to exit Define.

## Next action
Continue Define by resolving the type-registration/reconstruction contract. Do not implement before Define is complete and explicitly approved.

## Verification
- Lock claim was verified against remote main.
- This checkpoint records the user's typed-reconstruction decision.
- No project source changes; no build or tests run.
