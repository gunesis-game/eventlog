# Acceptance test strategy

**Status:** Acceptance structure approved during Define; scenarios are agreed, but no implementation or test execution has started.

This document defines the bottom-up acceptance-test layers and their observable pass conditions. Tests proceed from the lowest storage boundary upward. Each layer verifies its own contract without depending on higher-level components; higher layers build on the guarantees established below them. A small end-to-end suite verifies integration across layers but does not replace lower-layer tests.

The acceptance suite establishes correctness and recovery behavior. Benchmark workloads and the measurement protocol are specified separately in [benchmarks.md](benchmarks.md) and have been approved. Numeric product performance and memory targets remain deferred until representative baselines exist and require separate approval; they are not correctness acceptance criteria.

## Layer 1 — Disk and storage

Tests exercise filesystem interaction independently of Events, Channels, transactions, and the event-log runtime.

| ID | Scenario | Pass condition |
|---|---|---|
| D-01 | Create storage file | File is created at the expected location. |
| D-02 | Write data | Written bytes can be read back unchanged. |
| D-03 | Append data | New bytes are appended without changing existing bytes. |
| D-04 | Read data | Reads return the correct bytes and handle EOF correctly. |
| D-05 | Truncate file | File length and remaining contents match the requested truncation. |
| D-06 | Flush/sync | The storage layer reports sync failures rather than silently succeeding. |
| D-07 | Close and reopen | Previously written data remains readable after reopening. |
| D-08 | Missing storage directory | The storage layer handles the missing-directory condition according to its defined contract. |
| D-09 | Permission and I/O failures | Errors are surfaced without being mistaken for successful operations. |
| D-10 | Rename/replace | File replacement behaves correctly, including failure cases. |
| D-11 | Platform parity | The same storage guarantees hold on Linux and Windows. |

Transaction recovery, incomplete-transaction truncation, and committed-record recovery belong to higher layers.

## Layer 2 — Persistence format and transaction

Tests verify on-disk representation and the boundary at which a group of records becomes a committed transaction. They do not require Events, Channels, or Readers.

| ID | Scenario | Pass condition |
|---|---|---|
| F-01 | Encode and decode a valid record | Decoding reproduces the original record without changing its meaning. |
| F-02 | Persistence format version | Supported versions are identified and decoded correctly. |
| F-03 | Unsupported format version | Recovery reports an explicit error; it does not guess or silently reinterpret the data. |
| F-04 | Record framing | Record boundaries are identified correctly, including consecutive records. |
| F-05 | Incomplete record | An incomplete trailing record is distinguishable from a complete record. |
| F-06 | Malformed record | A complete malformed record is rejected explicitly. |
| T-01 | Begin transaction | Transaction records are not visible as committed before the commit boundary. |
| T-02 | Successful commit | All records in a transaction become committed together. |
| T-03 | Interrupted transaction | Recovery discards the incomplete transaction from its beginning. |
| T-04 | Complete transaction | Recovery preserves every record in a completed transaction. |
| T-05 | Transaction ordering | Committed transactions retain their original order. |
| T-06 | Repeated recovery | Recovering the same storage state repeatedly produces the same result. |

An incomplete trailing transaction is recoverable. A complete but malformed record is corruption and must fail closed. Checksums, exact framing sizes, and binary layout are determined by the persistence design, not assumed by these acceptance cases.

## Layer 3 — Metadata and recovery

Tests verify reconstruction of operational state from the authoritative committed append log. Derived metadata must never become a competing source of truth.

| ID | Scenario | Pass condition |
|---|---|---|
| M-01 | Initial metadata creation | Metadata is correctly derived from committed records. |
| M-02 | Missing metadata | Missing metadata is rebuilt from the authoritative append log. |
| M-03 | Inconsistent metadata | Incorrect metadata is detected and rebuilt, or recovery fails explicitly if safe reconstruction is impossible. |
| M-04 | Stale metadata | Metadata left behind by an interrupted operation does not hide or invent committed history. |
| M-05 | Recovery after commit | A transaction committed before a crash is recovered even if metadata updates were incomplete. |
| M-06 | Recovery after incomplete transaction | Uncommitted records do not appear in reconstructed metadata. |
| M-07 | Repeated metadata rebuild | Rebuilding metadata multiple times produces equivalent results. |
| M-08 | Corrupt authoritative history | Recovery fails explicitly rather than using metadata to conceal corruption. |
| M-09 | Recovery consistency | All reconstructed metadata agrees with the committed append history. |

## Layer 4 — Core event-log behavior

Tests exercise event-log domain behavior using the persistence and recovery guarantees established by lower layers.

