# Cycle 1 — Current handoff (2026-11-24)

## State
- **Cycle ID:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182
- **Sequence / execution order:** 1 / 1
- **Workflow / phase:** standard / define
- **Registry:** in-progress
- **Lock:** active for ChatGPT-GPT-6, claim commit `b230cc83bc522b6a5f7cbc13107c4f34dfc503a7`.
- **Implementation:** none; no build or Go tests started.

## Correction after user review
The earlier cleanup was wrong: it reduced active docs by stripping requirements/design and leaving substance mainly in history. User clarified that this was not a simple relocation exercise. Active project docs must be fully curated and complete; the architecture must retain the complete design. Shortened summaries and archive-only requirements are unacceptable. History is secondary provenance, never the place to look up current specification.

## Curation approach
- Restored the comprehensive baseline requirements and architecture sections from pre-cleanup source.
- Integrated later approved decisions by topic, replacing superseded proposals rather than repeating chronology.
- Kept active requirements and architecture as full specifications: API/config, fields, identity, occurrence semantics, persistence formats, transaction integrity, recovery, reader behavior, scheduler/notifications, durability, segments/index/manifest, compaction and platform constraints.
- Open details remain explicitly open; intended behavior is not represented as implementation evidence.
- Preserved historical archives; no checkpoint deletion.

## Review still required
- Compare restored active requirements/architecture against original snapshots and the full decision trail to ensure every durable detail is included and contradictions are resolved.
- Check that outdated reader API direction was removed from the current architecture and only the approved PrepareLog public surface remains.
- Inspect diff, lifecycle state and links; run/await CI for exact final commit.
- After validation, pause Cycle 1 in Define and release the lock with a durable checkpoint.

## Next Define work
1. Resolve final API/interface/selector signatures, constraints, pointer/value and occurrence conventions.
2. Specify prepared-value association and internal lifecycle/cancellation/resource/error behavior.
3. Resolve designated identity-field mutation mechanics and snapshots/index/Channel association.
4. Finish notification accessor/payload/error, backlog persistence/replay boundary, retry termination, ordering and shutdown drain failure.
5. Define snapshot consistency and consumer synchronization details.
6. Complete acceptance matrix and benchmark plan; reconcile and seek explicit Define approval before implementation.
