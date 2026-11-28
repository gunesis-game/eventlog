# Project Performance Strategy

## Purpose and status

This document applies the [Performance Engineering Standard](../standards/performance-engineering.md) to eventlog. The workload catalogue, metrics, measurement protocol, resource ceilings, and artifact retention are defined once, in [benchmarks.md](benchmarks.md) (approved as D-040 and D-041); this document records applicability, environments, and the CI policy.

**Current status:** The benchmark smoke suite (one scenario per layer, listed in [benchmarks.md](benchmarks.md#smoke-suite)) is implemented and runs locally (command in [commands.md](commands.md)). No baseline has been measured and no benchmark artifact has been retained. No numeric performance or memory target is approved.

## Applicability

Performance engineering applies to the disk-backed append-only log, transaction commit and recovery, metadata reconstruction, Event append and retrieval, Channel replay, live tailing, notification delivery, and web storage. Correctness, recovery safety, predictable memory use, and Linux/Windows parity take priority over numeric performance claims ([requirements.md](requirements.md#quality-expectations-and-constraints)). No workload category is excluded.

## Environments

- Linux and Windows are measured separately and are never directly compared on absolute timings.
- Web-storage workloads run only against the in-process Google services emulator (`internal/googletest`), never against real Google services, so measurements spend no Google quota. They report Google calls and bytes per operation, from the emulator's counts, alongside time and memory, for comparison with the free-tier quotas and Firebase's download allowance. Network latency, where it matters, is modelled with the emulator's configurable per-call delay. The emulator stores its files on a real filesystem, so its disk work is part of the measurement; each run records the emulator's storage folder location and per-call delay. Runs with real accounts are manual only and are not benchmark evidence.

## CI policy

No benchmark workflow is configured. Benchmark results are informational: no performance or memory value may fail CI. Correctness tests remain independently enforceable.

## Targets and approvals

Numeric targets are deferred until representative baselines exist and require a separately approved decision. Changes to targets, thresholds, workload definitions, measurement policy, or evidence retention require approval and a decision record in [decisions.md](decisions.md).
