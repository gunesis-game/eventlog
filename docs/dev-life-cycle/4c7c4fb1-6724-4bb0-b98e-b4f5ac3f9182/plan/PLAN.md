# Cycle 1 — Define plan

## Identity and status
- **Cycle ID:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182
- **Sequence / execution order:** 1 / 1
- **Workflow:** standard
- **Status / phase:** paused / define
- **Created:** 2026-11-22
- **Updated:** 2026-11-24

## Objective
Define eventlog's requirements, public API, persistence guarantees, internal lifecycle and acceptance criteria before implementation.

## Context and scope
eventlog is an embedded, disk-backed append-only Go event log. This cycle defines the product, contracts, identity, persistence, recovery, reading, lifecycle and quality criteria. Implementation, builds, Go tests, dependencies and multi-process/distributed writers are out of scope. No implementation/build/tests have started.

The complete pre-cleanup plan and handoff checkpoints are preserved verbatim in [plan history](../history/PLAN-history-2026-11-24.md) and [handoff history](../history/HANDOFF-history-2026-11-24.md). These archives are provenance only; current approved decisions below govern.

## Current approved baseline
Authoritative durable decisions are consolidated in `docs/project/requirements.md` and `docs/project/architecture.md`.

- **Public API:** sole approved entry point is `PrepareLog[TEvent, TChannel](ctx) (TEvent, TChannel, error)`. Consumers receive values, not a Log handle; internal Log owns append/read, persistence, recovery, scheduler and lifecycle.
- **Configuration:** mandatory, immutable, one-time `Config(options ...Option) error` with required `WithStorageDirectory(path)`. Nil, duplicate, missing, invalid options fail. Relative paths are resolved and lexically cleaned to absolute during Config; Config does not create storage. PrepareLog creates storage. `ErrConfigNotInitialized` and `ErrConfigAlreadySet` are approved.
- **Fields:** typed explicit primitive selectors; nullable primitive pointers; no nested fields; strict names/types/ranges and validation before writes.
- **Identity:** deterministic UUID v5 with canonical encoding. Event identity is concrete type + exposed field-name schema + designated identity field name/value, global across channels. Non-identity changes create immutable occurrences under the same logical ID; latest occurrence promotes HEAD. Channel identity is similarly canonical. Identical physical records are deduplicated. Identity-field mutation handling remains open.
- **Persistence:** strict versioned NDJSON channel/event/commit records; global sequence; commit markers; transaction count and SHA-256 framing over exact transaction bytes including newline; incomplete trailing transaction rolls back; complete malformed records fail.
- **Storage:** manifest-selected immutable segments; monotonic non-reused segment IDs; rebuildable generation-bound index; rotation at transaction boundaries; reader-safe generation-pinned compaction.
- **Durability:** sync every 1 second or 1 MiB pending writes. Flush and Close are durability boundaries; Append before sync is not power-loss durable.
- **Reader:** internal channel-scoped replay/live tail with gap-free, duplicate-free handoff, internal cursors, concrete reconstruction and type filtering.
- **Change manager:** one internal scheduler at 100 ms; no goroutine per field. Consumer synchronizes its own field writes.
- **Notifications:** channel accessor (signature TBD); post-persistence immutable Event snapshots; guaranteed eventual delivery without blocking persistence/scheduler; recovered backlog first, orderly shutdown drains, crash recovery restores pending deliveries; persistent failures become typed errors. Retry/termination and delivery error details remain open.
- **Platforms:** standard library and local filesystem; Linux/Windows parity; macOS excluded.

## Open Define decisions
1. Final Go-valid API signatures, constraints, selectors, pointer/value convention and occurrence result shape while preserving the approved PrepareLog call.
2. Prepared-value association, internal lifecycle, cancellation, reader/resource cleanup and terminal errors.
3. Identity-field mutation response and precise Channel change semantics.
4. Notification accessor/message/error shape, backlog durability/replay boundary, retry termination, delivery/error ordering and shutdown failure.
5. Snapshot consistency and consumer synchronization requirements.
6. Benchmark scenarios, memory/performance targets and complete acceptance matrix.

## Tasks and exit criteria
- [x] Purpose, ownership, scope, constraints and platform requirements.
- [x] Fields model, selector validation and canonical identity.
- [x] Typed persistence, transaction markers, checksum and recovery.
- [x] Durability, reader replay/live contract.
- [x] Segments, manifest, index and compaction.
- [x] Configuration contract and errors.
- [x] Poll cadence, Event identity/occurrence distinction, notification direction.
- [ ] Finalize remaining API/lifecycle/error details.
- [ ] Complete acceptance matrix and benchmark plan.
- [ ] Reconcile current documentation and obtain explicit approval to exit Define.

**Acceptance:** approved baseline is unambiguous; open questions are actionable; requirements/architecture/plan/handoff/registry agree; history is subordinate to current decisions; no implementation begins before explicit Define approval.

## Documentation correction checkpoint — 2026-11-24
- Recurated the full active requirements and architecture, restoring substantive design rather than leaving it in history.
- Current active project docs contain the design baseline and integrate latest decisions. Archived pre-cleanup versions remain unchanged for provenance.
- Reviewed original requirements/architecture headings and the complete decision checkpoint sequence from the plan archive to ensure the current topics are represented.

## Verification and next action
- Manually inspected active requirements/architecture headings and sections for API, configuration, fields/selectors, identity/tag/UUID protocol, transaction/commit, schema, recovery, reader, scheduler/notifications, storage/manifest/index, compaction and portability.
- Lifecycle validator passed on commit `7243ec1bc8e3950c5376882333849e27b662c624` (run 45316732733). This run validated registry/lock only; subsequent project-doc-only commits did not trigger the workflow because its path filter excludes `docs/project/**`. No manual dispatch capability was available.
- An earlier claim-only commit had a failed validator because the lock was active while the cycle registry was still paused; the error was `must match exactly one in-progress registry cycle`. The registry was corrected in commit 7243ec1 and CI passed thereafter.
- No implementation, build or Go tests were started.

**Next:** continue Define on remaining API/lifecycle/identity-mutation/notification details, acceptance matrix and benchmarks; do not start implementation until Define is reconciled and explicitly approved.