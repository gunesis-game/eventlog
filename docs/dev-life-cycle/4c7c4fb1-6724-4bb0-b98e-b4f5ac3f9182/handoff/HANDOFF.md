# Cycle 1 — Define Handoff

**Cycle ID:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182  
**Phase:** Define  
**Status:** In progress  
**Execution lock:** Active — claimed by ChatGPT GPT-6 on 2026-11-23 23:21:56 SAST

## Latest approved decisions (current)

- `Open()` creates a missing log as a zero-byte file and automatically recovers existing logs.
- `Append(channel, event) error` is the API direction. It validates both objects before writing; new channel + event are persisted atomically in one transaction. No standalone channel creation API; no channel-only entities.
- Every persisted record, including `channel`, `event`, and `commit`, has `schema_version: 1`. Schema selection is per record. Unsupported future versions are rejected. No header is added.
- Transactions write channel (if new), event occurrence, then commit marker. Commit marker is first-class, consumes a global sequence, and includes transaction ID (first transaction record sequence), transaction-record count (excluding marker), and SHA-256 checksum of exact serialized transaction-record bytes (excluding marker).
- Partial/incomplete trailing transaction recovery truncates back to transaction start; only committed transactions are exposed.
- JSON Schema Draft 2020-12; standalone versioned schema files are canonical and embedded with `go:embed`.
- Persisted fields use strict typed JSON objects. Strings/bools use native values; integer values use decimal strings (schema validates shape, Go validates exact per-type bounds); floats are raw IEEE hex strings preserving width and signed zero; null is type-only. Non-finite floats rejected.
- Deterministic IDs use UUID v5 with fixed namespace and distinct event/channel domain prefixes. Canonical identity encoding is field-order independent, sorted bytewise UTF-8, explicit type tags, uint32 big-endian framing, exact primitive widths/bits, package path + struct type name.
- Channel identity uses channel type + all channel fields. Event identity uses event type + fields, excluding channel. Logical event IDs are global; repeated append creates a new occurrence, latest occurrence defines HEAD/current channel and order, while associations/history are retained.
- Global sequence numbers are persisted in segment records. Segment files are source of truth; index is rebuildable. Segment-first writes, atomic index replacement.
- Compaction order approved: temporary replacement segments, validate/checksum, sync, rename to final, atomically replace index, then remove old segments. Recovery ignores temp compaction artifacts and cleans only after finalized sources validate.
- Sync policy: automatic sync every 1 second or 1 MiB (whichever first); explicit `Flush()` is a durability boundary; `Close()` flushes and syncs; Append isn't crash-durable before sync.
- Field constructors: `FieldOf` for non-nullable and `FieldOfNullable` for nullable; reflection resolves direct selected field pointer on zero-valued probe and requires unique match. Validation occurs at append, all-or-nothing.
- Primitive allowlist: string, bool, all signed/unsigned integers (including byte/rune aliases), float32/float64, named types; excludes uintptr/complex. Nullable primitive pointers supported. Reject non-finite floats, duplicate/empty names; names case-sensitive/untrimmed. Canonical field order bytewise UTF-8.
- Integer range validation decision: JSON Schema validates decimal-string structure; Go enforces exact per-type bounds.

## Current plan audit / reconciliation

Earlier plan sections contain stale proposed APIs, open-decision lists, and historical checkpoints. Newer approved checkpoints take precedence; reconcile stale wording as Define documentation cleanup. Known unresolved areas include: exact transaction byte framing/checksum newline scope; public reader/replay/tailing contract and replay-to-live transition; segment rotation/size limits; index/manifest format and rebuild/recovery order; performance/portability assumptions; consolidated API signatures/error semantics; acceptance criteria and test plan. Durability vs atomicity wording must distinguish recoverability from unsynced crash loss.

## Next design question

Define the v1 public API surface and error model: exact type signatures, reader interface, context handling, and stable error categories. Recommend documenting the current agreed APIs (`Open`, `Append`, `Flush`, `Close`, unified reader) and defining errors for validation, corruption/recovery, unsupported schema, I/O, and closed-state operations.

## Session instructions

- Resume from this handoff and the current PLAN; do not repeat approved decisions.
- Define phase only. No implementation, build, or test until Define is complete and explicitly approved.
- Execution lock is active; do not start another worker. Release it only when handing off.


## Latest checkpoint — checksum newline framing — 2026-11-23

- User approved SHA-256 over exact serialized NDJSON transaction record bytes including terminating newline(s), excluding commit marker and its newline. Recovery uses the same byte scope.
- The next unresolved design question is segment rotation and size limits (fixed target size, transaction-boundary rotation, and handling oversized transactions).


## Latest checkpoint — unified reader — 2026-11-23

- Unified replay/live stream approved. It starts from committed history, catches up concurrent appends, transitions gap-free and duplicate-free into live tailing, supports global sequence cursor, optional channel filtering, context cancellation, ordered delivery, and explicit errors. Re-appends are separate occurrences.


## Latest checkpoint — segment rotation — 2026-11-23

- Fixed target segment size approved; rotate only at transaction boundaries. Transactions are never split. An oversized transaction may occupy its own segment.


## Latest checkpoint — rebuildable index — 2026-11-23

- User approved a replaceable local index derived from authoritative segment files. Index tracks event IDs, channel associations, HEAD/latest occurrence, sequence positions, and offsets. `Open()` validates and rebuilds when missing or inconsistent; replacement is atomic.


## Latest checkpoint — portability and performance — 2026-11-23

- Standard-library-only Go, local filesystem storage, Linux and Windows only (macOS excluded). Linux and Windows are equal-priority supported targets. Any platform-specific unsupported or unreliable behavior must use an alternative strategy that works on both; core features must not become platform-exclusive.
- Performance targets qualitative until benchmarks. Prioritize correctness, recovery, predictable memory use, and cross-platform parity.
