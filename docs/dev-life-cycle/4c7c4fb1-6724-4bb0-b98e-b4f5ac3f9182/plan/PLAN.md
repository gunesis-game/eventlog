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
- `PrepareLog[TEvent, TChannel](ctx, newChannel) (TEvent, TChannel, error)` with factory `func(chan Event, chan error) TChannel` (D-047).
- `Config(options ...Option) error`, `WithStorageDirectory(path)`, `ErrConfigAlreadySet`, `ErrConfigNotInitialized`.
- Consumer types implement `Fields() Fields` using `FieldOf`, `FieldOfNullable`, and exactly one `IdentityOf` (D-044); Channel also implements `Notifications() <-chan Event` and `Errors() <-chan error`.
- `ErrSchemaConflict` is the typed schema-conflict error (D-016).
- No public shutdown API and no execution-lock concept.

## Implementation progress (verified against the local working tree, 2026-11-27)
1. [x] Module/toolchain and Linux/Windows Go CI scaffold; CI now also runs gofmt and `go test -race` on Linux.
2. [x] Canonical v1 schemas (plus the `uuid` field-value variant for identity fields), embedded schema validation, serialization, record framing.
3. [x] Storage primitives, atomic file replacement, directory sync, and single-process ownership lock (Linux `flock`, Windows unshared handle).
4. [x] Transaction/stream validation, global sequence continuation, and zero-start-sequence rejection.
5. [x] Field selectors and schema resolution, typed-field encoding/reconstruction, canonical UUID v5 identity with v1 framing limits, decoder registration and schema conflict, Channel identity/snapshots, zero-identity rules (D-045), and identity-mutation rejection.
6. [x] Segment store with rotation, manifest, derived index rebuild, recovery and fail-closed corruption handling, size-triggered compaction (D-048) with reader pins; unified Channel Reader with decoder waits, bounded backpressure, delivery retry/exhaustion, and terminal per-Channel errors.
7. [x] Process runtime, shared initialization, `Config`/`PrepareLog`, change manager with bounded FIFO ingress, and acceptance tests including subprocess process-termination recovery.
8. [x] Verification on exact SHA `9f3d6df` passed on Linux and Windows CI (run 45964555490). Re-run for the final implementation SHA at the exit gate.
9. [~] Benchmarks: representative smoke set (B-F01, B-L01, B-T03/B-M06, replay delivery) exists. The full approved catalogue and profile protocol, and baseline capture, remain.

## Known gaps and follow-ups
- Consumers cannot synchronize their field mutations with polling through the public API, so concurrent consumer writes are data races under Go's memory model (see the architecture implementation notes). Needs a design decision.
- The in-memory index and `index.json` grow with total history, and every process start replays full Channel history; memory and startup cost scale with history size.
- The full benchmark catalogue (benchmarks.md) is not yet implemented.

## Next action
Resolve the consumer-synchronization design gap, implement the full benchmark catalogue, and complete the coding-standard review; then assess the Implement exit gate on the final SHA. Do not transition phases until that gate is satisfied.

## Phase exit
Implement exit requires the mandatory full verification suite, applicable performance checks, coding-standard review, and evidence tied to the exact final implementation SHA. Do not transition phases until the documented gate is satisfied and user authorization is obtained where required.
