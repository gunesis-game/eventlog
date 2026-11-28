# Benchmark strategy

**Status:** The workload scenarios, workload profiles, diagnostic resource ceilings, measurement protocol (D-040), and artifact retention (D-041) are approved. The [smoke suite](#smoke-suite) is implemented; no baseline has been measured. Numeric performance and product memory thresholds are not approved.

This document is the single definition of eventlog's benchmark workloads, metrics, and measurement protocol; [performance-strategy.md](performance-strategy.md) records applicability, environments, and CI policy. Acceptance tests ([acceptance.md](acceptance.md)) establish correctness, while benchmarks measure performance and resource behavior. A benchmark result is not valid unless its workload also verifies the relevant correctness guarantees.

## Initial workload profiles and resource budgets

The following profiles are approved as initial benchmark workload scales. They describe workloads to exercise, not guaranteed implementation performance or hard CI pass/fail thresholds.

| Profile | Sustained/burst Event rate | Average Event size | Retained history | Channel count |
|---|---:|---:|---:|---:|
| Typical | 100 Events/second | 1 KiB | 100,000 Events | 10 |
| Heavy | 1,000 Events/second | 1 KiB | 1,000,000 Events | 50 |
| Stress | 5,000 Events/second burst | 1 KiB | 1,000,000 Events | 100 |

- The Stress profile includes slow consumers and bounded notification backpressure.
- Larger payloads are covered by separate size-variation workloads; the 1 KiB average is not a maximum Event size.
- The profiles are starting points. Benchmark results may motivate revision, but changes must be documented rather than silently tuning workloads to improve results.

Initial per-process resource budgets on the standard GitHub-hosted runner class used for these benchmarks:

| Profile | Process memory budget |
|---|---:|
| Typical | 512 MiB |
| Heavy | 1 GiB |
| Stress | 2 GiB |

- Benchmark data stored on the runner is capped at 4 GiB per workload execution.
- These memory and disk budgets are diagnostic execution ceilings, not product memory/performance acceptance thresholds, measured capabilities, or performance guarantees. If a workload exceeds a ceiling, stop or contain that execution where practicable, mark its result **incomplete/resource-limited**, and record the observed usage and termination reason. Do not report it as a successful benchmark or infer that the product failed an unapproved memory target.
- Revisit the ceilings only if observed runner capacity or workload needs justify a change.
- The profiles and ceilings apply to both Linux and Windows.

## Measurement methodology

Unless a workload specifies otherwise:

- Record throughput in operations/second, records/second, or MB/second as appropriate.
- Record latency distributions, at minimum p50, p95, and p99; do not rely on averages alone.
- Record CPU usage, memory usage, allocations, and bytes allocated per operation where applicable.
- Measure peak memory for recovery, replay, reconstruction, and backlog workloads.
- Distinguish ordinary write/append latency from durable commit latency wherever the storage contract requires synchronization.
- Vary one major workload dimension at a time where practical, including record size, payload size, history size, transaction size, concurrency, Reader count, and consumer speed.
- Include correctness checks and report failures; never count lost, duplicated, reordered, or incorrectly recovered data as successful throughput.
- **Smoke suite:** execute each selected representative scenario once, without repeated statistical sampling. Treat results as informational early-warning evidence only.
- **Full/manual measurements:** perform one unreported warm-up execution followed by five measured repetitions for each workload/environment combination. Preserve the five individual results; report the median, minimum-to-maximum range, and median absolute deviation (MAD), alongside the workload's per-run latency percentiles and other applicable metrics. Do not discard an inconvenient sample unless a documented execution fault invalidates it; retain and explain invalidated samples.
- Record with every run the exact source SHA, runner image/class (observed, not assumed from a historical specification), allocated CPU and memory, available disk, operating system, filesystem, Go toolchain version, configuration, workload inputs, and exact command.
- Run Linux and Windows separately and keep their results in separate series; do not treat raw timings across different hardware, runner images, or filesystems as directly comparable.
- No numeric performance or product memory pass/fail thresholds are approved. Establish representative baselines first, then propose any justified numeric targets as a separately approved decision. Until then, all performance results are informational and no performance or memory measurement may fail CI. Correctness failures remain independently enforceable.

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

## Layer 7 — Runtime initialization and process recovery

| ID | Workload | Measurements |
|---|---|---|
| B-RT01 | Runtime initialization | Initialization latency, allocations, resource usage |
| B-RT02 | Configuration validation | Validation latency and allocations for valid/invalid options |
| B-RT03 | Concurrent PrepareLog calls | Preparation throughput, latency, contention |
| B-RT04 | Repeated preparation against shared runtime | Per-call latency and resource growth |
| B-RT05 | Abrupt process termination with pending committed deliveries | Restart recovery time, pending-history replay time, duplicate exposure |
| B-RT06 | Process termination during an incomplete append transaction | Recovery time and proof that the incomplete trailing transaction is discarded |
| B-RT07 | Repeated process start/termination cycles | Startup/recovery time, memory and OS-resource stability across process instances |
| B-RT08 | Process termination with multiple active Channel Readers | Recovery time and independent resumption of each Channel's pending history |
| B-RT09 | Preparation cancellation under shared runtime load | Cancellation response time and impact on unrelated callers |
| B-RT10 | Resource cleanup after Reader failure | Cleanup latency and residual resource usage |

Measure initialization latency, goroutine count, memory, open file descriptors/handles, and other relevant resources within each process instance. Include concurrent preparation and active delivery. Verify one caller's cancellation does not disrupt shared workers. Process termination is external: eventlog provides no shutdown, drain, final-sync, or cleanup operation. Recovery checks must distinguish committed pending occurrences (which are replayed) from incomplete trailing transactions (which are discarded).

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

The execution strategy fits the resource limits of free GitHub-hosted runners for a public repository. It needs no self-hosted runners, paid performance hardware, scheduled benchmark jobs, or expensive benchmark execution on every push to `main`. There are two execution paths, and no scheduled benchmark runs:

1. **Smoke suite:** a small, fast, representative subset for early detection of obvious regressions. Intended to run on Linux; no benchmark workflow is configured yet, so it currently runs locally ([commands.md](commands.md)).
2. **Manual benchmark runs:** the full matrix or selected extended workloads, only when explicitly requested. This includes Windows coverage and resource-intensive investigations such as very large histories, sustained load, high concurrency, slow-consumer/backpressure stress, and interruption/recovery workloads. Extended workload sizes remain to be selected within actual free-runner limits.

### Smoke suite

The smoke suite contains one representative scenario from each of the eight layers, plus three extra workloads. Every smoke benchmark checks the correctness of its workload and fails rather than hangs when a commit or notification does not arrive.

| Layer | Scenario | Benchmark |
|---|---|---|
| 1. Disk and storage | B-D03 Small append operations (128 bytes) | `BenchmarkSmallAppend` (`internal/filesystem`) |
| 2. Persistence format and transactions | B-T01 Commit a single-record transaction (1 KiB) | `BenchmarkCommitSingleTransaction` (`internal/storage`) |
| 3. Metadata and recovery | B-M01 Load valid metadata (manifest and index of 1,000 occurrences) | `BenchmarkLoadMetadata` (`internal/storage`) |
| 4. Core event-log behavior | B-L01 Append individual Events (64 transactions per operation, until committed) | `BenchmarkAppendEvent` (root) |
| 5. Channel and Reader | B-R01 Initial replay of 1,000 committed occurrences | `BenchmarkReplayDelivery` (root) |
| 6. Notification delivery | B-N01 Successful live notification delivery | `BenchmarkNotificationDelivery` (root) |
| 7. Runtime lifecycle | B-RT03 Eight concurrent PrepareLog calls on one runtime | `BenchmarkConcurrentPrepare` (root) |
| 8. Public API and end-to-end | B-API03 Append and receive an Event through the public API | `BenchmarkPublicAppendAndReceive` (root, external test package) |
| Extra | B-D02 Ranged read (200 bytes) | `BenchmarkReadRoundTrip` (`internal/filesystem`) |
| Extra | B-F01 Encode and validate a 1 KiB Event record | `BenchmarkEncodeEventRecord` (`internal/record`) |
| Extra | B-T03 / B-M06 Recover 1,000 and 10,000 committed transactions | `BenchmarkRecoverHistory` (`internal/storage`) |

B-T01 measures commits without a per-commit sync; the store's periodic sync is the durability boundary, so durable commit latency is not measured yet. `BenchmarkPublicAppendAndReceive` calls the one-time `Config` once per process and leaves its storage directory in the system temporary directory, because the process-lifetime runtime holds it until exit.
Constraints for the smoke suite:

- One execution per scenario; the five-repetition protocol applies to full/manual measurements only.
- Small, fixed, bounded datasets and short durations; no large-history, sustained-load, or high-concurrency stress workloads.
- No timing- or memory-based pass/fail thresholds. It is an early-warning signal, not a statistical performance gate.
- Correctness tests remain mandatory and independent of benchmark results.

## Benchmark artifact retention and comparable-run selection

Retain benchmark artifacts for **20 days** from the workflow run that produced them. This applies to uploaded benchmark evidence, including raw measured samples, summaries, workload inputs, correctness outcomes, and environment metadata. GitHub Actions run-history retention is separate and defined in the [Git Workflow Standard](../standards/git-workflow.md#workflow-run-retention-and-scheduling).

- Retain evidence from successful, failed, and incomplete/resource-limited runs for the retention period so that investigations remain possible. A cancelled run needs no artifact retention.
- For full/manual measurements, retain all individual measured samples, the warm-up result where available, summary statistics, correctness outcome, and the run metadata listed under [Measurement methodology](#measurement-methodology).
- For smoke runs, retain the summary and environment metadata; raw output may also be retained where useful.
- Expiry after 20 days applies to benchmark evidence only, never to specifications or decision records.

Select a comparison baseline only from the **latest successful, complete, correctness-valid run** that is materially comparable to the run being evaluated. Comparability requires the same workload ID and workload parameters, operating system, filesystem, materially equivalent runner image/class and allocated resources, Go toolchain version, and relevant configuration. Different source SHAs are expected when measuring changes and do not by themselves make runs incomparable.

- Do not use failed, cancelled, incomplete/resource-limited, or correctness-invalid runs as baselines.
- Treat a material change to runner, resources, OS, filesystem, Go toolchain, workload parameters, or configuration as a new comparison series rather than silently combining results.
- Preserve and report invalidated samples with their reason.
- A successful comparable run is eligible evidence, not an approved performance target or acceptance threshold.

## Open items

- Measure durable commit latency (B-T01 with sync) separately from append latency.
- Measure representative performance and memory baselines, then propose any justified numeric targets for approval.

No benchmark results or performance guarantees are claimed by this document.