# Project Testing Strategy

## Purpose and status

This document applies the [Testing Standard](../standards/testing.md) to eventlog (approved as D-042). The acceptance scenarios are defined in [acceptance.md](acceptance.md); this strategy maps them to architectural boundaries, feedback tiers, and test files.

**Current status:** Executable suites exist beside the code they test (D-053) and run with the commands in [commands.md](commands.md), which also records the latest observed local and CI results. CI runs the full suite on Linux and Windows.

## Architecture and test ownership

Component responsibilities are defined in [architecture.md](architecture.md). The acceptance layers are the test ownership map:

| Component / boundary | Direct dependants / consumers | Focused tests | Contract / integration tests | Notes |
|---|---|---|---|---|
| Disk and storage | Persistence and transaction layer | D-01–D-11 | Platform storage tests | Filesystem behavior, I/O errors, Linux and Windows parity |
| Persistence format and transactions | Recovery and metadata | F-01–F-06, T-01–T-06 | Interrupted transaction recovery | Distinguish incomplete trailing transactions from complete malformed records |
| Metadata and recovery | Event-log runtime | M-01–M-09 | Restart and reconstruction tests | Committed append history remains authoritative |
| Core Event/Channel behavior | Channel Reader | L-01–L-12 | Append, identity, concurrency and restart | Preserve transaction atomicity and sequence order |
| Channel and Reader | Notification delivery | R-01–R-14 | Replay-to-live continuity | Each Reader progresses independently |
| Notification delivery | Consumer Channel interface | N-01–N-14 | FIFO, retry, backpressure and error reporting | Validate bounded memory and the documented at-least-once/duplicate boundary |
| Runtime lifecycle | Public preparation API | RT-01–RT-13 | Initialization, caller-Context cancellation, process termination, and recovery | No public shutdown control; test process-termination recovery without adding public controls |
| Public API and integration | Library consumers | API-01–API-13 | Consumer-defined Event/Channel end-to-end tests | Public API compatibility is an explicit acceptance concern |
| Multi-device and refused writes | Storage, Reader, transaction processor | MD-01–MD-17, WP-01–WP-04 | Causal delivery, conflicts, convergence, other devices' logs, read-only storage | Several devices share one local directory |
| Web storage | Consumers using `Web` storage | WS-01–WS-14 | Drive backend and change notifications against the Google services emulator | Not built; only the service clients and emulator exist |

## Test suites and tiers

| Tier | Suite(s) | Trigger / impact rule | Command | Blocking? |
|---|---|---|---|---|
| Fast feedback | Tests of the changed package and its direct dependants | Every meaningful implementation iteration | `go test -count=1 ./internal/<package>/...` or `go test -count=1 -run <Test> .` | Yes |
| Integration feedback | Cross-boundary tests: transaction/recovery, replay-to-live continuity, notification delivery, consumer API contracts | Any change crossing a component boundary or affecting a public contract | `go test -count=1 ./...` | Yes when the affected contract is in scope |
| Full verification | Every package, race detector, vet, build, gofmt, Linux and Windows | Before a change is considered complete; CI on every push to `main` | The full command set in [commands.md](commands.md) | Yes |

Package dependencies point one way (see [architecture.md](architecture.md#package-layout)), so a change to a lower package also runs the tests of every package above it.

## Test design and quality policy

- Implement acceptance tests from the lowest storage boundary upward; higher-level tests must not mask lower-level failures.
- Cover success, invalid input, corruption, interruption, concurrency, recovery and resource cleanup where specified by the acceptance matrix.
- Test actual consumer-defined Event and Channel types at the public API boundary.
- Exercise Linux and Windows for the platform-sensitive storage and integration guarantees.
- Keep fault injection and process-termination controls test-only; do not add them to the public API.
- Verify deterministic ordering, transaction atomicity, no missed or duplicated history during normal replay-to-live progression, bounded notification memory, and explicit error behavior. Crash-window duplicates remain permitted by the documented delivery contract.
- Use real filesystem behavior for storage and recovery tests. Use fakes only where they isolate a boundary without replacing the behavior under test.
- Web-storage code runs against the in-process Google services emulator (`internal/googletest`); no automated test calls real Google services (D-081).
- Avoid timing-only assertions where synchronization or observable state can establish the condition more reliably.
- No coverage percentage or other numeric quality threshold is approved; do not invent or enforce one.

## Suite locations

| Acceptance layer | Test files |
|---|---|
| Disk and storage (D-*) | `internal/filesystem/contract_test.go` (client, server, and OS backend: create, append, read, truncate, sync, replace, rename, remove, list, lock, reopen, missing folder, path and token rejection, HTTP/2-only with all concurrent requests on one connection), `internal/filesystem/permission_test.go`, `internal/storage/store_test.go` (lock exclusivity, missing directory, cleanup) |
| Persistence format and transactions (F-*, T-*) | `internal/record/*_test.go`, `internal/storage/store_test.go` |
| Metadata and recovery (M-*) | `internal/storage/store_test.go` (index rebuild, manifest recovery, corruption, compaction) |
| Core Event/Channel behavior (L-*) | `internal/identity/*_test.go`, `internal/fieldschema/*_test.go`, `internal/storage/store_test.go`, `runtime_test.go`, `declaredschema_test.go` |
| Channel and Reader (R-*) | `runtime_test.go` |
| Notification delivery (N-*) | `runtime_test.go`, `process_test.go` |
| Runtime lifecycle (RT-*) | `runtime_test.go`, `process_test.go` (subprocess Config contract and process termination) |
| Public API and integration (API-*) | `example_test.go` (external consumer compile check), `process_test.go`, `runtime_test.go` |
| Multi-device and refused writes (MD-*, WP-*) | `multidevice_test.go`, `permission_test.go`, `internal/causal/hlc_test.go`, `internal/storage/{causal,delivered,layout,otherdevices,permission}_test.go` |
| Web storage (WS-*): service clients and emulator only | `internal/googleauth/googleauth_test.go`, `internal/drive/drive_test.go`, `internal/firebase/firebase_test.go`, `internal/googletest/*_test.go` |
| Benchmarks (smoke set) | `internal/filesystem/client_bench_test.go`, `internal/record/record_bench_test.go`, `internal/storage/store_bench_test.go`, `benchmark_test.go`, `publicapi_bench_test.go` |

Test-only controls (closing an in-process runtime, delivery fault injection, lowered storage and batch thresholds, a failing store wrapper injected through the runtime's storage interface) are unexported and are not part of the public API. All tests, including subprocess children, change live instances only through transactions and run clean under the race detector.

## Evidence

Follow the evidence rules of the [Testing Standard](../standards/testing.md#evidence-and-reporting): associate every result with the exact commit SHA, and treat a skipped, cancelled, missing, or incomplete required result as not passed. CI workflows are listed in [commands.md](commands.md#ci).

## Exclusions and approvals

No acceptance layer is excluded. Performance measurement is separate and governed by [performance-strategy.md](performance-strategy.md) and [benchmarks.md](benchmarks.md). Any future exclusion, non-blocking suite, or reduced platform check requires rationale, risk, compensating evidence, approval, and a decision ID.
