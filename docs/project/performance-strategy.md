# Project Performance Strategy

## Purpose and status

This document is the project-specific implementation of the [Performance Engineering Standard](../standards/performance-engineering.md). The approved workload catalogue is maintained in [benchmarks.md](benchmarks.md).

**Current status:** Benchmark workload scenarios and the measurement protocol are approved during Define. Numeric product performance and memory targets remain unapproved pending representative baselines. The initial GitHub-hosted runner resource ceilings are diagnostic execution limits, not product acceptance thresholds. No benchmark implementation or measurements have started.

## Applicability and goals

Performance engineering applies to the disk-backed append-only log, transaction commit and recovery, metadata reconstruction, Event append/retrieval, Channel replay, live tailing, and notification delivery.

Relevant constraints are defined in [requirements.md](requirements.md) and the workload intent is detailed in [benchmarks.md](benchmarks.md). No numeric throughput, latency, startup-time, CPU, allocation, or memory target is currently approved. Workload definitions are not implied thresholds.

## Workload catalogue

The benchmark catalogue in [benchmarks.md](benchmarks.md) is authoritative. It covers:

- Disk and storage operations (B-D workload IDs).
- Persistence encoding, decoding and transactions (B-F and B-T workload IDs).
- Metadata and recovery (B-M workload IDs).
- Core Event-log behavior (B-L workload IDs).
- Channel and Reader behavior (B-R workload IDs).
- Notification delivery and lifecycle workloads, where defined in the benchmark catalogue.

Do not duplicate the workload definitions here. Changes to workload scope belong in benchmarks.md and require the applicable approval and decision record.

## Metrics and targets

Measure the metrics specified by each workload, as applicable:

- Throughput in operations/second, records/second, or MB/second.
- Latency distributions, at minimum p50, p95, and p99.
- CPU usage, memory usage, allocations, and bytes allocated per operation.
- Peak memory for recovery, replay, reconstruction, and backlog workloads.
- Durable commit latency separately from ordinary write/append latency wherever synchronization is required.

Baselines and numeric product targets remain TBD. Establish baselines using the protocol in benchmarks.md before proposing thresholds; do not derive targets from a single measurement or treat an informational baseline as an acceptance threshold.

## Environment and execution

- Compare Linux and Windows for comparable workloads and document filesystem and environment differences.
- Record hardware, operating system, filesystem, Go toolchain version, configuration, workload inputs, and exact benchmark command with every result.
- Vary major workload dimensions independently where practical, including record/payload size, history size, transaction size, concurrency, Reader count, and consumer speed.
- Include correctness checks; lost, duplicated, reordered, or incorrectly recovered committed data invalidates the measurement.
- The approved protocol is one warm-up followed by five measured repetitions per workload/environment for full/manual measurements. Report the median, min-to-max range, and median absolute deviation (MAD), retaining individual samples. The PR smoke suite runs once per scenario and is informational.
- Record source SHA, runner image/class and observed resources, OS, filesystem, Go toolchain, configuration, inputs, and exact command. Compare only materially comparable environments and report Linux and Windows separately.
- The memory and disk budgets in benchmarks.md are diagnostic execution ceilings. An exceeded ceiling makes that run incomplete/resource-limited; it is not a product acceptance failure or evidence of a product memory guarantee.
- No benchmark CI workflow is currently configured. Results remain informational; no numeric performance or product-memory threshold may fail CI. Correctness checks remain independent.

## Baselines, reporting and retention

No baseline revision or benchmark artifact exists yet. When measurements begin, retain benchmark artifacts for 20 days and preserve the exact source SHA, environment description, workload parameters, raw data, summary distributions, and correctness outcome. Select only the latest successful, complete, correctness-valid run with the same workload parameters and a materially comparable OS, filesystem, runner environment/resources, Go toolchain, and configuration. Keep materially different environments in separate comparison series. Failed, cancelled, incomplete/resource-limited, and correctness-invalid runs are not eligible baselines. See benchmarks.md for the authoritative retention and selection rules.

## Exclusions and approvals

No workload category is excluded. Numeric targets are intentionally absent pending approval. Changes to targets, thresholds, workload definitions, measurement policy, or evidence retention require approval and decision-ledger recording.

## Define approval checklist

- [x] Applicability and workload categories are documented.
- [x] Workload scenarios are defined in benchmarks.md.
- [ ] Representative baselines are measured once an implementation exists; environment recording and comparability controls are defined.
- [x] Numeric targets are explicitly deferred until representative baselines exist; no hard performance or product-memory threshold is currently approved.
- [x] Execution, statistical reporting, artifact retention, and comparable-run selection are defined; numeric regression thresholds remain deferred pending representative baselines and separate approval.
- [x] The measurement and evidence policy is approved and recorded in D-040/D-041; numeric targets remain deferred pending representative baselines.
