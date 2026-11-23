# Cycle 1 — Current Handoff

**Cycle ID:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182  
**Workflow:** Standard  
**Phase:** Define  
**Status:** Blocked pending user design approval  
**Execution lock:** Claimed and verified in commit 043aa48a3442a0717914b731cf2f6f6a2c7cbfa2; release commit follows this checkpoint.

## Checkpoint
- Resumed Cycle 1 from Define and verified lock was released.
- Claimed execution lock and verified remote main claim commit.
- Re-read current plan and handoff.
- User explicitly decided: **all sequence/cursor management stays internal to the log**. Consumers do not pass sequence cursors to Read or manage storage sequence positions.
- Updated public API proposal accordingly; proposal remains pending approval as a whole.
- No source implementation, build or tests performed.

## Approved design baseline
Preserve consolidated baseline in PLAN.md: append-only disk-backed Go log; single-process ownership; concurrent goroutines supported; Event/Channel Fields() contracts; primitive allowlist and nullable primitives; deterministic UUID v5 identity; typed versioned NDJSON records channel/event/commit; transaction commits and SHA-256 framing; recovery/rollback; periodic sync; channel-scoped replay-to-live reader; segment rotation; rebuildable index; manifest and reader-pinned compaction; stdlib-only Linux/Windows support.

## Pending proposal for approval
A minimal API surface:
- `Open(path string) (*Log, error)`
- `(*Log).Append(channel Channel, event Event) error`
- `(*Log).ChannelID(channel Channel) (UUID, error)`
- `(*Log).Read(ctx context.Context, channelID UUID) (*Reader, error)` — no consumer-supplied cursor; replay from earliest retained channel history, then live.
- `(*Reader).Next() (Occurrence, error)`, `(*Reader).Close() error`
- `(*Log).Flush() error`, `(*Log).Compact() error`, `(*Log).Close() error`
- stable sentinel errors for validation, invalid channel, closed log/reader, corruption, unsupported schema, and storage failure; wrapping preserves cause and supports errors.Is.

Proposed public Occurrence metadata: event UUID, channel UUID, event type name, and selected typed Fields. **Global sequence, transaction IDs, and storage cursors are internal and are not exposed to consumers.** Reader semantics: channel-only, replay from the earliest retained occurrence then live without gaps/duplicates, context cancellation, Close wakes waiters. EOF only after closed and no more data can be delivered.

These are discussion proposals only; detailed names, types, error cases and reader close semantics remain open until approval.

## Remaining Define work
1. Approve or revise API/error proposal.
2. Finalize Reader/Occurrence, cancellation, and close semantics.
3. Define benchmark scenarios and memory/performance criteria.
4. Create acceptance-test matrix for transaction failures, recovery, checksums/sequences, replay/live races, index rebuild, and compaction crash windows.
5. Final consistency review and explicit approval to exit Define.

## Next action
Await user feedback on the revised API proposal, then continue the Define decisions. Do not implement before Define is complete and explicitly approved.

## Verification
- Lock claim was verified against remote main.
- Handoff checkpoint committed before lock release.
- No project source changes; no build or tests run.
