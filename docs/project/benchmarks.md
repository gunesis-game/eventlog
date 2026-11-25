# Benchmark strategy

**Status:** Benchmark workload scenarios, initial workload profiles, per-process diagnostic resource ceilings, and the measurement protocol are approved during Define. Numeric performance and product memory acceptance thresholds remain unapproved pending representative baselines. No benchmark implementation or measurements have started.

This document defines the benchmark workloads used to evaluate the eventlog implementation. It is separate from the acceptance-test matrix: acceptance tests establish correctness, while benchmarks measure performance and resource behavior. A benchmark result is not valid unless its workload also verifies the relevant correctness guarantees.

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
- The ceilings bound benchmark resource consumption on the selected runner class; they do not establish a product memory guarantee. Revisit them if observed runner capacity or workload needs justify a change.
- Record the actual runner image/class, allocated CPU and memory, available disk, OS, filesystem, and toolchain version with every result. Runner specifications can change; record observed environment details rather than assuming a historical specification.
- Run Linux and Windows separately. The profiles and resource budgets apply to both, but raw timings across different runner images/platforms must not be treated as directly comparable.

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
- **PR smoke suite:** execute each selected representative scenario once, without repeated statistical sampling. Treat results as informational early-warning evidence only.
- **Full/manual measurements:** perform one unreported warm-up execution followed by five measured repetitions for each workload/environment combination. Preserve the five individual results; report the median, minimum-to-maximum range, and median absolute deviation (MAD), alongside the workload's per-run latency percentiles and other applicable metrics. Do not discard an inconvenient sample unless a documented execution fault invalidates it; retain and explain invalidated samples.
- Record the exact source SHA, runner image/class, allocated CPU and memory, available disk, operating system, filesystem, Go toolchain version, configuration, workload inputs, and exact command for every run. Compare baselines only within materially comparable environments; keep Linux and Windows results separate.
- No numeric performance or product memory pass/fail thresholds are approved. Establish representative baselines first, then propose any justified numeric targets as a separately approved decision. Until then, all performance results are informational; no performance or memory measurement may fail CI based on a numeric threshold. Correctness failures remain independently enforceable.

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

The approved execution strategy is designed around the resource limits of free GitHub-hosted runners for a public repository. It avoids requiring self-hosted runners, paid performance hardware, scheduled benchmark jobs, or expensive benchmark execution on every push to `main`.

There are two execution paths:

1. **Main-branch smoke suite:** Run a small, fast, representative subset on Linux for early detection of obvious performance regressions.
2. **Manual benchmark runs:** Run the full approved benchmark matrix or selected extended workloads only when explicitly requested. This includes Windows coverage and resource-intensive investigations such as very large histories, sustained-load tests, high concurrency, slow-consumer/backpressure stress, and interruption/recovery workloads.

There are no scheduled benchmark runs.

### Approved main-branch smoke suite

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
- One benchmark execution per scenario; no repeated statistical sampling in the PR path. The five-repetition protocol applies to full/manual measurements, not the smoke suite.
- Small, fixed, bounded datasets and short benchmark durations.
- No large-history, sustained-load, or high-concurrency stress workloads.
- No timing- or product-memory-based pass/fail thresholds. The smoke suite is an early-warning signal, not a statistically strong performance gate.
- Correctness tests remain mandatory and independent of benchmark results; benchmark success cannot substitute for acceptance tests.

The full benchmark matrix is not required on every push to `main`. Linux and Windows remain first-class correctness and benchmark platforms, but Windows benchmark runs are manual. Keep platform results separate and do not treat their absolute timings as directly comparable without accounting for runner and environment differences.

Performance benchmark results are informational and diagnostic. There are no hard performance thresholds in CI, and performance measurements must not fail a CI run based on timing, throughput, or resource-use values. Correctness and acceptance tests remain independently enforceable and may fail CI for correctness defects. Extended manual workload sizes remain to be selected within actual free-runner limits. The warm-up and five-measured-repetition protocol is approved; any future numeric performance regression gates remain deferred until representative baselines and observed measurement noise justify a separately approved target. Benchmark execution must remain bounded and must not consume disproportionate CI capacity.

## Benchmark artifact retention and comparable-run selection

Retain benchmark artifacts for **20 days** from the workflow run that produced them. This applies to uploaded benchmark evidence, including raw measured samples, summaries, workload inputs, correctness outcomes, and environment metadata. It is separate from GitHub Actions workflow run-history retention below.

- Retain evidence from successful, failed, and incomplete/resource-limited runs for the retention period so that investigations remain possible.
- A cancelled run does not need benchmark artifact retention.
- For full/manual measurements, retain all individual measured samples, the unreported warm-up marker/result where available, summary statistics, correctness outcome, exact source SHA, and the complete environment/workload/command metadata already required by this document.
- For smoke runs, retain the summary and environment metadata; raw output may also be retained where useful.
- Expiry after 20 days is an evidence-storage policy, not permission to delete source specifications, decision records, or other project governance history.

Select a comparison baseline only from the **latest successful, complete, correctness-valid run** that is materially comparable to the run being evaluated. Comparability requires the same workload ID and workload parameters, operating system, filesystem, materially equivalent runner image/class and allocated resources, Go toolchain version, and relevant configuration. Record the source SHA for every run; different source SHAs are expected when measuring changes and do not by themselves make runs incomparable.

- Do not use failed, cancelled, incomplete/resource-limited, or correctness-invalid runs as baselines.
- Keep Linux and Windows in separate comparison series.
- Treat material changes to runner image/class, allocated resources, OS, filesystem, Go toolchain, workload parameters, or relevant configuration as a new comparison series rather than silently combining results.
- Preserve and report invalidated samples with their reason; do not silently discard them or use them to claim a successful baseline.
- A successful comparable run is eligible evidence, not an automatically approved product performance target or acceptance threshold.

## GitHub Actions run-history retention

For each individual GitHub Actions workflow, retain the four most recent workflow runs. When that workflow starts, its run-history cleanup should remove older runs belonging to that same workflow, leaving the four newest runs available for comparison.

Retention is per workflow, not a repository-wide limit. Runs belonging to other workflows must not be deleted by this workflow's cleanup. The retained run history is intended to support comparison of benchmark results and execution behavior.

Retain failed runs as well as successful runs so failures remain available for investigation. Cancelled runs are excluded from the four-run retention set and should be removed during cleanup. The four retained runs are therefore the four most recent non-cancelled runs for that workflow.

The eventual cleanup implementation must account for the currently running workflow run, required GitHub token permissions, and concurrent runs so that it does not accidentally delete one of the four runs intended to be retained. This policy does not itself specify a benchmark schedule or create scheduled runs.

## Remaining Define work

- Establish representative performance and memory baselines when an implementation exists; then propose any justified numeric product targets and thresholds for approval.
- Reassess the diagnostic runner resource ceilings only if observed runner capacity or workload needs justify a change.
- Complete the consistency review across requirements, architecture, decisions, acceptance tests, and benchmark strategy.

No benchmark results or performance guarantees are claimed by this document.
