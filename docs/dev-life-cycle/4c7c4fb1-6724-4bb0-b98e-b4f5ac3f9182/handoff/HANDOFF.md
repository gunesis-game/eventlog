# Cycle 1 — Define Handoff

**Cycle ID:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182  
**Phase:** Define  
**Status:** In progress  
**Execution lock:** Released for next session  
**Last updated:** 2026-11-23 23:19 SAST

## Latest approved decisions

- `Open()` creates an empty log if missing; existing logs recover automatically.
- New logs start as **zero-byte files** (no header/meta record).
- `Append(channel, event) error` is transactional. New-channel transaction writes channel, event, then commit; existing channel writes event then commit.
- Transaction ID is the first record's global sequence number. Commit marker is a first-class `kind: commit` record, gets its own sequence, count/checksum exclude marker, and SHA-256 covers exact transaction-record bytes.
- Partial commit marker recovery truncates to transaction start, discarding transaction contents.
- JSON Schema Draft 2020-12 is adopted for persisted record validation. Standalone versioned schemas are canonical and embedded into Go via `go:embed`.
- Field values use a uniform explicit typed JSON object. Strings and booleans use native JSON values; integers use decimal strings; float32/float64 use raw IEEE hex bit strings (8/16 hex chars) preserving exact width, negative zero and bits; null has type only.
- JSON Schema validates decimal-string form; Go validates exact bounds per integer type. Reject non-finite floats. Schema constraints are strict (`oneOf`, `const` tags, required properties, `additionalProperties: false`).
- Canonical binary identity encoding remains unchanged and separate from JSON persisted value encoding.

## Plan audit findings (not yet reconciled in main body)

- Channel creation API/lifecycle conflict: explicit channel creation vs implicit channel creation by Append; settle API.
- Commit marker taxonomy and transaction count/sequence semantics were reconciled by later checkpoint; ensure old envelope section updated.
- JSON float semantics were reconciled with exact tagged hex representation; remove stale competing numeric proposal.
- Stale unresolved-decision lists/proposed baseline need refresh, while historical checkpoints remain labeled.
- Atomicity vs durability (periodic sync) guarantee needs exact clarification.
- Event/channel relationship wording (single channel per occurrence vs multiple associations over event history) needs reconciliation.
- Temporary file cleanup ordering: clean only after finalized source segments validate.
- FieldOf/FieldOfNullable API examples are stale; consolidate current signatures and selector-error behavior.

## Next open design question

Where should the JSON Schema version be recorded/selected by persisted record envelope? (Schema versioning is approved; exact linkage/version marker is open.)

## Session instructions

- Resume from this handoff and the current PLAN; don't repeat approved decisions.
- Continue Define only. Do not implement/build/test until Define is complete and phase is explicitly approved.
- Claim execution lock before work; release it when handing off.