| ID | Scenario | Pass condition |
|---|---|---|
| L-01 | Append a valid Event | The Event is committed and can be retrieved. |
| L-02 | Append invalid Event data | Validation fails and no partial data is persisted. |
| L-03 | Append with a new Channel | Channel and Event are committed atomically. |
| L-04 | Append with invalid Channel data | Neither Channel nor Event is persisted. |
| L-05 | Event identity | The same Event identity is handled according to the defined identity rules. |
| L-06 | Event occurrences | Repeated occurrences are represented correctly without losing occurrence history. |
| L-07 | Global sequence | Every committed occurrence receives the correct position in global sequence order. |
| L-08 | Retrieval by identity | Retrieval returns the correct Event and occurrence history. |
| L-09 | Typed field validation | Supported field types and nullable values are validated and persisted correctly. |
| L-10 | Unsupported field types | Unsupported types are rejected without partial persistence. |
| L-11 | Concurrent appends | Concurrent operations preserve transaction atomicity and global ordering. |
| L-12 | Append followed by restart | Committed Events and their identities are recovered correctly. |

Event identity and an occurrence of that Event are distinct. Tests must cover both without treating repeated occurrences as duplicate Events or losing their history. Exact identity semantics come from the approved requirements and design.

## Layer 5 — Channel and Reader

Tests verify how each Channel Reader consumes committed global event history, from replay through the live tail. Replay and live delivery form one continuous ordered history without gaps or duplicates at the transition.

| ID | Scenario | Pass condition |
|---|---|---|
| R-01 | Reader initial replay | Reader receives all applicable committed history in global sequence order. |
| R-02 | Reader starts at live tail | Reader correctly handles an empty history and subsequent appends. |
| R-03 | Replay transitions to live | No committed occurrence is missed or duplicated at the transition. |
| R-04 | Append during replay | Concurrent appends are delivered in global sequence order. |
| R-05 | Multiple Channel Readers | Each Reader independently receives its applicable history. |
| R-06 | Independent Reader progress | One slow Reader does not change another Reader's ordering or progress. |
| R-07 | Reader restart | A restarted Reader resumes from the correct pending position. |
| R-08 | Heterogeneous Events | The Reader handles different concrete Event types through the common `Event` interface. |
| R-09 | Snapshot consistency | Delivered Event snapshots remain consistent with the committed state at the defined snapshot boundary. |
| R-10 | Required decoder not registered | Reader waits for required registration without silently discarding persisted Events. |
| R-11 | Decoder registration | Reader resumes correctly when the required decoder becomes available. |
| R-12 | Unknown persisted Event type | The system does not silently skip an Event it cannot decode. |
| R-13 | Reader terminal error | A terminal Reader failure is isolated to that Reader. |
| R-14 | Wake-up coalescing | Missing or coalesced wake-up hints cannot cause missed committed history. |

Wake-up notifications are hints only. The committed append history remains authoritative, so a missed wake-up cannot mean a missed Event.

## Layer 6 — Notification delivery

Tests verify delivery through the approved public Channel interface, `Notifications() <-chan Event`, including ordering, retry, bounded memory, and backpressure.

| ID | Scenario | Pass condition |
|---|---|---|
| N-01 | Successful notification | Committed Event occurrences are delivered to the consumer. |
| N-02 | FIFO delivery | Notifications preserve the correct occurrence order. |
| N-03 | Heterogeneous notification | Different concrete Event types are delivered through `Event`. |
| N-04 | Slow consumer | Slow consumption does not block persistence. |
| N-05 | Full pending buffer | Reader waits when its finite buffer is full and resumes when capacity becomes available. |
| N-06 | Bounded memory | Pending notification memory remains bounded under sustained consumer backpressure. |
| N-07 | Temporary delivery failure | The same pending occurrence is retried. |
| N-08 | Retry success | Successful retry resumes normal FIFO delivery. |
| N-09 | Retry exhaustion | The affected Channel Reader terminates and reports its failure through `Errors()`. |
| N-10 | Retry ordering | Later occurrences cannot overtake the occurrence being retried. |
| N-11 | Recovery of pending notifications | Previously committed but undelivered occurrences are delivered before newer ones. |
| N-12 | Independent notification streams | Failure or backpressure in one Channel does not stop other Channel Readers. |
| N-13 | Consumer crash boundary | Tests demonstrate the documented possibility of duplicate processing around a crash, without claiming exactly-once delivery. |
| N-14 | Error reporting | Fatal delivery errors are reported best-effort, with the first fatal error retained internally. |

The notification stream is not an acknowledgement mechanism. A successful send does not prove that the consumer processed or persisted the Event. Exactly-once consumer processing is not guaranteed. Retry limits, backoff, and pending-buffer capacity remain internal implementation/test policy, not public configuration.

