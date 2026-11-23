# Cycle 1 — Current Handoff

**Cycle ID:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182  
**Workflow:** Standard  
**Phase:** Define  
**Status:** In progress  
**Execution lock:** Active — ChatGPT GPT-6. Claim commit: 23ccba45fd17e96b039a54d032c0fd25a6ab8621; claim-time correction commit: 41cecfbf49d0a0a62b6ce08670f4a063a3cf01a6.

## Current checkpoint

The execution lock was claimed on main and the active remote lock was verified. Plan contradictions have been reconciled in the current authoritative sections. Durable requirements and architecture docs were updated with approved decisions. The registry still correctly identifies Cycle 1 as the sole in-progress standard cycle in Define. This handoff is the current checkpoint; older session notes in PLAN.md are historical.

No implementation, build, or Go tests have started.

## Approved baseline (do not reopen)

- Embedded general-purpose append-only event storage; single process owns log, concurrent goroutines supported, no multi-process writer coordination.
- Consumer Event/Channel contracts expose `Fields() Fields`; only explicitly selected Fields persist.
- `Append(channel, event) error`; validate both before write; new channel persisted atomically with first event; no standalone channel-create API.
- Primitive allowlist: string, bool, signed/unsigned integers (byte/rune aliases), float32/float64 and named primitive types; exclude uintptr/complex. Nullable primitive pointers supported. Non-finite floats rejected; exact Go integer bounds checked.
- `FieldOf` / `FieldOfNullable`; reflection resolves direct selected pointer on zero-valued probe and requires unique selector match. Field validation occurs at Append, all-or-nothing.
- Field names are case-sensitive, not trimmed; reject empty/duplicate. Sort canonical fields bytewise UTF-8.
- Deterministic UUID v5 with fixed namespace and distinct event/channel domain prefixes. Canonical encoding sorted fields, type tags, uint32 big-endian framing, exact primitive widths/bits, package path/type identity.
- Event identity global from event type+fields (not channel). Each occurrence belongs to one channel. Reappend is a distinct occurrence and preserves associations/history; latest occurrence sets HEAD/current channel.
- NDJSON built-in JSON, zero-byte new log, no header. Every channel/event/commit record has `schema_version: 1`; Draft 2020-12 schemas versioned, standalone canonical, embedded using `go:embed`.
- Typed JSON values: strings/bools native, integers decimal strings, float32/64 IEEE raw-bit hex strings (8/16 lowercase digits), null type-only. Schema enforces structure; Go checks integer ranges and nonfinite float rejection.
- Record kinds: `channel`, `event`, `commit`. Global sequence assigned to all records. Transaction ID = sequence of first transaction record. New channel transaction writes channel,event,commit; existing channel writes event,commit. Commit sequence consumes global sequence but excluded from transaction record count/checksum.
- SHA-256 checksum covers exact serialized transaction record bytes including trailing newline per transaction record; commit marker and its newline excluded.
- Recovery exposes committed transactions only; incomplete trailing transaction (including partial commit marker) truncates from transaction start. Complete malformed record is corruption, never silently skipped. Unsupported schema version is rejected.
- `Open()` creates missing log as zero bytes and recovers existing storage.
- Sync: automatic sync every 1 second or 1 MiB (first threshold wins); `Flush()` and `Close()` sync boundaries. Append is not power-loss durable before sync.
- Unified replay/live reader: ordered committed history, concurrent catch-up and live tail transition without gaps/duplicates; global cursor, optional channel filter, context cancellation, explicit errors.
- Segment files authoritative; rebuildable index stores event/channel links, latest occurrence/HEAD, sequences and offsets. Open validates/rebuilds index and atomically replaces it.
- Fixed target segment size, rotate only at transaction boundaries; oversized transaction can occupy one segment.
- Compaction: create replacement temp segments, validate/checksum, sync, rename final, atomically replace index, then remove old segments. Temp files cleaned only after finalized source segments validate.
- Standard library only; local filesystem; Linux and Windows equally supported, macOS excluded. Correctness/recovery/memory predictability prioritized; numeric performance targets deferred to benchmarks.

## Contradictions reconciled in PLAN.md

1. Channel lifecycle/API: explicit creation statements superseded; no standalone API, atomic implicit creation with first append.
2. Record taxonomy: two-kind descriptions superseded; three envelope kinds including commit.
3. Float JSON: ordinary JSON number proposal superseded by typed IEEE-hex strings.
4. Stale open decisions and unapproved baseline replaced by a current-only unresolved list and approved baseline.
5. Old handoff snapshots retained as historical notes; this file is the sole current handoff.
6. Atomicity separated from durability: recovery rollback vs unsynced crash loss window.
7. Event/channel semantics clarified: one channel per occurrence, associations retained across logical-event reappend, latest HEAD/current channel.
8. Sequence/count/checksum accounting clarified: marker sequence counts globally, but commit marker excluded from count and checksum.
9. Compaction cleanup order clarified: only after finalized source segments validate.
10. API drift partly resolved: constructor/validation behavior current; precise API/error signatures remain open.

## Reconciliation changes

- Plan reconciled and stale checkpoints marked historical. Plan commit: `fbcc5a590001a9e25ec7bc8585ca1c3a0adf5205`; reconciliation checkpoint commit: `d534a08c88342e1b8e8a163ee716f94c6341e363`.
- Handoff rewritten as the sole current checkpoint (commit `7618a76dcded8a77a1a36b0c771a29cca843b3ad`).
- Requirements documented (commit `3c87844abb9a60fc7c4b9386a498d6de208e493c`).
- Architecture documented (commit `21db0b1b77ab2d8578e65afe7629622471d3a5a7`).
- Validator workflow succeeded for lock claim, plan reconciliation and handoff reconciliation revisions. The latest plan checkpoint's workflow run is still to be confirmed.

## Remaining Define work

1. Consolidate public API signatures and error categories, including reader shape/cursor/filter/context/close semantics.
2. Specify segment naming/layout, index schema, manifest necessity and Open recovery/rebuild ordering.
3. Define reader safety and compaction interaction, including close/resource behavior.
4. Define benchmark scenarios and memory/performance criteria.
5. Build an acceptance-test matrix covering transaction failures, recovery, checksum/sequence invariants, replay/live races, index rebuild and compaction crash windows.
6. Finish the consistency review and obtain explicit user approval to exit Define.

## Session rules

- Resume by verifying the execution lock first. If it is active and owned by this worker/session, continue; if ownership is uncertain or another worker owns it, stop per lifecycle rules.
- Stay on main; serialized development only.
- Define phase only: no implementation, build or tests until Define is complete and explicitly approved.
- Keep plan, registry phase, and handoff consistent.
- Release the lock only when handing off/pausing/completing, after checkpointing.

## Next action

Complete the public API/error model and then settle the remaining storage/recovery/reader details. Do not repeat approved design decisions.
