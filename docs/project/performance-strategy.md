# Project Performance Strategy

## Purpose and status

This document is the project-specific implementation of the [Performance Engineering Standard](../standards/performance-engineering.md). The approved workload catalogue is maintained in [benchmarks.md](benchmarks.md).

**Current status:** Benchmark workload scenarios are approved during Define. Numeric performance and memory targets, target hardware, required repetitions, warm-up policy, and statistical acceptance rules have not been approved. No benchmark implementation or measurements have started.

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

Baselines and numeric targets remain TBD. Do not derive targets from a single measurement or treat an informational baseline as an acceptance threshold.

## Environment and execution

- Compare Linux and Windows for comparable workloads and document filesystem and environment differences.
- Record hardware, operating system, filesystem, Go toolchain version, configuration, workload inputs, and exact benchmark command with every result.
- Vary major workload dimensions independently where practical, including record/payload size, history size, transaction size, concurrency, Reader count, and consumer speed.
- Include correctness checks; lost, duplicated, reordered, or incorrectly recovered committed data invalidates the measurement.
- Benchmark commands, suite locations, runner class, warm-up, repetitions, duration, sampling, and statistical treatment are not yet established.
- No benchmark CI workflow is currently configured. Benchmark results are informational until explicit numeric targets and a separate approved CI policy exist; hard performance thresholds must not fail CI without an approved policy.

## Baselines, reporting and retention

No baseline revision or benchmark artifact exists yet. When measurements begin, retain the exact source SHA, environment description, workload parameters, raw data, summary distributions, and correctness outcome. Define artifact retention and comparable-run selection before relying on historical trends.

## Exclusions and approvals

No workload category is excluded. Numeric targets are intentionally absent pending approval. Changes to targets, thresholds, workload definitions, measurement policy, or evidence retention require approval and decision-ledger recording.

## Define approval checklist

- [x] Applicability and workload categories are documented.
- [x] Workload scenarios are defined in benchmarks.md.
- [ ] Baselines and environment controls are defined.
- [ ] Numeric targets are approved or explicitly absent with rationale.
- [ ] Execution, reporting, retention, and regression handling are defined.
- [ ] The completed strategy is approved and recorded in the decision ledger.
