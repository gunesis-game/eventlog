# Performance Engineering Standard

## Purpose and authority

This standard defines a repeatable approach to performance requirements, measurement, benchmarking, analysis, and regression detection. The project-specific [performance strategy](../project/performance-strategy.md) defines applicable workloads, metrics, environments, baselines, targets, commands, and automation.

Performance engineering begins with the requirements. Benchmarks are designed around meaningful workloads and acceptance needs, not added only after optimization work.

## Applicability

Every project must assess performance relevance. A project may determine that formal benchmarks or quantitative targets are not applicable, but the decision and rationale must be recorded and approved. Do not invent targets or claim performance quality without evidence.

## Workload design

For each applicable performance concern, define representative and, where useful, worst-case workloads. Record:

- The user or system operation represented and why it matters.
- Input sizes, data distributions, concurrency, arrival patterns, and workload mix.
- Warm-up, steady-state, duration, repetitions, and sampling approach.
- Setup, teardown, and whether initialization, allocation, I/O, or network costs are included.
- Expected workload growth and relevant scalability dimensions.

Workloads must be repeatable and sufficiently representative to support meaningful comparison. Avoid benchmarks that measure incidental harness overhead unless that overhead is intentionally part of the target.

## Metrics

Select metrics that match the system and workload. Applicable examples include:

- Throughput and completed work per unit time.
- Latency distributions, including tail percentiles where meaningful.
- CPU time/utilization, memory footprint, allocation rate, and garbage-collection cost.
- I/O, network, queueing, contention, and resource saturation.
- Startup, initialization, recovery, and scalability behavior.

Define units, collection method, aggregation, and interpretation. Do not rely only on averages when tail behavior or variance matters.

## Baselines, targets, and comparison

Record a baseline before making performance claims or optimization decisions. Define targets only when supported by product requirements, user expectations, capacity constraints, or measured operational needs. State the workload, environment, metric, threshold, and measurement method together.

Comparisons must control or disclose material differences in hardware, runtime, operating system, dependencies, configuration, load, and measurement method. Separate measurement noise from meaningful change. Repeat suspicious or high-impact results before drawing conclusions.

A target or threshold must not be introduced, changed, or removed silently. Treat it as a material project decision and record its rationale and approval.

## Benchmark quality and execution

- Keep benchmark setup and measured regions explicit.
- Avoid shared mutable state and uncontrolled background activity where possible.
- Use adequate repetitions and report variability.
- Separate correctness tests from performance measurements; benchmark results do not prove functional correctness.
- Do not use performance thresholds as CI gates. CI benchmark results are informational and must not fail a workflow on a performance value. Making a threshold a CI gate requires an explicit, recorded policy change that defines its variance policy, tolerance, environment, and failure response.
- Manual performance evaluations may compare results with approved targets.
- Preserve enough metadata and output to reproduce and interpret results.

## Regression analysis

Investigate material regressions against a comparable baseline. Identify the affected workload and metric, quantify the change, assess measurement confidence, and trace likely causes. Record whether the result is a confirmed regression, inconclusive, or an expected trade-off. Do not conceal regressions by replacing baselines without explanation.

## Evidence and retention

Performance evidence must identify the exact commit SHA, benchmark suite/workload, environment, tool/runtime versions, configuration, raw or summarized measurements, and comparison baseline. Report missing, failed, cancelled, and incomplete runs distinctly.

The project strategy defines retention and artifact policy. Retain enough data to compare trends and reproduce important findings while controlling storage. Never delete evidence outside the documented retention policy.

## Project strategy and exceptions

Before implementation begins, the project performance strategy must be approved. It records applicability, workloads, metrics, baselines, targets (or explicit absence), environments, commands, CI/manual execution policy, reporting, retention, and regression handling. A project may use informational benchmarks without hard thresholds when justified. Changes to targets, gates, workloads, or retention are material decisions requiring approval and durable recording.
