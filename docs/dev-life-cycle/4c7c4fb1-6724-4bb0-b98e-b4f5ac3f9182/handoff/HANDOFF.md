# Cycle 1 — Current Handoff

**Cycle ID:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182  
**Workflow:** Standard  
**Phase:** Define  
**Status:** Blocked pending user design approval  
**Execution lock:** Released. Verified current lock file at handoff checkpoint; no active owner.

## Latest checkpoint
- User confirmed that **all global sequence/cursor management remains internal**. Consumers neither pass nor receive storage cursors/sequences.
- User selected **typed reconstruction**: consumers receive their original concrete Go event structs, not generic persisted fields.
- User rejected separate event registration: the consumer already supplies the concrete event value to `Append(channel, event)`; do not add a separate `Register[T]()` API.
- Intended consumer-facing shape remains struct-oriented: append supplies channel and event values; read selects by a channel value; Next returns a reconstructed concrete event through the Event interface.
- A possible Channel factory/constructor method was discussed as one way to reconstruct concrete event types after restart, but it is **not approved**. Do not assume it is the answer.
- Important unresolved technical design: Go cannot instantiate an arbitrary concrete type from a persisted type name alone after restart. Determine how type reconstruction is made possible without separate registration, while honoring the user's requirement that Append is the point where type information is supplied. Investigate whether the Channel contract can provide construction/decoding, whether an event prototype is supplied as part of Read, or another design consistent with the existing Fields contracts. Present trade-offs before deciding.
- Do not reintroduce public EventID/ChannelID fields or generic Fields as a substitute for typed reconstruction unless a concrete consumer need justifies them.
- No source implementation, build or tests performed.

## Approved design baseline
Preserve consolidated baseline in PLAN.md: append-only disk-backed Go log; single-process ownership; concurrent goroutines supported; Event/Channel Fields() contracts; primitive allowlist and nullable primitives; deterministic UUID v5 identity; typed versioned NDJSON records channel/event/commit; transaction commits and SHA-256 framing; recovery/rollback; periodic sync; channel-scoped replay-to-live reader; segment rotation; rebuildable index; manifest and reader-pinned compaction; stdlib-only Linux/Windows support.

## Current API direction (not finalized)
- `Open(path string) (*Log, error)`
- `(*Log).Append(channel Channel, event Event) error`
- Channel-based `Read(ctx, channel)` with no consumer-supplied UUID or cursor.
- `(*Reader).Next() (Event, error)` returns the reconstructed concrete event.
- Reader replays matching channel history then tails live events without gaps/duplicates.
- Global sequence, transaction IDs, storage offsets and cursors remain internal.
- Flush, Compact, Close and sentinel error model remain proposed.

## Remaining Define work
1. Resolve typed reconstruction without separate registration, including restart behavior and unknown/unsupported type handling.
2. Finalize Event/Channel contracts, Read/Next signatures, and whether Channel needs a factory/decoder capability.
3. Finalize reader cancellation, close, and terminal-error semantics.
4. Finalize API/error model.
5. Define benchmark scenarios and memory/performance criteria.
6. Create acceptance-test matrix for transaction failures, recovery, checksums/sequences, replay/live races, typed reconstruction failures, index rebuild, and compaction crash windows.
7. Final consistency review and explicit approval to exit Define.

## Next action
Resume by investigating viable Go type-reconstruction designs that require no separate registration and preserve Append as the source of concrete event type information. Bring a concise recommendation and trade-offs to the user. Do not implement before Define is complete and explicitly approved.

## Verification
- Execution lock was already in released state when the user requested unlocking; no release mutation was necessary.
- This handoff checkpoint records the user's rejection of separate registration and the unresolved reconstruction constraint.
- No source implementation; no build or tests run.
