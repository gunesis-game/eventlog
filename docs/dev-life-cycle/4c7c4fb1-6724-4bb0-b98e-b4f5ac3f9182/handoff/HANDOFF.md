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
2. Finalize the public API/error contract and reader Occurrence/cancellation/close details.
3. Define benchmark scenarios and memory/performance criteria.
4. Build an acceptance-test matrix covering transaction failures, recovery, checksum/sequence invariants, replay/live races, index rebuild and compaction crash windows.
6. Finish the consistency review and obtain explicit user approval to exit Define.

## Session rules

- Resume by verifying the execution lock first. If it is active and owned by this worker/session, continue; if ownership is uncertain or another worker owns it, stop per lifecycle rules.
- Stay on main; serialized development only.
- Define phase only: no implementation, build or tests until Define is complete and explicitly approved.
- Keep plan, registry phase, and handoff consistent.
- Release the lock only when handing off/pausing/completing, after checkpointing.

## Next action

Complete the public API/error model and then settle the remaining storage/recovery/reader details. Do not repeat approved design decisions.


## Approved reader decision (latest)

- Reader API is channel-scoped only: every reader requires exactly one typed `UUID`; there is no all-channel/unfiltered read.
- Empty/zero UUID is invalid, not a wildcard/sentinel.
- Add `ChannelID(channel Channel) (UUID, error)` to resolve deterministic IDs.
- API direction: `Read(ctx context.Context, after uint64, channelID UUID) (*Reader, error)`, `Next() (Occurrence, error)`, and `Close() error`.
- Cursor is global and exclusive; `0` starts at the earliest occurrence for the selected channel. Replay/live transition remains gap-free and channel-filtered.
- Exact occurrence fields and detailed cancellation/error semantics remain open. No implementation, build, or tests have started.


## Define decision — storage layout, manifest, index, recovery, and compaction (approved)

### Storage layout and segment lifecycle

The storage directory contains a manifest, a rebuildable index, and a segments directory. Segment IDs are monotonically increasing integers and are never reused, including after compaction; they are not record sequence numbers. Finalized segment files are immutable. Only the active segment may be written. Record sequence numbers remain global across all segments.

```text
eventlog/
├── manifest.json
├── index.json
└── segments/
    ├── segment-000001.ndjson
    ├── segment-000002.ndjson
    └── ...
```
### Manifest

A minimal versioned manifest identifies the active finalized segment set and the next segment ID. It is authoritative for the active generation and contains no event payloads. Example shape:

```json
{
  "format_version": 1,
  "generation": 3,
  "segments": ["segment-000004.ndjson", "segment-000005.ndjson"],
  "next_segment_id": 6
}
```
Manifest replacement is the atomic generation-switch/compaction commit point: write temporary manifest, validate, sync, then atomically replace it. Old segments are retained until the new manifest is finalized. Unreferenced segment files are recovery candidates for safe cleanup, not automatically authoritative. If manifest corruption leaves multiple plausible complete generations, Open must fail with a recovery/corruption error rather than guessing.

### Rebuildable index

The index is derived metadata and is bound to the manifest generation. It includes event IDs, channel associations, latest occurrence/HEAD, global sequence, segment ID, and byte offsets. If absent, inconsistent, or stale for the active generation, Open rebuilds it from validated authoritative segments and atomically replaces it. Index metadata never overrides segment contents.

### Open recovery order

1. Acquire exclusive single-process ownership.
2. Validate the manifest and identify its active segment generation.
3. Validate active segment files, record schemas, and sequence continuity.
4. Validate transaction boundaries, commit markers, counts, and checksums.
5. Roll back an incomplete trailing transaction by truncating from its transaction start.
6. Validate or rebuild the generation-bound index and atomically replace it if needed.
7. Clean orphan/temp artifacts only after the active finalized source segments validate and cleanup is safe.
8. Open for appends and mandatory channel-scoped readers.

If the manifest is corrupt or missing in an existing storage directory, recovery may proceed only if exactly one complete generation can be determined unambiguously; otherwise report corruption/recovery failure. Do not select the newest-looking files by filename alone.

### Reader-safe compaction

Each reader pins the generation it started against. Compaction constructs replacement segments and index without modifying files used by current readers, validates and syncs them, then atomically switches the manifest. New readers use the new generation; existing readers continue using their pinned generation. Old generation files are deleted only after the last reader releases its pin. This policy applies equally on Linux and Windows, avoiding platform-specific reader semantics.
