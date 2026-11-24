# Benchmark strategy

**Status:** Benchmark workload scenarios approved during Define. Numeric performance and memory targets have not yet been approved. No benchmark implementation or measurements have started.

This document defines the benchmark workloads used to evaluate the eventlog implementation. It is separate from the acceptance-test matrix: acceptance tests establish correctness, while benchmarks measure performance and resource behavior. A benchmark result is not valid unless its workload also verifies the relevant correctness guarantees.

## Measurement methodology

Unless a workload specifies otherwise:

- Record throughput in operations/second, records/second, or MB/second as appropriate.
- Record latency distributions, at minimum p50, p95, and p99; do not rely on averages alone.
- Record CPU usage, memory usage, allocations, and bytes allocated per operation where applicable.
- Measure peak memory for recovery, replay, reconstruction, and backlog workloads.
- Distinguish ordinary write/append latency from durable commit latency wherever the storage contract requires synchronization.
- Vary one major workload dimension at a time where practical, including record size, payload size, history size, transaction size, concurrency, Reader count, and consumer speed.
- Document hardware, operating system, filesystem, runtime/toolchain version, configuration, and benchmark command so results can be compared meaningfully.
- Run comparable workloads on Linux and Windows. Document environmental differences rather than treating unlike hardware or filesystems as directly comparable.
- Include correctness checks and report failures; never count lost, duplicated, reordered, or incorrectly recovered data as successful throughput.
- Numeric thresholds, target hardware, required repetitions, warm-up policy, and statistical acceptance rules remain to be agreed separately. No performance or memory target is implied by these workload definitions.

## Layer 1 — Disk and storage

| ID | Workload | Measurements |
|---|---|---|
| B-D01 | Sequential writes | Throughput and write latency |
| B-D02 | Sequential reads | Throughput and read latency |
| B-D03 | Small append operations | Operations/second and latency distribution |
| B-D04 | Large append operations | Throughput and latency distribution |
| B-D05 | Flush/sync operations | Sync latency distribution |
| B-D06 | File reopen and recovery reads | Startup time and read throughput |
| B-D07 | File truncation and replacement | Operation latency |
| B-D08 | Concurrent storage access | Throughput and contention as concurrency increases |

Use representative small, medium, and large records. Distinguish ordinary write latency from durable commit latency where synchronization is required. Record throughput (ops/sec or MB/sec), p50/p95/p99 latency, CPU, and memory allocation/bytes per operation where applicable.

## Layer 2 — Persistence format and transactions

| ID | Workload | Measurements |
|---|---|---|
| B-F01 | Encode records | Records/second, throughput, latency, allocations |
| B-F02 | Decode records | Records/second, throughput, latency, allocations |
| B-F03 | Encode/decode across record sizes | Throughput and latency for small, medium, and large records |
| B-F04 | Frame and parse record streams | Throughput, latency, and allocations across batches |
| B-T01 | Commit single-record transactions | Commit throughput and durable commit latency |
| B-T02 | Commit multi-record transactions | Throughput and latency across transaction sizes |
| B-T03 | Recover completed transactions | Recovery throughput and startup time |
| B-T04 | Recover after interrupted transactions | Recovery time and work to identify/discard incomplete transactions |
| B-T05 | Recover large transaction histories | Recovery time and memory usage as history size increases |

Separate encoding/decoding cost from storage I/O and durable commit cost. Vary record size and transaction size independently. Measure steady-state transaction throughput and recovery performance. Measure allocations and peak recovery memory. Malformed-record handling is a robustness concern, not normal throughput.

## Layer 3 — Metadata and recovery

| ID | Workload | Measurements |
|---|---|---|
| B-M01 | Load valid metadata | Metadata load latency and allocations |
| B-M02 | Rebuild missing metadata | Rebuild duration, throughput, and peak memory |
| B-M03 | Rebuild inconsistent or stale metadata | Rebuild duration and correctness-validation overhead |
| B-M04 | Recover with metadata update interrupted | Recovery duration and committed-history processing throughput |
| B-M05 | Repeated metadata rebuild | Duration and resource usage across repeated runs |
| B-M06 | Recover large histories | Recovery duration, memory growth, and scaling with history size |
| B-M07 | Validate metadata against append history | Validation duration and throughput |

Vary committed history size and metadata size independently. Include different Event distributions. Distinguish normal metadata loading from full reconstruction. Measure peak reconstruction and validation memory. Verify reconstructed metadata against authoritative committed history.

## Layer 4 — Core event-log behavior

