# Cycle 1 — Implementation plan

## Identity and status
- **Cycle ID:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182
- **Sequence / execution order:** 1 / 1
- **Workflow:** standard
- **Status / phase:** in-progress / implement
- **Updated:** 2026-11-27

## Objective and scope
Implement eventlog according to approved requirements, public API, architecture, acceptance criteria, testing strategy, performance strategy, and Git workflow. Implement the approved single-process disk-backed append-only event log in small, testable increments. Preserve consumer-facing API and behavior. Multi-process/distributed writers, public API changes, and any public shutdown API remain out of scope absent separate approval.

## Governing project documents
- [Requirements](../../../project/requirements.md)
- [Architecture](../../../project/architecture.md)
- [Decision register](../../../project/decisions.md)
- [Acceptance matrix](../../../project/acceptance.md)
- [Testing strategy](../../../project/testing-strategy.md)
- [Performance strategy](../../../project/performance-strategy.md) and [benchmarks](../../../project/benchmarks.md)
- [Git workflow](../../../project/git-workflow.md)

## Approved public API constraints
- `PrepareLog[TEvent, TChannel](ctx, newEvent, newChannel) (TEvent, TChannel, error)` with `newEvent func(chan<- func() error) TEvent` and `newChannel func(chan Event, chan *ChannelError) TChannel` (D-049, D-052).
- `Event` is `Fields() Fields` plus `Transaction() chan<- func() error`; `Channel` is `Fields() Fields`, `Notifications() <-chan Event`, `Errors() <-chan *ChannelError`.
- `ChannelError{Kind, Cause, Events}` with `ErrorKind` values `TransactionRefused`, `TransactionPanicked`, `InvalidState`, `CommitFailed`, `ReaderFailed`, `DeliveryFailed`.
- `Config`, `WithStorage` with `Local{Directory}` (D-068; `Web{SettingsFile, DeviceStateDirectory}` designed, not implemented), `FieldOf`, `FieldOfNullable`, `IdentityOf`, `ErrConfigAlreadySet`, `ErrConfigNotInitialized`, `ErrSchemaConflict`.
- No new public package functions (user rule); no public shutdown API; no execution-lock concept.

## Implementation progress (verified against the local working tree, 2026-11-27)
1. [x] Module/toolchain and Linux/Windows Go CI, with gofmt and `go test -race` on Linux.
2. [x] Canonical v1 schemas (with the `uuid` field-value variant), embedded schema validation, serialization, record framing.
3. [x] Storage primitives, atomic replace (with a bounded Windows rename retry), directory sync, single-process ownership lock.
4. [x] Transaction/stream validation for multi-occurrence transactions (D-050) and global sequence continuation.
5. [x] Field selectors, typed encoding/reconstruction, canonical UUID v5 identity with v1 limits, decoder registration and schema conflict.
6. [x] Segment store, rotation, manifest, derived index, fail-closed recovery, size-triggered compaction with reader pins; unified Channel Reader with decoder waits, backpressure, delivery retry, and fatal `ReaderFailed`/`DeliveryFailed`.
7. [x] Transaction model replacing polling (D-049–D-052): Event factories, shared ordered transaction channel per Channel with exact-boundary intake, group commit, prefix-rule rejection with rollback, pause-until-received `ChannelError` delivery. Polling, the change manager, and their tests and docs are removed.
8. [x] Exact-SHA CI for `69dbed5` passed on Linux and Windows (run 45985063315). Re-run for the final implementation SHA at the exit gate.
10. [x] Package layout (D-053): `internal/identity`, `internal/fieldschema`, `internal/record` (with schemas), `internal/storage` split into store, segment, index, index file, recovery, and compaction types; Reader split from the Channel runtime; runtime depends on a storage interface; tests beside their code; public API verified unchanged with `go doc`.
9. [~] Benchmarks: smoke set (B-F01, B-L01, B-L05, B-T03/B-M06, replay). The full catalogue, profile protocol, and baselines remain.

11. [x] File-system boundary, Phase 1 (D-054): `internal/filesystem` with the `Backend` interface, OS backend, localhost token-authenticated HTTP server, and client; `internal/storage` makes no OS calls; contract tests cover the HTTP contract; public API unchanged.
13. [x] Transport (D-055): HTTP/2 over cleartext TCP with prior knowledge from the standard library, one client connection, no deprecated mechanism; research in docs/research/os-connection-limits.md. Readers fetch occurrences in batched ranged reads.
12. [ ] File-system boundary, Phase 2: web storage with Google Drive as the provider, `WithStorage(Web{SettingsFile, DeviceStateDirectory})` (D-061 to D-068), 1 MiB segments for Drive. Planned with item 14.

14. [ ] Multi-device operation (D-056 to D-071 recorded; design D-060 approved): per-device logs, causal delivery, conflict reporting, remote changes applied to live instances.

## Known gaps and follow-ups
- The in-memory index and `index.json` grow with total history, and every process start replays full Channel history.
- The full benchmark catalogue (benchmarks.md) is not yet implemented.
- Coding-standard review of the final implementation is not yet done.

## Next action
Settle the open Phase 2 questions (callbacks), record the Phase 2 decisions, and implement the Google Drive backend. The full benchmark catalogue, the coding-standard review, and the Implement exit gate follow.

## Phase exit
Implement exit requires the mandatory full verification suite, applicable performance checks, coding-standard review, and evidence tied to the exact final implementation SHA. Do not transition phases until the documented gate is satisfied and user authorization is obtained where required.
