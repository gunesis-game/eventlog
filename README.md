# eventlog

eventlog is an embedded, disk-backed, append-only event log for Go. Applications use it to persist immutable events, rebuild in-memory state after a restart, and move from replaying history to tailing live events. Several devices can share one log through shared storage, with causal ordering and conflict reporting and no central coordinator.

This README is written for AI agents and new contributors. It says where each kind of truth lives. Everything that applies to a change, whether planned, in progress, or just landed, is in [Making changes](#making-changes).

## Current state

- **Built:** the public API (`Config`, `PrepareLog`, field selectors, transactions, notifications, `ChannelError`), local storage, recovery, compaction, and multi-device operation on local storage (causal delivery, conflicts, convergence, storage that refuses writes). Also built: the Google service clients and the in-process Google services emulator.
- **Approved but not built:** `Web` storage with the Google Drive backend, and Firebase change notifications. The design is in [architecture.md](docs/project/architecture.md#multi-device-operation) and the acceptance criteria are in Layer 10 of [acceptance.md](docs/project/acceptance.md).

## Where truth lives

Read only the documents your task needs. When two documents disagree, the owner listed here wins.

| Question | Owner |
|---|---|
| What must the product do? | [requirements.md](docs/project/requirements.md) |
| How is it designed: packages, formats, algorithms, lifecycle? | [architecture.md](docs/project/architecture.md) |
| Why was something decided, and is that decision still current? | [decisions.md](docs/project/decisions.md) (check the State column) |
| What counts as correct (scenario IDs such as `L-03` or `MD-07`)? | [acceptance.md](docs/project/acceptance.md) |
| Which tests cover which component, and what to run for a change? | [testing-strategy.md](docs/project/testing-strategy.md) |
| How is performance measured (workload IDs such as `B-L01`)? | [benchmarks.md](docs/project/benchmarks.md), with policy in [performance-strategy.md](docs/project/performance-strategy.md) |
| Commands, CI workflows, last observed results | [commands.md](docs/project/commands.md) |
| Coding, testing, performance, and Git standards | [docs/standards/](docs/standards/README.md) |
| Background research behind a decision | [docs/research/](docs/research/) |

## Making changes

This section is the complete list of what applies to a change. It covers deciding on the change, making it, and finishing it. Nothing about changes is stated elsewhere in this README.

### Before you start

- **Material changes need a decision.** If a change alters requirements, architecture, the public API, a quality gate, or a policy, get user approval first. Then follow [Decision maintenance](docs/project/decisions.md#decision-maintenance) and integrate the outcome into the owning specification, not into a new document.
- **The public API is approved and frozen.** Do not add, remove, or change exported identifiers without explicit user approval. `go doc -all .` shows the current surface.
- **Check that the plan fits the fixed constraints.** Dependencies are the Go standard library only, except `github.com/santhosh-tekuri/jsonschema/v6` (D-046). Linux and Windows are equal-priority platforms, macOS is excluded, and core behavior must not rely on platform-exclusive features.
- **Find what the change touches.** Use [testing-strategy.md](docs/project/testing-strategy.md) to see which components and tests are affected, and [acceptance.md](docs/project/acceptance.md) to see whether scenarios already exist for the behavior. Planned but unbuilt features, such as web storage in Layer 10, already have scenarios.

### While you work

- **Work on `main` only.** Commit directly to `main`, one coherent change per commit. No branches, pull requests, force-pushes, or history rewrites ([Git workflow](docs/standards/git-workflow.md#project-policy)).
- **Follow the [coding standard](docs/standards/coding.md),** in particular one primary code unit per file, with the file named after it.
- **Tests live beside the code they test.** Fault injection and other test controls stay unexported. No automated test or benchmark calls real Google services; use `internal/googletest`.

### What must change with the code

Tests, acceptance scenarios, benchmarks, and specifications grow with the code. Update them in the same commit as the code change:

1. **Acceptance:** add or update scenarios in [acceptance.md](docs/project/acceptance.md) for the new or changed behavior, in the layer it belongs to, and name the tests that cover each one. When a planned feature is built, add its tests to the scenarios that already exist for it.
2. **Tests:** write the tests and update the suite locations in [testing-strategy.md](docs/project/testing-strategy.md).
3. **Benchmarks:** if the change adds or alters a performance-relevant path (storage, recovery, transactions, Readers, delivery, preparation, or web storage), add or update its workloads in [benchmarks.md](docs/project/benchmarks.md) with a `B-` ID.
   - Add a smoke benchmark for it, or update the existing one, and list it in the [smoke suite](docs/project/benchmarks.md#smoke-suite) table.
   - Every benchmark checks that its workload produced correct results, and fails rather than hangs.
   - Web-storage benchmarks run against `internal/googletest` and report Google calls and bytes per operation ([performance-strategy.md](docs/project/performance-strategy.md#environments)).
   - Benchmarks are informational. Do not add numeric performance or memory targets or CI performance gates; they need an approved decision.
4. **Specifications:** update the owning document from [Where truth lives](#where-truth-lives). Update [commands.md](docs/project/commands.md) when commands or CI change. State each fact in one place and link to it from elsewhere.

A change is incomplete if new behavior has no acceptance scenario or a performance-relevant path has no benchmark.

### Finishing a change

- **Verify:** run the full command set under [Commands](#commands) before calling the change complete, and check the CI run for the exact commit after pushing.
- **Report only observed evidence.** Tie every result to the exact commit SHA. A skipped, cancelled, or missing check is not a pass. Record newly observed results in [commands.md](docs/project/commands.md#observed-results).
- **Record what changed:**
  - When a feature moves from planned to built, update [Current state](#current-state) in this README and the build status in [architecture.md](docs/project/architecture.md#multi-device-operation).
  - When a decision replaces or refines an earlier one, update the earlier decision's State in [decisions.md](docs/project/decisions.md).

## Reporting back to the user

Goal: keep both the user's reading and the session's context small. Give the least that lets the user act, and expand only on request.

- Lead with the outcome and whether it is complete. Then list what changed by effect, what was verified (with the commit SHA), and what was not.
- Put decisions, assumptions, deviations, and open problems where they cannot be missed.
- Leave out routine steps, unchanged things, and restated instructions. Point to files and line numbers instead of pasting code or command output.
- End every report with a one-line offer to elaborate, naming the topics available, for example "Want me to elaborate on the benchmark changes or the new acceptance scenarios?" When the user picks one, cover only that topic.
- Never claim results you did not observe (see [Finishing a change](#finishing-a-change)).

## Commands

```sh
go build ./...
go vet ./...
gofmt -l .                                        # must print nothing
go test -count=1 ./...                            # full suite
go test -race -count=1 ./...                      # race detector (needs cgo)
go test -run '^$' -bench . -benchtime=3x ./...    # benchmark smoke suite
```

CI ([Go verification](.github/workflows/go.yml)) runs gofmt, tests, race tests, build, and vet on Linux and Windows for every push to `main`. Details are in [commands.md](docs/project/commands.md).

## Repository layout

| Path | Contents |
|---|---|
| `*.go` (root) | Public API and the process runtime: Channel runtimes, Readers, transaction processors, decoder registry |
| `internal/identity`, `internal/fieldschema`, `internal/record` | Identity encoding, field schemas and values, record format and embedded JSON Schemas |
| `internal/storage`, `internal/filesystem` | Segment store, recovery, compaction, other devices' logs; the HTTP/2 file-system boundary and OS backend |
| `internal/causal` | Hybrid logical clock |
| `internal/googleauth`, `internal/drive`, `internal/firebase` | Google service clients for the future web storage backend |
| `internal/googletest` | Test-only emulator of the Google services |
| `docs/` | Specifications, standards, and research |
| `scripts/`, `.github/workflows/` | CI helper scripts and workflows |