| ID | Workload | Measurements |
|---|---|---|
| B-L01 | Append individual Events | Append throughput, latency distribution, allocations |
| B-L02 | Append Events with Channel creation | Throughput and latency, including Channel creation overhead |
| B-L03 | Retrieve Events by identity | Lookup latency and throughput across history sizes |
| B-L04 | Retrieve occurrence history | Retrieval throughput and latency across occurrence counts |
| B-L05 | Append Events concurrently | Aggregate throughput, per-append latency, contention by writer count |
| B-L06 | Append repeated occurrences of one identity | Throughput and latency as occurrence history grows |
| B-L07 | Append and recover Events | Combined append/restart/recovery duration and resource usage |
| B-L08 | Validate typed Event fields | Validation throughput, latency, and allocations |
| B-L09 | Append Events with varied payload sizes | Throughput, latency, and allocations across payload sizes |

Vary history size, writer count, payload size, and identity distribution independently. Include unique identities and repeated occurrences. Distinguish append acceptance latency from durable commit latency where required. Measure contention and tail latency. Verify no lost, duplicated, or incorrectly ordered committed occurrences.

## Layer 5 — Channel and Reader

| ID | Workload | Measurements |
|---|---|---|
| B-R01 | Initial replay of committed history | Replay throughput, completion time, memory |
| B-R02 | Replay with concurrent appends | Replay throughput, append impact, ordering overhead |
| B-R03 | Empty history transitioning to live tail | Live-delivery latency and CPU |
| B-R04 | Multiple independent Channel Readers | Aggregate throughput and per-Reader latency as Reader count increases |
| B-R05 | Readers with different processing speeds | Per-Reader progress, latency, and isolation |
| B-R06 | Reader restart with pending history | Restart time and time to resume pending delivery |
| B-R07 | Heterogeneous concrete Event types | Dispatch throughput, latency, allocations |
| B-R08 | Snapshot consistency during concurrent appends | Snapshot completion time and consistency-validation overhead |
| B-R09 | Coalesced or missed wake-up hints | Recheck overhead, detection latency, CPU |
| B-R10 | Missing decoder followed by registration | Waiting resource usage and time to resume processing |

Vary history size, Reader count, Event-type diversity, and append rate independently. Measure replay separately from live-tail delivery and test continuity across the transition. Include slow Readers and measure retained memory per Reader. Treat wake-up hints only as hints; verify that authoritative committed history is not missed.

## Layer 6 — Notification delivery

| ID | Workload | Measurements |
|---|---|---|
| B-N01 | Successful notification delivery | Delivery throughput and end-to-end latency |
| B-N02 | FIFO delivery under sustained load | Throughput, latency distribution, ordering overhead |
| B-N03 | Slow consumer | Delivery backlog growth, Reader progress, persistence impact |
| B-N04 | Full bounded pending buffer | Time to backpressure, memory ceiling, resume latency |
| B-N05 | Temporary delivery failures with eventual success | Retry overhead, recovery time, delivery latency |
| B-N06 | Retry exhaustion | Time to terminal failure, resource usage, isolation impact |
| B-N07 | Multiple Channel streams with uneven consumers | Aggregate throughput and per-Channel latency |
| B-N08 | Recovery with committed pending notifications | Recovery-to-delivery time and backlog drain throughput |
| B-N09 | Consumer interruption at delivery boundaries | Duplicate exposure and recovery behavior |
| B-N10 | Sustained notification load | Long-run throughput, latency stability, memory boundedness |

Vary consumer speed, Event size, pending-buffer occupancy, and Channel count independently. Measure end-to-end notification latency separately from append/commit latency. Include retry and backpressure in sustained-load tests. Verify retry ordering and bounded memory. A successful channel send does not prove consumer processing and does not establish exactly-once delivery.

## Layer 7 — Runtime lifecycle

| ID | Workload | Measurements |
|---|---|---|
| B-RT01 | Runtime initialization | Initialization latency, allocations, resource usage |
| B-RT02 | Configuration validation | Validation latency and allocations for valid/invalid options |
| B-RT03 | Concurrent PrepareLog calls | Preparation throughput, latency, contention |
| B-RT04 | Repeated preparation against shared runtime | Per-call latency and resource growth |
| B-RT05 | Graceful shutdown with pending deliveries | Drain duration, throughput, completion rate |
| B-RT06 | Shutdown after grace-period expiry | Timeout handling latency, cleanup duration, resource release |
| B-RT07 | Repeated runtime lifecycle cycles | Resource growth, goroutine count, memory stability |
| B-RT08 | Shutdown with multiple active Channel Readers | Shutdown duration and per-Reader cleanup |
| B-RT09 | Preparation cancellation under shared runtime load | Cancellation response time and impact on unrelated callers |
| B-RT10 | Resource cleanup after Reader failure | Cleanup latency and residual resource usage |