## Layer 7 — Runtime lifecycle

Tests verify process-lifetime runtime ownership, initialization, concurrent use, caller-scoped cancellation, and recovery after process termination.

| ID | Scenario | Pass condition |
|---|---|---|
| RT-01 | Runtime initialization | Runtime initializes successfully and establishes the required resources. |
| RT-02 | Configuration validation | Invalid configuration is rejected without leaving partially initialized runtime state. |
| RT-03 | Repeated configuration | Configuration calls follow the approved configuration contract. |
| RT-04 | Concurrent preparation | Concurrent `PrepareLog` calls safely obtain the appropriate runtime instances. |
| RT-05 | Preparation cancellation | Cancelling a caller's Context stops that caller's preparation or wait without cancelling shared runtime workers. |
| RT-06 | Runtime resource ownership | Shared runtime resources remain owned by the process and are not tied to an individual preparation caller. |
| RT-07 | Process-lifetime workers | Cancelling or completing one caller's Context does not stop shared append, Reader, or delivery workers. |
| RT-08 | Process termination during append | An incomplete trailing transaction is discarded from its beginning during recovery; no partial transaction is exposed. |
| RT-09 | Pending delivery after process termination | Committed undelivered occurrences still present in the authoritative append log are recovered and delivered before newer occurrences on restart. |
| RT-10 | Delivery crash window | Tests permit duplicate consumer delivery around process termination and do not claim exactly-once processing. |
| RT-11 | No graceful process-exit contract | No library-managed process-exit hook, signal handler, graceful drain, final sync, or stream-closure guarantee is required. |
| RT-12 | Independent Reader failure | A failed Channel Reader does not cause unrelated Readers to fail. |
| RT-13 | Repeated process lifetimes | Repeated process starts and recovery do not leak persistent state or corrupt committed history. |

These tests must exercise the process-lifetime contract. They must not introduce a public shutdown method, configurable shutdown timeout, process-wide signal interception, or new consumer-facing cancellation mechanism. Process termination is tested through subprocess boundaries rather than an in-process runtime shutdown API.

## Layer 8 — Public API and end-to-end integration

Tests verify that consumers can use the library through its approved public API and that all lower layers work together. End-to-end tests complement, but do not replace, lower-layer tests.

| ID | Scenario | Pass condition |
|---|---|---|
| API-01 | Public API compilation | A consumer can compile against the approved public API. |
| API-02 | Configuration | Consumer can configure storage using the approved `Config(options ...Option) error` API. |
| API-03 | PrepareLog | Consumer can prepare concrete Event and Channel types using the approved generic API. |
| API-04 | Channel interface | Consumer can receive notifications through `Notifications() <-chan Event`. |
| API-05 | Error interface | Consumer can receive reported errors through `Errors() <-chan error`. |
| API-06 | Concrete Event types | Consumer receives concrete Event values through the common Event interface. |
| API-07 | Multiple Channels | Consumer can use multiple independent Channels in one runtime. |
| API-08 | Append and replay | Consumer appends Events, restarts the runtime, and receives the committed history. |
| API-09 | Replay and live continuity | Consumer receives historical and newly committed Events in the correct order without gaps or duplicates during normal replay-to-live progression; crash-window duplicates remain permitted by the delivery contract. |
| API-10 | Concurrent consumers | Multiple consumers operate concurrently without violating the approved guarantees. |
| API-11 | Recovery after process termination | Consumer can restart after an abrupt process termination and recover committed history. |
| API-12 | Platform integration | The same public workflows work on Linux and Windows. |
| API-13 | Public API stability | No unapproved public API changes, callbacks, handles, or lifecycle controls are introduced. |

End-to-end tests use actual consumer-defined Event and Channel types, not only internal test types.

## Test execution and evidence

- Implement and run tests from Layer 1 upward. A higher layer must not be used to mask a failure in a lower layer.
- Exercise actual interruption boundaries where relevant, including process termination during append, after commit but before derived metadata is fully updated, and during compaction.
- Repeat recovery after interruption to establish that recovery is stable and idempotent.
- Run the platform-specific storage and integration cases on both supported operating systems: Linux and Windows.
- Keep test-only fault injection and process termination controls out of the public API.
- Record test commands, platform, outcomes, and relevant failure evidence when implementation begins. This document defines acceptance criteria; it is not evidence that tests have run.

## Separate Define work

The acceptance scenarios above are agreed. Benchmark workload scenarios are separately defined in [Benchmark strategy](benchmarks.md) and are also agreed. The following remain separate Define work:
- Establish representative performance and memory baselines after implementation exists; then propose any justified numeric product targets or thresholds for separate approval.
- Establish representative performance and memory baselines after implementation exists; any numeric product targets require separate justification and approval.