Measure goroutine count, memory, open file descriptors/handles, and other relevant resources before and after lifecycle cycles. Separate graceful shutdown from grace-period expiry. Include concurrent preparation and active delivery. Verify one caller's cancellation does not disrupt shared workers. The shutdown grace period is fixed internal policy, not a tunable public configuration option.

## Layer 8 — Public API and end-to-end

| ID | Workload | Measurements |
|---|---|---|
| B-API01 | Public API consumer compilation | Build time and dependency footprint |
| B-API02 | Configure and prepare a log | End-to-end preparation latency and allocations |
| B-API03 | Append and receive an Event | End-to-end commit-to-notification latency |
| B-API04 | Append and replay after restart | Restart-to-replay completion time |
| B-API05 | Multiple Channels in one runtime | Aggregate throughput, per-Channel latency, resource usage |
| B-API06 | Concurrent consumer workload | Aggregate throughput, latency distribution, resource usage |
| B-API07 | Abrupt process termination and recovery | Restart time, recovery throughput, time to resume delivery |
| B-API08 | Linux and Windows integration workload | Comparative throughput, latency, resource usage |
| B-API09 | Long-running end-to-end workload | Throughput/latency stability, memory and resource growth |
| B-API10 | Public API compatibility check | Build compatibility against representative existing consumer code |

Use consumer-defined concrete Event and Channel types. Include the complete public workflow from `Config` through `PrepareLog`, append, notification, restart, and recovery. Keep compilation/API compatibility separate from runtime performance. Compare Linux and Windows while documenting hardware and filesystem differences. Verify that the public API remains unchanged: no internal Log handle, consumer callback, public shutdown control, or unapproved lifecycle API.

## Benchmark execution strategy

The approved execution strategy is designed around the resource limits of free GitHub-hosted runners for a public repository. It avoids requiring self-hosted runners, paid performance hardware, scheduled benchmark jobs, or expensive benchmark execution on every pull request.

There are two execution paths:

1. **Pull-request smoke suite:** Run a small, fast, representative subset on Linux for early detection of obvious performance regressions.
2. **Manual benchmark runs:** Run the full approved benchmark matrix or selected extended workloads only when explicitly requested. This includes Windows coverage and resource-intensive investigations such as very large histories, sustained-load tests, high concurrency, slow-consumer/backpressure stress, and interruption/recovery workloads.

There are no scheduled benchmark runs.

### Approved pull-request smoke suite

The PR smoke suite contains one representative benchmark scenario from each of the eight layers:

| Layer | Scenario | Workload |
|---|---|---|
| 1. Disk and storage | B-D03 | Small append operations |
| 2. Persistence format and transactions | B-T01 | Commit a single-record transaction |
| 3. Metadata and recovery | B-M01 | Load valid metadata |
| 4. Core event-log behavior | B-L01 | Append individual Events |
| 5. Channel and Reader | B-R01 | Initial replay of committed history |
| 6. Notification delivery | B-N01 | Successful notification delivery |
| 7. Runtime lifecycle | B-RT03 | Concurrent PrepareLog calls |
| 8. Public API and end-to-end | B-API03 | Append and receive an Event |

Resource constraints for this suite:

- Linux only.
- One benchmark execution per scenario initially; no repeated statistical sampling in the PR path.
- Small, fixed, bounded datasets and short benchmark durations.
- No large-history, sustained-load, or high-concurrency stress workloads.
- No strict timing-based pass/fail thresholds initially. The smoke suite is an early-warning signal, not a statistically strong performance gate.
- Correctness tests remain mandatory and independent of benchmark results; benchmark success cannot substitute for acceptance tests.

The full benchmark matrix is not required on every pull request. Linux and Windows remain first-class correctness and benchmark platforms, but Windows benchmark runs are manual. Keep platform results separate and do not treat their absolute timings as directly comparable without accounting for runner and environment differences.

Performance benchmark results are informational and diagnostic. There are no hard performance thresholds in CI, and performance measurements must not fail a pull request based on timing, throughput, or resource-use values. Correctness and acceptance tests remain independently enforceable and may fail CI for correctness defects. The exact manual workload sizes, repetition counts, and any future performance regression gates remain to be established against actual free-runner limits and observed measurement noise. Benchmark execution must remain bounded and must not consume disproportionate CI capacity.

## Remaining Define work

- Agree numeric performance targets and thresholds for the workloads.
- Agree measurable memory targets and measurement methodology, including representative target environments and resource ceilings.
- Complete the consistency review across requirements, architecture, decisions, acceptance tests, and benchmark strategy.

No benchmark results or performance guarantees are claimed by this document.
